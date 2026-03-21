package health_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/standalone/module/health"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) *fhttp.Router {
	t.Helper()
	db := testutil.SetupDB(t)
	router := testutil.NewRouter()
	api := router.Group("/api")
	health.Register(api, db)
	return router
}

func TestHealth_OK(t *testing.T) {
	t.Parallel()

	router := setup(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	if resp["go"] == nil {
		t.Error("missing go version")
	}
	if resp["uptime"] == nil {
		t.Error("missing uptime")
	}

	db, ok := resp["database"].(map[string]any)
	if !ok {
		t.Fatal("missing database field")
	}
	if db["ok"] != true {
		t.Errorf("database.ok = %v, want true", db["ok"])
	}
}

func TestHealth_JSONContentType(t *testing.T) {
	t.Parallel()

	router := setup(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := testutil.Do(router, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	// Verify response is valid JSON.
	var m json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Errorf("response is not valid JSON: %v", err)
	}
}

func TestHealth_ResponseFields(t *testing.T) {
	t.Parallel()
	router := setup(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	for _, field := range []string{"status", "uptime", "go", "goroutines", "memory_mb", "database"} {
		if _, ok := resp[field]; !ok {
			t.Errorf("missing field %q", field)
		}
	}

	db := resp["database"].(map[string]any)
	if _, ok := db["ok"]; !ok {
		t.Error("missing database.ok")
	}
	if _, ok := db["error"]; !ok {
		t.Error("missing database.error")
	}

	// error should be empty when healthy.
	if db["error"] != "" {
		t.Errorf("database.error = %v, want empty string", db["error"])
	}
}

func TestHealth_Degraded(t *testing.T) {
	t.Parallel()

	// Create a DB and close it to simulate failure.
	db := testutil.SetupDB(t)
	router := testutil.NewRouter()
	api := router.Group("/api")
	health.Register(api, db)

	// Close the database to make queries fail.
	db.Stop(context.Background())

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 503 {
		t.Fatalf("status = %d, want 503\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	if resp["status"] != "degraded" {
		t.Errorf("status = %v, want degraded", resp["status"])
	}

	dbField := resp["database"].(map[string]any)
	if dbField["ok"] != false {
		t.Errorf("database.ok = %v, want false", dbField["ok"])
	}
	if dbField["error"] == "" {
		t.Error("database.error should contain an error message")
	}
}
