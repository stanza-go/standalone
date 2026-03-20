package dashboard_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/cron"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/queue"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/dashboard"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*fhttp.Router, *auth.Auth, *sqlite.DB) {
	t.Helper()
	db := testutil.SetupDB(t)
	a := testutil.NewAdminAuth()
	logger := testutil.NewLogger(t)

	q := queue.New(db, queue.WithLogger(logger))
	if err := q.Start(context.Background()); err != nil {
		t.Fatalf("queue start: %v", err)
	}
	t.Cleanup(func() { q.Stop(context.Background()) })

	s := cron.NewScheduler(cron.WithLogger(logger))
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("cron start: %v", err)
	}
	t.Cleanup(func() { s.Stop(context.Background()) })

	router := testutil.NewRouter()
	api := router.Group("/api")
	admin := api.Group("/admin")
	admin.Use(a.RequireAuth())
	admin.Use(auth.RequireScope("admin"))
	dashboard.Register(admin, db, q, s)

	return router, a, db
}

func TestDashboard_Success(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/dashboard", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	// Verify top-level sections.
	sections := []string{"system", "database", "queue", "cron", "stats"}
	for _, section := range sections {
		if resp[section] == nil {
			t.Errorf("missing section: %s", section)
		}
	}

	// Verify system stats.
	system := resp["system"].(map[string]any)
	if system["go_version"] == nil {
		t.Error("missing go_version")
	}
	if system["goroutines"] == nil {
		t.Error("missing goroutines")
	}

	// Verify database stats.
	database := resp["database"].(map[string]any)
	if database["tables"] == nil {
		t.Error("missing tables count")
	}
	tables := int(database["tables"].(float64))
	if tables < 4 {
		t.Errorf("tables = %d, want >= 4 (settings, admins, refresh_tokens, users)", tables)
	}

	// Verify app stats.
	stats := resp["stats"].(map[string]any)
	totalAdmins := int(stats["total_admins"].(float64))
	if totalAdmins < 1 {
		t.Errorf("total_admins = %d, want >= 1", totalAdmins)
	}
}

func TestDashboard_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/dashboard", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestDashboard_QueueStats(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/dashboard", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	qs := resp["queue"].(map[string]any)
	// All queue counters should be present (even if 0).
	for _, key := range []string{"pending", "running", "completed", "failed", "dead", "cancelled"} {
		if qs[key] == nil {
			t.Errorf("queue missing key: %s", key)
		}
	}
}

func TestDashboard_CronStats(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/dashboard", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	cronStats := resp["cron"].(map[string]any)
	if cronStats["total"] == nil {
		t.Error("cron missing total")
	}
	if cronStats["enabled"] == nil {
		t.Error("cron missing enabled")
	}
}
