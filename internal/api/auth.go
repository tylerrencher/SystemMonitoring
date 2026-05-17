package api

import (
	"encoding/json"
	"net/http"
	"time"

	appdb "github.com/tylerrencher/systemmonitoring/internal/db"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := appdb.GetUserByName(r.Context(), s.pool, body.Username)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, expiresAt, err := appdb.CreateSession(r.Context(), s.pool, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Expires:  expiresAt,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":   user.ID,
			"name": user.Name,
			"role": user.Role,
		},
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("session")
	if cookie != nil {
		appdb.DeleteSession(r.Context(), s.pool, cookie.Value) //nolint:errcheck
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Expires:  time.Unix(0, 0),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	w.WriteHeader(http.StatusNoContent)
}
