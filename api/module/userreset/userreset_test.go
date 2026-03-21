package userreset

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/email"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*sqlite.DB, *fhttp.Router, *email.Client) {
	t.Helper()
	db := testutil.SetupDB(t)
	router := testutil.NewRouter()
	logger := testutil.NewLogger(t)

	// Email client without API key — won't send emails but won't fail.
	emailClient := email.New("")

	api := router.Group("/api")
	Register(api, db, emailClient, logger)

	return db, router, emailClient
}

func createTestUser(t *testing.T, db *sqlite.DB, emailAddr, password string) int64 {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.Exec(
		"INSERT INTO users (email, password, name, is_active, created_at, updated_at) VALUES (?, ?, ?, 1, ?, ?)",
		emailAddr, hash, "Test User", now, now,
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return result.LastInsertID
}

func doJSON(router *fhttp.Router, method, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestForgotPasswordSuccess(t *testing.T) {
	db, router, _ := setup(t)
	createTestUser(t, db, "test@example.com", "oldpassword123")

	rec := doJSON(router, "POST", "/api/auth/forgot-password", map[string]string{
		"email": "test@example.com",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify token was stored.
	var count int
	row := db.QueryRow("SELECT COUNT(*) FROM password_reset_tokens WHERE email = 'test@example.com'")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 token, got %d", count)
	}
}

func TestForgotPasswordUnknownEmail(t *testing.T) {
	_, router, _ := setup(t)

	// Should still return 200 to prevent email enumeration.
	rec := doJSON(router, "POST", "/api/auth/forgot-password", map[string]string{
		"email": "nonexistent@example.com",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestForgotPasswordInvalidatesOldTokens(t *testing.T) {
	db, router, _ := setup(t)
	createTestUser(t, db, "test@example.com", "oldpassword123")

	// Request first token.
	doJSON(router, "POST", "/api/auth/forgot-password", map[string]string{
		"email": "test@example.com",
	})

	// Request second token — first should be invalidated.
	doJSON(router, "POST", "/api/auth/forgot-password", map[string]string{
		"email": "test@example.com",
	})

	// Only 1 unused token should remain.
	var count int
	row := db.QueryRow("SELECT COUNT(*) FROM password_reset_tokens WHERE email = 'test@example.com' AND used_at IS NULL")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 unused token, got %d", count)
	}
}

func TestForgotPasswordValidation(t *testing.T) {
	_, router, _ := setup(t)

	tests := []struct {
		name  string
		body  map[string]string
		code  int
	}{
		{"missing email", map[string]string{}, http.StatusUnprocessableEntity},
		{"invalid email", map[string]string{"email": "notanemail"}, http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doJSON(router, "POST", "/api/auth/forgot-password", tt.body)
			if rec.Code != tt.code {
				t.Fatalf("expected %d, got %d: %s", tt.code, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestResetPasswordFullFlow(t *testing.T) {
	db, router, _ := setup(t)
	createTestUser(t, db, "test@example.com", "oldpassword123")

	// Step 1: Request reset.
	doJSON(router, "POST", "/api/auth/forgot-password", map[string]string{
		"email": "test@example.com",
	})

	// Get the token hash from DB and generate a known token for testing.
	// Instead, we'll directly insert a known token.
	token := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	tokenHash := auth.HashToken(token)
	expiresAt := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
	_, err := db.Exec(
		"INSERT INTO password_reset_tokens (id, email, token_hash, expires_at) VALUES ('test-id-2', 'test@example.com', ?, ?)",
		tokenHash, expiresAt,
	)
	if err != nil {
		t.Fatalf("insert known token: %v", err)
	}

	// Step 2: Reset password with the known token.
	rec := doJSON(router, "POST", "/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "newpassword456",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify token is marked as used.
	var usedAt string
	row := db.QueryRow("SELECT COALESCE(used_at, '') FROM password_reset_tokens WHERE id = 'test-id-2'")
	if err := row.Scan(&usedAt); err != nil {
		t.Fatalf("query used_at: %v", err)
	}
	if usedAt == "" {
		t.Fatal("expected token to be marked as used")
	}

	// Verify new password works.
	var passwordHash string
	row = db.QueryRow("SELECT password FROM users WHERE email = 'test@example.com'")
	if err := row.Scan(&passwordHash); err != nil {
		t.Fatalf("query password: %v", err)
	}
	if !auth.VerifyPassword(passwordHash, "newpassword456") {
		t.Fatal("new password does not verify")
	}
	if auth.VerifyPassword(passwordHash, "oldpassword123") {
		t.Fatal("old password should not verify")
	}
}

func TestResetPasswordInvalidToken(t *testing.T) {
	_, router, _ := setup(t)

	rec := doJSON(router, "POST", "/api/auth/reset-password", map[string]string{
		"token":    "nonexistent-token",
		"password": "newpassword456",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestResetPasswordExpiredToken(t *testing.T) {
	db, router, _ := setup(t)
	createTestUser(t, db, "test@example.com", "oldpassword123")

	// Insert an expired token.
	token := "expired-token-0000000000000000000000000000000000000000000000000000"
	tokenHash := auth.HashToken(token)
	expiresAt := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	_, err := db.Exec(
		"INSERT INTO password_reset_tokens (id, email, token_hash, expires_at) VALUES ('expired-id', 'test@example.com', ?, ?)",
		tokenHash, expiresAt,
	)
	if err != nil {
		t.Fatalf("insert expired token: %v", err)
	}

	rec := doJSON(router, "POST", "/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "newpassword456",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify old password still works.
	var passwordHash string
	row := db.QueryRow("SELECT password FROM users WHERE email = 'test@example.com'")
	if err := row.Scan(&passwordHash); err != nil {
		t.Fatalf("query password: %v", err)
	}
	if !auth.VerifyPassword(passwordHash, "oldpassword123") {
		t.Fatal("old password should still verify after expired token attempt")
	}
}

func TestResetPasswordUsedToken(t *testing.T) {
	db, router, _ := setup(t)
	createTestUser(t, db, "test@example.com", "oldpassword123")

	// Insert a used token.
	token := "used-token-000000000000000000000000000000000000000000000000000000"
	tokenHash := auth.HashToken(token)
	expiresAt := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
	usedAt := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		"INSERT INTO password_reset_tokens (id, email, token_hash, expires_at, used_at) VALUES ('used-id', 'test@example.com', ?, ?, ?)",
		tokenHash, expiresAt, usedAt,
	)
	if err != nil {
		t.Fatalf("insert used token: %v", err)
	}

	rec := doJSON(router, "POST", "/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "newpassword456",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestResetPasswordValidation(t *testing.T) {
	_, router, _ := setup(t)

	tests := []struct {
		name string
		body map[string]string
		code int
	}{
		{"missing token", map[string]string{"password": "newpassword"}, http.StatusUnprocessableEntity},
		{"missing password", map[string]string{"token": "abc123"}, http.StatusUnprocessableEntity},
		{"short password", map[string]string{"token": "abc123", "password": "short"}, http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doJSON(router, "POST", "/api/auth/reset-password", tt.body)
			if rec.Code != tt.code {
				t.Fatalf("expected %d, got %d: %s", tt.code, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestResetPasswordRevokesRefreshTokens(t *testing.T) {
	db, router, _ := setup(t)
	uid := createTestUser(t, db, "test@example.com", "oldpassword123")

	// Create a refresh token for the user.
	_, err := db.Exec(
		"INSERT INTO refresh_tokens (id, entity_type, entity_id, token_hash, expires_at) VALUES ('rt-1', 'user', ?, 'hash123', ?)",
		uid, time.Now().Add(24*time.Hour).UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert refresh token: %v", err)
	}

	// Insert a valid reset token.
	token := "revoke-test-00000000000000000000000000000000000000000000000000000"
	tokenHash := auth.HashToken(token)
	expiresAt := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
	_, err = db.Exec(
		"INSERT INTO password_reset_tokens (id, email, token_hash, expires_at) VALUES ('revoke-id', 'test@example.com', ?, ?)",
		tokenHash, expiresAt,
	)
	if err != nil {
		t.Fatalf("insert reset token: %v", err)
	}

	// Reset password.
	rec := doJSON(router, "POST", "/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "newpassword456",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify refresh token was revoked.
	var count int
	row := db.QueryRow("SELECT COUNT(*) FROM refresh_tokens WHERE entity_type = 'user' AND entity_id = ?", uid)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count refresh tokens: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 refresh tokens after reset, got %d", count)
	}
}

func TestForgotPasswordWithEmailClient(t *testing.T) {
	// Test that when email is configured, it actually calls the Resend API.
	var gotRequest bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequest = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_test"}`))
	}))
	defer srv.Close()

	db := testutil.SetupDB(t)
	router := testutil.NewRouter()
	logger := testutil.NewLogger(t)

	emailClient := email.New("re_test_key",
		email.WithFrom("noreply@test.com"),
		email.WithEndpoint(srv.URL),
	)

	api := router.Group("/api")
	Register(api, db, emailClient, logger)

	createTestUser(t, db, "test@example.com", "oldpassword123")

	rec := doJSON(router, "POST", "/api/auth/forgot-password", map[string]string{
		"email": "test@example.com",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !gotRequest {
		t.Fatal("expected email API to be called")
	}
}

func TestResetPasswordDeactivatedUser(t *testing.T) {
	db, router, _ := setup(t)
	createTestUser(t, db, "test@example.com", "oldpassword123")

	// Deactivate the user.
	_, err := db.Exec("UPDATE users SET is_active = 0 WHERE email = 'test@example.com'")
	if err != nil {
		t.Fatalf("deactivate user: %v", err)
	}

	// Insert a valid reset token.
	token := "deactivated-0000000000000000000000000000000000000000000000000000"
	tokenHash := auth.HashToken(token)
	expiresAt := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
	_, err = db.Exec(
		"INSERT INTO password_reset_tokens (id, email, token_hash, expires_at) VALUES ('deact-id', 'test@example.com', ?, ?)",
		tokenHash, expiresAt,
	)
	if err != nil {
		t.Fatalf("insert reset token: %v", err)
	}

	rec := doJSON(router, "POST", "/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "newpassword456",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for deactivated user, got %d: %s", rec.Code, rec.Body.String())
	}
}
