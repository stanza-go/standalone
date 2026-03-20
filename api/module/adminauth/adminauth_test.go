package adminauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminauth"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*fhttp.Router, *auth.Auth, *sqlite.DB) {
	t.Helper()
	db := testutil.SetupDB(t)
	a := testutil.NewAdminAuth()
	logger := testutil.NewLogger(t)
	router := testutil.NewRouter()
	api := router.Group("/api")
	adminauth.Register(api, a, db, logger)
	return router, a, db
}

func TestLogin_Success(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/auth/login", map[string]string{
		"email":    "admin@stanza.dev",
		"password": "admin",
	})
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	admin, ok := resp["admin"].(map[string]any)
	if !ok {
		t.Fatal("missing admin in response")
	}
	if admin["email"] != "admin@stanza.dev" {
		t.Errorf("email = %v, want admin@stanza.dev", admin["email"])
	}
	if admin["role"] != "superadmin" {
		t.Errorf("role = %v, want superadmin", admin["role"])
	}

	// Should set access and refresh token cookies.
	cookies := rec.Result().Cookies()
	var hasAccess, hasRefresh bool
	for _, c := range cookies {
		if c.Name == auth.AccessTokenCookie {
			hasAccess = true
		}
		if c.Name == auth.RefreshTokenCookie {
			hasRefresh = true
		}
	}
	if !hasAccess {
		t.Error("missing access_token cookie")
	}
	if !hasRefresh {
		t.Error("missing refresh_token cookie")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/auth/login", map[string]string{
		"email":    "admin@stanza.dev",
		"password": "wrong",
	})
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_NonexistentEmail(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/auth/login", map[string]string{
		"email":    "nobody@stanza.dev",
		"password": "admin",
	})
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	tests := []struct {
		name string
		body map[string]string
	}{
		{"empty email", map[string]string{"email": "", "password": "admin"}},
		{"empty password", map[string]string{"email": "admin@stanza.dev", "password": ""}},
		{"both empty", map[string]string{"email": "", "password": ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := testutil.JSONRequest(t, "POST", "/api/admin/auth/login", tt.body)
			rec := testutil.Do(router, req)

			if rec.Code != 400 {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestStatus_WithValidRefreshToken(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	// Login first to get a refresh token.
	loginReq := testutil.JSONRequest(t, "POST", "/api/admin/auth/login", map[string]string{
		"email":    "admin@stanza.dev",
		"password": "admin",
	})
	loginRec := testutil.Do(router, loginReq)
	if loginRec.Code != 200 {
		t.Fatalf("login failed: %d", loginRec.Code)
	}

	// Extract refresh token from login response cookies.
	var refreshToken string
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == auth.RefreshTokenCookie {
			refreshToken = c.Value
		}
	}
	if refreshToken == "" {
		t.Fatal("no refresh token in login response")
	}

	// Call status endpoint with refresh token.
	statusReq := httptest.NewRequest("GET", "/api/admin/auth/", nil)
	testutil.AddRefreshToken(statusReq, refreshToken)
	statusRec := testutil.Do(router, statusReq)

	if statusRec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", statusRec.Code, statusRec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, statusRec, &resp)
	admin, ok := resp["admin"].(map[string]any)
	if !ok {
		t.Fatal("missing admin in response")
	}
	if admin["email"] != "admin@stanza.dev" {
		t.Errorf("email = %v, want admin@stanza.dev", admin["email"])
	}

	// Should issue a fresh access token.
	var hasAccess bool
	for _, c := range statusRec.Result().Cookies() {
		if c.Name == auth.AccessTokenCookie {
			hasAccess = true
		}
	}
	if !hasAccess {
		t.Error("status should set a fresh access_token cookie")
	}

	_ = a
	_ = db
}

func TestStatus_NoRefreshToken(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/auth/", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestStatus_ExpiredRefreshToken(t *testing.T) {
	t.Parallel()
	router, _, db := setup(t)

	// Insert an expired refresh token directly.
	refreshToken := "test-expired-token"
	tokenHash := auth.HashToken(refreshToken)
	expiredAt := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO refresh_tokens (id, entity_type, entity_id, token_hash, expires_at) VALUES (?, ?, ?, ?, ?)`,
		"test-id-expired", "admin", "1", tokenHash, expiredAt,
	)
	if err != nil {
		t.Fatalf("insert refresh token: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/admin/auth/", nil)
	req.AddCookie(&http.Cookie{Name: auth.RefreshTokenCookie, Value: refreshToken})
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogout(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	// Login first.
	loginReq := testutil.JSONRequest(t, "POST", "/api/admin/auth/login", map[string]string{
		"email":    "admin@stanza.dev",
		"password": "admin",
	})
	loginRec := testutil.Do(router, loginReq)
	if loginRec.Code != 200 {
		t.Fatalf("login failed: %d", loginRec.Code)
	}

	var refreshToken string
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == auth.RefreshTokenCookie {
			refreshToken = c.Value
		}
	}

	// Logout.
	logoutReq := httptest.NewRequest("POST", "/api/admin/auth/logout", nil)
	if refreshToken != "" {
		logoutReq.AddCookie(&http.Cookie{Name: auth.RefreshTokenCookie, Value: refreshToken})
	}
	logoutRec := testutil.Do(router, logoutReq)

	if logoutRec.Code != 200 {
		t.Fatalf("logout status = %d, want 200", logoutRec.Code)
	}

	// Status should now fail.
	statusReq := httptest.NewRequest("GET", "/api/admin/auth/", nil)
	if refreshToken != "" {
		testutil.AddRefreshToken(statusReq, refreshToken)
	}
	statusRec := testutil.Do(router, statusReq)

	if statusRec.Code != 401 {
		t.Errorf("status after logout = %d, want 401", statusRec.Code)
	}
}

func TestLogout_NoToken(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/auth/logout", nil)
	rec := testutil.Do(router, req)

	// Logout should succeed even without a token.
	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
