// Package testutil provides shared test infrastructure for standalone
// integration tests. It bootstraps a real SQLite database with migrations
// and seed data, and provides helpers for authenticated HTTP testing.
package testutil

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/migration"
	"github.com/stanza-go/standalone/seed"
)

// testKey is a fixed 32-byte signing key for deterministic test tokens.
var testKey = []byte("test-signing-key-for-integration!")

// SetupDB creates a temporary SQLite database with all migrations applied
// and seed data inserted. The database is automatically cleaned up when
// the test finishes.
func SetupDB(t *testing.T) *sqlite.DB {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")

	db := sqlite.New(dbPath)
	if err := db.Start(context.Background()); err != nil {
		t.Fatalf("db start: %v", err)
	}

	migration.Register(db)
	n, err := db.Migrate()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if n == 0 {
		t.Fatal("expected migrations to run")
	}

	logger := NewLogger(t)
	if err := seed.Run(db, logger); err != nil {
		t.Fatalf("seed: %v", err)
	}

	t.Cleanup(func() {
		db.Stop(context.Background())
	})

	return db
}

// NewLogger returns a logger that discards output.
func NewLogger(t *testing.T) *log.Logger {
	t.Helper()
	return log.New(log.WithWriter(io.Discard))
}

// NewAdminAuth returns an auth.Auth configured for admin routes with
// secure cookies disabled (for testing).
func NewAdminAuth() *auth.Auth {
	return auth.New(testKey, auth.WithSecureCookies(false))
}

// NewUserAuth returns an auth.Auth configured for user routes with
// cookie path /api and secure cookies disabled.
func NewUserAuth() *auth.Auth {
	return auth.New(testKey,
		auth.WithCookiePath("/api"),
		auth.WithSecureCookies(false),
	)
}

// NewRouter creates a fresh router for testing.
func NewRouter() *fhttp.Router {
	return fhttp.NewRouter()
}

// Do executes a request against a router and returns the recorder.
func Do(router *fhttp.Router, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// JSONRequest builds an httptest request with a JSON body.
func JSONRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// AddAdminAuth adds an admin access token cookie to the request.
func AddAdminAuth(t *testing.T, req *http.Request, a *auth.Auth, uid string) {
	t.Helper()
	token, err := a.IssueAccessToken(uid, []string{"admin", "admin:users", "admin:settings", "admin:jobs", "admin:logs", "admin:audit", "admin:uploads", "admin:database", "admin:roles", "admin:notifications"})
	if err != nil {
		t.Fatalf("issue admin token: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: auth.AccessTokenCookie, Value: token})
}

// AddUserAuth adds a user access token cookie to the request.
func AddUserAuth(t *testing.T, req *http.Request, a *auth.Auth, uid string) {
	t.Helper()
	token, err := a.IssueAccessToken(uid, []string{"user"})
	if err != nil {
		t.Fatalf("issue user token: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: auth.AccessTokenCookie, Value: token})
}

// AddRefreshToken adds a refresh token cookie to the request.
func AddRefreshToken(req *http.Request, token string) {
	req.AddCookie(&http.Cookie{Name: auth.RefreshTokenCookie, Value: token})
}

// DecodeJSON reads the response body into the given value.
func DecodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, rec.Body.String())
	}
}

// SetEnv sets an environment variable for the duration of the test.
func SetEnv(t *testing.T, key, value string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}
