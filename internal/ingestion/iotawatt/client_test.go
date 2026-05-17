package iotawatt

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetSeries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("show") != "series" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"series": []map[string]string{
				{"name": "Main_W", "unit": "Watts"},
				{"name": "Dryer", "unit": "Watts"},
				{"name": "Volts_120", "unit": "Volts"},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.Listener.Addr().String())
	series, err := c.GetSeries()
	if err != nil {
		t.Fatalf("GetSeries error: %v", err)
	}
	if len(series) != 3 {
		t.Fatalf("got %d series, want 3", len(series))
	}

	cases := []struct{ name, unit string }{
		{"Main_W", "Watts"},
		{"Dryer", "Watts"},
		{"Volts_120", "Volts"},
	}
	for i, want := range cases {
		if series[i].Name != want.name || series[i].Unit != want.unit {
			t.Errorf("series[%d] = {%q, %q}, want {%q, %q}",
				i, series[i].Name, series[i].Unit, want.name, want.unit)
		}
	}
}

func TestQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return two rows: [watts0, volts0, time], [watts1, volts1, time]
		json.NewEncoder(w).Encode([][]any{
			{250.5, 120.1, "2026-05-15T12:00:00"},
			{300.0, 119.5, "2026-05-15T12:00:05"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.Listener.Addr().String())
	series := []Series{
		{Name: "Main_W", Unit: "Watts"},
		{Name: "Volts_120", Unit: "Volts"},
	}

	begin := time.Now().Add(-time.Hour)
	end := time.Now()
	readings, err := c.Query(series, begin, end, 5)
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}

	// 2 series × 2 rows = 4 readings
	if len(readings) != 4 {
		t.Fatalf("got %d readings, want 4", len(readings))
	}

	// First row, first series: Watts
	r0 := readings[0]
	if r0.Series != "Main_W" {
		t.Errorf("readings[0].Series = %q, want %q", r0.Series, "Main_W")
	}
	if r0.Watts == nil || *r0.Watts != 250.5 {
		t.Errorf("readings[0].Watts = %v, want 250.5", r0.Watts)
	}
	if r0.Volts != nil {
		t.Errorf("readings[0].Volts should be nil for Watts series, got %v", r0.Volts)
	}

	// First row, second series: Volts
	r1 := readings[1]
	if r1.Series != "Volts_120" {
		t.Errorf("readings[1].Series = %q, want %q", r1.Series, "Volts_120")
	}
	if r1.Volts == nil || *r1.Volts != 120.1 {
		t.Errorf("readings[1].Volts = %v, want 120.1", r1.Volts)
	}
	if r1.Watts != nil {
		t.Errorf("readings[1].Watts should be nil for Volts series, got %v", r1.Watts)
	}

	// All timestamps should be the same date
	for i, r := range readings {
		if r.Time.IsZero() {
			t.Errorf("readings[%d].Time is zero", i)
		}
		if got := r.Time.Format("2006-01-02"); got != "2026-05-15" {
			t.Errorf("readings[%d].Time date = %q, want 2026-05-15", i, got)
		}
	}
}

func TestQueryEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([][]any{})
	}))
	defer srv.Close()

	c := NewClient(srv.Listener.Addr().String())
	series := []Series{{Name: "Main_W", Unit: "Watts"}}
	readings, err := c.Query(series, time.Now().Add(-time.Hour), time.Now(), 5)
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if len(readings) != 0 {
		t.Errorf("got %d readings, want 0", len(readings))
	}
}

func TestQuerySkipsMalformedRows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 2 series means each row needs 3 elements (val0, val1, time).
		// Rows with only 2 elements are skipped; the 3-element row is valid.
		json.NewEncoder(w).Encode([][]any{
			{250.5, "2026-05-15T12:00:00"},          // missing one value → skip
			{250.5},                                  // way too short → skip
			{300.0, 119.5, "2026-05-15T12:00:10"},   // valid
		})
	}))
	defer srv.Close()

	c := NewClient(srv.Listener.Addr().String())
	series := []Series{
		{Name: "Main_W", Unit: "Watts"},
		{Name: "Volts_120", Unit: "Volts"},
	}
	readings, err := c.Query(series, time.Now().Add(-time.Hour), time.Now(), 5)
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	// Only the last row is valid: 2 series × 1 row = 2 readings
	if len(readings) != 2 {
		t.Errorf("got %d readings, want 2 (only valid row processed)", len(readings))
	}
}
