package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func evaluate(ctx context.Context, pool *pgxpool.Pool, def AlertDef) (bool, error) {
	switch def.Type {
	case "threshold_breach":
		var p thresholdBreachParams
		if err := json.Unmarshal(def.Params, &p); err != nil {
			return false, fmt.Errorf("parse params: %w", err)
		}
		return evalThresholdBreach(ctx, pool, def.DataSource, p)
	case "short_cycle":
		var p shortCycleParams
		if err := json.Unmarshal(def.Params, &p); err != nil {
			return false, fmt.Errorf("parse params: %w", err)
		}
		return evalShortCycle(ctx, pool, p)
	case "scheduled_check":
		var p scheduledCheckParams
		if err := json.Unmarshal(def.Params, &p); err != nil {
			return false, fmt.Errorf("parse params: %w", err)
		}
		return evalScheduledCheck(ctx, pool, def.DataSource, p)
	case "differential":
		var p differentialParams
		if err := json.Unmarshal(def.Params, &p); err != nil {
			return false, fmt.Errorf("parse params: %w", err)
		}
		return evalDifferential(ctx, pool, p)
	case "count_per_window":
		var p countPerWindowParams
		if err := json.Unmarshal(def.Params, &p); err != nil {
			return false, fmt.Errorf("parse params: %w", err)
		}
		return evalCountPerWindow(ctx, pool, p)
	case "cycle_complete":
		var p cycleCompleteParams
		if err := json.Unmarshal(def.Params, &p); err != nil {
			return false, fmt.Errorf("parse params: %w", err)
		}
		return evalCycleComplete(ctx, pool, p)
	default:
		return false, fmt.Errorf("unknown alert type %q", def.Type)
	}
}

// --- param structs ---

type thresholdBreachParams struct {
	Series       string  `json:"series"`
	Operator     string  `json:"operator"`
	Threshold    float64 `json:"threshold"`
	Low          float64 `json:"low"`
	High         float64 `json:"high"`
	SustainedMin int     `json:"sustained_min"`
}

type shortCycleParams struct {
	Series       string  `json:"series"`
	OnThresholdW float64 `json:"on_threshold_w"`
	MaxCycleMin  int     `json:"max_cycle_min"`
	CycleCount   int     `json:"cycle_count"`
	WindowMin    int     `json:"window_min"`
}

type scheduledCheckParams struct {
	CheckTime string  `json:"check_time"`
	Series    string  `json:"series"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
}

type differentialParams struct {
	SeriesA      string  `json:"series_a"`
	SeriesB      string  `json:"series_b"`
	MaxDiffW     float64 `json:"max_diff_w"`
	SustainedMin int     `json:"sustained_min"`
}

type countPerWindowParams struct {
	Series       string  `json:"series"`
	OnThresholdW float64 `json:"on_threshold_w"`
	MinGapMin    int     `json:"min_gap_min"`
	MaxCount     int     `json:"max_count"`
	WindowHours  int     `json:"window_hours"`
}

type cycleCompleteParams struct {
	Series        string  `json:"series"`
	OnThresholdW  float64 `json:"on_threshold_w"`
	MinRunMin     int     `json:"min_run_min"`
	OffConfirmMin int     `json:"off_confirm_min"`
}

// --- evaluators ---

func evalThresholdBreach(ctx context.Context, pool *pgxpool.Pool, dataSource string, p thresholdBreachParams) (bool, error) {
	if dataSource != "iotawatt" {
		var val *float64
		err := pool.QueryRow(ctx, `
			SELECT value_numeric FROM solar_totals
			WHERE metric = $1 AND time >= NOW() - INTERVAL '5 minutes'
			ORDER BY time DESC LIMIT 1
		`, p.Series).Scan(&val)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil || val == nil {
			return false, err
		}
		return applyOp(p.Operator, *val, p.Threshold, p.Low, p.High), nil
	}

	if p.SustainedMin == 0 {
		var val *float64
		err := pool.QueryRow(ctx, `
			SELECT avg_watts FROM iotawatt_1min
			WHERE series = $1 AND bucket >= NOW() - INTERVAL '3 minutes'
			ORDER BY bucket DESC LIMIT 1
		`, p.Series).Scan(&val)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil || val == nil {
			return false, err
		}
		return applyOp(p.Operator, *val, p.Threshold, p.Low, p.High), nil
	}

	rows, err := pool.Query(ctx, `
		SELECT avg_watts FROM iotawatt_1min
		WHERE series = $1
		  AND bucket >= NOW() - ($2::int + 1) * INTERVAL '1 minute'
		ORDER BY bucket ASC
	`, p.Series, p.SustainedMin)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var w *float64
		if err := rows.Scan(&w); err != nil || w == nil {
			continue
		}
		if applyOp(p.Operator, *w, p.Threshold, p.Low, p.High) {
			count++
		}
	}
	return count >= p.SustainedMin, rows.Err()
}

// evalShortCycle uses raw iotawatt_readings for cycles <= 2 min (sub-minute precision needed)
// and iotawatt_1min for longer cycles.
func evalShortCycle(ctx context.Context, pool *pgxpool.Pool, p shortCycleParams) (bool, error) {
	var count int
	var err error

	if p.MaxCycleMin <= 2 {
		err = pool.QueryRow(ctx, `
			WITH data AS (
				SELECT time AS t, COALESCE(watts, 0) >= $1 AS is_on
				FROM iotawatt_readings
				WHERE series = $2
				  AND time >= NOW() - $3::int * INTERVAL '1 minute'
				ORDER BY time
			),
			lagged AS (
				SELECT t, is_on,
				    LAG(is_on, 1, is_on) OVER (ORDER BY t) AS prev_is_on
				FROM data
			),
			grp AS (
				SELECT t, is_on,
				    SUM(CASE WHEN is_on != prev_is_on THEN 1 ELSE 0 END)
				    OVER (ORDER BY t) AS g
				FROM lagged
			),
			runs AS (
				SELECT g, is_on,
				    EXTRACT(EPOCH FROM (MAX(t) - MIN(t))) / 60.0 AS run_min
				FROM grp GROUP BY g, is_on
			)
			SELECT COUNT(*) FROM runs WHERE is_on AND run_min < $4
		`, p.OnThresholdW, p.Series, p.WindowMin, p.MaxCycleMin).Scan(&count)
	} else {
		err = pool.QueryRow(ctx, `
			WITH data AS (
				SELECT bucket, avg_watts >= $1 AS is_on
				FROM iotawatt_1min
				WHERE series = $2
				  AND bucket >= NOW() - $3::int * INTERVAL '1 minute'
				ORDER BY bucket
			),
			lagged AS (
				SELECT bucket, is_on,
				    LAG(is_on, 1, is_on) OVER (ORDER BY bucket) AS prev_is_on
				FROM data
			),
			grp AS (
				SELECT bucket, is_on,
				    SUM(CASE WHEN is_on != prev_is_on THEN 1 ELSE 0 END)
				    OVER (ORDER BY bucket) AS g
				FROM lagged
			),
			runs AS (
				SELECT g, is_on, COUNT(*) AS run_min
				FROM grp GROUP BY g, is_on
			)
			SELECT COUNT(*) FROM runs WHERE is_on AND run_min < $4
		`, p.OnThresholdW, p.Series, p.WindowMin, p.MaxCycleMin).Scan(&count)
	}

	return count >= p.CycleCount, err
}

func evalScheduledCheck(ctx context.Context, pool *pgxpool.Pool, dataSource string, p scheduledCheckParams) (bool, error) {
	parts := strings.SplitN(p.CheckTime, ":", 2)
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid check_time %q", p.CheckTime)
	}
	hour, _ := strconv.Atoi(parts[0])
	min, _ := strconv.Atoi(parts[1])

	now := time.Now()
	checkTime := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
	if now.Before(checkTime) {
		return false, nil
	}

	if dataSource != "iotawatt" {
		var val *float64
		err := pool.QueryRow(ctx, `
			SELECT value_numeric FROM solar_totals
			WHERE metric = $1 AND time >= NOW() - INTERVAL '5 minutes'
			ORDER BY time DESC LIMIT 1
		`, p.Series).Scan(&val)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil || val == nil {
			return false, err
		}
		return applyOp(p.Operator, *val, p.Threshold, 0, 0), nil
	}

	var val *float64
	err := pool.QueryRow(ctx, `
		SELECT avg_watts FROM iotawatt_1min
		WHERE series = $1 AND bucket >= NOW() - INTERVAL '3 minutes'
		ORDER BY bucket DESC LIMIT 1
	`, p.Series).Scan(&val)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil || val == nil {
		return false, err
	}
	return applyOp(p.Operator, *val, p.Threshold, 0, 0), nil
}

func evalDifferential(ctx context.Context, pool *pgxpool.Pool, p differentialParams) (bool, error) {
	var count int
	err := pool.QueryRow(ctx, `
		WITH pivot AS (
			SELECT bucket,
			    MAX(CASE WHEN series = $1 THEN avg_watts END) AS a,
			    MAX(CASE WHEN series = $2 THEN avg_watts END) AS b
			FROM iotawatt_1min
			WHERE series IN ($1, $2)
			  AND bucket >= NOW() - ($3::int + 1) * INTERVAL '1 minute'
			GROUP BY bucket
		)
		SELECT COUNT(*)
		FROM pivot
		WHERE a IS NOT NULL AND b IS NOT NULL AND ABS(a - b) > $4
	`, p.SeriesA, p.SeriesB, p.SustainedMin, p.MaxDiffW).Scan(&count)
	return count >= p.SustainedMin, err
}

func evalCountPerWindow(ctx context.Context, pool *pgxpool.Pool, p countPerWindowParams) (bool, error) {
	var count int
	err := pool.QueryRow(ctx, `
		WITH data AS (
			SELECT bucket, avg_watts >= $1 AS is_on
			FROM iotawatt_1min
			WHERE series = $2
			  AND bucket >= NOW() - $3::int * INTERVAL '1 hour'
			ORDER BY bucket
		),
		lagged AS (
			SELECT bucket, is_on,
			    LAG(is_on, 1, is_on) OVER (ORDER BY bucket) AS prev_is_on
			FROM data
		),
		grp AS (
			SELECT bucket, is_on,
			    SUM(CASE WHEN is_on != prev_is_on THEN 1 ELSE 0 END)
			    OVER (ORDER BY bucket) AS g
			FROM lagged
		),
		runs AS (
			SELECT g, is_on, COUNT(*) AS run_min
			FROM grp GROUP BY g, is_on
		),
		on_sessions AS (
			SELECT r.g
			FROM runs r
			LEFT JOIN runs prev ON prev.g = r.g - 1
			WHERE r.is_on
			  AND (prev.g IS NULL OR (NOT prev.is_on AND prev.run_min >= $4))
		)
		SELECT COUNT(*) FROM on_sessions
	`, p.OnThresholdW, p.Series, p.WindowHours, p.MinGapMin).Scan(&count)
	return count > p.MaxCount, err
}

// evalCycleComplete fires in a 15-minute window after the off-confirmation period starts,
// preventing re-firing on subsequent ticks while the appliance remains off.
func evalCycleComplete(ctx context.Context, pool *pgxpool.Pool, p cycleCompleteParams) (bool, error) {
	var breached bool
	err := pool.QueryRow(ctx, `
		WITH data AS (
			SELECT bucket, avg_watts >= $1 AS is_on
			FROM iotawatt_1min
			WHERE series = $2
			  AND bucket >= NOW() - INTERVAL '8 hours'
			ORDER BY bucket
		),
		lagged AS (
			SELECT bucket, is_on,
			    LAG(is_on, 1, is_on) OVER (ORDER BY bucket) AS prev_is_on
			FROM data
		),
		grp AS (
			SELECT bucket, is_on,
			    SUM(CASE WHEN is_on != prev_is_on THEN 1 ELSE 0 END)
			    OVER (ORDER BY bucket) AS g
			FROM lagged
		),
		runs AS (
			SELECT g, is_on,
			    COUNT(*) AS run_min,
			    MIN(bucket) AS run_start
			FROM grp GROUP BY g, is_on
		)
		SELECT EXISTS (
			SELECT 1
			FROM runs off_run
			JOIN runs on_run ON on_run.g = off_run.g - 1
			WHERE NOT off_run.is_on
			  AND off_run.run_min >= $3
			  AND on_run.is_on
			  AND on_run.run_min >= $4
			  AND off_run.run_start BETWEEN
			        NOW() - ($3::int + 15) * INTERVAL '1 minute'
			        AND NOW() - $3::int * INTERVAL '1 minute'
		)
	`, p.OnThresholdW, p.Series, p.OffConfirmMin, p.MinRunMin).Scan(&breached)
	return breached, err
}

func applyOp(op string, val, threshold, low, high float64) bool {
	switch op {
	case "gt":
		return val > threshold
	case "lt":
		return val < threshold
	case "between":
		return val > low && val < high
	default:
		return false
	}
}
