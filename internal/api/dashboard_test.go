package api

import "testing"

func TestClassifyPowerSource(t *testing.T) {
	const solarThresh = 50.0
	const genThresh = 100.0

	tests := []struct {
		name                    string
		pv, battery, generator  float32
		want                    string
	}{
		{
			name:      "battery only: no PV, no generator, discharging",
			pv:        0, battery: -500, generator: 0,
			want: "battery_only",
		},
		{
			name:      "battery only: PV below threshold",
			pv:        30, battery: -200, generator: 0,
			want: "battery_only",
		},
		{
			name:      "battery only: PV exactly at threshold (not above)",
			pv:        50, battery: -200, generator: 0,
			want: "battery_only",
		},
		{
			name:      "solar only: PV above threshold, battery charging",
			pv:        1200, battery: 500, generator: 0,
			want: "solar_only",
		},
		{
			name:      "solar only: PV above threshold, battery idle (zero)",
			pv:        200, battery: 0, generator: 0,
			want: "solar_only",
		},
		{
			name:      "battery+solar: PV above threshold, battery discharging",
			pv:        800, battery: -300, generator: 0,
			want: "battery_solar",
		},
		{
			name:      "generator: generator above threshold, takes priority",
			pv:        0, battery: -500, generator: 5000,
			want: "generator",
		},
		{
			name:      "generator: generator above threshold even with PV active",
			pv:        1000, battery: 100, generator: 200,
			want: "generator",
		},
		{
			name:      "generator below threshold: not generator state",
			pv:        0, battery: -500, generator: 50,
			want: "battery_only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyPowerSource(tt.pv, tt.battery, tt.generator, solarThresh, genThresh)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
