package userauth_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/userauth"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*fhttp.Router, *auth.Auth, *sqlite.DB) {
	t.Helper()
	db := testutil.SetupDB(t)
	a := testutil.NewUserAuth()
	logger := testutil.NewLogger(t)
	router := testutil.NewRouter()
	api := router.Group("/api")
	userauth.Register(api, a, db, logger)
	return router, a, db
}

func TestRegister_Success(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "user@example.com",
		"password": "password123",
		"name":     "Test User",
	})
	rec := testutil.Do(router, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	user, ok := resp["user"].(map[string]any)
	if !ok {
		t.Fatal("missing user in response")
	}
	if user["email"] != "user@example.com" {
		t.Errorf("email = %v, want user@example.com", user["email"])
	}
	if user["name"] != "Test User" {
		t.Errorf("name = %v, want Test User", user["name"])
	}

	// Should auto-login (set cookies).
	var hasAccess, hasRefresh bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.AccessTokenCookie {
			hasAccess = true
		}
		if c.Name == auth.RefreshTokenCookie {
			hasRefresh = true
		}
	}
	if !hasAccess {
		t.Error("missing access_token cookie after register")
	}
	if !hasRefresh {
		t.Error("missing refresh_token cookie after register")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	body := map[string]string{
		"email":    "dup@example.com",
		"password": "password123",
	}

	// Register first time.
	req := testutil.JSONRequest(t, "POST", "/api/auth/register", body)
	rec := testutil.Do(router, req)
	if rec.Code != 201 {
		t.Fatalf("first register failed: %d", rec.Code)
	}

	// Register same email again.
	req2 := testutil.JSONRequest(t, "POST", "/api/auth/register", body)
	rec2 := testutil.Do(router, req2)
	if rec2.Code != 409 {
		t.Errorf("status = %d, want 409 (duplicate)", rec2.Code)
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "short@example.com",
		"password": "123",
	})
	rec := testutil.Do(router, req)

	if rec.Code != 422 {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestRegister_MissingFields(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	tests := []struct {
		name string
		body map[string]string
	}{
		{"empty email", map[string]string{"email": "", "password": "password123"}},
		{"empty password", map[string]string{"email": "test@example.com", "password": ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := testutil.JSONRequest(t, "POST", "/api/auth/register", tt.body)
			rec := testutil.Do(router, req)
			if rec.Code != 422 {
				t.Errorf("status = %d, want 422", rec.Code)
			}
		})
	}
}

func TestLogin_Success(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	// Register a user first.
	regReq := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "login@example.com",
		"password": "password123",
		"name":     "Login User",
	})
	regRec := testutil.Do(router, regReq)
	if regRec.Code != 201 {
		t.Fatalf("register failed: %d", regRec.Code)
	}

	// Login.
	loginReq := testutil.JSONRequest(t, "POST", "/api/auth/login", map[string]string{
		"email":    "login@example.com",
		"password": "password123",
	})
	loginRec := testutil.Do(router, loginReq)

	if loginRec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", loginRec.Code, loginRec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, loginRec, &resp)

	user, ok := resp["user"].(map[string]any)
	if !ok {
		t.Fatal("missing user in response")
	}
	if user["email"] != "login@example.com" {
		t.Errorf("email = %v, want login@example.com", user["email"])
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	// Register.
	regReq := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "wrong@example.com",
		"password": "password123",
	})
	testutil.Do(router, regReq)

	// Login with wrong password.
	loginReq := testutil.JSONRequest(t, "POST", "/api/auth/login", map[string]string{
		"email":    "wrong@example.com",
		"password": "wrongpassword",
	})
	loginRec := testutil.Do(router, loginReq)

	if loginRec.Code != 401 {
		t.Errorf("status = %d, want 401", loginRec.Code)
	}
}

func TestStatus_ValidSession(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	// Register to get tokens.
	regReq := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "status@example.com",
		"password": "password123",
		"name":     "Status User",
	})
	regRec := testutil.Do(router, regReq)
	if regRec.Code != 201 {
		t.Fatalf("register failed: %d", regRec.Code)
	}

	var refreshToken string
	for _, c := range regRec.Result().Cookies() {
		if c.Name == auth.RefreshTokenCookie {
			refreshToken = c.Value
		}
	}
	if refreshToken == "" {
		t.Fatal("no refresh token after register")
	}

	// Status check.
	statusReq := httptest.NewRequest("GET", "/api/auth/", nil)
	testutil.AddRefreshToken(statusReq, refreshToken)
	statusRec := testutil.Do(router, statusReq)

	if statusRec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", statusRec.Code, statusRec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, statusRec, &resp)
	user := resp["user"].(map[string]any)
	if user["email"] != "status@example.com" {
		t.Errorf("email = %v, want status@example.com", user["email"])
	}
}

func TestStatus_NoToken(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/auth/", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogout_RevokesSession(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	// Register.
	regReq := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "logout@example.com",
		"password": "password123",
	})
	regRec := testutil.Do(router, regReq)
	if regRec.Code != 201 {
		t.Fatalf("register failed: %d", regRec.Code)
	}

	var refreshToken string
	for _, c := range regRec.Result().Cookies() {
		if c.Name == auth.RefreshTokenCookie {
			refreshToken = c.Value
		}
	}

	// Logout.
	logoutReq := httptest.NewRequest("POST", "/api/auth/logout", nil)
	if refreshToken != "" {
		logoutReq.AddCookie(&http.Cookie{Name: auth.RefreshTokenCookie, Value: refreshToken})
	}
	logoutRec := testutil.Do(router, logoutReq)
	if logoutRec.Code != 200 {
		t.Fatalf("logout failed: %d", logoutRec.Code)
	}

	// Status should now fail.
	statusReq := httptest.NewRequest("GET", "/api/auth/", nil)
	if refreshToken != "" {
		testutil.AddRefreshToken(statusReq, refreshToken)
	}
	statusRec := testutil.Do(router, statusReq)
	if statusRec.Code != 401 {
		t.Errorf("status after logout = %d, want 401", statusRec.Code)
	}
}

func TestRegister_EmailNormalization(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	// Register with mixed case.
	regReq := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "  Mixed@Example.COM  ",
		"password": "password123",
	})
	regRec := testutil.Do(router, regReq)
	if regRec.Code != 201 {
		t.Fatalf("register failed: %d", regRec.Code)
	}

	// Login with lowercase.
	loginReq := testutil.JSONRequest(t, "POST", "/api/auth/login", map[string]string{
		"email":    "mixed@example.com",
		"password": "password123",
	})
	loginRec := testutil.Do(router, loginReq)
	if loginRec.Code != 200 {
		t.Errorf("login with lowercase failed: %d", loginRec.Code)
	}
}

func TestRegister_InvalidJSON(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestRegister_InvalidEmail(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "not-an-email",
		"password": "password123",
	})
	rec := testutil.Do(router, req)

	if rec.Code != 422 {
		t.Errorf("status = %d, want 422\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader("bad json"))
	req.Header.Set("Content-Type", "application/json")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	tests := []struct {
		name string
		body map[string]string
	}{
		{"empty email", map[string]string{"email": "", "password": "password123"}},
		{"empty password", map[string]string{"email": "test@example.com", "password": ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := testutil.JSONRequest(t, "POST", "/api/auth/login", tt.body)
			rec := testutil.Do(router, req)
			if rec.Code != 422 {
				t.Errorf("status = %d, want 422", rec.Code)
			}
		})
	}
}

func TestLogin_NonExistentUser(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/auth/login", map[string]string{
		"email":    "nobody@example.com",
		"password": "password123",
	})
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_DeactivatedUser(t *testing.T) {
	t.Parallel()
	router, _, db := setup(t)

	// Register a user.
	regReq := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "deactivated@example.com",
		"password": "password123",
	})
	rec := testutil.Do(router, regReq)
	if rec.Code != 201 {
		t.Fatalf("register failed: %d", rec.Code)
	}

	// Deactivate the user.
	_, _ = db.Exec("UPDATE users SET is_active = 0 WHERE email = 'deactivated@example.com'")

	// Try to login.
	loginReq := testutil.JSONRequest(t, "POST", "/api/auth/login", map[string]string{
		"email":    "deactivated@example.com",
		"password": "password123",
	})
	loginRec := testutil.Do(router, loginReq)

	if loginRec.Code != 401 {
		t.Errorf("status = %d, want 401 for deactivated user", loginRec.Code)
	}
}

func TestLogin_DeletedUser(t *testing.T) {
	t.Parallel()
	router, _, db := setup(t)

	regReq := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "deleted@example.com",
		"password": "password123",
	})
	rec := testutil.Do(router, regReq)
	if rec.Code != 201 {
		t.Fatalf("register failed: %d", rec.Code)
	}

	_, _ = db.Exec("UPDATE users SET deleted_at = datetime('now') WHERE email = 'deleted@example.com'")

	loginReq := testutil.JSONRequest(t, "POST", "/api/auth/login", map[string]string{
		"email":    "deleted@example.com",
		"password": "password123",
	})
	loginRec := testutil.Do(router, loginReq)

	if loginRec.Code != 401 {
		t.Errorf("status = %d, want 401 for deleted user", loginRec.Code)
	}
}

func TestLogin_SetsCookies(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	regReq := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "cookies@example.com",
		"password": "password123",
	})
	testutil.Do(router, regReq)

	loginReq := testutil.JSONRequest(t, "POST", "/api/auth/login", map[string]string{
		"email":    "cookies@example.com",
		"password": "password123",
	})
	loginRec := testutil.Do(router, loginReq)

	var hasAccess, hasRefresh bool
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == auth.AccessTokenCookie {
			hasAccess = true
		}
		if c.Name == auth.RefreshTokenCookie {
			hasRefresh = true
		}
	}
	if !hasAccess {
		t.Error("missing access_token cookie after login")
	}
	if !hasRefresh {
		t.Error("missing refresh_token cookie after login")
	}
}

func TestStatus_InvalidToken(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/auth/", nil)
	testutil.AddRefreshToken(req, "invalid-token-value")
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Errorf("status = %d, want 401 for invalid token", rec.Code)
	}
}

func TestStatus_DeactivatedUser(t *testing.T) {
	t.Parallel()
	router, _, db := setup(t)

	// Register to get tokens.
	regReq := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "deact-status@example.com",
		"password": "password123",
	})
	regRec := testutil.Do(router, regReq)
	if regRec.Code != 201 {
		t.Fatalf("register failed: %d", regRec.Code)
	}

	var refreshToken string
	for _, c := range regRec.Result().Cookies() {
		if c.Name == auth.RefreshTokenCookie {
			refreshToken = c.Value
		}
	}

	// Deactivate user.
	_, _ = db.Exec("UPDATE users SET is_active = 0 WHERE email = 'deact-status@example.com'")

	// Status check should fail.
	statusReq := httptest.NewRequest("GET", "/api/auth/", nil)
	testutil.AddRefreshToken(statusReq, refreshToken)
	statusRec := testutil.Do(router, statusReq)

	if statusRec.Code != 401 {
		t.Errorf("status = %d, want 401 for deactivated user", statusRec.Code)
	}
}

func TestStatus_ExpiredToken(t *testing.T) {
	t.Parallel()
	router, _, db := setup(t)

	// Register to get tokens.
	regReq := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "expired@example.com",
		"password": "password123",
	})
	regRec := testutil.Do(router, regReq)
	if regRec.Code != 201 {
		t.Fatalf("register failed: %d", regRec.Code)
	}

	var refreshToken string
	for _, c := range regRec.Result().Cookies() {
		if c.Name == auth.RefreshTokenCookie {
			refreshToken = c.Value
		}
	}

	// Expire the token in DB.
	_, _ = db.Exec("UPDATE refresh_tokens SET expires_at = '2020-01-01T00:00:00Z' WHERE entity_type = 'user'")

	statusReq := httptest.NewRequest("GET", "/api/auth/", nil)
	testutil.AddRefreshToken(statusReq, refreshToken)
	statusRec := testutil.Do(router, statusReq)

	if statusRec.Code != 401 {
		t.Errorf("status = %d, want 401 for expired token", statusRec.Code)
	}
}

func TestStatus_RefreshesAccessToken(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	regReq := testutil.JSONRequest(t, "POST", "/api/auth/register", map[string]string{
		"email":    "refresh@example.com",
		"password": "password123",
	})
	regRec := testutil.Do(router, regReq)
	if regRec.Code != 201 {
		t.Fatalf("register failed: %d", regRec.Code)
	}

	var refreshToken string
	for _, c := range regRec.Result().Cookies() {
		if c.Name == auth.RefreshTokenCookie {
			refreshToken = c.Value
		}
	}

	statusReq := httptest.NewRequest("GET", "/api/auth/", nil)
	testutil.AddRefreshToken(statusReq, refreshToken)
	statusRec := testutil.Do(router, statusReq)

	if statusRec.Code != 200 {
		t.Fatalf("status = %d, want 200", statusRec.Code)
	}

	// Should set a new access token cookie.
	var hasAccess bool
	for _, c := range statusRec.Result().Cookies() {
		if c.Name == auth.AccessTokenCookie {
			hasAccess = true
		}
	}
	if !hasAccess {
		t.Error("status endpoint should set a fresh access_token cookie")
	}
}

func TestLogout_WithoutToken(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Errorf("status = %d, want 200 (logout without token should still succeed)", rec.Code)
	}
}
