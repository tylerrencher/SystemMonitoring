//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/tylerrencher/systemmonitoring/internal/alerts"
	"github.com/tylerrencher/systemmonitoring/internal/api"
	"github.com/tylerrencher/systemmonitoring/internal/config"
	"github.com/tylerrencher/systemmonitoring/internal/testhelper"
)

func putJSON(t *testing.T, client *http.Client, url string, body any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	return resp
}

func deleteRequest(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	return resp
}

func loginAs(t *testing.T, client *http.Client, baseURL, username, password string) {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/auth/login",
		map[string]string{"username": username, "password": password})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: %d", resp.StatusCode)
	}
}

// TestListAlerts verifies the public endpoint returns seeded alert definitions.
func TestListAlerts(t *testing.T) {
	ts := httptest.NewServer(apiServer.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/alerts")
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d: %s", resp.StatusCode, body)
	}

	var items []map[string]any
	if err := json.Unmarshal([]byte(body), &items); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if len(items) < 15 {
		t.Errorf("got %d alerts, want at least 15 (migration seeds)", len(items))
	}

	// Verify a known entry is present with the expected fields
	var found bool
	for _, item := range items {
		if item["key"] == "battery_critical" {
			found = true
			if item["name"] != "Battery Critically Low" {
				t.Errorf("battery_critical name = %v, want Battery Critically Low", item["name"])
			}
			if item["type"] != "threshold_breach" {
				t.Errorf("battery_critical type = %v, want threshold_breach", item["type"])
			}
		}
		// Every item must have key, name, type
		for _, field := range []string{"key", "name", "type"} {
			if _, ok := item[field]; !ok {
				t.Errorf("alert item missing field %q: %v", field, item)
			}
		}
	}
	if !found {
		t.Error("battery_critical alert not found in list")
	}
}

// TestAlertPreferencesCRUD verifies subscribe, read-back, and replace flows.
func TestAlertPreferencesCRUD(t *testing.T) {
	testhelper.Truncate(t, dbPool, "users")

	insertTestUser(t, "pref_user", "viewer", "pass")

	ts := httptest.NewServer(apiServer.Handler())
	defer ts.Close()
	client := newClient(ts)
	loginAs(t, client, ts.URL, "pref_user", "pass")

	getPrefs := func() ([]string, map[string]string) {
		t.Helper()
		resp, _ := client.Get(ts.URL + "/api/v1/alerts/preferences")
		body := readBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET preferences got %d: %s", resp.StatusCode, body)
		}
		var result struct {
			Subscribed []string          `json:"subscribed"`
			Mutes      map[string]string `json:"mutes"`
		}
		if err := json.Unmarshal([]byte(body), &result); err != nil {
			t.Fatalf("parse preferences: %v", err)
		}
		sort.Strings(result.Subscribed)
		return result.Subscribed, result.Mutes
	}

	// Initially empty
	subs, mutes := getPrefs()
	if len(subs) != 0 {
		t.Errorf("expected no subscriptions initially, got %v", subs)
	}
	if len(mutes) != 0 {
		t.Errorf("expected no mutes initially, got %v", mutes)
	}

	// Subscribe to two alerts
	resp := putJSON(t, client, ts.URL+"/api/v1/alerts/preferences",
		map[string][]string{"keys": {"battery_critical", "house_overload"}})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT preferences got %d", resp.StatusCode)
	}

	subs, _ = getPrefs()
	want := []string{"battery_critical", "house_overload"}
	sort.Strings(want)
	if len(subs) != len(want) || subs[0] != want[0] || subs[1] != want[1] {
		t.Errorf("subscribed = %v, want %v", subs, want)
	}

	// Replace with a single key
	resp = putJSON(t, client, ts.URL+"/api/v1/alerts/preferences",
		map[string][]string{"keys": {"battery_critical"}})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT preferences (replace) got %d", resp.StatusCode)
	}

	subs, _ = getPrefs()
	if len(subs) != 1 || subs[0] != "battery_critical" {
		t.Errorf("after replace: subscribed = %v, want [battery_critical]", subs)
	}

	// Clear all
	resp = putJSON(t, client, ts.URL+"/api/v1/alerts/preferences",
		map[string][]string{"keys": {}})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT preferences (clear) got %d", resp.StatusCode)
	}

	subs, _ = getPrefs()
	if len(subs) != 0 {
		t.Errorf("after clear: subscribed = %v, want []", subs)
	}
}

// TestMuteUnmuteFlow verifies muting and unmuting an alert updates preferences.
func TestMuteUnmuteFlow(t *testing.T) {
	testhelper.Truncate(t, dbPool, "users")

	insertTestUser(t, "mute_user", "viewer", "pass")

	ts := httptest.NewServer(apiServer.Handler())
	defer ts.Close()
	client := newClient(ts)
	loginAs(t, client, ts.URL, "mute_user", "pass")

	getMutes := func() map[string]string {
		t.Helper()
		resp, _ := client.Get(ts.URL + "/api/v1/alerts/preferences")
		body := readBody(t, resp)
		var result struct {
			Mutes map[string]string `json:"mutes"`
		}
		if err := json.Unmarshal([]byte(body), &result); err != nil {
			t.Fatalf("parse preferences: %v", err)
		}
		return result.Mutes
	}

	// Mute battery_critical for 1h
	resp := postJSON(t, client, ts.URL+"/api/v1/alerts/battery_critical/mute",
		map[string]string{"duration": "1h"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("mute got %d", resp.StatusCode)
	}

	mutes := getMutes()
	mutedUntil, ok := mutes["battery_critical"]
	if !ok {
		t.Fatal("battery_critical not in mutes after mute request")
	}
	// Verify the muted_until is roughly 1 hour from now
	until, err := time.Parse(time.RFC3339, mutedUntil)
	if err != nil {
		t.Fatalf("parse muted_until %q: %v", mutedUntil, err)
	}
	remaining := time.Until(until)
	if remaining < 55*time.Minute || remaining > 65*time.Minute {
		t.Errorf("muted_until = %v, expected ~1h from now", remaining)
	}

	// Unmute
	dresp := deleteRequest(t, client, ts.URL+"/api/v1/alerts/battery_critical/mute")
	dresp.Body.Close()
	if dresp.StatusCode != http.StatusNoContent {
		t.Fatalf("unmute got %d", dresp.StatusCode)
	}

	mutes = getMutes()
	if _, ok := mutes["battery_critical"]; ok {
		t.Error("battery_critical still in mutes after unmute")
	}
}

// TestMuteDurations checks that all three valid durations are accepted.
func TestMuteDurations(t *testing.T) {
	testhelper.Truncate(t, dbPool, "users")
	insertTestUser(t, "duration_user", "viewer", "pass")

	ts := httptest.NewServer(apiServer.Handler())
	defer ts.Close()
	client := newClient(ts)
	loginAs(t, client, ts.URL, "duration_user", "pass")

	for _, d := range []string{"1h", "4h", "tomorrow"} {
		resp := postJSON(t, client, ts.URL+"/api/v1/alerts/battery_critical/mute",
			map[string]string{"duration": d})
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("duration %q: got %d, want 204", d, resp.StatusCode)
		}
		// Unmute between iterations so muted_until isn't extended by GREATEST
		dresp := deleteRequest(t, client, ts.URL+"/api/v1/alerts/battery_critical/mute")
		dresp.Body.Close()
	}
}

// TestDashboardActiveAlertsField verifies the dashboard response includes active_alerts.
func TestDashboardActiveAlertsField(t *testing.T) {
	testhelper.Truncate(t, dbPool, "users")
	insertTestUser(t, "alerts_user", "viewer", "pass")

	state := alerts.NewState()
	srv := api.NewServer(&config.Config{
		SolarActiveThresholdW:     50,
		GeneratorActiveThresholdW: 100,
		TopConsumersWindow:        30 * time.Second,
	}, dbPool).WithAlertState(state)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := newClient(ts)
	loginAs(t, client, ts.URL, "alerts_user", "pass")

	resp, err := client.Get(ts.URL + "/api/v1/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard got %d: %s", resp.StatusCode, body)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("parse body: %v", err)
	}

	v, ok := result["active_alerts"]
	if !ok {
		t.Fatal("dashboard response missing active_alerts field")
	}
	alerts, ok := v.([]any)
	if !ok {
		t.Fatalf("active_alerts is not an array, got %T: %v", v, v)
	}
	if len(alerts) != 0 {
		t.Errorf("expected empty active_alerts (no engine running), got %v", alerts)
	}
}
