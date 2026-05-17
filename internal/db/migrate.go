package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Migrate(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, f := range files {
		version, err := versionFromFilename(filepath.Base(f))
		if err != nil {
			return err
		}

		var exists bool
		pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`,
			version,
		).Scan(&exists)

		if exists {
			continue
		}

		sql, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}

		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply migration %d: %w", version, err)
		}

		if _, err := pool.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`, version,
		); err != nil {
			return fmt.Errorf("record migration %d: %w", version, err)
		}

		log.Printf("applied migration %d (%s)", version, filepath.Base(f))
	}

	return nil
}

func versionFromFilename(name string) (int, error) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) < 1 {
		return 0, fmt.Errorf("unexpected migration filename: %s", name)
	}
	v, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("non-numeric version in %s: %w", name, err)
	}
	return v, nil
}
