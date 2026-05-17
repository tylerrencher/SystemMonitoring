package solar

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		device, metric string
		want           TopicClass
	}{
		// Per-inverter time-series metrics
		{"inverter_1", "pv_power", ClassTimeSeries},
		{"inverter_2", "pv_power", ClassTimeSeries},
		{"inverter_1", "battery_voltage", ClassTimeSeries},
		{"inverter_1", "battery_current", ClassTimeSeries},
		{"inverter_1", "load_power", ClassTimeSeries},
		{"inverter_1", "load_power_essential", ClassTimeSeries},
		{"inverter_1", "generator_power", ClassTimeSeries},
		{"inverter_1", "grid_power", ClassTimeSeries},
		{"inverter_1", "grid_voltage", ClassTimeSeries},
		{"inverter_1", "ac_output_voltage", ClassTimeSeries},
		{"inverter_1", "temperature", ClassTimeSeries},
		{"inverter_1", "work_mode", ClassTimeSeries},
		// Total metrics
		{"total", "battery_state_of_charge", ClassTotal},
		{"total", "battery_power", ClassTotal},
		{"total", "battery_temperature", ClassTotal},
		{"total", "pv_power", ClassTotal},
		{"total", "load_power", ClassTotal},
		{"total", "grid_power", ClassTotal},
		{"total", "grid_voltage", ClassTotal},
		{"total", "load_percentage", ClassTotal},
		// Unknown total metric → ClassUnknown (not in totalMetrics)
		{"total", "battery_voltage", ClassUnknown},
		{"total", "temperature", ClassUnknown},
		{"total", "unknown_key", ClassUnknown},
		// Unknown inverter metric → ClassConfig
		{"inverter_1", "serial_number", ClassConfig},
		{"inverter_1", "firmware_version", ClassConfig},
		{"inverter_1", "unknown_key", ClassConfig},
	}

	for _, tt := range tests {
		got := Classify(tt.device, tt.metric)
		if got != tt.want {
			t.Errorf("Classify(%q, %q) = %v, want %v", tt.device, tt.metric, got, tt.want)
		}
	}
}
