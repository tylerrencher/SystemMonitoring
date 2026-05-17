package config

import (
	"testing"
	"time"
)

func TestParseDevices(t *testing.T) {
	t.Run("multiple devices", func(t *testing.T) {
		devices, err := parseDevices("pwrmone1=pwrmone1.local,pwrmona1=pwrmona1.local,pwrmona2=pwrmona2.local")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(devices) != 3 {
			t.Fatalf("got %d devices, want 3", len(devices))
		}
		cases := []IoTawattDevice{
			{Name: "pwrmone1", Host: "pwrmone1.local"},
			{Name: "pwrmona1", Host: "pwrmona1.local"},
			{Name: "pwrmona2", Host: "pwrmona2.local"},
		}
		for i, want := range cases {
			if devices[i] != want {
				t.Errorf("devices[%d] = %+v, want %+v", i, devices[i], want)
			}
		}
	})

	t.Run("single device", func(t *testing.T) {
		devices, err := parseDevices("main=192.168.1.10")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(devices) != 1 || devices[0].Name != "main" || devices[0].Host != "192.168.1.10" {
			t.Errorf("got %+v", devices)
		}
	})

	t.Run("empty string returns empty slice", func(t *testing.T) {
		devices, err := parseDevices("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(devices) != 0 {
			t.Errorf("got %d devices, want 0", len(devices))
		}
	})

	t.Run("trailing comma ignored", func(t *testing.T) {
		devices, err := parseDevices("a=b,")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(devices) != 1 {
			t.Errorf("got %d devices, want 1", len(devices))
		}
	})

	t.Run("invalid pair returns error", func(t *testing.T) {
		_, err := parseDevices("badformat")
		if err == nil {
			t.Error("expected error for invalid format")
		}
	})

	t.Run("missing host returns error", func(t *testing.T) {
		_, err := parseDevices("nameonly")
		if err == nil {
			t.Error("expected error for missing host")
		}
	})
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"135", 135.0},
		{"50.5", 50.5},
		{"0", 0.0},
		{"", 0.0},
		{"abc", 0.0},
		{"-10", -10.0},
	}

	for _, tt := range tests {
		got := parseFloat(tt.input)
		if got != tt.want {
			t.Errorf("parseFloat(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"10s", 10 * time.Second, false},
		{"1m", time.Minute, false},
		{"30s", 30 * time.Second, false},
		{"24h", 24 * time.Hour, false},
		{"bad", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		got, err := parseDuration(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseDuration(%q): expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseDuration(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
