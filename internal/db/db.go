package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return pool, nil
}

func GetWatermark(ctx context.Context, pool *pgxpool.Pool, source string) (time.Time, bool, error) {
	var t time.Time
	err := pool.QueryRow(ctx,
		`SELECT last_ingested_at FROM ingestion_watermarks WHERE source = $1`,
		source,
	).Scan(&t)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return t, true, nil
}

func SetWatermark(ctx context.Context, pool *pgxpool.Pool, source string, t time.Time) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO ingestion_watermarks (source, last_ingested_at, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (source) DO UPDATE
		SET last_ingested_at = EXCLUDED.last_ingested_at,
		    updated_at = now()
	`, source, t)
	return err
}
