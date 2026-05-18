package api

import (
	"context"
	"net/http"
	"time"

	"github.com/tylerrencher/systemmonitoring/internal/alerts"
)

type DashboardResponse struct {
	BatterySoCPct       float32              `json:"battery_soc_pct"`
	BatteryPowerW       float32              `json:"battery_power_w"`
	PVPowerW            float32              `json:"pv_power_w"`
	LoadPowerW          float32              `json:"load_power_w"`
	GeneratorPowerW     float32              `json:"generator_power_w"`
	PowerSource         string               `json:"power_source"`
	TodayConsumptionKWh float32              `json:"today_consumption_kwh"`
	TopConsumers        []Consumer           `json:"top_consumers"`
	Weather             *WeatherSnapshot     `json:"weather,omitempty"`
	ActiveAlerts        []alerts.ActiveAlert `json:"active_alerts"`
}

type Consumer struct {
	Device   string  `json:"device"`
	Series   string  `json:"series"`
	AvgWatts float32 `json:"avg_watts"`
}

type WeatherSnapshot struct {
	TempF    *float32 `json:"temp_f,omitempty"`
	Humidity *float32 `json:"humidity,omitempty"`
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	window := s.cfg.TopConsumersWindow
	if v := r.URL.Query().Get("window"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			window = d
		}
	}

	data, err := s.queryDashboard(r.Context(), window)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (s *Server) queryDashboard(ctx context.Context, window time.Duration) (*DashboardResponse, error) {
	resp := &DashboardResponse{
		TopConsumers: []Consumer{},
	}

	// Latest value per solar total metric
	rows, err := s.pool.Query(ctx, `
		SELECT metric, value_numeric
		FROM (
			SELECT metric, value_numeric,
			       ROW_NUMBER() OVER (PARTITION BY metric ORDER BY time DESC) AS rn
			FROM solar_totals
			WHERE metric IN ('battery_state_of_charge','battery_power','pv_power','load_power')
			  AND time >= NOW() - INTERVAL '5 minutes'
		) sub
		WHERE rn = 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var metric string
		var val *float32
		if err := rows.Scan(&metric, &val); err != nil || val == nil {
			continue
		}
		switch metric {
		case "battery_state_of_charge":
			resp.BatterySoCPct = *val
		case "battery_power":
			resp.BatteryPowerW = *val
		case "pv_power":
			resp.PVPowerW = *val
		case "load_power":
			resp.LoadPowerW = *val
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Generator power: sum latest reading per inverter
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(value_numeric), 0)
		FROM (
			SELECT DISTINCT ON (inverter) value_numeric
			FROM solar_readings
			WHERE metric = 'generator_power'
			  AND time >= NOW() - INTERVAL '5 minutes'
			ORDER BY inverter, time DESC
		) sub
	`).Scan(&resp.GeneratorPowerW)
	if err != nil {
		return nil, err
	}

	resp.PowerSource = classifyPowerSource(
		resp.PVPowerW, resp.BatteryPowerW, resp.GeneratorPowerW,
		s.cfg.SolarActiveThresholdW, s.cfg.GeneratorActiveThresholdW,
	)

	// IoTawatt: kWh consumed today (House series, since local midnight)
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	totalRow := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(avg_watts), 0)::real / 60000.0
		FROM iotawatt_1min
		WHERE series = 'House'
		  AND bucket >= $1
		  AND bucket < $2
	`, midnight, now)
	if err := totalRow.Scan(&resp.TodayConsumptionKWh); err != nil {
		return nil, err
	}

	// Top consumers over the configured window
	since := now.Add(-window)

	consRows, err := s.pool.Query(ctx, `
		SELECT device, series, AVG(watts)::real AS avg_watts
		FROM iotawatt_readings
		WHERE time >= $1 AND watts IS NOT NULL
		GROUP BY device, series
		ORDER BY avg_watts DESC
		LIMIT 10
	`, since)
	if err != nil {
		return nil, err
	}
	defer consRows.Close()
	for consRows.Next() {
		var c Consumer
		if err := consRows.Scan(&c.Device, &c.Series, &c.AvgWatts); err != nil {
			continue
		}
		resp.TopConsumers = append(resp.TopConsumers, c)
	}
	if err := consRows.Err(); err != nil {
		return nil, err
	}

	// Weather snapshot
	wRows, err := s.pool.Query(ctx, `
		SELECT metric, value_numeric
		FROM (
			SELECT metric, value_numeric,
			       ROW_NUMBER() OVER (PARTITION BY metric ORDER BY time DESC) AS rn
			FROM weather_readings
			WHERE metric IN ('temp_f', 'humidity')
			  AND time >= NOW() - INTERVAL '10 minutes'
		) sub
		WHERE rn = 1
	`)
	if err == nil {
		defer wRows.Close()
		snap := &WeatherSnapshot{}
		for wRows.Next() {
			var metric string
			var val *float32
			if err := wRows.Scan(&metric, &val); err != nil || val == nil {
				continue
			}
			switch metric {
			case "temp_f":
				snap.TempF = val
			case "humidity":
				snap.Humidity = val
			}
		}
		if snap.TempF != nil || snap.Humidity != nil {
			resp.Weather = snap
		}
	}

	if s.alertState != nil {
		resp.ActiveAlerts = s.alertState.Active()
	} else {
		resp.ActiveAlerts = []alerts.ActiveAlert{}
	}

	return resp, nil
}

func classifyPowerSource(pv, battery, generator float32, solarThresh, genThresh float64) string {
	if float64(generator) > genThresh {
		return "generator"
	}
	if float64(pv) > solarThresh {
		if battery >= 0 {
			return "solar_only"
		}
		return "battery_solar"
	}
	return "battery_only"
}
