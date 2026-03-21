package adminroles_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminroles"
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
	adminroles.Register(admin, db)

	return router, a, db
}

func TestListRoles(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/roles/", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	roles, ok := resp["roles"].([]any)
	if !ok {
		t.Fatal("missing roles in response")
	}

	// Seed creates 3 system roles.
	if len(roles) < 3 {
		t.Fatalf("expected at least 3 roles, got %d", len(roles))
	}

	// Verify superadmin role has expected scopes.
	superadmin := roles[0].(map[string]any)
	if superadmin["name"] != "superadmin" {
		t.Errorf("first role name = %q, want superadmin", superadmin["name"])
	}
	if superadmin["is_system"] != true {
		t.Error("superadmin should be a system role")
	}
	scopes, ok := superadmin["scopes"].([]any)
	if !ok || len(scopes) < 5 {
		t.Errorf("superadmin should have at least 5 scopes, got %d", len(scopes))
	}
}

func TestListRoles_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/roles/", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestListRoles_InsufficientScope(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)
	a := testutil.NewAdminAuth()

	// Token with only base "admin" scope — no admin:roles.
	token, err := a.IssueAccessToken("1", []string{"admin"})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/admin/roles/", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessTokenCookie, Value: token})
	rec := testutil.Do(router, req)

	if rec.Code != 403 {
		t.Fatalf("status = %d, want 403\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateRole(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/roles/", map[string]any{
		"name":        "editor",
		"description": "Content editor role",
		"scopes":      []string{"admin", "admin:users"},
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	role := resp["role"].(map[string]any)

	if role["name"] != "editor" {
		t.Errorf("name = %q, want editor", role["name"])
	}
	scopes := role["scopes"].([]any)
	if len(scopes) != 2 {
		t.Errorf("scopes count = %d, want 2", len(scopes))
	}
}

func TestCreateRole_DuplicateName(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	// "superadmin" already exists as a system role.
	req := testutil.JSONRequest(t, "POST", "/api/admin/roles/", map[string]any{
		"name":   "superadmin",
		"scopes": []string{"admin"},
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateRole_UnknownScope(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/roles/", map[string]any{
		"name":   "bad-role",
		"scopes": []string{"admin", "admin:nonexistent"},
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateRole_BaseScope(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	// Create role without "admin" scope — should be auto-added.
	req := testutil.JSONRequest(t, "POST", "/api/admin/roles/", map[string]any{
		"name":   "testrole",
		"scopes": []string{"admin:logs"},
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	role := resp["role"].(map[string]any)
	scopes := role["scopes"].([]any)

	hasAdmin := false
	for _, s := range scopes {
		if s == "admin" {
			hasAdmin = true
		}
	}
	if !hasAdmin {
		t.Error("base 'admin' scope should be auto-added")
	}
}

func TestUpdateRole(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	// Create a custom role first.
	req := testutil.JSONRequest(t, "POST", "/api/admin/roles/", map[string]any{
		"name":   "custom",
		"scopes": []string{"admin"},
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)
	if rec.Code != 201 {
		t.Fatalf("create: status = %d\nbody: %s", rec.Code, rec.Body.String())
	}

	var createResp map[string]any
	testutil.DecodeJSON(t, rec, &createResp)
	roleID := createResp["role"].(map[string]any)["id"].(float64)

	// Update it.
	req = testutil.JSONRequest(t, "PUT", fmt.Sprintf("/api/admin/roles/%d", int(roleID)), map[string]any{
		"name":        "customized",
		"description": "Updated description",
		"scopes":      []string{"admin", "admin:logs", "admin:audit"},
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec = testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("update: status = %d\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	role := resp["role"].(map[string]any)

	if role["name"] != "customized" {
		t.Errorf("name = %q, want customized", role["name"])
	}
	if role["description"] != "Updated description" {
		t.Errorf("description = %q, want 'Updated description'", role["description"])
	}
	scopes := role["scopes"].([]any)
	if len(scopes) != 3 {
		t.Errorf("scopes count = %d, want 3", len(scopes))
	}
}

func TestUpdateSystemRole_CannotRename(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	// Try to rename system role "superadmin" (id=1).
	req := testutil.JSONRequest(t, "PUT", "/api/admin/roles/1", map[string]any{
		"name": "mega-admin",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d\nbody: %s", rec.Code, rec.Body.String())
	}

	// Name should remain "superadmin" despite request.
	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	role := resp["role"].(map[string]any)
	if role["name"] != "superadmin" {
		t.Errorf("system role was renamed to %q", role["name"])
	}
}

func TestUpdateSystemRole_CanChangeScopes(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	// System roles can have their scopes modified.
	req := testutil.JSONRequest(t, "PUT", "/api/admin/roles/3", map[string]any{
		"scopes": []string{"admin", "admin:logs"},
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	scopes := resp["role"].(map[string]any)["scopes"].([]any)
	if len(scopes) != 2 {
		t.Errorf("scopes count = %d, want 2", len(scopes))
	}
}

func TestDeleteRole(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	// Create then delete.
	req := testutil.JSONRequest(t, "POST", "/api/admin/roles/", map[string]any{
		"name":   "deleteme",
		"scopes": []string{"admin"},
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)
	if rec.Code != 201 {
		t.Fatalf("create: status = %d", rec.Code)
	}

	var createResp map[string]any
	testutil.DecodeJSON(t, rec, &createResp)
	roleID := int(createResp["role"].(map[string]any)["id"].(float64))

	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/roles/%d", roleID), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec = testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("delete: status = %d\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteSystemRole_Forbidden(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("DELETE", "/api/admin/roles/1", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteRole_InUse(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	// Create a role.
	req := testutil.JSONRequest(t, "POST", "/api/admin/roles/", map[string]any{
		"name":   "inuse",
		"scopes": []string{"admin"},
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)
	if rec.Code != 201 {
		t.Fatalf("create role: status = %d", rec.Code)
	}

	var createResp map[string]any
	testutil.DecodeJSON(t, rec, &createResp)
	roleID := int(createResp["role"].(map[string]any)["id"].(float64))

	// Assign the role to an admin.
	_, err := db.Exec("UPDATE admins SET role = 'inuse' WHERE id = 1")
	if err != nil {
		t.Fatalf("assign role: %v", err)
	}

	// Try to delete — should fail.
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/roles/%d", roleID), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec = testutil.Do(router, req)

	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestScopesEndpoint(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/roles/scopes", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	scopes := resp["scopes"].([]any)
	if len(scopes) < 5 {
		t.Errorf("expected at least 5 scopes, got %d", len(scopes))
	}
}

func TestRoleNamesEndpoint(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/role-names", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	roles := resp["roles"].([]any)
	if len(roles) < 3 {
		t.Errorf("expected at least 3 role names, got %d", len(roles))
	}
}

func TestValidateRoleExists(t *testing.T) {
	t.Parallel()
	db := testutil.SetupDB(t)

	if !adminroles.ValidateRoleExists(db, "superadmin") {
		t.Error("superadmin should exist")
	}
	if !adminroles.ValidateRoleExists(db, "admin") {
		t.Error("admin should exist")
	}
	if adminroles.ValidateRoleExists(db, "nonexistent") {
		t.Error("nonexistent should not exist")
	}
}

func TestCreateRole_InvalidJSON(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/roles/", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateRole_MissingName(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/roles/", map[string]any{
		"scopes": []string{"admin"},
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateRole_NameTooShort(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/roles/", map[string]any{
		"name":   "x",
		"scopes": []string{"admin"},
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateRole_InvalidID(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "PUT", "/api/admin/roles/notanumber", map[string]any{
		"name": "test",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateRole_InvalidJSON(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("PUT", "/api/admin/roles/1", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateRole_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "PUT", "/api/admin/roles/99999", map[string]any{
		"name": "ghost",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateRole_DuplicateName(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	// Create a custom role.
	req := testutil.JSONRequest(t, "POST", "/api/admin/roles/", map[string]any{
		"name":   "uniquerole",
		"scopes": []string{"admin"},
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)
	if rec.Code != 201 {
		t.Fatalf("create: status = %d\nbody: %s", rec.Code, rec.Body.String())
	}

	var createResp map[string]any
	testutil.DecodeJSON(t, rec, &createResp)
	roleID := int(createResp["role"].(map[string]any)["id"].(float64))

	// Try to rename it to "admin" (already exists as a system role).
	req = testutil.JSONRequest(t, "PUT", fmt.Sprintf("/api/admin/roles/%d", roleID), map[string]any{
		"name": "admin",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec = testutil.Do(router, req)

	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteRole_InvalidID(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("DELETE", "/api/admin/roles/notanumber", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteRole_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("DELETE", "/api/admin/roles/99999", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateRole_DescriptionOnly(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	// Update only the description of a system role (name stays the same).
	req := testutil.JSONRequest(t, "PUT", "/api/admin/roles/1", map[string]any{
		"description": "Updated superadmin description",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	role := resp["role"].(map[string]any)
	if role["description"] != "Updated superadmin description" {
		t.Errorf("description = %q, want 'Updated superadmin description'", role["description"])
	}
	if role["name"] != "superadmin" {
		t.Errorf("name changed to %q, should stay superadmin", role["name"])
	}
}

func TestUpdateRole_UnknownScope(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "PUT", "/api/admin/roles/1", map[string]any{
		"scopes": []string{"admin", "admin:nonexistent"},
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestListRoles_ResponseStructure(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/roles/", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	roles := resp["roles"].([]any)

	// Verify each role has all expected fields.
	for _, r := range roles {
		role := r.(map[string]any)
		for _, field := range []string{"id", "name", "description", "is_system", "scopes", "admin_count", "created_at", "updated_at"} {
			if _, ok := role[field]; !ok {
				t.Errorf("role %v missing field %q", role["name"], field)
			}
		}
	}

	// Verify admin_count for superadmin is at least 1 (seed admin).
	superadmin := roles[0].(map[string]any)
	if superadmin["admin_count"].(float64) < 1 {
		t.Errorf("superadmin admin_count = %v, want >= 1", superadmin["admin_count"])
	}
}
