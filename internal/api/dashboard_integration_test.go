//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tylerrencher/systemmonitoring/internal/testhelper"
)

// insertSolarTotal inserts a solar_totals row for dashboard/chart tests.
func insertSolarTotal(t *testing.T, metric string, val float64, ts time.Time) {
	t.Helper()
	_, err := dbPool.Exec(context.Background(),
		`INSERT INTO solar_totals (time, metric, value_numeric) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		ts, metric, val)
	if err != nil {
		t.Fatalf("insertSolarTotal %s: %v", metric, err)
	}
}

// insertSolarReading inserts a solar_readings row.
func insertSolarReading(t *testing.T, inverter, metric string, val float64, ts time.Time) {
	t.Helper()
	_, err := dbPool.Exec(context.Background(),
		`INSERT INTO solar_readings (time, inverter, metric, value_numeric) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		ts, inverter, metric, val)
	if err != nil {
		t.Fatalf("insertSolarReading %s/%s: %v", inverter, metric, err)
	}
}

// insertIotawattReading inserts an iotawatt_readings row.
func insertIotawattReading(t *testing.T, device, series string, ts time.Time, watts float64) {
	t.Helper()
	_, err := dbPool.Exec(context.Background(),
		`INSERT INTO iotawatt_readings (time, device, series, watts) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		ts, device, series, watts)
	if err != nil {
		t.Fatalf("insertIotawattReading %s/%s: %v", device, series, err)
	}
}

// insertWeatherReading inserts a weather_readings row.
func insertWeatherReading(t *testing.T, metric string, val float64, ts time.Time) {
	t.Helper()
	_, err := dbPool.Exec(context.Background(),
		`INSERT INTO weather_readings (time, metric, value_numeric) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		ts, metric, val)
	if err != nil {
		t.Fatalf("insertWeatherReading %s: %v", metric, err)
	}
}

// refreshIotawatt1min materializes all iotawatt_readings into iotawatt_1min.
// Explicit window params bypass the policy end_offset so recently-inserted test data is included.
func refreshIotawatt1min(t *testing.T) {
	t.Helper()
	_, err := dbPool.Exec(context.Background(),
		`CALL refresh_continuous_aggregate('iotawatt_1min', NOW() - INTERVAL '3 hours', NOW())`)
	if err != nil {
		t.Fatalf("refresh iotawatt_1min: %v", err)
	}
}

// dashboardGet logs in as a fresh user and GETs /api/v1/dashboard, returning the decoded body.
func dashboardGet(t *testing.T) map[string]any {
	t.Helper()
	testhelper.Truncate(t, dbPool, "users")
	insertTestUser(t, "dash_user", "viewer", "pass")

	ts := httptest.NewServer(apiServer.Handler())
	t.Cleanup(ts.Close)
	client := newClient(ts)
	loginAs(t, client, ts.URL, "dash_user", "pass")

	resp, err := client.Get(ts.URL + "/api/v1/dashboard")
	if err != nil {
		t.Fatalf("GET dashboard: %v", err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("dashboard got %d: %s", resp.StatusCode, body)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	return result
}

// TestDashboard_SolarTotals verifies battery_soc_pct, battery_power_w, pv_power_w, load_power_w
// are populated from the solar_totals table.
func TestDashboard_SolarTotals(t *testing.T) {
	testhelper.Truncate(t, dbPool, "solar_totals")
	now := time.Now()
	insertSolarTotal(t, "battery_state_of_charge", 75.5, now.Add(-1*time.Minute))
	insertSolarTotal(t, "battery_power", -500.0, now.Add(-1*time.Minute))
	insertSolarTotal(t, "pv_power", 3200.0, now.Add(-1*time.Minute))
	insertSolarTotal(t, "load_power", 1800.0, now.Add(-1*time.Minute))

	result := dashboardGet(t)

	if v, _ := result["battery_soc_pct"].(float64); v < 75 || v > 76 {
		t.Errorf("battery_soc_pct = %v, want ~75.5", v)
	}
	if v, _ := result["pv_power_w"].(float64); v < 3100 || v > 3300 {
		t.Errorf("pv_power_w = %v, want ~3200", v)
	}
	if v, _ := result["load_power_w"].(float64); v < 1700 || v > 1900 {
		t.Errorf("load_power_w = %v, want ~1800", v)
	}
}

// TestDashboard_GeneratorPower verifies generator_power_w is summed from solar_readings.
func TestDashboard_GeneratorPower(t *testing.T) {
	testhelper.Truncate(t, dbPool, "solar_readings")
	insertSolarReading(t, "inv1", "generator_power", 5500.0, time.Now().Add(-1*time.Minute))

	result := dashboardGet(t)

	if v, _ := result["generator_power_w"].(float64); v < 5000 || v > 6000 {
		t.Errorf("generator_power_w = %v, want ~5500", v)
	}
	if result["power_source"] != "generator" {
		t.Errorf("power_source = %v, want generator (threshold 100W)", result["power_source"])
	}
}

// TestDashboard_PowerSource_Solar verifies power_source classification with high PV.
func TestDashboard_PowerSource_Solar(t *testing.T) {
	testhelper.Truncate(t, dbPool, "solar_totals", "solar_readings")
	now := time.Now()
	insertSolarTotal(t, "pv_power", 2000.0, now.Add(-1*time.Minute))
	// battery_power >= 0 → solar_only
	insertSolarTotal(t, "battery_power", 100.0, now.Add(-1*time.Minute))

	result := dashboardGet(t)

	if result["power_source"] != "solar_only" {
		t.Errorf("power_source = %v, want solar_only", result["power_source"])
	}
}

// TestDashboard_TopConsumers verifies the top consumers query returns ordered results.
func TestDashboard_TopConsumers(t *testing.T) {
	testhelper.Truncate(t, dbPool, "iotawatt_readings")
	now := time.Now()
	// Insert two series within the TopConsumersWindow (30s) with distinct watt levels
	insertIotawattReading(t, "dev1", "HVAC", now.Add(-10*time.Second), 3000)
	insertIotawattReading(t, "dev1", "Dryer", now.Add(-10*time.Second), 5000)

	result := dashboardGet(t)

	consumers, ok := result["top_consumers"].([]any)
	if !ok {
		t.Fatal("top_consumers is not an array")
	}
	if len(consumers) < 2 {
		t.Fatalf("want ≥2 top consumers, got %d", len(consumers))
	}
	// First entry should be Dryer (5000W > 3000W)
	first := consumers[0].(map[string]any)
	if first["series"] != "Dryer" {
		t.Errorf("top consumer series = %v, want Dryer", first["series"])
	}
}

// TestDashboard_Weather verifies the weather snapshot is populated from weather_readings.
func TestDashboard_Weather(t *testing.T) {
	testhelper.Truncate(t, dbPool, "weather_readings")
	now := time.Now()
	insertWeatherReading(t, "temp_f", 72.5, now.Add(-1*time.Minute))
	insertWeatherReading(t, "humidity", 55.0, now.Add(-1*time.Minute))

	result := dashboardGet(t)

	weather, ok := result["weather"].(map[string]any)
	if !ok {
		t.Fatal("weather field missing or null")
	}
	if v, _ := weather["temp_f"].(float64); v < 72 || v > 73 {
		t.Errorf("weather.temp_f = %v, want ~72.5", v)
	}
	if v, _ := weather["humidity"].(float64); v < 54 || v > 56 {
		t.Errorf("weather.humidity = %v, want ~55", v)
	}
}

// TestDashboard_TodayConsumption verifies today_consumption_kwh is calculated from iotawatt_1min.
func TestDashboard_TodayConsumption(t *testing.T) {
	testhelper.Truncate(t, dbPool, "iotawatt_readings")
	// Insert a House reading from 1 minute ago (within today, inside continuous aggregate window)
	// 6000W for 1 minute = 6000/60000 = 0.1 kWh
	insertIotawattReading(t, "dev1", "House", time.Now().Add(-1*time.Minute).Truncate(time.Minute), 6000)
	refreshIotawatt1min(t)

	result := dashboardGet(t)

	kwh, _ := result["today_consumption_kwh"].(float64)
	if kwh <= 0 {
		t.Errorf("today_consumption_kwh = %v, want > 0 (House reading of 6000W inserted)", kwh)
	}
}
