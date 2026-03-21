package adminsettings_test

import (
	"net/http/httptest"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminsettings"
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
	adminsettings.Register(admin, db)

	return router, a, db
}

func TestListSettings(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/settings", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	settings, ok := resp["settings"].([]any)
	if !ok {
		t.Fatal("missing settings in response")
	}

	// Seed should have created some settings.
	if len(settings) == 0 {
		t.Fatal("expected at least 1 setting from seed")
	}

	// Verify setting structure.
	setting := settings[0].(map[string]any)
	for _, field := range []string{"key", "value", "group_name", "updated_at"} {
		if _, ok := setting[field]; !ok {
			t.Errorf("missing field %q in setting", field)
		}
	}
}

func TestListSettings_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/settings", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestUpdateSetting_Success(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	// Insert a known setting.
	_, err := db.Exec(
		`INSERT INTO settings (key, value, group_name, updated_at) VALUES (?, ?, ?, ?)`,
		"test_key", "old_value", "general", "2026-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert setting: %v", err)
	}

	req := testutil.JSONRequest(t, "PUT", "/api/admin/settings/test_key", map[string]string{
		"value": "new_value",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	if resp["key"] != "test_key" {
		t.Errorf("key = %v, want test_key", resp["key"])
	}
	if resp["value"] != "new_value" {
		t.Errorf("value = %v, want new_value", resp["value"])
	}
	if resp["group_name"] != "general" {
		t.Errorf("group_name = %v, want general", resp["group_name"])
	}
}

func TestUpdateSetting_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := testutil.JSONRequest(t, "PUT", "/api/admin/settings/nonexistent_key", map[string]string{
		"value": "some_value",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateSetting_EmptyValue(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	// Insert a known setting.
	_, err := db.Exec(
		`INSERT INTO settings (key, value, group_name, updated_at) VALUES (?, ?, ?, ?)`,
		"empty_test", "has_value", "general", "2026-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert setting: %v", err)
	}

	// Updating to empty string should succeed.
	req := testutil.JSONRequest(t, "PUT", "/api/admin/settings/empty_test", map[string]string{
		"value": "",
	})
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	if resp["value"] != "" {
		t.Errorf("value = %v, want empty string", resp["value"])
	}
}

func TestUpdateSetting_VerifyPersistence(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	_, err := db.Exec(
		`INSERT INTO settings (key, value, group_name, updated_at) VALUES (?, ?, ?, ?)`,
		"persist_test", "original", "general", "2026-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert setting: %v", err)
	}

	// Update.
	updateReq := testutil.JSONRequest(t, "PUT", "/api/admin/settings/persist_test", map[string]string{
		"value": "updated",
	})
	testutil.AddAdminAuth(t, updateReq, a, "1")
	testutil.Do(router, updateReq)

	// Verify via list.
	listReq := httptest.NewRequest("GET", "/api/admin/settings", nil)
	testutil.AddAdminAuth(t, listReq, a, "1")
	listRec := testutil.Do(router, listReq)

	var resp map[string]any
	testutil.DecodeJSON(t, listRec, &resp)

	settings := resp["settings"].([]any)
	found := false
	for _, s := range settings {
		setting := s.(map[string]any)
		if setting["key"] == "persist_test" {
			found = true
			if setting["value"] != "updated" {
				t.Errorf("value = %v, want updated", setting["value"])
			}
		}
	}
	if !found {
		t.Error("persist_test setting not found in list")
	}
}
