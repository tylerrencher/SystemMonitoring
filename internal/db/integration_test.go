//go:build integration

package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	appdb "github.com/tylerrencher/systemmonitoring/internal/db"
	"github.com/tylerrencher/systemmonitoring/internal/testhelper"
	"golang.org/x/crypto/bcrypt"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	p, cleanup := testhelper.SetupSuite()
	pool = p
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// insertUser inserts a test user and returns its ID.
func insertUser(t *testing.T, name, role string) int {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	var id int
	err = pool.QueryRow(context.Background(), `
		INSERT INTO users (name, password_hash, role) VALUES ($1, $2, $3) RETURNING id
	`, name, string(hash), role).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func TestGetUserByName(t *testing.T) {
	testhelper.Truncate(t, pool, "users")
	insertUser(t, "alice", "admin")

	user, err := appdb.GetUserByName(context.Background(), pool, "alice")
	if err != nil {
		t.Fatalf("GetUserByName: %v", err)
	}
	if user.Name != "alice" {
		t.Errorf("Name = %q, want %q", user.Name, "alice")
	}
	if user.Role != "admin" {
		t.Errorf("Role = %q, want %q", user.Role, "admin")
	}
	if user.PasswordHash == "" {
		t.Error("PasswordHash should not be empty")
	}
	if user.ID <= 0 {
		t.Errorf("ID = %d, want > 0", user.ID)
	}
}

func TestGetUserByNameNotFound(t *testing.T) {
	testhelper.Truncate(t, pool, "users")

	_, err := appdb.GetUserByName(context.Background(), pool, "nobody")
	if err == nil {
		t.Error("expected error for non-existent user")
	}
}

func TestCreateAndGetSession(t *testing.T) {
	testhelper.Truncate(t, pool, "users")
	id := insertUser(t, "bob", "viewer")

	token, expiresAt, err := appdb.CreateSession(context.Background(), pool, id)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if len(token) != 64 {
		t.Errorf("token length = %d, want 64 (32 hex bytes)", len(token))
	}
	if expiresAt.Before(time.Now().Add(29 * 24 * time.Hour)) {
		t.Errorf("expiresAt = %v, want ~30 days from now", expiresAt)
	}

	user, err := appdb.GetSessionUser(context.Background(), pool, token)
	if err != nil {
		t.Fatalf("GetSessionUser: %v", err)
	}
	if user.ID != id {
		t.Errorf("user.ID = %d, want %d", user.ID, id)
	}
	if user.Name != "bob" {
		t.Errorf("user.Name = %q, want %q", user.Name, "bob")
	}
	if user.Role != "viewer" {
		t.Errorf("user.Role = %q, want %q", user.Role, "viewer")
	}
}

func TestGetSessionInvalidToken(t *testing.T) {
	_, err := appdb.GetSessionUser(context.Background(), pool, "not-a-real-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestCreateSessionTwoUsersIndependent(t *testing.T) {
	testhelper.Truncate(t, pool, "users")
	id1 := insertUser(t, "user1", "admin")
	id2 := insertUser(t, "user2", "viewer")

	tok1, _, err := appdb.CreateSession(context.Background(), pool, id1)
	if err != nil {
		t.Fatalf("CreateSession user1: %v", err)
	}
	tok2, _, err := appdb.CreateSession(context.Background(), pool, id2)
	if err != nil {
		t.Fatalf("CreateSession user2: %v", err)
	}

	if tok1 == tok2 {
		t.Error("tokens should be unique")
	}

	u1, _ := appdb.GetSessionUser(context.Background(), pool, tok1)
	u2, _ := appdb.GetSessionUser(context.Background(), pool, tok2)
	if u1.ID != id1 {
		t.Errorf("tok1 resolved to user %d, want %d", u1.ID, id1)
	}
	if u2.ID != id2 {
		t.Errorf("tok2 resolved to user %d, want %d", u2.ID, id2)
	}
}

func TestDeleteSession(t *testing.T) {
	testhelper.Truncate(t, pool, "users")
	id := insertUser(t, "carol", "viewer")

	token, _, err := appdb.CreateSession(context.Background(), pool, id)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := appdb.DeleteSession(context.Background(), pool, token); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	_, err = appdb.GetSessionUser(context.Background(), pool, token)
	if err == nil {
		t.Error("expected error after session deleted")
	}
}

func TestDeleteNonExistentSessionNoError(t *testing.T) {
	err := appdb.DeleteSession(context.Background(), pool, "does-not-exist")
	if err != nil {
		t.Errorf("DeleteSession on missing token should not error, got: %v", err)
	}
}

func TestWatermarkNotFoundBeforeSet(t *testing.T) {
	source := "test/watermark/notfound"
	pool.Exec(context.Background(), "DELETE FROM ingestion_watermarks WHERE source = $1", source) //nolint:errcheck

	_, found, err := appdb.GetWatermark(context.Background(), pool, source)
	if err != nil {
		t.Fatalf("GetWatermark: %v", err)
	}
	if found {
		t.Error("expected not found before SetWatermark")
	}
}

func TestWatermarkSetAndGet(t *testing.T) {
	source := "test/watermark/set"
	pool.Exec(context.Background(), "DELETE FROM ingestion_watermarks WHERE source = $1", source) //nolint:errcheck

	mark := time.Now().Truncate(time.Millisecond).UTC()
	if err := appdb.SetWatermark(context.Background(), pool, source, mark); err != nil {
		t.Fatalf("SetWatermark: %v", err)
	}

	got, found, err := appdb.GetWatermark(context.Background(), pool, source)
	if err != nil {
		t.Fatalf("GetWatermark: %v", err)
	}
	if !found {
		t.Fatal("expected found after SetWatermark")
	}
	if !got.Equal(mark) {
		t.Errorf("watermark = %v, want %v", got, mark)
	}
}

func TestWatermarkUpdate(t *testing.T) {
	source := "test/watermark/update"
	pool.Exec(context.Background(), "DELETE FROM ingestion_watermarks WHERE source = $1", source) //nolint:errcheck

	mark1 := time.Now().Add(-time.Hour).Truncate(time.Millisecond).UTC()
	mark2 := time.Now().Truncate(time.Millisecond).UTC()

	appdb.SetWatermark(context.Background(), pool, source, mark1) //nolint:errcheck
	appdb.SetWatermark(context.Background(), pool, source, mark2) //nolint:errcheck

	got, _, _ := appdb.GetWatermark(context.Background(), pool, source)
	if !got.Equal(mark2) {
		t.Errorf("updated watermark = %v, want %v", got, mark2)
	}
}
