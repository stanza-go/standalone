package admincron_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/cron"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/standalone/module/admincron"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*fhttp.Router, *auth.Auth, *cron.Scheduler) {
	t.Helper()

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
	admincron.Register(admin, s)

	return router, a, s
}

func TestListEntries(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

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
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/cron", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestTrigger_Success(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

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
	router, a, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/cron/nonexistent/trigger", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestDisable_Success(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

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
	router, a, _ := setup(t)

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
	router, a, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/cron/nonexistent/enable", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDisable_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/cron/nonexistent/disable", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
