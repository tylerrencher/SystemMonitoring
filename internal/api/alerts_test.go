package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMuteAlertBadJSON(t *testing.T) {
	s := newTestServer()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/battery_critical/mute",
		strings.NewReader("{not json"))
	s.muteAlert(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestMuteAlertInvalidDuration(t *testing.T) {
	durations := []string{"2h", "24h", "never", "", "1H"}
	for _, d := range durations {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/battery_critical/mute",
			strings.NewReader(`{"duration":"`+d+`"}`))
		newTestServer().muteAlert(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("duration %q: got %d, want %d", d, w.Code, http.StatusBadRequest)
		}
	}
}

func TestPutPreferencesBadJSON(t *testing.T) {
	s := newTestServer()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/alerts/preferences",
		strings.NewReader("{not json"))
	s.putPreferences(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// All four preference/mute routes are wrapped in s.auth — no session cookie → 401
// without any DB access.
func TestAlertPreferencesRequireAuth(t *testing.T) {
	handler := newTestServer().Handler()

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/alerts/preferences"},
		{http.MethodPut, "/api/v1/alerts/preferences"},
		{http.MethodPost, "/api/v1/alerts/battery_critical/mute"},
		{http.MethodDelete, "/api/v1/alerts/battery_critical/mute"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(c.method, c.path, nil)
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401", c.method, c.path, w.Code)
		}
	}
}
