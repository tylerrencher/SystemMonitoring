//go:build integration

package iotawatt_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tylerrencher/systemmonitoring/internal/ingestion/iotawatt"
	"github.com/tylerrencher/systemmonitoring/internal/testhelper"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	p, cleanup := testhelper.SetupSuite()
	pool = p
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// mockIoTawatt starts an httptest server that serves realistic IoTawatt
// responses: two Watts series and one Volts series.
func mockIoTawatt(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("show") == "series" {
			json.NewEncoder(w).Encode(map[string]any{
				"series": []map[string]string{
					{"name": "Main_W", "unit": "Watts"},
					{"name": "Dryer", "unit": "Watts"},
					{"name": "Volts_120", "unit": "Volts"},
				},
			})
			return
		}
		// Data endpoint: return two rows regardless of time range
		json.NewEncoder(w).Encode([][]any{
			{1500.0, 800.0, 120.1, "2026-05-15T10:00:00"},
			{1600.0, 850.0, 120.2, "2026-05-15T10:00:05"},
		})
	}))
}

func rowCount(t *testing.T, device string) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM iotawatt_readings WHERE device = $1`, device).Scan(&count)
	if err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

func TestBackfillInsertsReadings(t *testing.T) {
	testhelper.Truncate(t, pool, "iotawatt_readings", "ingestion_watermarks")

	srv := mockIoTawatt(t)
	defer srv.Close()

	scraper := iotawatt.NewScraper("test-device", srv.Listener.Addr().String(), pool, 0)

	from := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 15, 11, 0, 0, 0, time.UTC)
	if err := scraper.Backfill(context.Background(), from, to); err != nil {
		t.Fatalf("Backfill: %v", err)
	}

	// 3 series × 2 rows = 6 readings
	count := rowCount(t, "test-device")
	if count != 6 {
		t.Errorf("got %d rows, want 6 (3 series × 2 rows)", count)
	}
}

func TestBackfillIdempotent(t *testing.T) {
	testhelper.Truncate(t, pool, "iotawatt_readings", "ingestion_watermarks")

	srv := mockIoTawatt(t)
	defer srv.Close()

	scraper := iotawatt.NewScraper("test-device-2", srv.Listener.Addr().String(), pool, 0)

	from := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 15, 11, 0, 0, 0, time.UTC)

	if err := scraper.Backfill(context.Background(), from, to); err != nil {
		t.Fatalf("first Backfill: %v", err)
	}
	firstCount := rowCount(t, "test-device-2")

	if err := scraper.Backfill(context.Background(), from, to); err != nil {
		t.Fatalf("second Backfill: %v", err)
	}
	secondCount := rowCount(t, "test-device-2")

	if secondCount != firstCount {
		t.Errorf("row count changed after second Backfill: %d → %d (ON CONFLICT DO NOTHING not working)",
			firstCount, secondCount)
	}
}

func TestBackfillWattsAndVoltsRoutedCorrectly(t *testing.T) {
	testhelper.Truncate(t, pool, "iotawatt_readings", "ingestion_watermarks")

	srv := mockIoTawatt(t)
	defer srv.Close()

	scraper := iotawatt.NewScraper("test-device-3", srv.Listener.Addr().String(), pool, 0)

	from := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 15, 11, 0, 0, 0, time.UTC)
	if err := scraper.Backfill(context.Background(), from, to); err != nil {
		t.Fatalf("Backfill: %v", err)
	}

	// Watts series should have watts populated, volts null
	var wattsNull int
	pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM iotawatt_readings
		WHERE device = 'test-device-3' AND series = 'Main_W' AND watts IS NULL
	`).Scan(&wattsNull)
	if wattsNull > 0 {
		t.Errorf("%d Main_W rows have NULL watts", wattsNull)
	}

	// Volts series should have volts populated, watts null
	var voltsNull int
	pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM iotawatt_readings
		WHERE device = 'test-device-3' AND series = 'Volts_120' AND volts IS NULL
	`).Scan(&voltsNull)
	if voltsNull > 0 {
		t.Errorf("%d Volts_120 rows have NULL volts", voltsNull)
	}

	var wattsPopulated int
	pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM iotawatt_readings
		WHERE device = 'test-device-3' AND series = 'Volts_120' AND watts IS NOT NULL
	`).Scan(&wattsPopulated)
	if wattsPopulated > 0 {
		t.Errorf("%d Volts_120 rows incorrectly have watts set", wattsPopulated)
	}
}
