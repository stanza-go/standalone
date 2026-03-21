package admincron_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/cron"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/admincron"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*fhttp.Router, *auth.Auth, *cron.Scheduler, *sqlite.DB) {
	t.Helper()

	db := testutil.SetupDB(t)
	a := testutil.NewAdminAuth()
	logger := testutil.NewLogger(t)

	s := cron.NewScheduler(cron.WithLogger(logger))
	if err := s.Add("test-job", "*/5 * * * *", func(_ context.Context) error {
		return nil
	}); err != nil {
		t.Fatalf("add cron job: %v", err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("cron start: %v", err)
	}
	t.Cleanup(func() { s.Stop(context.Background()) })

	router := testutil.NewRouter()
	api := router.Group("/api")
	admin := api.Group("/admin")
	admin.Use(a.RequireAuth())
	admin.Use(auth.RequireScope("admin"))
	admincron.Register(admin, s, db)

	return router, a, s, db
}

func TestListEntries(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/cron", nil)
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
	if len(entries) == 0 {
		t.Fatal("expected at least 1 cron entry")
	}

	entry := entries[0].(map[string]any)
	if entry["name"] != "test-job" {
		t.Errorf("name = %v, want test-job", entry["name"])
	}
	if entry["schedule"] != "*/5 * * * *" {
		t.Errorf("schedule = %v, want */5 * * * *", entry["schedule"])
	}
	if entry["enabled"] != true {
		t.Errorf("enabled = %v, want true", entry["enabled"])
	}
}

func TestListEntries_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/cron", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestTrigger_Success(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/cron/test-job/trigger", nil)
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

func TestTrigger_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/cron/nonexistent/trigger", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestDisable_Success(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/cron/test-job/disable", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	// Verify disabled.
	listReq := httptest.NewRequest("GET", "/api/admin/cron", nil)
	testutil.AddAdminAuth(t, listReq, a, "1")
	listRec := testutil.Do(router, listReq)

	var resp map[string]any
	testutil.DecodeJSON(t, listRec, &resp)
	entries := resp["entries"].([]any)
	entry := entries[0].(map[string]any)
	if entry["enabled"] != false {
		t.Error("expected enabled = false after disable")
	}
}

func TestEnable_Success(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	// Disable first.
	disableReq := httptest.NewRequest("POST", "/api/admin/cron/test-job/disable", nil)
	testutil.AddAdminAuth(t, disableReq, a, "1")
	testutil.Do(router, disableReq)

	// Enable.
	req := httptest.NewRequest("POST", "/api/admin/cron/test-job/enable", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	// Verify enabled.
	listReq := httptest.NewRequest("GET", "/api/admin/cron", nil)
	testutil.AddAdminAuth(t, listReq, a, "1")
	listRec := testutil.Do(router, listReq)

	var resp map[string]any
	testutil.DecodeJSON(t, listRec, &resp)
	entries := resp["entries"].([]any)
	entry := entries[0].(map[string]any)
	if entry["enabled"] != true {
		t.Error("expected enabled = true after enable")
	}
}

func TestEnable_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/cron/nonexistent/enable", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDisable_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/cron/nonexistent/disable", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestTrigger_AuditLog(t *testing.T) {
	t.Parallel()
	router, a, _, db := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/cron/test-job/trigger", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var action, entityType, entityID string
	err := db.QueryRow("SELECT action, entity_type, entity_id FROM audit_log WHERE action = 'cron.trigger'").
		Scan(&action, &entityType, &entityID)
	if err != nil {
		t.Fatalf("audit log query: %v", err)
	}
	if entityType != "cron" {
		t.Errorf("entity_type = %q, want cron", entityType)
	}
	if entityID != "test-job" {
		t.Errorf("entity_id = %q, want test-job", entityID)
	}
}

func TestDisable_AuditLog(t *testing.T) {
	t.Parallel()
	router, a, _, db := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/cron/test-job/disable", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var action, entityType, entityID string
	err := db.QueryRow("SELECT action, entity_type, entity_id FROM audit_log WHERE action = 'cron.disable'").
		Scan(&action, &entityType, &entityID)
	if err != nil {
		t.Fatalf("audit log query: %v", err)
	}
	if entityType != "cron" {
		t.Errorf("entity_type = %q, want cron", entityType)
	}
	if entityID != "test-job" {
		t.Errorf("entity_id = %q, want test-job", entityID)
	}
}

func TestRuns_Empty(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/cron/test-job/runs", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	runs, ok := resp["runs"].([]any)
	if !ok {
		t.Fatal("missing runs in response")
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}
	if resp["total"] != float64(0) {
		t.Errorf("total = %v, want 0", resp["total"])
	}
}

func TestRuns_WithData(t *testing.T) {
	t.Parallel()
	router, a, _, db := setup(t)

	// Insert test run records.
	for i := 0; i < 3; i++ {
		_, err := db.Exec(
			"INSERT INTO cron_runs (name, started_at, duration_ms, status, error) VALUES (?, ?, ?, ?, ?)",
			"test-job", "2026-03-21T10:00:00Z", 150+i, "success", "",
		)
		if err != nil {
			t.Fatalf("insert cron_run: %v", err)
		}
	}
	// Insert a failed run.
	_, err := db.Exec(
		"INSERT INTO cron_runs (name, started_at, duration_ms, status, error) VALUES (?, ?, ?, ?, ?)",
		"test-job", "2026-03-21T09:00:00Z", 5000, "error", "something broke",
	)
	if err != nil {
		t.Fatalf("insert cron_run: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/admin/cron/test-job/runs", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	runs := resp["runs"].([]any)
	if len(runs) != 4 {
		t.Fatalf("expected 4 runs, got %d", len(runs))
	}
	if resp["total"] != float64(4) {
		t.Errorf("total = %v, want 4", resp["total"])
	}

	// First run should be most recent (ordered by started_at DESC).
	first := runs[0].(map[string]any)
	if first["status"] != "success" {
		t.Errorf("first run status = %v, want success", first["status"])
	}

	// Last run should be the failed one (oldest).
	last := runs[3].(map[string]any)
	if last["status"] != "error" {
		t.Errorf("last run status = %v, want error", last["status"])
	}
	if last["error"] != "something broke" {
		t.Errorf("last run error = %v, want 'something broke'", last["error"])
	}
}

func TestRuns_Pagination(t *testing.T) {
	t.Parallel()
	router, a, _, db := setup(t)

	// Insert 5 runs.
	for i := 0; i < 5; i++ {
		_, err := db.Exec(
			"INSERT INTO cron_runs (name, started_at, duration_ms, status, error) VALUES (?, ?, ?, ?, ?)",
			"test-job", "2026-03-21T10:00:00Z", 100, "success", "",
		)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	req := httptest.NewRequest("GET", "/api/admin/cron/test-job/runs?limit=2&offset=0", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	runs := resp["runs"].([]any)
	if len(runs) != 2 {
		t.Errorf("expected 2 runs with limit=2, got %d", len(runs))
	}
	if resp["total"] != float64(5) {
		t.Errorf("total = %v, want 5", resp["total"])
	}
}

func TestRuns_FiltersByName(t *testing.T) {
	t.Parallel()
	router, a, _, db := setup(t)

	// Insert runs for different jobs.
	_, _ = db.Exec(
		"INSERT INTO cron_runs (name, started_at, duration_ms, status, error) VALUES (?, ?, ?, ?, ?)",
		"test-job", "2026-03-21T10:00:00Z", 100, "success", "",
	)
	_, _ = db.Exec(
		"INSERT INTO cron_runs (name, started_at, duration_ms, status, error) VALUES (?, ?, ?, ?, ?)",
		"other-job", "2026-03-21T10:00:00Z", 200, "success", "",
	)

	req := httptest.NewRequest("GET", "/api/admin/cron/test-job/runs", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	runs := resp["runs"].([]any)
	if len(runs) != 1 {
		t.Errorf("expected 1 run for test-job, got %d", len(runs))
	}
	if resp["total"] != float64(1) {
		t.Errorf("total = %v, want 1", resp["total"])
	}
}

func TestRuns_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/cron/test-job/runs", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestEnable_AuditLog(t *testing.T) {
	t.Parallel()
	router, a, _, db := setup(t)

	// Disable first so enable has something to do.
	disableReq := httptest.NewRequest("POST", "/api/admin/cron/test-job/disable", nil)
	testutil.AddAdminAuth(t, disableReq, a, "1")
	testutil.Do(router, disableReq)

	req := httptest.NewRequest("POST", "/api/admin/cron/test-job/enable", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var action, entityType, entityID string
	err := db.QueryRow("SELECT action, entity_type, entity_id FROM audit_log WHERE action = 'cron.enable'").
		Scan(&action, &entityType, &entityID)
	if err != nil {
		t.Fatalf("audit log query: %v", err)
	}
	if entityType != "cron" {
		t.Errorf("entity_type = %q, want cron", entityType)
	}
	if entityID != "test-job" {
		t.Errorf("entity_id = %q, want test-job", entityID)
	}
}
