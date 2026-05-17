package weather

import (
	"testing"
	"time"
)

func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"humidity", "humidity"},
		{"tempf", "tempf"},
		{"tempF", "temp_f"},
		{"windSpeedMph", "wind_speed_mph"},
		{"windGustMph", "wind_gust_mph"},
		{"baromRelIn", "barom_rel_in"},
		{"baromAbsIn", "barom_abs_in"},
		{"solarRadiation", "solar_radiation"},
		{"uv", "uv"},
		{"UV", "u_v"},
		{"", ""},
	}

	for _, tt := range tests {
		got := camelToSnake(tt.input)
		if got != tt.want {
			t.Errorf("camelToSnake(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeMetric(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"tempf", "temp_f"},
		{"tempinf", "temp_in_f"},
		{"temp1f", "temp1_f"},
		{"temp10f", "temp10_f"},
		{"dewptf", "dew_point_f"},
		{"humidity", "humidity"},        // no mapping → unchanged
		{"wind_speed_mph", "wind_speed_mph"}, // already snake → unchanged
	}

	for _, tt := range tests {
		got := normalizeMetric(tt.input)
		if got != tt.want {
			t.Errorf("normalizeMetric(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractTime(t *testing.T) {
	t.Run("valid dateutc", func(t *testing.T) {
		ms := float64(1747306800000)
		rec := map[string]interface{}{"dateutc": ms}
		got := extractTime(rec)
		if got.IsZero() {
			t.Fatal("expected non-zero time")
		}
		want := time.UnixMilli(1747306800000).UTC()
		if !got.Equal(want) {
			t.Errorf("extractTime = %v, want %v", got, want)
		}
	})

	t.Run("missing dateutc", func(t *testing.T) {
		rec := map[string]interface{}{"tempf": 72.5}
		got := extractTime(rec)
		if !got.IsZero() {
			t.Errorf("expected zero time, got %v", got)
		}
	})

	t.Run("empty map", func(t *testing.T) {
		got := extractTime(map[string]interface{}{})
		if !got.IsZero() {
			t.Errorf("expected zero time, got %v", got)
		}
	})

	t.Run("dateutc wrong type", func(t *testing.T) {
		rec := map[string]interface{}{"dateutc": "not-a-number"}
		got := extractTime(rec)
		if !got.IsZero() {
			t.Errorf("expected zero time for wrong type, got %v", got)
		}
	})
}
