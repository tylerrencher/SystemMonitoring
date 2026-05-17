package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	appdb "github.com/tylerrencher/systemmonitoring/internal/db"
)

const baseURL = "https://api.ambientweather.net/v1"

// Ambient Weather free API only exposes lastData via /v1/devices.
// The /v1/devices/{mac}/query historical endpoint requires a paid tier.
// We poll /v1/devices every interval and store the lastData reading.
// Backfill is not available via free API — data accumulates from first run forward.

type Poller struct {
	apiKey     string
	appKey     string
	macAddress string
	pool       *pgxpool.Pool
	interval   time.Duration
	httpClient *http.Client
}

func NewPoller(apiKey, appKey, macAddress string, pool *pgxpool.Pool, interval time.Duration) *Poller {
	return &Poller{
		apiKey:     apiKey,
		appKey:     appKey,
		macAddress: macAddress,
		pool:       pool,
		interval:   interval,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *Poller) Run(ctx context.Context) {
	if p.apiKey == "" || p.appKey == "" || p.macAddress == "" {
		log.Printf("[weather] skipping: AMBIENT_API_KEY, AMBIENT_APP_KEY, AMBIENT_MAC_ADDRESS not set")
		return
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	if err := p.poll(ctx); err != nil {
		log.Printf("[weather] poll error: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.poll(ctx); err != nil {
				log.Printf("[weather] poll error: %v", err)
			}
		}
	}
}

func (p *Poller) poll(ctx context.Context) error {
	url := fmt.Sprintf("%s/devices?apiKey=%s&applicationKey=%s",
		baseURL, p.apiKey, p.appKey)

	resp, err := p.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ambient weather API status %d", resp.StatusCode)
	}

	var devices []struct {
		MACAddress string                 `json:"macAddress"`
		LastData   map[string]interface{} `json:"lastData"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return err
	}

	for _, dev := range devices {
		if !strings.EqualFold(dev.MACAddress, p.macAddress) {
			continue
		}

		t := extractTime(dev.LastData)
		if t.IsZero() {
			return fmt.Errorf("no dateutc in lastData")
		}

		// Skip if we already stored this reading
		watermark, found, err := appdb.GetWatermark(ctx, p.pool, "ambient_weather")
		if err != nil {
			return err
		}
		if found && !t.After(watermark) {
			return nil
		}

		if err := p.writeRecord(ctx, dev.LastData); err != nil {
			return err
		}

		log.Printf("[weather] stored reading at %s", t.Format(time.RFC3339))
		return appdb.SetWatermark(ctx, p.pool, "ambient_weather", t)
	}

	return fmt.Errorf("device %s not found in API response", p.macAddress)
}

func (p *Poller) writeRecord(ctx context.Context, rec map[string]interface{}) error {
	t := extractTime(rec)
	if t.IsZero() {
		return nil
	}

	batch := &pgx.Batch{}
	skip := map[string]bool{
		"dateutc": true, "date": true, "macAddress": true,
		"PASSKEY": true, "lastRain": true, "tz": true,
	}

	for key, val := range rec {
		if skip[key] {
			continue
		}
		metric := normalizeMetric(camelToSnake(key))
		var numVal *float32
		var textVal *string
		switch v := val.(type) {
		case float64:
			f := float32(v)
			numVal = &f
		case string:
			if strings.TrimSpace(v) != "" {
				textVal = &v
			}
		}
		if numVal == nil && textVal == nil {
			continue
		}
		batch.Queue(`
			INSERT INTO weather_readings (time, metric, value_numeric, value_text)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT DO NOTHING
		`, t, metric, numVal, textVal)
	}

	br := p.pool.SendBatch(ctx, batch)
	defer br.Close()
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			log.Printf("[weather] insert error row %d: %v", i, err)
		}
	}
	return nil
}

// Backfill is not available via the Ambient Weather free API.
// Data accumulates from first run forward.
func (p *Poller) Backfill(ctx context.Context, from, to time.Time) error {
	log.Printf("[weather] backfill not available on Ambient Weather free API tier — skipping")
	return nil
}

func extractTime(rec map[string]interface{}) time.Time {
	if v, ok := rec["dateutc"]; ok {
		if ms, ok := v.(float64); ok {
			return time.UnixMilli(int64(ms)).UTC()
		}
	}
	return time.Time{}
}

// normalizeMetric maps Ambient Weather field names (after camelToSnake) to the
// canonical metric names used in queries. Ambient sends "tempf" (no camel case),
// so camelToSnake leaves it as "tempf"; we want "temp_f".
var ambientRename = map[string]string{
	"tempf":   "temp_f",
	"tempinf": "temp_in_f",
	"temp1f":  "temp1_f",
	"temp2f":  "temp2_f",
	"temp3f":  "temp3_f",
	"temp4f":  "temp4_f",
	"temp5f":  "temp5_f",
	"temp6f":  "temp6_f",
	"temp7f":  "temp7_f",
	"temp8f":  "temp8_f",
	"temp9f":  "temp9_f",
	"temp10f": "temp10_f",
	"dewptf":  "dew_point_f",
}

func normalizeMetric(m string) string {
	if canonical, ok := ambientRename[m]; ok {
		return canonical
	}
	return m
}

func camelToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteRune('_')
			}
			b.WriteRune(r + 32)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
