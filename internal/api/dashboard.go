package api

import (
	"context"
	"net/http"
	"time"
)

type DashboardResponse struct {
	BatterySoCPct     float32          `json:"battery_soc_pct"`
	BatteryPowerW     float32          `json:"battery_power_w"`
	PVPowerW          float32          `json:"pv_power_w"`
	LoadPowerW        float32          `json:"load_power_w"`
	GeneratorPowerW   float32          `json:"generator_power_w"`
	PowerSource       string           `json:"power_source"`
	TotalConsumptionW float32          `json:"total_consumption_w"`
	TopConsumers      []Consumer       `json:"top_consumers"`
	Weather           *WeatherSnapshot `json:"weather,omitempty"`
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

	// IoTawatt: total consumption and top consumers over window
	since := time.Now().Add(-window)

	totalRow := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(avg_w), 0)::real
		FROM (
			SELECT AVG(watts) AS avg_w
			FROM iotawatt_readings
			WHERE time >= $1 AND watts IS NOT NULL
			GROUP BY device, series
		) sub
	`, since)
	if err := totalRow.Scan(&resp.TotalConsumptionW); err != nil {
		return nil, err
	}

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
