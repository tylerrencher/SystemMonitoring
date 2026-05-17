package api

import (
	"testing"
	"time"
)

func TestAutoResolution(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		from time.Time
		want string
	}{
		{"30 minutes → raw", now.Add(-30 * time.Minute), "raw"},
		{"1 hour → raw", now.Add(-time.Hour), "raw"},
		// Boundaries: span < 2h → raw, span < 7d → 1min, span < 90d → 1hour
		{"2 hours exactly → 1min", now.Add(-2 * time.Hour), "1min"},
		{"2 hours minus 1s → raw", now.Add(-2*time.Hour + time.Second), "raw"},
		{"24 hours → 1min", now.Add(-24 * time.Hour), "1min"},
		{"6 days → 1min", now.Add(-6 * 24 * time.Hour), "1min"},
		{"7 days exactly → 1hour", now.Add(-7 * 24 * time.Hour), "1hour"},
		{"7 days minus 1s → 1min", now.Add(-7*24*time.Hour + time.Second), "1min"},
		{"30 days → 1hour", now.Add(-30 * 24 * time.Hour), "1hour"},
		{"89 days → 1hour", now.Add(-89 * 24 * time.Hour), "1hour"},
		{"90 days exactly → 1day", now.Add(-90 * 24 * time.Hour), "1day"},
		{"90 days minus 1s → 1hour", now.Add(-90*24*time.Hour + time.Second), "1hour"},
		{"91 days → 1day", now.Add(-91 * 24 * time.Hour), "1day"},
		{"1 year → 1day", now.Add(-365 * 24 * time.Hour), "1day"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := autoResolution(tt.from, now)
			if got != tt.want {
				t.Errorf("autoResolution = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIotawattTable(t *testing.T) {
	tests := []struct {
		resolution  string
		wantTable   string
		wantErr     bool
	}{
		{"1min", "iotawatt_1min", false},
		{"1hour", "iotawatt_1hour", false},
		{"1day", "iotawatt_1day", false},
		{"raw", "", true},
		{"5min", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.resolution, func(t *testing.T) {
			got, err := iotawattTable(tt.resolution)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for resolution %q, got table %q", tt.resolution, got)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.wantTable {
				t.Errorf("got %q, want %q", got, tt.wantTable)
			}
		})
	}
}

func TestSolarTable(t *testing.T) {
	tests := []struct {
		resolution string
		wantTable  string
		wantErr    bool
	}{
		{"1min", "solar_readings_1min", false},
		{"1hour", "solar_readings_1hour", false},
		{"1day", "solar_readings_1day", false},
		{"raw", "", true},
		{"5min", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.resolution, func(t *testing.T) {
			got, err := solarTable(tt.resolution)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for resolution %q, got table %q", tt.resolution, got)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.wantTable {
				t.Errorf("got %q, want %q", got, tt.wantTable)
			}
		})
	}
}

func TestSolarTotalsTable(t *testing.T) {
	tests := []struct {
		resolution string
		wantTable  string
		wantErr    bool
	}{
		{"1min", "solar_totals_1min", false},
		{"1hour", "solar_totals_1hour", false},
		{"1day", "solar_totals_1day", false},
		{"raw", "", true},
		{"5min", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.resolution, func(t *testing.T) {
			got, err := solarTotalsTable(tt.resolution)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for resolution %q, got table %q", tt.resolution, got)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.wantTable {
				t.Errorf("got %q, want %q", got, tt.wantTable)
			}
		})
	}
}

func TestParseChartTimeRange(t *testing.T) {
	t.Run("defaults: empty strings give 24h window ending now", func(t *testing.T) {
		before := time.Now().UTC().Add(-time.Second)
		from, to, err := parseChartTimeRange("", "")
		after := time.Now().UTC().Add(time.Second)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if to.Before(before) || to.After(after) {
			t.Errorf("to = %v, expected near now", to)
		}
		diff := to.Sub(from)
		if diff < 23*time.Hour+59*time.Minute || diff > 24*time.Hour+time.Minute {
			t.Errorf("from-to span = %v, want ~24h", diff)
		}
	})

	t.Run("explicit RFC3339 timestamps", func(t *testing.T) {
		from, to, err := parseChartTimeRange(
			"2026-05-15T00:00:00Z",
			"2026-05-15T12:00:00Z",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !from.Equal(time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("from = %v", from)
		}
		if !to.Equal(time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)) {
			t.Errorf("to = %v", to)
		}
	})

	t.Run("invalid from returns error", func(t *testing.T) {
		_, _, err := parseChartTimeRange("not-a-time", "")
		if err == nil {
			t.Error("expected error for invalid from")
		}
	})

	t.Run("invalid to returns error", func(t *testing.T) {
		_, _, err := parseChartTimeRange("", "not-a-time")
		if err == nil {
			t.Error("expected error for invalid to")
		}
	})

	t.Run("only to specified", func(t *testing.T) {
		_, to, err := parseChartTimeRange("", "2026-05-15T06:00:00Z")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !to.Equal(time.Date(2026, 5, 15, 6, 0, 0, 0, time.UTC)) {
			t.Errorf("to = %v", to)
		}
	})
}
