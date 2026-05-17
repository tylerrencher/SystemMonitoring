package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tylerrencher/systemmonitoring/internal/config"
)

func newTestServer() *Server {
	return &Server{
		cfg:  &config.Config{},
		pool: nil, // nil pool — only test paths that return before DB access
	}
}

// --- Auth middleware ---

func TestAuthMiddlewareNoCookie(t *testing.T) {
	s := newTestServer()
	dummy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	s.auth(dummy).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareEmptyCookieValue(t *testing.T) {
	s := &Server{
		cfg:  &config.Config{},
		pool: nil,
	}
	dummy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	// Cookie present but empty value — pool is nil so GetSessionUser would panic,
	// but we expect it to reach that call. Skip this case; it requires DB.
	// Instead verify: no cookie at all → 401.
	_ = req
	_ = dummy
	_ = s
	_ = w
	t.Skip("requires DB for non-empty session cookie verification")
}

// --- Dev CORS middleware ---

func TestDevCORSPreflight(t *testing.T) {
	s := &Server{
		cfg:  &config.Config{DevCORSOrigin: "http://localhost:5173"},
		pool: nil,
	}
	dummy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/dashboard", nil)
	s.devCORS(dummy).ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("preflight got %d, want %d", w.Code, http.StatusNoContent)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("ACAO = %q, want %q", got, "http://localhost:5173")
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("ACAC = %q, want %q", got, "true")
	}
}

func TestDevCORSPassThrough(t *testing.T) {
	s := &Server{
		cfg:  &config.Config{DevCORSOrigin: "http://localhost:5173"},
		pool: nil,
	}
	dummy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	s.devCORS(dummy).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("ACAO header missing on non-preflight response")
	}
}

// --- Login handler ---

func TestLoginBadJSON(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{not json"))
	s.login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestLoginEmptyBody(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(""))
	s.login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- Chart handlers (called directly, bypass auth middleware) ---

func TestChartPowerMissingSeries(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/charts/power?device=pwrmone1", nil)
	s.chartPower(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestChartPowerMissingBoth(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/charts/power", nil)
	s.chartPower(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestChartPowerBadResolution(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/charts/power?device=pwrmone1&series=Main_W&resolution=bad", nil)
	s.chartPower(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestChartSolarMissingMetric(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/charts/solar", nil)
	s.chartSolar(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestChartSolarBadResolutionNoInverter(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/charts/solar?metric=pv_power&resolution=bad", nil)
	s.chartSolar(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestChartSolarBadResolutionWithInverter(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/charts/solar?metric=pv_power&inverter=inverter_1&resolution=bad", nil)
	s.chartSolar(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestChartWeatherMissingMetric(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/charts/weather", nil)
	s.chartWeather(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestChartPowerSeriesWithoutDevice(t *testing.T) {
	s := newTestServer()

	// device is optional; bad resolution → 400 before DB access
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/charts/power?series=Main_W&resolution=bad", nil)
	s.chartPower(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestChartPowerBadFromTimestamp(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/charts/power?device=pwrmone1&series=Main_W&from=not-a-time", nil)
	s.chartPower(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestChartPowerBadToTimestamp(t *testing.T) {
	s := newTestServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/charts/power?device=pwrmone1&series=Main_W&to=not-a-time", nil)
	s.chartPower(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
	}
}
