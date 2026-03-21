package usermgmt_test

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/usermgmt"
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
	usermgmt.Register(admin, a, db, nil)

	return router, a, db
}

func createUser(t *testing.T, router *fhttp.Router, a *auth.Auth, email, password, name string) int {
	t.Helper()
	req := testutil.JSONRequest(t, "POST", "/api/admin/users", map[string]string{
		"email":    email,
		"password": password,
		"name":     name,
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)
	if rec.Code != 201 {
		t.Fatalf("create user status = %d\nbody: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	user := resp["user"].(map[string]any)
	return int(user["id"].(float64))
}

func TestListUsers(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/users", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	if _, ok := resp["users"]; !ok {
		t.Fatal("missing users in response")
	}
	if _, ok := resp["total"]; !ok {
		t.Fatal("missing total in response")
	}
}

func TestListUsers_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/users", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestCreateUser(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "POST", "/api/admin/users", map[string]string{
		"email":    "new@example.com",
		"password": "password123",
		"name":     "New User",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	user := resp["user"].(map[string]any)
	if user["email"] != "new@example.com" {
		t.Errorf("email = %v, want new@example.com", user["email"])
	}
	if user["name"] != "New User" {
		t.Errorf("name = %v, want New User", user["name"])
	}
	if user["is_active"] != true {
		t.Errorf("is_active = %v, want true", user["is_active"])
	}
}

func TestCreateUser_MissingFields(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	tests := []struct {
		name string
		body map[string]string
	}{
		{"missing email", map[string]string{"password": "pass123"}},
		{"missing password", map[string]string{"email": "test@example.com"}},
		{"both empty", map[string]string{"email": "", "password": ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := testutil.JSONRequest(t, "POST", "/api/admin/users", tt.body)
			testutil.AddAdminAuth(t, req, a, "1")
			rec := testutil.Do(router, req)
			if rec.Code != 422 {
				t.Errorf("status = %d, want 422", rec.Code)
			}
		})
	}
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	createUser(t, router, a, "dup@example.com", "password123", "First")

	req := testutil.JSONRequest(t, "POST", "/api/admin/users", map[string]string{
		"email":    "dup@example.com",
		"password": "password123",
		"name":     "Second",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestGetUser(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	id := createUser(t, router, a, "get@example.com", "password123", "Get User")

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/admin/users/%d", id), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	user := resp["user"].(map[string]any)
	if user["email"] != "get@example.com" {
		t.Errorf("email = %v, want get@example.com", user["email"])
	}

	// active_sessions should be present.
	if _, ok := resp["active_sessions"]; !ok {
		t.Error("missing active_sessions in response")
	}
}

func TestGetUser_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/users/99999", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestUpdateUser(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	id := createUser(t, router, a, "update@example.com", "password123", "Original")

	req := testutil.JSONRequest(t, "PUT", fmt.Sprintf("/api/admin/users/%d", id), map[string]string{
		"name": "Updated Name",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	user := resp["user"].(map[string]any)
	if user["name"] != "Updated Name" {
		t.Errorf("name = %v, want Updated Name", user["name"])
	}
}

func TestUpdateUser_Deactivate(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	id := createUser(t, router, a, "deactivate@example.com", "password123", "Deactivate")

	req := testutil.JSONRequest(t, "PUT", fmt.Sprintf("/api/admin/users/%d", id), map[string]any{
		"is_active": false,
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	user := resp["user"].(map[string]any)
	if user["is_active"] != false {
		t.Errorf("is_active = %v, want false", user["is_active"])
	}
}

func TestUpdateUser_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "PUT", "/api/admin/users/99999", map[string]string{
		"name": "Nobody",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteUser(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	id := createUser(t, router, a, "delete@example.com", "password123", "Delete Me")

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/users/%d", id), nil)
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

	// Verify soft-deleted: GET should return 404.
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/admin/users/%d", id), nil)
	testutil.AddAdminAuth(t, getReq, a, "1")
	getRec := testutil.Do(router, getReq)
	if getRec.Code != 404 {
		t.Errorf("get after delete: status = %d, want 404", getRec.Code)
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("DELETE", "/api/admin/users/99999", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestImpersonate_Success(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	id := createUser(t, router, a, "impersonate@example.com", "password123", "Target")

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/admin/users/%d/impersonate", id), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	if resp["token"] == nil || resp["token"] == "" {
		t.Error("missing token in impersonate response")
	}

	user := resp["user"].(map[string]any)
	if user["email"] != "impersonate@example.com" {
		t.Errorf("email = %v, want impersonate@example.com", user["email"])
	}
}

func TestImpersonate_InactiveUser(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	id := createUser(t, router, a, "inactive@example.com", "password123", "Inactive")

	// Deactivate the user.
	deactivateReq := testutil.JSONRequest(t, "PUT", fmt.Sprintf("/api/admin/users/%d", id), map[string]any{
		"is_active": false,
	})
	testutil.AddAdminAuth(t, deactivateReq, a, "1")
	testutil.Do(router, deactivateReq)

	// Impersonate should fail.
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/admin/users/%d/impersonate", id), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestImpersonate_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/users/99999/impersonate", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestListUsers_Search(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	createUser(t, router, a, "alice@example.com", "password123", "Alice Smith")
	createUser(t, router, a, "bob@example.com", "password123", "Bob Jones")

	req := httptest.NewRequest("GET", "/api/admin/users?search=alice", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	users := resp["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("expected 1 user matching 'alice', got %d", len(users))
	}
}

func TestListUsers_Pagination(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	for i := range 3 {
		createUser(t, router, a, fmt.Sprintf("page%d@example.com", i), "password123", fmt.Sprintf("User %d", i))
	}

	req := httptest.NewRequest("GET", "/api/admin/users?limit=2&offset=0", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	users := resp["users"].([]any)
	if len(users) != 2 {
		t.Errorf("expected 2 users with limit=2, got %d", len(users))
	}

	total := resp["total"].(float64)
	if int(total) < 3 {
		t.Errorf("total = %v, want >= 3", total)
	}
}
