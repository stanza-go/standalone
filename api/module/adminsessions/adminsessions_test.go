package adminsessions_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminsessions"
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
	adminsessions.Register(admin, db)

	return router, a, db
}

func insertSession(t *testing.T, db *sqlite.DB, id, entityType, entityID string, expiresAt time.Time) {
	t.Helper()
	tokenHash := auth.HashToken("test-token-" + id)
	_, err := db.Exec(
		`INSERT INTO refresh_tokens (id, entity_type, entity_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, entityType, entityID, tokenHash,
		expiresAt.UTC().Format(time.RFC3339),
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
}

func TestListSessions_Empty(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/sessions", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	sessions, ok := resp["sessions"].([]any)
	if !ok {
		t.Fatal("missing sessions in response")
	}
	// May have 0 sessions (no active refresh tokens from seed).
	_ = sessions
}

func TestListSessions_WithActiveSessions(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	insertSession(t, db, "sess-1", "admin", "1", time.Now().Add(24*time.Hour))
	insertSession(t, db, "sess-2", "user", "100", time.Now().Add(24*time.Hour))

	req := httptest.NewRequest("GET", "/api/admin/sessions", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	sessions := resp["sessions"].([]any)
	if len(sessions) < 2 {
		t.Errorf("expected at least 2 sessions, got %d", len(sessions))
	}

	// Verify session fields.
	session := sessions[0].(map[string]any)
	for _, field := range []string{"id", "entity_type", "entity_id", "created_at", "expires_at"} {
		if _, ok := session[field]; !ok {
			t.Errorf("missing field %q in session", field)
		}
	}
}

func TestListSessions_ExcludesExpired(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	// Insert an expired session.
	insertSession(t, db, "sess-expired", "admin", "1", time.Now().Add(-1*time.Hour))
	// Insert an active session.
	insertSession(t, db, "sess-active", "admin", "1", time.Now().Add(24*time.Hour))

	req := httptest.NewRequest("GET", "/api/admin/sessions", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	sessions := resp["sessions"].([]any)
	for _, s := range sessions {
		sess := s.(map[string]any)
		if sess["id"] == "sess-expired" {
			t.Error("expired session should not be listed")
		}
	}
}

func TestListSessions_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/sessions", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRevokeSession_Success(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	insertSession(t, db, "sess-to-revoke", "admin", "1", time.Now().Add(24*time.Hour))

	req := httptest.NewRequest("DELETE", "/api/admin/sessions/sess-to-revoke", nil)
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

	// Verify session is gone.
	listReq := httptest.NewRequest("GET", "/api/admin/sessions", nil)
	testutil.AddAdminAuth(t, listReq, a, "1")
	listRec := testutil.Do(router, listReq)

	var listResp map[string]any
	testutil.DecodeJSON(t, listRec, &listResp)
	sessions := listResp["sessions"].([]any)
	for _, s := range sessions {
		sess := s.(map[string]any)
		if sess["id"] == "sess-to-revoke" {
			t.Error("revoked session should not appear in list")
		}
	}
}

func TestRevokeSession_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("DELETE", "/api/admin/sessions/nonexistent", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
}
