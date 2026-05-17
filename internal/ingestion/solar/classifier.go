package solar

// timeSeriesMetrics are per-inverter metrics stored as time-series on every reading.
var timeSeriesMetrics = map[string]bool{
	"ac_output_frequency":          true,
	"ac_output_voltage":            true,
	"battery_current":              true,
	"battery_voltage":              true,
	"device_mode":                  true,
	"force_generator_on":           true,
	"generator_power":              true,
	"grid_frequency":               true,
	"grid_power":                   true,
	"grid_power_ct":                true,
	"grid_power_ct_1":              true,
	"grid_power_ct_2":              true,
	"grid_power_ld":                true,
	"grid_power_ld_1":              true,
	"grid_power_ld_2":              true,
	"grid_voltage":                 true,
	"grid_voltage_1":               true,
	"grid_voltage_2":               true,
	"load_percentage":              true,
	"load_power":                   true,
	"load_power_essential":         true,
	"load_power_non-essential":     true,
	"pv_current_1":                 true,
	"pv_current_2":                 true,
	"pv_current_3":                 true,
	"pv_power":                     true,
	"pv_power_1":                   true,
	"pv_power_2":                   true,
	"pv_power_3":                   true,
	"pv_voltage_1":                 true,
	"pv_voltage_2":                 true,
	"pv_voltage_3":                 true,
	"temperature":                  true,
	"work_mode":                    true,
}

// totalMetrics are total/ topics stored in solar_totals (not per-inverter).
var totalMetrics = map[string]bool{
	"battery_power":           true,
	"battery_state_of_charge": true,
	"battery_temperature":     true,
	"grid_frequency":          true,
	"grid_power":              true,
	"grid_voltage":            true,
	"load_percentage":         true,
	"load_power":              true,
	"pv_power":                true,
}

type TopicClass int

const (
	ClassUnknown    TopicClass = iota
	ClassTimeSeries            // → solar_readings
	ClassTotal                 // → solar_totals
	ClassConfig                // → solar_config_changes
)

// Classify returns the classification for a metric on a given device.
// device is "inverter_1", "inverter_2", or "total".
func Classify(device, metric string) TopicClass {
	if device == "total" {
		if totalMetrics[metric] {
			return ClassTotal
		}
		return ClassUnknown
	}
	if timeSeriesMetrics[metric] {
		return ClassTimeSeries
	}
	return ClassConfig
}
