package adminnotifications_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminnotifications"
	"github.com/stanza-go/standalone/module/notifications"
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

	withNotifications := admin.Group("")
	withNotifications.Use(auth.RequireScope("admin:notifications"))
	adminnotifications.Register(withNotifications, db)

	return router, a, db
}

func TestListNotifications_Empty(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/notifications", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	items := resp["notifications"].([]any)
	if len(items) != 0 {
		t.Fatalf("expected 0 notifications, got %d", len(items))
	}
	if resp["total"].(float64) != 0 {
		t.Fatalf("expected total=0, got %v", resp["total"])
	}
	if resp["unread"].(float64) != 0 {
		t.Fatalf("expected unread=0, got %v", resp["unread"])
	}
}

func TestListNotifications_WithData(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	// Create notifications for admin 1.
	notifications.NotifyAdmin(db, 1, "info", "Test Title", "Test message")
	notifications.NotifyAdmin(db, 1, "warning", "Warning Title", "Warning message")
	// Create notification for admin 2 — should not appear.
	notifications.NotifyAdmin(db, 2, "info", "Other admin", "Not mine")

	req := httptest.NewRequest("GET", "/api/admin/notifications", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	items := resp["notifications"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(items))
	}
	if resp["total"].(float64) != 2 {
		t.Fatalf("expected total=2, got %v", resp["total"])
	}
	if resp["unread"].(float64) != 2 {
		t.Fatalf("expected unread=2, got %v", resp["unread"])
	}

	// Most recent first.
	first := items[0].(map[string]any)
	if first["title"] != "Warning Title" {
		t.Errorf("expected newest first, got title=%q", first["title"])
	}
}

func TestListNotifications_UnreadFilter(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	id1, _ := notifications.NotifyAdmin(db, 1, "info", "Unread", "msg1")
	notifications.NotifyAdmin(db, 1, "info", "Will be read", "msg2")

	// Mark second as read.
	_ = id1
	db.Exec("UPDATE notifications SET read_at = '2026-01-01T00:00:00Z' WHERE title = 'Will be read'")

	req := httptest.NewRequest("GET", "/api/admin/notifications?unread=true", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	items := resp["notifications"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 unread notification, got %d", len(items))
	}
	if items[0].(map[string]any)["title"] != "Unread" {
		t.Errorf("expected unread notification, got %q", items[0].(map[string]any)["title"])
	}
}

func TestUnreadCount(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	notifications.NotifyAdmin(db, 1, "info", "N1", "")
	notifications.NotifyAdmin(db, 1, "info", "N2", "")
	notifications.NotifyAdmin(db, 1, "info", "N3", "")

	req := httptest.NewRequest("GET", "/api/admin/notifications/unread-count", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	if resp["unread"].(float64) != 3 {
		t.Fatalf("expected unread=3, got %v", resp["unread"])
	}
}

func TestMarkRead(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	id, _ := notifications.NotifyAdmin(db, 1, "info", "To read", "msg")

	req := httptest.NewRequest("POST", "/api/admin/notifications/"+itoa(id)+"/read", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	// Verify unread count is now 0.
	count := notifications.UnreadCount(db, notifications.EntityAdmin, 1)
	if count != 0 {
		t.Fatalf("expected 0 unread after marking read, got %d", count)
	}
}

func TestMarkRead_AlreadyRead(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	id, _ := notifications.NotifyAdmin(db, 1, "info", "Already read", "msg")
	db.Exec("UPDATE notifications SET read_at = '2026-01-01T00:00:00Z' WHERE id = ?", id)

	req := httptest.NewRequest("POST", "/api/admin/notifications/"+itoa(id)+"/read", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestMarkRead_WrongAdmin(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	// Create notification for admin 2.
	id, _ := notifications.NotifyAdmin(db, 2, "info", "Not mine", "msg")

	// Try to mark as read as admin 1.
	req := httptest.NewRequest("POST", "/api/admin/notifications/"+itoa(id)+"/read", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404 (wrong admin)\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestMarkAllRead(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	notifications.NotifyAdmin(db, 1, "info", "N1", "")
	notifications.NotifyAdmin(db, 1, "info", "N2", "")
	notifications.NotifyAdmin(db, 1, "info", "N3", "")

	req := httptest.NewRequest("POST", "/api/admin/notifications/read-all", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	if resp["marked"].(float64) != 3 {
		t.Fatalf("expected marked=3, got %v", resp["marked"])
	}

	count := notifications.UnreadCount(db, notifications.EntityAdmin, 1)
	if count != 0 {
		t.Fatalf("expected 0 unread after mark-all, got %d", count)
	}
}

func TestDeleteNotification(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	id, _ := notifications.NotifyAdmin(db, 1, "info", "To delete", "msg")

	req := httptest.NewRequest("DELETE", "/api/admin/notifications/"+itoa(id), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	// Verify it's gone.
	var count int
	db.QueryRow("SELECT COUNT(*) FROM notifications WHERE id = ?", id).Scan(&count)
	if count != 0 {
		t.Fatal("notification should be deleted")
	}
}

func TestDeleteNotification_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _ := setup(t)

	req := httptest.NewRequest("DELETE", "/api/admin/notifications/99999", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteNotification_WrongAdmin(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)

	id, _ := notifications.NotifyAdmin(db, 2, "info", "Not mine", "msg")

	req := httptest.NewRequest("DELETE", "/api/admin/notifications/"+itoa(id), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404 (wrong admin)\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestNotifications_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/notifications", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestNotifications_InsufficientScope(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)
	a := testutil.NewAdminAuth()

	// Token with only base "admin" scope — no admin:notifications.
	token, err := a.IssueAccessToken("1", []string{"admin"})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/admin/notifications", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessTokenCookie, Value: token})
	rec := testutil.Do(router, req)

	if rec.Code != 403 {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func itoa(id int64) string {
	return fmt.Sprintf("%d", id)
}
