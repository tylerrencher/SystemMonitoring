//go:build integration

package api_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tylerrencher/systemmonitoring/internal/testhelper"
)

// chartGet logs in and GETs the given chart URL, returning the decoded JSON array.
func chartGet(t *testing.T, url string) []any {
	t.Helper()
	testhelper.Truncate(t, dbPool, "users")
	insertTestUser(t, "chart_user", "viewer", "pass")

	ts := httptest.NewServer(apiServer.Handler())
	t.Cleanup(ts.Close)
	client := newClient(ts)
	loginAs(t, client, ts.URL, "chart_user", "pass")

	resp, err := client.Get(ts.URL + url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("chart got %d: %s", resp.StatusCode, body)
	}
	var result []any
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	return result
}

// chartGetStrings logs in and GETs a URL that returns a JSON string array.
func chartGetStrings(t *testing.T, url string) []string {
	t.Helper()
	testhelper.Truncate(t, dbPool, "users")
	insertTestUser(t, "list_user", "viewer", "pass")

	ts := httptest.NewServer(apiServer.Handler())
	t.Cleanup(ts.Close)
	client := newClient(ts)
	loginAs(t, client, ts.URL, "list_user", "pass")

	resp, err := client.Get(ts.URL + url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("got %d: %s", resp.StatusCode, body)
	}
	var result []string
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	return result
}

// TestSeriesList verifies DISTINCT series are returned from iotawatt_readings.
func TestSeriesList(t *testing.T) {
	testhelper.Truncate(t, dbPool, "iotawatt_readings")
	now := time.Now()
	insertIotawattReading(t, "dev1", "Main", now.Add(-5*time.Minute), 1000)
	insertIotawattReading(t, "dev1", "Dryer", now.Add(-5*time.Minute), 2000)
	insertIotawattReading(t, "dev1", "Main", now.Add(-4*time.Minute), 1100) // duplicate series, not distinct

	series := chartGetStrings(t, "/api/v1/series")

	found := map[string]bool{}
	for _, s := range series {
		found[s] = true
	}
	if !found["Main"] {
		t.Error("Main not in series list")
	}
	if !found["Dryer"] {
		t.Error("Dryer not in series list")
	}
	// DISTINCT: should only appear once
	count := 0
	for _, s := range series {
		if s == "Main" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Main appears %d times, want 1 (DISTINCT)", count)
	}
}

// TestInverterList verifies DISTINCT inverters are returned from solar_readings.
func TestInverterList(t *testing.T) {
	testhelper.Truncate(t, dbPool, "solar_readings")
	now := time.Now()
	insertSolarReading(t, "inv_a", "battery_voltage", 48.0, now.Add(-1*time.Minute))
	insertSolarReading(t, "inv_b", "battery_voltage", 48.0, now.Add(-1*time.Minute))

	inverters := chartGetStrings(t, "/api/v1/solar/inverters")

	found := map[string]bool{}
	for _, v := range inverters {
		found[v] = true
	}
	if !found["inv_a"] {
		t.Error("inv_a not in inverter list")
	}
	if !found["inv_b"] {
		t.Error("inv_b not in inverter list")
	}
}

// TestChartPower_Raw_WithData verifies raw iotawatt_readings are returned for the time range.
func TestChartPower_Raw_WithData(t *testing.T) {
	testhelper.Truncate(t, dbPool, "iotawatt_readings")
	ts1 := time.Now().Add(-30 * time.Minute).UTC()
	ts2 := time.Now().Add(-25 * time.Minute).UTC()
	insertIotawattReading(t, "dev1", "Main", ts1, 1500)
	insertIotawattReading(t, "dev1", "Main", ts2, 1600)

	from := ts1.Add(-5 * time.Minute).Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	url := fmt.Sprintf("/api/v1/charts/power?series=Main&from=%s&to=%s", from, to)

	points := chartGet(t, url)
	if len(points) < 2 {
		t.Errorf("want ≥2 data points, got %d", len(points))
	}
	// Verify point structure
	first := points[0].(map[string]any)
	if _, ok := first["time"]; !ok {
		t.Error("point missing time field")
	}
	if _, ok := first["avg_watts"]; !ok {
		t.Error("point missing avg_watts field")
	}
}

// TestChartPower_1min_WithData verifies aggregated iotawatt_1min data is returned.
func TestChartPower_1min_WithData(t *testing.T) {
	testhelper.Truncate(t, dbPool, "iotawatt_readings")
	// Insert readings spread across a 3-hour window (triggers 1min resolution in auto)
	base := time.Now().Add(-2 * time.Hour).Truncate(time.Minute)
	insertIotawattReading(t, "dev1", "House", base, 2000)
	insertIotawattReading(t, "dev1", "House", base.Add(-1*time.Minute), 2100)
	refreshIotawatt1min(t)

	from := base.Add(-5 * time.Minute).Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	url := fmt.Sprintf("/api/v1/charts/power?series=House&resolution=1min&from=%s&to=%s", from, to)

	points := chartGet(t, url)
	if len(points) < 1 {
		t.Errorf("want ≥1 aggregated point, got %d (real-time aggregate should include data)", len(points))
	}
	// 1min resolution returns avg_watts, max_watts, min_watts
	if len(points) > 0 {
		p := points[0].(map[string]any)
		if _, ok := p["avg_watts"]; !ok {
			t.Error("1min point missing avg_watts")
		}
	}
}

// TestChartPower_Raw_WithDevice verifies device-filtered raw query works.
func TestChartPower_Raw_WithDevice(t *testing.T) {
	testhelper.Truncate(t, dbPool, "iotawatt_readings")
	now := time.Now()
	insertIotawattReading(t, "dev1", "Main", now.Add(-10*time.Minute), 1000)
	insertIotawattReading(t, "dev2", "Main", now.Add(-10*time.Minute), 9999) // different device

	from := now.Add(-20 * time.Minute).Format(time.RFC3339)
	to := now.Format(time.RFC3339)
	url := fmt.Sprintf("/api/v1/charts/power?device=dev1&series=Main&from=%s&to=%s", from, to)

	points := chartGet(t, url)
	if len(points) != 1 {
		t.Errorf("want 1 point for device=dev1, got %d", len(points))
	}
}

// TestChartSolar_RawTotals verifies solar_totals is queried for chart without inverter.
func TestChartSolar_RawTotals(t *testing.T) {
	testhelper.Truncate(t, dbPool, "solar_totals")
	ts1 := time.Now().Add(-30 * time.Minute).UTC()
	insertSolarTotal(t, "battery_state_of_charge", 60.0, ts1)

	from := ts1.Add(-5 * time.Minute).Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	url := fmt.Sprintf("/api/v1/charts/solar?metric=battery_state_of_charge&from=%s&to=%s", from, to)

	points := chartGet(t, url)
	if len(points) < 1 {
		t.Errorf("want ≥1 solar total point, got %d", len(points))
	}
	p := points[0].(map[string]any)
	if _, ok := p["time"]; !ok {
		t.Error("solar point missing time field")
	}
	if _, ok := p["avg_value"]; !ok {
		t.Error("solar point missing avg_value field")
	}
}

// TestChartSolar_RawReadings_WithInverter verifies solar_readings is queried when inverter is specified.
func TestChartSolar_RawReadings_WithInverter(t *testing.T) {
	testhelper.Truncate(t, dbPool, "solar_readings")
	ts1 := time.Now().Add(-30 * time.Minute).UTC()
	insertSolarReading(t, "inv1", "battery_voltage", 48.5, ts1)

	from := ts1.Add(-5 * time.Minute).Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	url := fmt.Sprintf("/api/v1/charts/solar?metric=battery_voltage&inverter=inv1&from=%s&to=%s", from, to)

	points := chartGet(t, url)
	if len(points) < 1 {
		t.Errorf("want ≥1 solar reading point for inv1, got %d", len(points))
	}
}

// TestChartWeather verifies weather_readings are returned for the time range.
func TestChartWeather(t *testing.T) {
	testhelper.Truncate(t, dbPool, "weather_readings")
	ts1 := time.Now().Add(-30 * time.Minute).UTC()
	insertWeatherReading(t, "temp_f", 71.5, ts1)

	from := ts1.Add(-5 * time.Minute).Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	url := fmt.Sprintf("/api/v1/charts/weather?metric=temp_f&from=%s&to=%s", from, to)

	points := chartGet(t, url)
	if len(points) < 1 {
		t.Errorf("want ≥1 weather point, got %d", len(points))
	}
	p := points[0].(map[string]any)
	if _, ok := p["time"]; !ok {
		t.Error("weather point missing time field")
	}
	if _, ok := p["value"]; !ok {
		t.Error("weather point missing value field")
	}
}
