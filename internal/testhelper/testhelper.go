//go:build integration

package testhelper

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	appdb "github.com/tylerrencher/systemmonitoring/internal/db"
)

// SetupSuite starts a TimescaleDB container, runs migrations, and returns a
// live connection pool plus a cleanup function. Call from TestMain.
func SetupSuite() (*pgxpool.Pool, func()) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image: "timescale/timescaledb:latest-pg16",
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(90 * time.Second),
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("start container: %v", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		log.Fatalf("container host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		log.Fatalf("container port: %v", err)
	}

	dsn := fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())

	pool, err := appdb.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}

	_, filename, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(filename), "../../migrations")
	if err := appdb.Migrate(ctx, pool, migrationsDir); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	cleanup := func() {
		pool.Close()
		ctr.Terminate(ctx) //nolint:errcheck
	}

	return pool, cleanup
}

// Truncate removes all rows from the given tables. Table names must be
// hardcoded constants — never pass user-controlled strings here.
func Truncate(tb testing.TB, pool *pgxpool.Pool, tables ...string) {
	tb.Helper()
	ctx := context.Background()
	for _, table := range tables {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE"); err != nil {
			tb.Fatalf("truncate %s: %v", table, err)
		}
	}
}
