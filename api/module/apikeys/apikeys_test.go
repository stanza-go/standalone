package apikeys_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/apikeys"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*fhttp.Router, *auth.Auth, *sqlite.DB) {
	t.Helper()

	db := testutil.SetupDB(t)
	a := testutil.NewAdminAuth()

	router := testutil.NewRouter()
	api := router.Group("/api")
	admin := api.Group("/admin")
	admin.Use(a.RequireAuth())
	admin.Use(auth.RequireScope("admin"))
	apikeys.Register(admin, db)

	return router, a, db
}

func createAPIKey(t *testing.T, router *fhttp.Router, a *auth.Auth, name string) (int, string) {
	t.Helper()
	req := testutil.JSONRequest(t, "POST", "/api/admin/api-keys", map[string]string{
		"name": name,
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)
	if rec.Code != 201 {
		t.Fatalf("create api key status = %d\nbody: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	key := resp["api_key"].(map[string]any)
	return int(key["id"].(float64)), key["key"].(string)
}

func TestListAPIKeys_Empty(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/api-keys", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	keys, ok := resp["api_keys"].([]any)
	if !ok {
		t.Fatal("missing api_keys in response")
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
	if _, ok := resp["total"]; !ok {
		t.Error("missing total in response")
	}
}

func TestListAPIKeys_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/api-keys", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestCreateAPIKey_Success(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/api-keys", map[string]string{
		"name":   "Production API",
		"scopes": "read,write",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	key := resp["api_key"].(map[string]any)
	if key["name"] != "Production API" {
		t.Errorf("name = %v, want Production API", key["name"])
	}
	if key["scopes"] != "read,write" {
		t.Errorf("scopes = %v, want read,write", key["scopes"])
	}

	// Key should be present (only on creation).
	fullKey, ok := key["key"].(string)
	if !ok || fullKey == "" {
		t.Fatal("missing key in create response")
	}
	if len(fullKey) < 13 {
		t.Errorf("key too short: %q", fullKey)
	}

	// Key prefix should match.
	prefix, ok := key["key_prefix"].(string)
	if !ok || prefix == "" {
		t.Error("missing key_prefix")
	}
	if prefix != fullKey[:13] {
		t.Errorf("key_prefix = %q, want %q", prefix, fullKey[:13])
	}
}

func TestCreateAPIKey_MissingName(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/api-keys", map[string]string{
		"name": "",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateAPIKey_WithExpiration(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	future := time.Now().Add(24 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
	req := testutil.JSONRequest(t, "POST", "/api/admin/api-keys", map[string]string{
		"name":       "Expiring Key",
		"expires_at": future,
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	key := resp["api_key"].(map[string]any)
	if key["expires_at"] != future {
		t.Errorf("expires_at = %v, want %v", key["expires_at"], future)
	}
}

func TestCreateAPIKey_PastExpiration(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	past := time.Now().Add(-24 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
	req := testutil.JSONRequest(t, "POST", "/api/admin/api-keys", map[string]string{
		"name":       "Past Key",
		"expires_at": past,
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateAPIKey_Success(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	id, _ := createAPIKey(t, router, a, "Original Name")

	req := testutil.JSONRequest(t, "PUT", fmt.Sprintf("/api/admin/api-keys/%d", id), map[string]string{
		"name":   "Updated Name",
		"scopes": "admin",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	key := resp["api_key"].(map[string]any)
	if key["name"] != "Updated Name" {
		t.Errorf("name = %v, want Updated Name", key["name"])
	}
	if key["scopes"] != "admin" {
		t.Errorf("scopes = %v, want admin", key["scopes"])
	}
}

func TestUpdateAPIKey_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "PUT", "/api/admin/api-keys/99999", map[string]string{
		"name": "Nope",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestUpdateAPIKey_RevokedKey(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	id, _ := createAPIKey(t, router, a, "To Revoke")

	// Revoke first.
	delReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/api-keys/%d", id), nil)
	testutil.AddAdminAuth(t, delReq, a, "1")
	testutil.Do(router, delReq)

	// Try to update.
	req := testutil.JSONRequest(t, "PUT", fmt.Sprintf("/api/admin/api-keys/%d", id), map[string]string{
		"name": "Should Fail",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteAPIKey_Success(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	id, _ := createAPIKey(t, router, a, "To Delete")

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/api-keys/%d", id), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	if resp["ok"] != true {
		t.Error("expected ok: true")
	}
}

func TestDeleteAPIKey_AlreadyRevoked(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	id, _ := createAPIKey(t, router, a, "Double Delete")

	// First delete.
	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/api-keys/%d", id), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	testutil.Do(router, req)

	// Second delete.
	req2 := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/api-keys/%d", id), nil)
	testutil.AddAdminAuth(t, req2, a, "1")
	rec2 := testutil.Do(router, req2)

	if rec2.Code != 404 {
		t.Fatalf("status = %d, want 404", rec2.Code)
	}
}

func TestDeleteAPIKey_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("DELETE", "/api/admin/api-keys/99999", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestListAPIKeys_Pagination(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	for i := range 3 {
		createAPIKey(t, router, a, fmt.Sprintf("Key %d", i))
	}

	req := httptest.NewRequest("GET", "/api/admin/api-keys?limit=2&offset=0", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	keys := resp["api_keys"].([]any)
	if len(keys) != 2 {
		t.Errorf("expected 2 keys with limit=2, got %d", len(keys))
	}

	total := resp["total"].(float64)
	if int(total) != 3 {
		t.Errorf("total = %v, want 3", total)
	}
}

func TestValidator_ValidKey(t *testing.T) {
	t.Parallel()
	_, _, db := setup(t)

	// Create a key directly in DB.
	fullKey := "stza_" + "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	hash := sha256.Sum256([]byte(fullKey))
	keyHash := hex.EncodeToString(hash[:])

	_, err := db.Exec(
		`INSERT INTO api_keys (name, key_prefix, key_hash, scopes, created_by, created_at) VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		"validator-test", "stza_a1b2c3d4", keyHash, "read,write", 1,
	)
	if err != nil {
		t.Fatalf("insert api key: %v", err)
	}

	validator := apikeys.NewValidator(db)
	claims, err := validator(keyHash)
	if err != nil {
		t.Fatalf("validator error: %v", err)
	}

	if claims.UID == "" {
		t.Error("expected non-empty UID")
	}
	if len(claims.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(claims.Scopes))
	}
}

func TestValidator_RevokedKey(t *testing.T) {
	t.Parallel()
	_, _, db := setup(t)

	fullKey := "stza_" + "b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3"
	hash := sha256.Sum256([]byte(fullKey))
	keyHash := hex.EncodeToString(hash[:])

	_, err := db.Exec(
		`INSERT INTO api_keys (name, key_prefix, key_hash, scopes, created_by, created_at, revoked_at) VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		"revoked-test", "stza_b2c3d4e5", keyHash, "read", 1,
	)
	if err != nil {
		t.Fatalf("insert api key: %v", err)
	}

	validator := apikeys.NewValidator(db)
	_, err = validator(keyHash)
	if err == nil {
		t.Fatal("expected error for revoked key")
	}
}

func TestValidator_ExpiredKey(t *testing.T) {
	t.Parallel()
	_, _, db := setup(t)

	fullKey := "stza_" + "c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4"
	hash := sha256.Sum256([]byte(fullKey))
	keyHash := hex.EncodeToString(hash[:])

	past := time.Now().Add(-1 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
	_, err := db.Exec(
		`INSERT INTO api_keys (name, key_prefix, key_hash, scopes, created_by, created_at, expires_at) VALUES (?, ?, ?, ?, ?, datetime('now'), ?)`,
		"expired-test", "stza_c3d4e5f6", keyHash, "read", 1, past,
	)
	if err != nil {
		t.Fatalf("insert api key: %v", err)
	}

	validator := apikeys.NewValidator(db)
	_, err = validator(keyHash)
	if err == nil {
		t.Fatal("expected error for expired key")
	}
}

func TestValidator_NotFound(t *testing.T) {
	t.Parallel()
	_, _, db := setup(t)

	validator := apikeys.NewValidator(db)
	_, err := validator("nonexistent-hash")
	if err == nil {
		t.Fatal("expected error for non-existent key")
	}
}
