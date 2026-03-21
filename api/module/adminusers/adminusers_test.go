package adminusers_test

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminusers"
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
	adminusers.Register(admin, db, nil)
	return router, a, db
}

func TestListAdmins(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/admins", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	admins, ok := resp["admins"].([]any)
	if !ok {
		t.Fatal("missing admins in response")
	}
	if len(admins) == 0 {
		t.Error("expected at least 1 admin (seeded)")
	}

	total, ok := resp["total"].(float64)
	if !ok || total < 1 {
		t.Errorf("total = %v, want >= 1", resp["total"])
	}
}

func TestListAdmins_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/admins", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestCreateAdmin(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/admins", map[string]string{
		"email":    "new@stanza.dev",
		"password": "newpassword",
		"name":     "New Admin",
		"role":     "admin",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	admin, ok := resp["admin"].(map[string]any)
	if !ok {
		t.Fatal("missing admin in response")
	}
	if admin["email"] != "new@stanza.dev" {
		t.Errorf("email = %v, want new@stanza.dev", admin["email"])
	}
	if admin["name"] != "New Admin" {
		t.Errorf("name = %v, want New Admin", admin["name"])
	}
	if admin["role"] != "admin" {
		t.Errorf("role = %v, want admin", admin["role"])
	}
}

func TestCreateAdmin_DuplicateEmail(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/admins", map[string]string{
		"email":    "admin@stanza.dev",
		"password": "password",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 409 {
		t.Errorf("status = %d, want 409 (duplicate)", rec.Code)
	}
}

func TestCreateAdmin_MissingFields(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/admins", map[string]string{
		"email": "test@stanza.dev",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 422 {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestCreateAdmin_InvalidRole(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/admins", map[string]string{
		"email":    "role@stanza.dev",
		"password": "password123",
		"role":     "dictator",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 422 {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestUpdateAdmin(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	// Create an admin first.
	createReq := testutil.JSONRequest(t, "POST", "/api/admin/admins", map[string]string{
		"email":    "update@stanza.dev",
		"password": "password",
		"name":     "Original",
	})
	testutil.AddAdminAuth(t, createReq, a, "1")
	createRec := testutil.Do(router, createReq)
	if createRec.Code != 201 {
		t.Fatalf("create status = %d", createRec.Code)
	}

	var createResp map[string]any
	testutil.DecodeJSON(t, createRec, &createResp)
	admin := createResp["admin"].(map[string]any)
	id := int(admin["id"].(float64))

	// Update the admin.
	updateReq := testutil.JSONRequest(t, "PUT", fmt.Sprintf("/api/admin/admins/%d", id), map[string]string{
		"name": "Updated",
		"role": "viewer",
	})
	testutil.AddAdminAuth(t, updateReq, a, "1")
	updateRec := testutil.Do(router, updateReq)

	if updateRec.Code != 200 {
		t.Fatalf("update status = %d\nbody: %s", updateRec.Code, updateRec.Body.String())
	}

	var updateResp map[string]any
	testutil.DecodeJSON(t, updateRec, &updateResp)
	updated := updateResp["admin"].(map[string]any)
	if updated["name"] != "Updated" {
		t.Errorf("name = %v, want Updated", updated["name"])
	}
	if updated["role"] != "viewer" {
		t.Errorf("role = %v, want viewer", updated["role"])
	}
}

func TestUpdateAdmin_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "PUT", "/api/admin/admins/9999", map[string]string{
		"name": "Ghost",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteAdmin(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	// Create an admin to delete.
	createReq := testutil.JSONRequest(t, "POST", "/api/admin/admins", map[string]string{
		"email":    "delete@stanza.dev",
		"password": "password",
	})
	testutil.AddAdminAuth(t, createReq, a, "1")
	createRec := testutil.Do(router, createReq)
	if createRec.Code != 201 {
		t.Fatalf("create status = %d", createRec.Code)
	}

	var createResp map[string]any
	testutil.DecodeJSON(t, createRec, &createResp)
	admin := createResp["admin"].(map[string]any)
	id := int(admin["id"].(float64))

	// Delete the admin.
	deleteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/admins/%d", id), nil)
	testutil.AddAdminAuth(t, deleteReq, a, "1")
	deleteRec := testutil.Do(router, deleteReq)

	if deleteRec.Code != 200 {
		t.Fatalf("delete status = %d\nbody: %s", deleteRec.Code, deleteRec.Body.String())
	}

	// List should no longer include the deleted admin.
	listReq := httptest.NewRequest("GET", "/api/admin/admins", nil)
	testutil.AddAdminAuth(t, listReq, a, "1")
	listRec := testutil.Do(router, listReq)

	var listResp map[string]any
	testutil.DecodeJSON(t, listRec, &listResp)
	admins := listResp["admins"].([]any)
	for _, adm := range admins {
		a := adm.(map[string]any)
		if a["email"] == "delete@stanza.dev" {
			t.Error("deleted admin still appears in list")
		}
	}
}

func TestDeleteAdmin_SelfDeletion(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	// Admin 1 tries to delete itself.
	req := httptest.NewRequest("DELETE", "/api/admin/admins/1", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Errorf("status = %d, want 400 (self-deletion)", rec.Code)
	}
}

func TestDeleteAdmin_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("DELETE", "/api/admin/admins/9999", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestListAdmins_Pagination(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	// Create 3 more admins.
	for i := range 3 {
		req := testutil.JSONRequest(t, "POST", "/api/admin/admins", map[string]string{
			"email":    fmt.Sprintf("page%d@stanza.dev", i),
			"password": "password",
		})
		testutil.AddAdminAuth(t, req, a, "1")
		rec := testutil.Do(router, req)
		if rec.Code != 201 {
			t.Fatalf("create admin %d failed: %d", i, rec.Code)
		}
	}

	// Fetch page with limit 2.
	req := httptest.NewRequest("GET", "/api/admin/admins?limit=2&offset=0", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	admins := resp["admins"].([]any)
	if len(admins) != 2 {
		t.Errorf("got %d admins, want 2", len(admins))
	}

	total := int(resp["total"].(float64))
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
}

func TestCreateAdmin_InvalidJSON(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/admins", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateAdmin_ShortPassword(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/admins", map[string]string{
		"email":    "short@stanza.dev",
		"password": "123",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 422 {
		t.Errorf("status = %d, want 422 for short password", rec.Code)
	}
}

func TestCreateAdmin_DefaultRole(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/admins", map[string]string{
		"email":    "default-role@stanza.dev",
		"password": "password123",
		"name":     "Default Role Admin",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	admin := resp["admin"].(map[string]any)
	if admin["role"] != "admin" {
		t.Errorf("role = %v, want admin (default)", admin["role"])
	}
}

func TestCreateAdmin_AuditLog(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/admins", map[string]string{
		"email":    "audit-create@stanza.dev",
		"password": "password123",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	var action, entityType string
	err := db.QueryRow("SELECT action, entity_type FROM audit_log WHERE action = 'admin.create'").
		Scan(&action, &entityType)
	if err != nil {
		t.Fatalf("audit log query: %v", err)
	}
	if entityType != "admin" {
		t.Errorf("entity_type = %q, want admin", entityType)
	}
}

func TestUpdateAdmin_InvalidJSON(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("PUT", "/api/admin/admins/1", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestUpdateAdmin_InvalidID(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "PUT", "/api/admin/admins/abc", map[string]string{
		"name": "Test",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Errorf("status = %d, want 400 for invalid ID", rec.Code)
	}
}

func TestDeleteAdmin_InvalidID(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("DELETE", "/api/admin/admins/abc", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Errorf("status = %d, want 400 for invalid ID", rec.Code)
	}
}

func TestUpdateAdmin_SelfDeactivation(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	isActive := false
	req := testutil.JSONRequest(t, "PUT", "/api/admin/admins/1", map[string]any{
		"is_active": isActive,
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Errorf("status = %d, want 400 for self-deactivation", rec.Code)
	}
}

func TestUpdateAdmin_InvalidRole(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	// Create an admin first.
	createReq := testutil.JSONRequest(t, "POST", "/api/admin/admins", map[string]string{
		"email":    "invalid-role-update@stanza.dev",
		"password": "password123",
	})
	testutil.AddAdminAuth(t, createReq, a, "1")
	createRec := testutil.Do(router, createReq)
	if createRec.Code != 201 {
		t.Fatalf("create status = %d", createRec.Code)
	}

	var createResp map[string]any
	testutil.DecodeJSON(t, createRec, &createResp)
	id := int(createResp["admin"].(map[string]any)["id"].(float64))

	req := testutil.JSONRequest(t, "PUT", fmt.Sprintf("/api/admin/admins/%d", id), map[string]string{
		"role": "nonexistent-role",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Errorf("status = %d, want 400 for invalid role", rec.Code)
	}
}

func TestDeleteAdmin_AuditLog(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	createReq := testutil.JSONRequest(t, "POST", "/api/admin/admins", map[string]string{
		"email":    "audit-delete@stanza.dev",
		"password": "password123",
	})
	testutil.AddAdminAuth(t, createReq, a, "1")
	createRec := testutil.Do(router, createReq)
	if createRec.Code != 201 {
		t.Fatalf("create status = %d", createRec.Code)
	}

	var createResp map[string]any
	testutil.DecodeJSON(t, createRec, &createResp)
	id := int(createResp["admin"].(map[string]any)["id"].(float64))

	deleteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/admins/%d", id), nil)
	testutil.AddAdminAuth(t, deleteReq, a, "1")
	deleteRec := testutil.Do(router, deleteReq)

	if deleteRec.Code != 200 {
		t.Fatalf("delete status = %d", deleteRec.Code)
	}

	var action, entityType string
	err := db.QueryRow("SELECT action, entity_type FROM audit_log WHERE action = 'admin.delete'").
		Scan(&action, &entityType)
	if err != nil {
		t.Fatalf("audit log query: %v", err)
	}
	if entityType != "admin" {
		t.Errorf("entity_type = %q, want admin", entityType)
	}
}

func TestDeleteAdmin_RevokesRefreshTokens(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	// Create an admin.
	createReq := testutil.JSONRequest(t, "POST", "/api/admin/admins", map[string]string{
		"email":    "revoke-sessions@stanza.dev",
		"password": "password123",
	})
	testutil.AddAdminAuth(t, createReq, a, "1")
	createRec := testutil.Do(router, createReq)
	if createRec.Code != 201 {
		t.Fatalf("create status = %d", createRec.Code)
	}

	var createResp map[string]any
	testutil.DecodeJSON(t, createRec, &createResp)
	id := int(createResp["admin"].(map[string]any)["id"].(float64))
	idStr := fmt.Sprintf("%d", id)

	// Insert a fake refresh token for this admin.
	_, err := db.Exec(
		`INSERT INTO refresh_tokens (id, entity_type, entity_id, token_hash, expires_at) VALUES (?, 'admin', ?, 'fakehash', datetime('now', '+1 day'))`,
		"fake-token-id", idStr,
	)
	if err != nil {
		t.Fatalf("insert refresh token: %v", err)
	}

	// Delete the admin.
	deleteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/admins/%d", id), nil)
	testutil.AddAdminAuth(t, deleteReq, a, "1")
	testutil.Do(router, deleteReq)

	// Verify refresh token was deleted.
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM refresh_tokens WHERE entity_type = 'admin' AND entity_id = ?", idStr).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 refresh tokens after delete, got %d", count)
	}
}

func TestListAdmins_AdminStructure(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/admins", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	admins := resp["admins"].([]any)
	if len(admins) == 0 {
		t.Fatal("expected at least 1 admin")
	}

	admin := admins[0].(map[string]any)
	for _, field := range []string{"id", "email", "name", "role", "is_active", "created_at", "updated_at"} {
		if _, ok := admin[field]; !ok {
			t.Errorf("missing field %q in admin", field)
		}
	}
}
