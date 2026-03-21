package admindb_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/admindb"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*fhttp.Router, *auth.Auth, *sqlite.DB, string) {
	t.Helper()

	db := testutil.SetupDB(t)
	a := testutil.NewAdminAuth()
	backupsDir := filepath.Join(t.TempDir(), "backups")
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		t.Fatalf("create backups dir: %v", err)
	}

	router := testutil.NewRouter()
	api := router.Group("/api")
	admin := api.Group("/admin")
	admin.Use(a.RequireAuth())
	admin.Use(auth.RequireScope("admin"))
	admindb.Register(admin, db, backupsDir)

	return router, a, db, backupsDir
}

func TestInfo_Success(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/database", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	// Verify top-level keys.
	for _, key := range []string{"files", "pragmas", "tables", "migrations", "backups"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("missing key %q in response", key)
		}
	}

	// Verify files section.
	files := resp["files"].(map[string]any)
	if files["path"] == nil || files["path"] == "" {
		t.Error("missing db path")
	}
	if files["db_size_bytes"] == nil {
		t.Error("missing db_size_bytes")
	}

	// Verify pragmas.
	pragmas := resp["pragmas"].(map[string]any)
	if pragmas["journal_mode"] == nil {
		t.Error("missing journal_mode")
	}

	// Verify tables exist (at least settings, admins, users, refresh_tokens from migrations).
	tables := resp["tables"].([]any)
	if len(tables) < 4 {
		t.Errorf("expected at least 4 tables, got %d", len(tables))
	}

	// Verify table structure.
	table := tables[0].(map[string]any)
	if _, ok := table["name"]; !ok {
		t.Error("missing table name")
	}
	if _, ok := table["row_count"]; !ok {
		t.Error("missing table row_count")
	}

	// Verify migrations.
	migrations := resp["migrations"].([]any)
	if len(migrations) == 0 {
		t.Error("expected at least 1 migration")
	}

	migration := migrations[0].(map[string]any)
	for _, field := range []string{"version", "name", "applied_at"} {
		if _, ok := migration[field]; !ok {
			t.Errorf("missing migration field %q", field)
		}
	}
}

func TestInfo_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/database", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestBackup_Success(t *testing.T) {
	t.Parallel()
	router, a, _, backupsDir := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/database/backup", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	if resp["name"] == nil || resp["name"] == "" {
		t.Error("missing backup name")
	}
	if resp["size_bytes"] == nil {
		t.Error("missing size_bytes")
	}

	// Verify backup file exists on disk.
	name := resp["name"].(string)
	if _, err := os.Stat(filepath.Join(backupsDir, name)); err != nil {
		t.Errorf("backup file not found: %v", err)
	}
}

func TestBackup_AppearsInInfo(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	// Create a backup.
	backupReq := httptest.NewRequest("POST", "/api/admin/database/backup", nil)
	testutil.AddAdminAuth(t, backupReq, a, "1")
	backupRec := testutil.Do(router, backupReq)
	if backupRec.Code != 200 {
		t.Fatalf("backup status = %d", backupRec.Code)
	}

	// Verify it appears in info.
	infoReq := httptest.NewRequest("GET", "/api/admin/database", nil)
	testutil.AddAdminAuth(t, infoReq, a, "1")
	infoRec := testutil.Do(router, infoReq)

	var resp map[string]any
	testutil.DecodeJSON(t, infoRec, &resp)

	backups := resp["backups"].([]any)
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}

	backup := backups[0].(map[string]any)
	for _, field := range []string{"name", "size_bytes", "created_at"} {
		if _, ok := backup[field]; !ok {
			t.Errorf("missing backup field %q", field)
		}
	}
}
