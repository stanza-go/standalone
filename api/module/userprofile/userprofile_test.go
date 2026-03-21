package userprofile_test

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/userprofile"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*fhttp.Router, *auth.Auth, *sqlite.DB) {
	t.Helper()

	db := testutil.SetupDB(t)
	a := testutil.NewUserAuth()
	logger := testutil.NewLogger(t)

	router := testutil.NewRouter()
	api := router.Group("/api")
	user := api.Group("/user")
	user.Use(a.RequireAuth())
	user.Use(auth.RequireScope("user"))
	userprofile.Register(user, db, logger)

	return router, a, db
}

func createTestUser(t *testing.T, db *sqlite.DB, email, password, name string) int64 {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	result, err := db.Exec(
		`INSERT INTO users (email, password, name, is_active, created_at, updated_at) VALUES (?, ?, ?, 1, datetime('now'), datetime('now'))`,
		email, hash, name,
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return result.LastInsertID
}

func TestGetProfile_Success(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	uid := createTestUser(t, db, "profile@example.com", "password123", "Profile User")

	req := httptest.NewRequest("GET", "/api/user/profile", nil)
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	user := resp["user"].(map[string]any)
	if user["email"] != "profile@example.com" {
		t.Errorf("email = %v, want profile@example.com", user["email"])
	}
	if user["name"] != "Profile User" {
		t.Errorf("name = %v, want Profile User", user["name"])
	}
	for _, field := range []string{"id", "email", "name", "created_at", "updated_at"} {
		if _, ok := user[field]; !ok {
			t.Errorf("missing field %q in user", field)
		}
	}
}

func TestGetProfile_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/user/profile", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestGetProfile_DeletedUser(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	uid := createTestUser(t, db, "deleted@example.com", "password123", "Deleted")
	db.Exec(`UPDATE users SET deleted_at = datetime('now') WHERE id = ?`, uid)

	req := httptest.NewRequest("GET", "/api/user/profile", nil)
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404 (deleted user)", rec.Code)
	}
}

func TestGetProfile_InactiveUser(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	uid := createTestUser(t, db, "inactive@example.com", "password123", "Inactive")
	db.Exec(`UPDATE users SET is_active = 0 WHERE id = ?`, uid)

	req := httptest.NewRequest("GET", "/api/user/profile", nil)
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404 (inactive user)", rec.Code)
	}
}

func TestUpdateProfile_Name(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	uid := createTestUser(t, db, "update@example.com", "password123", "Old Name")

	req := testutil.JSONRequest(t, "PUT", "/api/user/profile", map[string]string{
		"name": "New Name",
	})
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	user := resp["user"].(map[string]any)
	if user["name"] != "New Name" {
		t.Errorf("name = %v, want New Name", user["name"])
	}
}

func TestUpdateProfile_Email(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	uid := createTestUser(t, db, "oldemail@example.com", "password123", "User")

	req := testutil.JSONRequest(t, "PUT", "/api/user/profile", map[string]string{
		"email": "NewEmail@Example.COM",
	})
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	user := resp["user"].(map[string]any)
	// Email should be lowercased.
	if user["email"] != "newemail@example.com" {
		t.Errorf("email = %v, want newemail@example.com", user["email"])
	}
}

func TestUpdateProfile_DuplicateEmail(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	createTestUser(t, db, "existing@example.com", "password123", "Existing")
	uid := createTestUser(t, db, "me@example.com", "password123", "Me")

	req := testutil.JSONRequest(t, "PUT", "/api/user/profile", map[string]string{
		"email": "existing@example.com",
	})
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateProfile_EmptyFields(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	uid := createTestUser(t, db, "empty@example.com", "password123", "User")

	req := testutil.JSONRequest(t, "PUT", "/api/user/profile", map[string]string{})
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestChangePassword_Success(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	uid := createTestUser(t, db, "passwd@example.com", "oldpassword1", "User")

	req := testutil.JSONRequest(t, "PUT", "/api/user/profile/password", map[string]string{
		"current_password": "oldpassword1",
		"new_password":     "newpassword1",
	})
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	if resp["status"] != "password updated" {
		t.Errorf("status = %v, want 'password updated'", resp["status"])
	}

	// Verify new password works by trying to change again.
	req2 := testutil.JSONRequest(t, "PUT", "/api/user/profile/password", map[string]string{
		"current_password": "newpassword1",
		"new_password":     "anotherpass1",
	})
	testutil.AddUserAuth(t, req2, a, itoa(uid))
	rec2 := testutil.Do(router, req2)
	if rec2.Code != 200 {
		t.Errorf("new password should work, got status %d", rec2.Code)
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	uid := createTestUser(t, db, "wrongpwd@example.com", "correctpassword", "User")

	req := testutil.JSONRequest(t, "PUT", "/api/user/profile/password", map[string]string{
		"current_password": "wrongpassword",
		"new_password":     "newpassword1",
	})
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestChangePassword_TooShort(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	uid := createTestUser(t, db, "short@example.com", "password123", "User")

	req := testutil.JSONRequest(t, "PUT", "/api/user/profile/password", map[string]string{
		"current_password": "password123",
		"new_password":     "short",
	})
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestChangePassword_MissingFields(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	uid := createTestUser(t, db, "missing@example.com", "password123", "User")

	tests := []struct {
		name string
		body map[string]string
	}{
		{"missing current", map[string]string{"new_password": "newpass12"}},
		{"missing new", map[string]string{"current_password": "password123"}},
		{"both empty", map[string]string{"current_password": "", "new_password": ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := testutil.JSONRequest(t, "PUT", "/api/user/profile/password", tt.body)
			testutil.AddUserAuth(t, req, a, itoa(uid))
			rec := testutil.Do(router, req)
			if rec.Code != 400 {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}
