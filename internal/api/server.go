package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tylerrencher/systemmonitoring/internal/alerts"
	"github.com/tylerrencher/systemmonitoring/internal/config"
)

type Server struct {
	cfg        *config.Config
	pool       *pgxpool.Pool
	static     fs.FS
	alertState *alerts.State
}

func NewServer(cfg *config.Config, pool *pgxpool.Pool) *Server {
	return &Server{cfg: cfg, pool: pool}
}

func (s *Server) WithStatic(fsys fs.FS) *Server {
	s.static = fsys
	return s
}

func (s *Server) WithAlertState(state *alerts.State) *Server {
	s.alertState = state
	return s
}

// Handler builds and returns the HTTP handler for the API. Used by Serve and
// by httptest-based integration tests.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.Handle("POST /api/v1/auth/logout", s.auth(http.HandlerFunc(s.logout)))
	mux.Handle("GET /api/v1/dashboard", s.auth(http.HandlerFunc(s.dashboard)))
	mux.HandleFunc("GET /api/v1/series", s.seriesList)
	mux.HandleFunc("GET /api/v1/solar/inverters", s.inverterList)
	mux.HandleFunc("GET /api/v1/charts/power", s.chartPower)
	mux.HandleFunc("GET /api/v1/charts/solar", s.chartSolar)
	mux.HandleFunc("GET /api/v1/charts/weather", s.chartWeather)
	mux.HandleFunc("GET /api/v1/ws", s.wsHandler)
	mux.HandleFunc("GET /api/v1/alerts", s.listAlerts)
	mux.Handle("GET /api/v1/alerts/preferences", s.auth(http.HandlerFunc(s.getPreferences)))
	mux.Handle("PUT /api/v1/alerts/preferences", s.auth(http.HandlerFunc(s.putPreferences)))
	mux.Handle("POST /api/v1/alerts/{key}/mute", s.auth(http.HandlerFunc(s.muteAlert)))
	mux.Handle("DELETE /api/v1/alerts/{key}/mute", s.auth(http.HandlerFunc(s.unmuteAlert)))

	if s.static != nil {
		mux.Handle("/", spaHandler(s.static))
	}

	var handler http.Handler = mux
	if s.cfg.DevCORSOrigin != "" {
		handler = s.devCORS(handler)
	}
	return handler
}

func (s *Server) Serve(ctx context.Context) error {
	srv := &http.Server{
		Addr:        s.cfg.APIListen,
		Handler:     s.Handler(),
		ReadTimeout: 15 * time.Second,
		// No WriteTimeout: WebSocket connections are long-lived
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx) //nolint:errcheck
	}()

	log.Printf("[api] listening on %s", s.cfg.APIListen)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// spaHandler serves static files and falls back to index.html for unknown paths
// so that the React client-side router handles route resolution.
// index.html is served with Cache-Control: no-store so mobile browsers always
// fetch the latest build; hashed JS/CSS assets are left to default FileServer
// caching since their filenames change on every rebuild.
func spaHandler(fsys fs.FS) http.Handler {
	server := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(fsys, path); err != nil {
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFileFS(w, r, fsys, "index.html")
			return
		}
		if path == "index.html" {
			w.Header().Set("Cache-Control", "no-store")
		}
		server.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
