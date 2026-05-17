package iotawatt

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	appdb "github.com/tylerrencher/systemmonitoring/internal/db"
)

type Scraper struct {
	device   string
	client   *Client
	pool     *pgxpool.Pool
	interval time.Duration
}

func NewScraper(device, host string, pool *pgxpool.Pool, interval time.Duration) *Scraper {
	return &Scraper{
		device:   device,
		client:   NewClient(host),
		pool:     pool,
		interval: interval,
	}
}

func (s *Scraper) Run(ctx context.Context) {
	series, err := s.getSeries(ctx)
	if err != nil {
		return // context cancelled
	}
	log.Printf("[iotawatt/%s] %d series loaded", s.device, len(series))

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.poll(ctx, series); err != nil {
				log.Printf("[iotawatt/%s] poll error: %v", s.device, err)
			}
		}
	}
}

// getSeries fetches the series list from the device, retrying with exponential
// backoff until it succeeds or the context is cancelled. The .local mDNS name
// may not resolve immediately at startup even when the device is reachable.
func (s *Scraper) getSeries(ctx context.Context) ([]Series, error) {
	const maxDelay = 2 * time.Minute
	delay := 5 * time.Second
	for {
		series, err := s.client.GetSeries()
		if err == nil {
			return series, nil
		}
		log.Printf("[iotawatt/%s] failed to get series: %v — retrying in %s", s.device, err, delay)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			delay = min(delay*2, maxDelay)
		}
	}
}

func (s *Scraper) poll(ctx context.Context, series []Series) error {
	watermark, found, err := appdb.GetWatermark(ctx, s.pool, "iotawatt/"+s.device)
	if err != nil {
		return err
	}

	begin := time.Now().Add(-s.interval * 2)
	if found {
		begin = watermark
	}
	end := time.Now()

	readings, err := s.client.Query(series, begin, end, 5)
	if err != nil {
		return err
	}
	if len(readings) == 0 {
		return nil
	}

	if err := s.writeBatch(ctx, readings); err != nil {
		return err
	}

	latest := readings[0].Time
	for _, r := range readings {
		if r.Time.After(latest) {
			latest = r.Time
		}
	}

	return appdb.SetWatermark(ctx, s.pool, "iotawatt/"+s.device, latest)
}

func (s *Scraper) writeBatch(ctx context.Context, readings []Reading) error {
	batch := &pgx.Batch{}
	for _, r := range readings {
		batch.Queue(`
			INSERT INTO iotawatt_readings (time, device, series, watts, volts)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
		`, r.Time, s.device, r.Series, r.Watts, r.Volts)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			log.Printf("[iotawatt/%s] insert error row %d: %v", s.device, i, err)
		}
	}
	return nil
}

// Backfill fetches data between from and to and writes it to the DB.
// Used by the backfill subcommand.
func (s *Scraper) Backfill(ctx context.Context, from, to time.Time) error {
	series, err := s.client.GetSeries()
	if err != nil {
		return err
	}

	const window = 24 * time.Hour
	cursor := from
	total := 0

	for cursor.Before(to) {
		windowEnd := cursor.Add(window)
		if windowEnd.After(to) {
			windowEnd = to
		}

		readings, err := s.client.Query(series, cursor, windowEnd, 5)
		if err != nil {
			return err
		}

		if len(readings) > 0 {
			if err := s.writeBatch(ctx, readings); err != nil {
				return err
			}
			total += len(readings)
		}

		log.Printf("[iotawatt/%s] backfill %s → %s: %d readings",
			s.device, cursor.Format(time.DateOnly), windowEnd.Format(time.DateOnly), len(readings))

		cursor = windowEnd
	}

	log.Printf("[iotawatt/%s] backfill complete: %d total readings", s.device, total)
	return nil
}
