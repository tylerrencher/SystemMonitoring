package api

import (
	"encoding/json"
	"net/http"
	"time"
)

type alertListItem struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type preferencesResponse struct {
	Subscribed []string          `json:"subscribed"`
	Mutes      map[string]string `json:"mutes"`
}

func (s *Server) listAlerts(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT key, name, type FROM alerts WHERE enabled = true ORDER BY name
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	items := []alertListItem{}
	for rows.Next() {
		var item alertListItem
		if err := rows.Scan(&item.Key, &item.Name, &item.Type); err != nil {
			continue
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getPreferences(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r.Context())

	subRows, err := s.pool.Query(r.Context(), `
		SELECT alert_key FROM alert_preferences WHERE user_id = $1
	`, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer subRows.Close()

	subscribed := []string{}
	for subRows.Next() {
		var key string
		if err := subRows.Scan(&key); err != nil {
			continue
		}
		subscribed = append(subscribed, key)
	}

	muteRows, err := s.pool.Query(r.Context(), `
		SELECT alert_key, muted_until FROM alert_mutes
		WHERE user_id = $1 AND muted_until > NOW()
	`, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer muteRows.Close()

	mutes := map[string]string{}
	for muteRows.Next() {
		var key string
		var until time.Time
		if err := muteRows.Scan(&key, &until); err != nil {
			continue
		}
		mutes[key] = until.UTC().Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, preferencesResponse{Subscribed: subscribed, Mutes: mutes})
}

func (s *Server) putPreferences(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r.Context())

	var body struct {
		Keys []string `json:"keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Keys == nil {
		body.Keys = []string{}
	}

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "transaction failed")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	if _, err := tx.Exec(r.Context(), `
		DELETE FROM alert_preferences WHERE user_id = $1
	`, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}

	for _, key := range body.Keys {
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO alert_preferences (user_id, alert_key)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, user.ID, key); err != nil {
			writeError(w, http.StatusInternalServerError, "update failed")
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "commit failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) muteAlert(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r.Context())
	key := r.PathValue("key")

	var body struct {
		Duration string `json:"duration"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var mutedUntil time.Time
	now := time.Now()
	switch body.Duration {
	case "1h":
		mutedUntil = now.Add(time.Hour)
	case "4h":
		mutedUntil = now.Add(4 * time.Hour)
	case "tomorrow":
		tomorrow := now.AddDate(0, 0, 1)
		mutedUntil = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 8, 0, 0, 0, now.Location())
	default:
		writeError(w, http.StatusBadRequest, `duration must be "1h", "4h", or "tomorrow"`)
		return
	}

	_, err := s.pool.Exec(r.Context(), `
		INSERT INTO alert_mutes (user_id, alert_key, muted_until)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, alert_key) DO UPDATE
		SET muted_until = GREATEST(EXCLUDED.muted_until, alert_mutes.muted_until)
	`, user.ID, key, mutedUntil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unmuteAlert(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r.Context())
	key := r.PathValue("key")

	_, err := s.pool.Exec(r.Context(), `
		DELETE FROM alert_mutes WHERE user_id = $1 AND alert_key = $2
	`, user.ID, key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
