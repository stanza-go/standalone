package adminaudit_test

import (
	"net/http/httptest"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminaudit"
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
	adminaudit.Register(admin, db)

	return router, a, db
}

func TestListAudit_Empty(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/audit", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	entries, ok := resp["entries"].([]any)
	if !ok {
		t.Fatal("missing entries in response")
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}

	total, _ := resp["total"].(float64)
	if total != 0 {
		t.Errorf("total = %v, want 0", total)
	}
}

func TestListAudit_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/audit", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestLog_And_List(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	// Manually insert an audit entry via the Log helper.
	// We need a request with auth context for the Log function.
	logReq := httptest.NewRequest("POST", "/api/admin/test", nil)
	testutil.AddAdminAuth(t, logReq, a, "1")

	// Simulate the auth middleware setting claims on context.
	token, err := a.IssueAccessToken("1", []string{"admin"})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	claims, err := a.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	ctx := auth.WithClaimsForTest(logReq.Context(), claims)
	logReq = logReq.WithContext(ctx)

	adminaudit.Log(db, logReq, "user.create", "user", "42", "test@example.com")
	adminaudit.Log(db, logReq, "admin.delete", "admin", "5", "")
	adminaudit.Log(db, logReq, "setting.update", "setting", "app_name", "My App")

	// List all entries.
	listReq := httptest.NewRequest("GET", "/api/admin/audit", nil)
	testutil.AddAdminAuth(t, listReq, a, "1")
	rec := testutil.Do(router, listReq)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	entries := resp["entries"].([]any)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	total, _ := resp["total"].(float64)
	if total != 3 {
		t.Errorf("total = %v, want 3", total)
	}

	// Entries should be newest first.
	first := entries[0].(map[string]any)
	if first["action"] != "setting.update" {
		t.Errorf("first entry action = %v, want setting.update", first["action"])
	}
	if first["entity_type"] != "setting" {
		t.Errorf("entity_type = %v, want setting", first["entity_type"])
	}
	if first["details"] != "My App" {
		t.Errorf("details = %v, want My App", first["details"])
	}
}

func TestListAudit_FilterByAction(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	// Insert entries.
	token, _ := a.IssueAccessToken("1", []string{"admin"})
	claims, _ := a.ValidateAccessToken(token)
	logReq := httptest.NewRequest("POST", "/test", nil)
	logReq = logReq.WithContext(auth.WithClaimsForTest(logReq.Context(), claims))

	adminaudit.Log(db, logReq, "user.create", "user", "1", "a@a.com")
	adminaudit.Log(db, logReq, "user.delete", "user", "2", "")
	adminaudit.Log(db, logReq, "admin.create", "admin", "3", "b@b.com")

	// Filter by user.create.
	req := httptest.NewRequest("GET", "/api/admin/audit?action=user.create", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	entries := resp["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	total, _ := resp["total"].(float64)
	if total != 1 {
		t.Errorf("total = %v, want 1", total)
	}
}

func TestListAudit_Search(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	token, _ := a.IssueAccessToken("1", []string{"admin"})
	claims, _ := a.ValidateAccessToken(token)
	logReq := httptest.NewRequest("POST", "/test", nil)
	logReq = logReq.WithContext(auth.WithClaimsForTest(logReq.Context(), claims))

	adminaudit.Log(db, logReq, "user.create", "user", "1", "alice@example.com")
	adminaudit.Log(db, logReq, "user.create", "user", "2", "bob@example.com")

	// Search for alice.
	req := httptest.NewRequest("GET", "/api/admin/audit?search=alice", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	entries := resp["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestListAudit_Pagination(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	token, _ := a.IssueAccessToken("1", []string{"admin"})
	claims, _ := a.ValidateAccessToken(token)
	logReq := httptest.NewRequest("POST", "/test", nil)
	logReq = logReq.WithContext(auth.WithClaimsForTest(logReq.Context(), claims))

	// Insert 5 entries.
	for i := 0; i < 5; i++ {
		adminaudit.Log(db, logReq, "user.create", "user", "1", "")
	}

	// Page 1: limit=2, offset=0.
	req := httptest.NewRequest("GET", "/api/admin/audit?limit=2&offset=0", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	entries := resp["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	total, _ := resp["total"].(float64)
	if total != 5 {
		t.Errorf("total = %v, want 5", total)
	}

	// Page 3: limit=2, offset=4 — should get 1 entry.
	req = httptest.NewRequest("GET", "/api/admin/audit?limit=2&offset=4", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec = testutil.Do(router, req)
	testutil.DecodeJSON(t, rec, &resp)

	entries = resp["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry on last page, got %d", len(entries))
	}
}

func TestRecentAudit(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	token, _ := a.IssueAccessToken("1", []string{"admin"})
	claims, _ := a.ValidateAccessToken(token)
	logReq := httptest.NewRequest("POST", "/test", nil)
	logReq = logReq.WithContext(auth.WithClaimsForTest(logReq.Context(), claims))

	// Insert 15 entries — recent should return only 10.
	for i := 0; i < 15; i++ {
		adminaudit.Log(db, logReq, "user.create", "user", "1", "")
	}

	req := httptest.NewRequest("GET", "/api/admin/audit/recent", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	entries := resp["entries"].([]any)
	if len(entries) != 10 {
		t.Fatalf("expected 10 entries (max), got %d", len(entries))
	}
}
