package userauth_test

import (
	"net/http"
	"net/http/httptest"
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

	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
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
			if rec.Code != 400 {
				t.Errorf("status = %d, want 400", rec.Code)
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
