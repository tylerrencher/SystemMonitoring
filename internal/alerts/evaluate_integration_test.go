//go:build integration

package alerts

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tylerrencher/systemmonitoring/internal/testhelper"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	p, cleanup := testhelper.SetupSuite()
	testPool = p
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// minsBucket returns a time exactly n minutes before now, truncated to the
// minute boundary so it aligns with time_bucket('1 minute', ...).
func minsBucket(n int) time.Time {
	return time.Now().Add(time.Duration(-n) * time.Minute).Truncate(time.Minute)
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}

// refreshIotawatt1min materializes all iotawatt_readings into iotawatt_1min.
// With NO DATA on creation, TimescaleDB sets the watermark to the creation time,
// so real-time aggregation doesn't cover older inserts. Explicit window params
// bypass the policy offsets and force a full materialization.
func refreshIotawatt1min(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`CALL refresh_continuous_aggregate('iotawatt_1min', NOW() - INTERVAL '3 hours', NOW())`)
	if err != nil {
		t.Fatalf("refresh iotawatt_1min: %v", err)
	}
}

func insertIotawatt(t *testing.T, series string, ts time.Time, watts float64) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO iotawatt_readings (time, device, series, watts) VALUES ($1, 'test', $2, $3)
		 ON CONFLICT DO NOTHING`,
		ts, series, watts)
	if err != nil {
		t.Fatalf("insertIotawatt %s@%v: %v", series, ts, err)
	}
}

func insertSolarTotal(t *testing.T, metric string, val float64, ts time.Time) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO solar_totals (time, metric, value_numeric) VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`,
		ts, metric, val)
	if err != nil {
		t.Fatalf("insertSolarTotal %s: %v", metric, err)
	}
}

func runEval(t *testing.T, def AlertDef) bool {
	t.Helper()
	fired, err := evaluate(context.Background(), testPool, def)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	return fired
}

// --- evalThresholdBreach (solar_totals path) ---

func TestEvalThresholdBreach_Solar_Fires(t *testing.T) {
	testhelper.Truncate(t, testPool, "solar_totals")
	insertSolarTotal(t, "battery_state_of_charge", 5.0, time.Now().Add(-1*time.Minute))

	def := AlertDef{
		Type: "threshold_breach", DataSource: "solar_totals",
		Params: mustJSON(thresholdBreachParams{Series: "battery_state_of_charge", Operator: "lt", Threshold: 10}),
	}
	if !runEval(t, def) {
		t.Error("want fire: SoC 5 < threshold 10")
	}
}

func TestEvalThresholdBreach_Solar_NoFire(t *testing.T) {
	testhelper.Truncate(t, testPool, "solar_totals")
	insertSolarTotal(t, "battery_state_of_charge", 80.0, time.Now().Add(-1*time.Minute))

	def := AlertDef{
		Type: "threshold_breach", DataSource: "solar_totals",
		Params: mustJSON(thresholdBreachParams{Series: "battery_state_of_charge", Operator: "lt", Threshold: 10}),
	}
	if runEval(t, def) {
		t.Error("want no fire: SoC 80 is not < 10")
	}
}

func TestEvalThresholdBreach_Solar_NoData(t *testing.T) {
	testhelper.Truncate(t, testPool, "solar_totals")

	def := AlertDef{
		Type: "threshold_breach", DataSource: "solar_totals",
		Params: mustJSON(thresholdBreachParams{Series: "battery_state_of_charge", Operator: "lt", Threshold: 10}),
	}
	if runEval(t, def) {
		t.Error("want no fire: no rows in table")
	}
}

func TestEvalThresholdBreach_Solar_Between_Fires(t *testing.T) {
	testhelper.Truncate(t, testPool, "solar_totals")
	insertSolarTotal(t, "battery_state_of_charge", 50.0, time.Now().Add(-1*time.Minute))

	def := AlertDef{
		Type: "threshold_breach", DataSource: "solar_totals",
		Params: mustJSON(thresholdBreachParams{Series: "battery_state_of_charge", Operator: "between", Low: 20, High: 80}),
	}
	if !runEval(t, def) {
		t.Error("want fire: 50 is between 20 and 80")
	}
}

// --- evalThresholdBreach (iotawatt instant path, SustainedMin=0) ---

func TestEvalThresholdBreach_Iotawatt_Instant_Fires(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	insertIotawatt(t, "House", minsBucket(1), 5000)
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "threshold_breach", DataSource: "iotawatt",
		Params: mustJSON(thresholdBreachParams{Series: "House", Operator: "gt", Threshold: 1000}),
	}
	if !runEval(t, def) {
		t.Error("want fire: 5000W > threshold 1000W")
	}
}

func TestEvalThresholdBreach_Iotawatt_Instant_NoFire(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	insertIotawatt(t, "House", minsBucket(1), 100)
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "threshold_breach", DataSource: "iotawatt",
		Params: mustJSON(thresholdBreachParams{Series: "House", Operator: "gt", Threshold: 1000}),
	}
	if runEval(t, def) {
		t.Error("want no fire: 100W is not > 1000W")
	}
}

func TestEvalThresholdBreach_Iotawatt_Instant_NoData(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "threshold_breach", DataSource: "iotawatt",
		Params: mustJSON(thresholdBreachParams{Series: "House", Operator: "gt", Threshold: 1000}),
	}
	if runEval(t, def) {
		t.Error("want no fire: no data in table")
	}
}

// --- evalThresholdBreach (iotawatt sustained path, SustainedMin>0) ---

func TestEvalThresholdBreach_Iotawatt_Sustained_Fires(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	// 3 consecutive minute buckets all above threshold — at least 2 fall within the 2-min window
	for _, n := range []int{2, 1, 0} {
		insertIotawatt(t, "House", minsBucket(n), 5000)
	}
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "threshold_breach", DataSource: "iotawatt",
		Params: mustJSON(thresholdBreachParams{Series: "House", Operator: "gt", Threshold: 1000, SustainedMin: 2}),
	}
	if !runEval(t, def) {
		t.Error("want fire: 3 buckets above threshold, need 2 sustained")
	}
}

func TestEvalThresholdBreach_Iotawatt_Sustained_AllBelow_NoFire(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	for _, n := range []int{2, 1, 0} {
		insertIotawatt(t, "House", minsBucket(n), 100)
	}
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "threshold_breach", DataSource: "iotawatt",
		Params: mustJSON(thresholdBreachParams{Series: "House", Operator: "gt", Threshold: 1000, SustainedMin: 2}),
	}
	if runEval(t, def) {
		t.Error("want no fire: all buckets below threshold")
	}
}

func TestEvalThresholdBreach_Iotawatt_Sustained_InsufficientCount_NoFire(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	// Only 1 of 3 buckets above threshold — count=1 < SustainedMin=3
	insertIotawatt(t, "House", minsBucket(2), 100)
	insertIotawatt(t, "House", minsBucket(1), 5000)
	insertIotawatt(t, "House", minsBucket(0), 100)
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "threshold_breach", DataSource: "iotawatt",
		Params: mustJSON(thresholdBreachParams{Series: "House", Operator: "gt", Threshold: 1000, SustainedMin: 3}),
	}
	if runEval(t, def) {
		t.Error("want no fire: only 1 bucket above threshold, need 3")
	}
}

// --- evalShortCycle (raw iotawatt_readings path, MaxCycleMin<=2) ---

func TestEvalShortCycle_RawReadings_Fires(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	now := time.Now()
	// 3 on-runs of ~30 seconds each (run_min < MaxCycleMin=1), within 30-min window
	for _, base := range []int{25, 22, 19} {
		insertIotawatt(t, "WellPump", now.Add(time.Duration(-base)*time.Minute), 5000)
		insertIotawatt(t, "WellPump", now.Add(time.Duration(-base)*time.Minute+30*time.Second), 5000)
		insertIotawatt(t, "WellPump", now.Add(time.Duration(-base+1)*time.Minute), 0)
	}

	def := AlertDef{
		Type: "short_cycle", DataSource: "iotawatt",
		Params: mustJSON(shortCycleParams{Series: "WellPump", OnThresholdW: 100, MaxCycleMin: 1, CycleCount: 3, WindowMin: 30}),
	}
	if !runEval(t, def) {
		t.Error("want fire: 3 short cycles (<1 min each) in 30-min window")
	}
}

func TestEvalShortCycle_RawReadings_NoFire(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	now := time.Now()
	// Only 1 short on-run
	insertIotawatt(t, "WellPump", now.Add(-25*time.Minute), 5000)
	insertIotawatt(t, "WellPump", now.Add(-25*time.Minute+30*time.Second), 5000)
	insertIotawatt(t, "WellPump", now.Add(-24*time.Minute), 0)

	def := AlertDef{
		Type: "short_cycle", DataSource: "iotawatt",
		Params: mustJSON(shortCycleParams{Series: "WellPump", OnThresholdW: 100, MaxCycleMin: 1, CycleCount: 3, WindowMin: 30}),
	}
	if runEval(t, def) {
		t.Error("want no fire: only 1 short cycle, need 3")
	}
}

// --- evalShortCycle (iotawatt_1min path, MaxCycleMin>2) ---

func TestEvalShortCycle_1min_Fires(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	// 3 on-runs of 1 bucket each (run_min=1 < MaxCycleMin=5), separated by off buckets
	for _, n := range []int{55, 53, 51} {
		insertIotawatt(t, "UpstrHVAC", minsBucket(n), 5000)   // on
		insertIotawatt(t, "UpstrHVAC", minsBucket(n-1), 0)    // off
	}
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "short_cycle", DataSource: "iotawatt",
		Params: mustJSON(shortCycleParams{Series: "UpstrHVAC", OnThresholdW: 100, MaxCycleMin: 5, CycleCount: 3, WindowMin: 60}),
	}
	if !runEval(t, def) {
		t.Error("want fire: 3 one-minute on-runs in 60-min window (MaxCycleMin=5)")
	}
}

func TestEvalShortCycle_1min_NoFire(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	insertIotawatt(t, "UpstrHVAC", minsBucket(55), 5000)
	insertIotawatt(t, "UpstrHVAC", minsBucket(54), 0)
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "short_cycle", DataSource: "iotawatt",
		Params: mustJSON(shortCycleParams{Series: "UpstrHVAC", OnThresholdW: 100, MaxCycleMin: 5, CycleCount: 3, WindowMin: 60}),
	}
	if runEval(t, def) {
		t.Error("want no fire: only 1 short cycle, need 3")
	}
}

// --- evalScheduledCheck ---

func TestEvalScheduledCheck_Iotawatt_PastTime_Fires(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	insertIotawatt(t, "Generator", minsBucket(1), 5000)
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "scheduled_check", DataSource: "iotawatt",
		Params: mustJSON(scheduledCheckParams{CheckTime: "00:00", Series: "Generator", Operator: "gt", Threshold: 100}),
	}
	if !runEval(t, def) {
		t.Error("want fire: check_time 00:00 has passed and generator above threshold")
	}
}

func TestEvalScheduledCheck_Iotawatt_FutureTime_NoFire(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	insertIotawatt(t, "Generator", minsBucket(1), 5000)
	refreshIotawatt1min(t)

	now := time.Now()
	if now.Hour() == 23 && now.Minute() >= 58 {
		t.Skip("skipping: within 2 minutes of midnight")
	}

	def := AlertDef{
		Type: "scheduled_check", DataSource: "iotawatt",
		Params: mustJSON(scheduledCheckParams{CheckTime: "23:59", Series: "Generator", Operator: "gt", Threshold: 100}),
	}
	if runEval(t, def) {
		t.Error("want no fire: check_time 23:59 has not arrived yet")
	}
}

func TestEvalScheduledCheck_Solar_PastTime_Fires(t *testing.T) {
	testhelper.Truncate(t, testPool, "solar_totals")
	insertSolarTotal(t, "battery_state_of_charge", 20.0, time.Now().Add(-1*time.Minute))

	def := AlertDef{
		Type: "scheduled_check", DataSource: "solar_totals",
		Params: mustJSON(scheduledCheckParams{CheckTime: "00:00", Series: "battery_state_of_charge", Operator: "lt", Threshold: 33}),
	}
	if !runEval(t, def) {
		t.Error("want fire: check_time passed and SoC 20 < 33")
	}
}

// --- evalDifferential ---

func TestEvalDifferential_Fires(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	// 10 buckets, both series present, large difference (7000W > MaxDiffW=2000)
	for n := 10; n >= 1; n-- {
		insertIotawatt(t, "HouseL1", minsBucket(n), 8000)
		insertIotawatt(t, "HouseL2", minsBucket(n), 1000)
	}
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "differential", DataSource: "iotawatt",
		Params: mustJSON(differentialParams{SeriesA: "HouseL1", SeriesB: "HouseL2", MaxDiffW: 2000, SustainedMin: 3}),
	}
	if !runEval(t, def) {
		t.Error("want fire: |8000-1000|=7000 > 2000 sustained for 3 minutes")
	}
}

func TestEvalDifferential_SmallDiff_NoFire(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	for n := 10; n >= 1; n-- {
		insertIotawatt(t, "HouseL1", minsBucket(n), 4000)
		insertIotawatt(t, "HouseL2", minsBucket(n), 3500)
	}
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "differential", DataSource: "iotawatt",
		Params: mustJSON(differentialParams{SeriesA: "HouseL1", SeriesB: "HouseL2", MaxDiffW: 2000, SustainedMin: 3}),
	}
	if runEval(t, def) {
		t.Error("want no fire: |4000-3500|=500 does not exceed 2000")
	}
}

func TestEvalDifferential_NoData_NoFire(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "differential", DataSource: "iotawatt",
		Params: mustJSON(differentialParams{SeriesA: "HouseL1", SeriesB: "HouseL2", MaxDiffW: 2000, SustainedMin: 3}),
	}
	if runEval(t, def) {
		t.Error("want no fire: no data")
	}
}

// --- evalCountPerWindow ---

func TestEvalCountPerWindow_TwoSessions_Fires(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	// Session 1: on for 5 buckets
	for n := 55; n >= 51; n-- {
		insertIotawatt(t, "HybdWH", minsBucket(n), 5000)
	}
	// Gap: off for 5 buckets (= MinGapMin=5)
	for n := 50; n >= 46; n-- {
		insertIotawatt(t, "HybdWH", minsBucket(n), 0)
	}
	// Session 2: on for 5 buckets
	for n := 45; n >= 41; n-- {
		insertIotawatt(t, "HybdWH", minsBucket(n), 5000)
	}
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "count_per_window", DataSource: "iotawatt",
		Params: mustJSON(countPerWindowParams{Series: "HybdWH", OnThresholdW: 100, MinGapMin: 5, MaxCount: 1, WindowHours: 1}),
	}
	if !runEval(t, def) {
		t.Error("want fire: 2 valid on-sessions exceeds MaxCount=1")
	}
}

func TestEvalCountPerWindow_OneSession_NoFire(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	for n := 55; n >= 51; n-- {
		insertIotawatt(t, "HybdWH", minsBucket(n), 5000)
	}
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "count_per_window", DataSource: "iotawatt",
		Params: mustJSON(countPerWindowParams{Series: "HybdWH", OnThresholdW: 100, MinGapMin: 5, MaxCount: 1, WindowHours: 1}),
	}
	if runEval(t, def) {
		t.Error("want no fire: only 1 on-session, MaxCount=1 requires >1")
	}
}

func TestEvalCountPerWindow_GapTooSmall_NoFire(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	// Session 1: on for 5 buckets
	for n := 55; n >= 51; n-- {
		insertIotawatt(t, "HybdWH", minsBucket(n), 5000)
	}
	// Gap: off for only 2 buckets (< MinGapMin=5) — second on-run not counted as new session
	for n := 50; n >= 49; n-- {
		insertIotawatt(t, "HybdWH", minsBucket(n), 0)
	}
	// "Session 2": on for 5 buckets — not counted because gap was too small
	for n := 48; n >= 44; n-- {
		insertIotawatt(t, "HybdWH", minsBucket(n), 5000)
	}
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "count_per_window", DataSource: "iotawatt",
		Params: mustJSON(countPerWindowParams{Series: "HybdWH", OnThresholdW: 100, MinGapMin: 5, MaxCount: 1, WindowHours: 1}),
	}
	if runEval(t, def) {
		t.Error("want no fire: second on-run not counted because gap=2 < MinGapMin=5")
	}
}

// --- evalCycleComplete ---

func TestEvalCycleComplete_Fires(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	// On-run: 5 buckets (MinRunMin=5)
	for n := 20; n >= 16; n-- {
		insertIotawatt(t, "Dryer", minsBucket(n), 5000)
	}
	// Off-run: 5 buckets starting at minsBucket(15) (OffConfirmMin=3, window NOW()-18min to NOW()-3min)
	for n := 15; n >= 11; n-- {
		insertIotawatt(t, "Dryer", minsBucket(n), 0)
	}
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "cycle_complete", DataSource: "iotawatt",
		Params: mustJSON(cycleCompleteParams{Series: "Dryer", OnThresholdW: 100, MinRunMin: 5, OffConfirmMin: 3}),
	}
	if !runEval(t, def) {
		t.Error("want fire: on-run completed, off-run started ~15 min ago (within 3+15=18 min window)")
	}
}

func TestEvalCycleComplete_OffTooLongAgo_NoFire(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	// Off-run started 40 minutes ago — well outside the (OffConfirmMin=3)+15=18 minute window
	for n := 50; n >= 46; n-- {
		insertIotawatt(t, "Dryer", minsBucket(n), 5000)
	}
	for n := 45; n >= 41; n-- {
		insertIotawatt(t, "Dryer", minsBucket(n), 0)
	}
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "cycle_complete", DataSource: "iotawatt",
		Params: mustJSON(cycleCompleteParams{Series: "Dryer", OnThresholdW: 100, MinRunMin: 5, OffConfirmMin: 3}),
	}
	if runEval(t, def) {
		t.Error("want no fire: off-run started ~45 min ago, outside 18-min window")
	}
}

func TestEvalCycleComplete_NoData_NoFire(t *testing.T) {
	testhelper.Truncate(t, testPool, "iotawatt_readings")
	refreshIotawatt1min(t)

	def := AlertDef{
		Type: "cycle_complete", DataSource: "iotawatt",
		Params: mustJSON(cycleCompleteParams{Series: "Dryer", OnThresholdW: 100, MinRunMin: 5, OffConfirmMin: 3}),
	}
	if runEval(t, def) {
		t.Error("want no fire: no data")
	}
}

// --- loadAlertDefs ---

func TestLoadAlertDefs_ReturnsSeedData(t *testing.T) {
	defs, err := loadAlertDefs(context.Background(), testPool)
	if err != nil {
		t.Fatalf("loadAlertDefs: %v", err)
	}
	if len(defs) < 15 {
		t.Errorf("want ≥15 seeded defs, got %d", len(defs))
	}

	var found bool
	for _, d := range defs {
		if d.Key == "battery_critical" {
			found = true
			if d.Name != "Battery Critically Low" {
				t.Errorf("Name = %q, want %q", d.Name, "Battery Critically Low")
			}
			if d.Type != "threshold_breach" {
				t.Errorf("Type = %q, want threshold_breach", d.Type)
			}
			if d.Cooldown != 5*time.Minute {
				t.Errorf("Cooldown = %v, want 5m", d.Cooldown)
			}
		}
	}
	if !found {
		t.Error("battery_critical not found in defs")
	}
}

func TestLoadAlertDefs_DisabledExcluded(t *testing.T) {
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO alerts (key, name, type, data_source, params, cooldown, enabled)
		VALUES ('test_disabled_alert', 'Test Disabled', 'threshold_breach', 'iotawatt',
		        '{"series":"X","operator":"gt","threshold":1}'::jsonb,
		        INTERVAL '5 minutes', false)
		ON CONFLICT (key) DO UPDATE SET enabled = false
	`)
	if err != nil {
		t.Fatalf("insert disabled alert: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM alerts WHERE key = 'test_disabled_alert'`) //nolint:errcheck
	})

	defs, err := loadAlertDefs(context.Background(), testPool)
	if err != nil {
		t.Fatalf("loadAlertDefs: %v", err)
	}
	for _, d := range defs {
		if d.Key == "test_disabled_alert" {
			t.Error("disabled alert should not be returned")
		}
	}
}

// --- loadRecipients ---

func insertAlertUser(t *testing.T, name string) int {
	t.Helper()
	var id int
	err := testPool.QueryRow(context.Background(),
		`INSERT INTO users (name, password_hash, role, email) VALUES ($1, 'hash', 'viewer', $2) RETURNING id`,
		name, name+"@example.com",
	).Scan(&id)
	if err != nil {
		t.Fatalf("insertAlertUser %q: %v", name, err)
	}
	return id
}

func TestLoadRecipients_SubscribedNoMute(t *testing.T) {
	testhelper.Truncate(t, testPool, "users")
	id := insertAlertUser(t, "alice")

	_, err := testPool.Exec(context.Background(),
		`INSERT INTO alert_preferences (user_id, alert_key) VALUES ($1, 'battery_critical')`, id)
	if err != nil {
		t.Fatalf("insert preference: %v", err)
	}

	rs, err := loadRecipients(context.Background(), testPool, []string{"battery_critical"})
	if err != nil {
		t.Fatalf("loadRecipients: %v", err)
	}
	if len(rs) != 1 {
		t.Fatalf("want 1 recipient, got %d", len(rs))
	}
	if rs[0].Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", rs[0].Email)
	}
	if rs[0].AlertKey != "battery_critical" {
		t.Errorf("AlertKey = %q, want battery_critical", rs[0].AlertKey)
	}
}

func TestLoadRecipients_ActiveMute_NotReturned(t *testing.T) {
	testhelper.Truncate(t, testPool, "users")
	id := insertAlertUser(t, "bob")

	testPool.Exec(context.Background(), //nolint:errcheck
		`INSERT INTO alert_preferences (user_id, alert_key) VALUES ($1, 'battery_critical')`, id)
	testPool.Exec(context.Background(), //nolint:errcheck
		`INSERT INTO alert_mutes (user_id, alert_key, muted_until) VALUES ($1, 'battery_critical', NOW() + INTERVAL '1 hour')`, id)

	rs, err := loadRecipients(context.Background(), testPool, []string{"battery_critical"})
	if err != nil {
		t.Fatalf("loadRecipients: %v", err)
	}
	if len(rs) != 0 {
		t.Errorf("want 0 recipients (active mute), got %d", len(rs))
	}
}

func TestLoadRecipients_ExpiredMute_Returned(t *testing.T) {
	testhelper.Truncate(t, testPool, "users")
	id := insertAlertUser(t, "carol")

	testPool.Exec(context.Background(), //nolint:errcheck
		`INSERT INTO alert_preferences (user_id, alert_key) VALUES ($1, 'battery_critical')`, id)
	testPool.Exec(context.Background(), //nolint:errcheck
		`INSERT INTO alert_mutes (user_id, alert_key, muted_until) VALUES ($1, 'battery_critical', NOW() - INTERVAL '1 minute')`, id)

	rs, err := loadRecipients(context.Background(), testPool, []string{"battery_critical"})
	if err != nil {
		t.Fatalf("loadRecipients: %v", err)
	}
	if len(rs) != 1 {
		t.Errorf("want 1 recipient (mute expired), got %d", len(rs))
	}
}

func TestLoadRecipients_EmptyKeys_ReturnsNil(t *testing.T) {
	rs, err := loadRecipients(context.Background(), testPool, nil)
	if err != nil {
		t.Fatalf("loadRecipients(nil): %v", err)
	}
	if rs != nil {
		t.Errorf("want nil for empty keys, got %v", rs)
	}
}

// --- setMute ---

func TestSetMute_InsertsRecord(t *testing.T) {
	testhelper.Truncate(t, testPool, "users")
	id := insertAlertUser(t, "dave")

	if err := setMute(context.Background(), testPool, id, "battery_critical", time.Hour); err != nil {
		t.Fatalf("setMute: %v", err)
	}

	var until time.Time
	testPool.QueryRow(context.Background(), //nolint:errcheck
		`SELECT muted_until FROM alert_mutes WHERE user_id = $1 AND alert_key = 'battery_critical'`, id,
	).Scan(&until)

	remaining := time.Until(until)
	if remaining < 55*time.Minute || remaining > 65*time.Minute {
		t.Errorf("muted_until is %v from now, expected ~1h", remaining)
	}
}

func TestSetMute_GREATEST_DoesNotShortenExisting(t *testing.T) {
	testhelper.Truncate(t, testPool, "users")
	id := insertAlertUser(t, "eve")

	setMute(context.Background(), testPool, id, "battery_critical", 4*time.Hour) //nolint:errcheck
	setMute(context.Background(), testPool, id, "battery_critical", time.Hour)   //nolint:errcheck

	var until time.Time
	testPool.QueryRow(context.Background(), //nolint:errcheck
		`SELECT muted_until FROM alert_mutes WHERE user_id = $1 AND alert_key = 'battery_critical'`, id,
	).Scan(&until)

	if time.Until(until) < 3*time.Hour+30*time.Minute {
		t.Error("GREATEST should have preserved the 4h mute when setting a shorter 1h mute")
	}
}

func TestSetMute_GREATEST_ExtendsWithLongerDuration(t *testing.T) {
	testhelper.Truncate(t, testPool, "users")
	id := insertAlertUser(t, "frank")

	setMute(context.Background(), testPool, id, "battery_critical", time.Hour)   //nolint:errcheck
	setMute(context.Background(), testPool, id, "battery_critical", 4*time.Hour) //nolint:errcheck

	var until time.Time
	testPool.QueryRow(context.Background(), //nolint:errcheck
		`SELECT muted_until FROM alert_mutes WHERE user_id = $1 AND alert_key = 'battery_critical'`, id,
	).Scan(&until)

	if time.Until(until) < 3*time.Hour+30*time.Minute {
		t.Error("setting a longer 4h mute after a 1h mute should extend the mute_until")
	}
}
