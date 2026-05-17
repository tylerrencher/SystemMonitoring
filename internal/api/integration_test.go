//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tylerrencher/systemmonitoring/internal/api"
	"github.com/tylerrencher/systemmonitoring/internal/config"
	"github.com/tylerrencher/systemmonitoring/internal/testhelper"
	"golang.org/x/crypto/bcrypt"
)

var (
	dbPool    *pgxpool.Pool
	apiServer *api.Server
)

func TestMain(m *testing.M) {
	p, cleanup := testhelper.SetupSuite()
	dbPool = p
	apiServer = api.NewServer(&config.Config{
		SolarActiveThresholdW:     50,
		GeneratorActiveThresholdW: 100,
		TopConsumersWindow:        30 * time.Second,
		IoTawattPollInterval:      10 * time.Second,
	}, dbPool)
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// newClient returns an HTTP client with a cookie jar pointed at ts.
func newClient(ts *httptest.Server) *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func insertTestUser(t *testing.T, name, role, password string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	_, err = dbPool.Exec(context.Background(), `
		INSERT INTO users (name, password_hash, role) VALUES ($1, $2, $3)
	`, name, string(hash), role)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func postJSON(t *testing.T, client *http.Client, url string, body any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := client.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func TestLoginSuccess(t *testing.T) {
	testhelper.Truncate(t, dbPool, "users")
	insertTestUser(t, "tyler", "admin", "secret123")

	ts := httptest.NewServer(apiServer.Handler())
	defer ts.Close()
	client := newClient(ts)

	resp := postJSON(t, client, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "tyler", "password": "secret123"})
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login got %d, body: %s", resp.StatusCode, body)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	user := result["user"].(map[string]any)
	if user["name"] != "tyler" {
		t.Errorf("user.name = %v, want tyler", user["name"])
	}
	if user["role"] != "admin" {
		t.Errorf("user.role = %v, want admin", user["role"])
	}

	// Check Set-Cookie attributes from the raw response (jar strips HttpOnly/SameSite)
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie in Set-Cookie header")
	}
	if !sessionCookie.HttpOnly {
		t.Error("session cookie should be HttpOnly")
	}
	if sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Error("session cookie should be SameSite=Strict")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	testhelper.Truncate(t, dbPool, "users")
	insertTestUser(t, "tyler", "admin", "correct-pass")

	ts := httptest.NewServer(apiServer.Handler())
	defer ts.Close()
	client := newClient(ts)

	resp := postJSON(t, client, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "tyler", "password": "wrong-pass"})
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestDashboardRequiresSession(t *testing.T) {
	ts := httptest.NewServer(apiServer.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestDashboardWithValidSession(t *testing.T) {
	testhelper.Truncate(t, dbPool, "users")
	insertTestUser(t, "tyler", "admin", "pass")

	ts := httptest.NewServer(apiServer.Handler())
	defer ts.Close()
	client := newClient(ts)

	// Login
	loginResp := postJSON(t, client, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "tyler", "password": "pass"})
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: %d", loginResp.StatusCode)
	}

	// Dashboard
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
	if _, ok := result["power_source"]; !ok {
		t.Error("missing power_source field")
	}
	if _, ok := result["top_consumers"]; !ok {
		t.Error("missing top_consumers field")
	}
	// Empty DB → battery_only
	if result["power_source"] != "battery_only" {
		t.Errorf("power_source = %v, want battery_only", result["power_source"])
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	testhelper.Truncate(t, dbPool, "users")
	insertTestUser(t, "tyler", "admin", "pass")

	ts := httptest.NewServer(apiServer.Handler())
	defer ts.Close()
	client := newClient(ts)

	// Login
	loginResp := postJSON(t, client, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "tyler", "password": "pass"})
	loginResp.Body.Close()

	// Logout
	logoutResp, _ := client.Post(ts.URL+"/api/v1/auth/logout", "application/json", nil)
	logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusNoContent {
		t.Errorf("logout got %d, want %d", logoutResp.StatusCode, http.StatusNoContent)
	}

	// Dashboard after logout → 401
	dashResp, _ := client.Get(ts.URL + "/api/v1/dashboard")
	dashResp.Body.Close()
	if dashResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("post-logout dashboard got %d, want %d", dashResp.StatusCode, http.StatusUnauthorized)
	}
}

func TestChartPowerEmptyDB(t *testing.T) {
	testhelper.Truncate(t, dbPool, "users")
	insertTestUser(t, "tyler", "admin", "pass")

	ts := httptest.NewServer(apiServer.Handler())
	defer ts.Close()
	client := newClient(ts)

	postJSON(t, client, ts.URL+"/api/v1/auth/login",
		map[string]string{"username": "tyler", "password": "pass"}).Body.Close()

	resp, err := client.Get(ts.URL + "/api/v1/charts/power?device=pwrmone1&series=Main_W")
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("chart/power got %d: %s", resp.StatusCode, body)
	}
	if strings.TrimSpace(body) != "[]" {
		t.Errorf("expected [], got: %s", body)
	}
}
