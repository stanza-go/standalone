package adminusers_test

import (
	"fmt"
	"net/http/httptest"
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
	adminusers.Register(admin, db)
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
