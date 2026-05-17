package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           int
	Name         string
	Role         string
	PasswordHash string
}

func GetUserByName(ctx context.Context, pool *pgxpool.Pool, name string) (*User, error) {
	u := &User{}
	err := pool.QueryRow(ctx, `
		SELECT id, name, role, password_hash FROM users WHERE name = $1
	`, name).Scan(&u.ID, &u.Name, &u.Role, &u.PasswordHash)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func CreateSession(ctx context.Context, pool *pgxpool.Pool, userID int) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(b)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	_, err := pool.Exec(ctx, `
		INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, $3)
	`, token, userID, expiresAt)
	return token, expiresAt, err
}

func GetSessionUser(ctx context.Context, pool *pgxpool.Pool, token string) (*User, error) {
	u := &User{}
	err := pool.QueryRow(ctx, `
		SELECT u.id, u.name, u.role
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token = $1 AND s.expires_at > NOW()
	`, token).Scan(&u.ID, &u.Name, &u.Role)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func DeleteSession(ctx context.Context, pool *pgxpool.Pool, token string) error {
	_, err := pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return err
}
