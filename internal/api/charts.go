package api

import (
	"fmt"
	"net/http"
	"time"
)

type ChartPoint struct {
	Time     time.Time `json:"time"`
	AvgWatts *float32  `json:"avg_watts"`
	MaxWatts *float32  `json:"max_watts,omitempty"`
	MinWatts *float32  `json:"min_watts,omitempty"`
}

type SolarChartPoint struct {
	Time     time.Time `json:"time"`
	AvgValue *float32  `json:"avg_value"`
	MaxValue *float32  `json:"max_value,omitempty"`
	MinValue *float32  `json:"min_value,omitempty"`
}

type WeatherChartPoint struct {
	Time  time.Time `json:"time"`
	Value *float32  `json:"value"`
}

func (s *Server) seriesList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT DISTINCT series FROM iotawatt_readings ORDER BY series`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()
	series := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			continue
		}
		series = append(series, v)
	}
	writeJSON(w, http.StatusOK, series)
}

func (s *Server) inverterList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT DISTINCT inverter FROM solar_readings ORDER BY inverter`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()
	inverters := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			continue
		}
		inverters = append(inverters, v)
	}
	writeJSON(w, http.StatusOK, inverters)
}

func (s *Server) chartPower(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	device := q.Get("device")
	series := q.Get("series")
	if series == "" {
		writeError(w, http.StatusBadRequest, "series is required")
		return
	}

	from, to, err := parseChartTimeRange(q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resolution := q.Get("resolution")
	if resolution == "" {
		resolution = autoResolution(from, to)
	}

	points := []ChartPoint{}

	if resolution == "raw" {
		var rows interface{ Next() bool; Scan(...any) error; Close() }
		var err error
		if device != "" {
			rows, err = s.pool.Query(r.Context(), `
				SELECT time, watts
				FROM iotawatt_readings
				WHERE device = $1 AND series = $2 AND time >= $3 AND time < $4
				ORDER BY time ASC LIMIT 10000
			`, device, series, from, to)
		} else {
			rows, err = s.pool.Query(r.Context(), `
				SELECT time, watts
				FROM iotawatt_readings
				WHERE series = $1 AND time >= $2 AND time < $3
				ORDER BY time ASC LIMIT 10000
			`, series, from, to)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var t time.Time
			var val *float32
			if err := rows.Scan(&t, &val); err != nil {
				continue
			}
			points = append(points, ChartPoint{Time: t, AvgWatts: val})
		}
	} else {
		table, err := iotawattTable(resolution)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var rows interface{ Next() bool; Scan(...any) error; Close() }
		// table is from a whitelist switch — not user-controlled
		if device != "" {
			rows, err = s.pool.Query(r.Context(), fmt.Sprintf(`
				SELECT bucket, avg_watts, max_watts, min_watts
				FROM %s
				WHERE device = $1 AND series = $2 AND bucket >= $3 AND bucket < $4
				ORDER BY bucket ASC
			`, table), device, series, from, to)
		} else {
			rows, err = s.pool.Query(r.Context(), fmt.Sprintf(`
				SELECT bucket, avg_watts, max_watts, min_watts
				FROM %s
				WHERE series = $1 AND bucket >= $2 AND bucket < $3
				ORDER BY bucket ASC
			`, table), series, from, to)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query failed")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var t time.Time
			var avg, max, min *float32
			if err := rows.Scan(&t, &avg, &max, &min); err != nil {
				continue
			}
			points = append(points, ChartPoint{Time: t, AvgWatts: avg, MaxWatts: max, MinWatts: min})
		}
	}

	writeJSON(w, http.StatusOK, points)
}

func (s *Server) chartSolar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	metric := q.Get("metric")
	if metric == "" {
		writeError(w, http.StatusBadRequest, "metric is required")
		return
	}
	inverter := q.Get("inverter")

	from, to, err := parseChartTimeRange(q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resolution := q.Get("resolution")
	if resolution == "" {
		resolution = autoResolution(from, to)
	}

	points := []SolarChartPoint{}

	if inverter == "" {
		if resolution == "raw" {
			rows, err := s.pool.Query(r.Context(), `
				SELECT time, value_numeric
				FROM solar_totals
				WHERE metric = $1 AND time >= $2 AND time < $3
				ORDER BY time ASC
				LIMIT 10000
			`, metric, from, to)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "query failed")
				return
			}
			defer rows.Close()
			for rows.Next() {
				var t time.Time
				var val *float32
				if err := rows.Scan(&t, &val); err != nil {
					continue
				}
				points = append(points, SolarChartPoint{Time: t, AvgValue: val})
			}
		} else {
			table, err := solarTotalsTable(resolution)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			// table is from a whitelist switch — not user-controlled
			rows, err := s.pool.Query(r.Context(), fmt.Sprintf(`
				SELECT bucket, avg_value, max_value, min_value
				FROM %s
				WHERE metric = $1 AND bucket >= $2 AND bucket < $3
				ORDER BY bucket ASC
			`, table), metric, from, to)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "query failed")
				return
			}
			defer rows.Close()
			for rows.Next() {
				var t time.Time
				var avg, max, min *float32
				if err := rows.Scan(&t, &avg, &max, &min); err != nil {
					continue
				}
				points = append(points, SolarChartPoint{Time: t, AvgValue: avg, MaxValue: max, MinValue: min})
			}
		}
	} else {
		if resolution == "raw" {
			rows, err := s.pool.Query(r.Context(), `
				SELECT time, value_numeric
				FROM solar_readings
				WHERE inverter = $1 AND metric = $2 AND time >= $3 AND time < $4
				ORDER BY time ASC
				LIMIT 10000
			`, inverter, metric, from, to)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "query failed")
				return
			}
			defer rows.Close()
			for rows.Next() {
				var t time.Time
				var val *float32
				if err := rows.Scan(&t, &val); err != nil {
					continue
				}
				points = append(points, SolarChartPoint{Time: t, AvgValue: val})
			}
		} else {
			table, err := solarTable(resolution)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			// table is from a whitelist switch — not user-controlled
			rows, err := s.pool.Query(r.Context(), fmt.Sprintf(`
				SELECT bucket, avg_value, max_value, min_value
				FROM %s
				WHERE inverter = $1 AND metric = $2 AND bucket >= $3 AND bucket < $4
				ORDER BY bucket ASC
			`, table), inverter, metric, from, to)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "query failed")
				return
			}
			defer rows.Close()
			for rows.Next() {
				var t time.Time
				var avg, max, min *float32
				if err := rows.Scan(&t, &avg, &max, &min); err != nil {
					continue
				}
				points = append(points, SolarChartPoint{Time: t, AvgValue: avg, MaxValue: max, MinValue: min})
			}
		}
	}

	writeJSON(w, http.StatusOK, points)
}

func (s *Server) chartWeather(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	metric := q.Get("metric")
	if metric == "" {
		writeError(w, http.StatusBadRequest, "metric is required")
		return
	}

	from, to, err := parseChartTimeRange(q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	rows, err := s.pool.Query(r.Context(), `
		SELECT time, value_numeric
		FROM weather_readings
		WHERE metric = $1 AND time >= $2 AND time < $3
		ORDER BY time ASC
		LIMIT 10000
	`, metric, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	points := []WeatherChartPoint{}
	for rows.Next() {
		var t time.Time
		var val *float32
		if err := rows.Scan(&t, &val); err != nil {
			continue
		}
		points = append(points, WeatherChartPoint{Time: t, Value: val})
	}

	writeJSON(w, http.StatusOK, points)
}

func parseChartTimeRange(fromStr, toStr string) (time.Time, time.Time, error) {
	to := time.Now().UTC()
	from := to.Add(-24 * time.Hour)

	if toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid to: %w", err)
		}
		to = t
	}
	if fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid from: %w", err)
		}
		from = t
	}

	return from, to, nil
}

func autoResolution(from, to time.Time) string {
	span := to.Sub(from)
	switch {
	case span < 2*time.Hour:
		return "raw"
	case span < 7*24*time.Hour:
		return "1min"
	case span < 90*24*time.Hour:
		return "1hour"
	default:
		return "1day"
	}
}

func iotawattTable(resolution string) (string, error) {
	switch resolution {
	case "1min":
		return "iotawatt_1min", nil
	case "1hour":
		return "iotawatt_1hour", nil
	case "1day":
		return "iotawatt_1day", nil
	default:
		return "", fmt.Errorf("unsupported resolution %q", resolution)
	}
}

func solarTable(resolution string) (string, error) {
	switch resolution {
	case "1min":
		return "solar_readings_1min", nil
	case "1hour":
		return "solar_readings_1hour", nil
	case "1day":
		return "solar_readings_1day", nil
	default:
		return "", fmt.Errorf("unsupported resolution %q", resolution)
	}
}

func solarTotalsTable(resolution string) (string, error) {
	switch resolution {
	case "1min":
		return "solar_totals_1min", nil
	case "1hour":
		return "solar_totals_1hour", nil
	case "1day":
		return "solar_totals_1day", nil
	default:
		return "", fmt.Errorf("unsupported resolution %q", resolution)
	}
}
