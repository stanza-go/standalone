package adminlogs_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/standalone/module/adminlogs"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*fhttp.Router, *auth.Auth, string) {
	t.Helper()

	a := testutil.NewAdminAuth()
	logsDir := t.TempDir()

	router := testutil.NewRouter()
	api := router.Group("/api")
	admin := api.Group("/admin")
	admin.Use(a.RequireAuth())
	admin.Use(auth.RequireScope("admin"))
	adminlogs.Register(admin, logsDir)

	return router, a, logsDir
}

func writeLogFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write log file: %v", err)
	}
}

func TestListFiles_Empty(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/logs/files", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	files, ok := resp["files"]
	if !ok {
		t.Fatal("missing files in response")
	}
	if files != nil {
		if arr, ok := files.([]any); ok && len(arr) != 0 {
			t.Errorf("expected 0 files, got %d", len(arr))
		}
	}
}

func TestListFiles_WithLogFiles(t *testing.T) {
	t.Parallel()
	router, a, logsDir := setup(t)

	writeLogFile(t, logsDir, "stanza.log", `{"level":"info","msg":"test"}`)
	writeLogFile(t, logsDir, "stanza-2026-03-20.log", `{"level":"info","msg":"old"}`)
	writeLogFile(t, logsDir, "other.txt", "not a log file")

	req := httptest.NewRequest("GET", "/api/admin/logs/files", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	files := resp["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("expected 2 stanza log files, got %d", len(files))
	}

	// stanza.log should be first.
	first := files[0].(map[string]any)
	if first["name"] != "stanza.log" {
		t.Errorf("first file = %v, want stanza.log", first["name"])
	}
}

func TestListFiles_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/logs/files", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestEntries_DefaultFile(t *testing.T) {
	t.Parallel()
	router, a, logsDir := setup(t)

	writeLogFile(t, logsDir, "stanza.log",
		`{"level":"info","msg":"line1"}`+"\n"+
			`{"level":"error","msg":"line2"}`+"\n"+
			`{"level":"info","msg":"line3"}`+"\n")

	req := httptest.NewRequest("GET", "/api/admin/logs", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	if resp["file"] != "stanza.log" {
		t.Errorf("file = %v, want stanza.log", resp["file"])
	}

	entries := resp["entries"].([]any)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Newest first.
	first := entries[0].(map[string]any)
	if first["msg"] != "line3" {
		t.Errorf("first entry msg = %v, want line3 (newest first)", first["msg"])
	}
}

func TestEntries_FilterByLevel(t *testing.T) {
	t.Parallel()
	router, a, logsDir := setup(t)

	writeLogFile(t, logsDir, "stanza.log",
		`{"level":"info","msg":"info line"}`+"\n"+
			`{"level":"error","msg":"error line"}`+"\n")

	req := httptest.NewRequest("GET", "/api/admin/logs?level=error", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	entries := resp["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 error entry, got %d", len(entries))
	}
	if entries[0].(map[string]any)["msg"] != "error line" {
		t.Error("expected the error line")
	}
}

func TestEntries_Search(t *testing.T) {
	t.Parallel()
	router, a, logsDir := setup(t)

	writeLogFile(t, logsDir, "stanza.log",
		`{"level":"info","msg":"user login"}`+"\n"+
			`{"level":"info","msg":"database query"}`+"\n")

	req := httptest.NewRequest("GET", "/api/admin/logs?search=login", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	entries := resp["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 matching entry, got %d", len(entries))
	}
}

func TestEntries_Limit(t *testing.T) {
	t.Parallel()
	router, a, logsDir := setup(t)

	writeLogFile(t, logsDir, "stanza.log",
		`{"level":"info","msg":"a"}`+"\n"+
			`{"level":"info","msg":"b"}`+"\n"+
			`{"level":"info","msg":"c"}`+"\n")

	req := httptest.NewRequest("GET", "/api/admin/logs?limit=2", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	entries := resp["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries with limit=2, got %d", len(entries))
	}
}

func TestEntries_MissingFile(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/logs", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200 (graceful empty)", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	entries := resp["entries"].([]any)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for missing file, got %d", len(entries))
	}
}

func TestEntries_InvalidFile(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	tests := []struct {
		name string
		file string
	}{
		{"path traversal", "../etc/passwd"},
		{"non-stanza prefix", "other.log"},
		{"non-log suffix", "stanza.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", "/api/admin/logs?file="+tt.file, nil)
			testutil.AddAdminAuth(t, req, a, "1")
			rec := testutil.Do(router, req)

			if rec.Code != 400 {
				t.Errorf("file=%q: status = %d, want 400", tt.file, rec.Code)
			}
		})
	}
}
