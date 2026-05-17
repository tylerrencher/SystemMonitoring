package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// Auth verified by middleware before upgrade.
	// Tailscale network is trusted; allow all origins.
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[api/ws] upgrade error: %v", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Read pump: detect client disconnect
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	interval := s.cfg.IoTawattPollInterval
	if v := r.URL.Query().Get("interval"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= time.Second {
			interval = d
		}
	}

	window := s.cfg.TopConsumersWindow
	if v := r.URL.Query().Get("window"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			window = d
		}
	}

	// Send initial snapshot before first tick
	if data, err := s.queryDashboard(ctx, window); err == nil {
		conn.WriteJSON(data) //nolint:errcheck
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			data, err := s.queryDashboard(ctx, window)
			if err != nil {
				log.Printf("[api/ws] dashboard query error: %v", err)
				return
			}
			if err := conn.WriteJSON(data); err != nil {
				return
			}
		}
	}
}
