package usernotifications_test

import (
	"fmt"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/notifications"
	"github.com/stanza-go/standalone/module/usernotifications"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*fhttp.Router, *auth.Auth, *sqlite.DB) {
	t.Helper()

	db := testutil.SetupDB(t)
	a := testutil.NewUserAuth()

	router := testutil.NewRouter()
	api := router.Group("/api")
	user := api.Group("/user")
	user.Use(a.RequireAuth())
	user.Use(auth.RequireScope("user"))
	usernotifications.Register(user, db)

	return router, a, db
}

// createTestUser inserts a user row and returns its ID.
func createTestUser(t *testing.T, db *sqlite.DB) int64 {
	t.Helper()
	seq := testSeq.Add(1)
	result, err := db.Exec(
		"INSERT INTO users (email, password, name) VALUES (?, ?, ?)",
		fmt.Sprintf("testuser_%d@example.com", seq),
		"$2a$10$fakehashfakehashfakehashfakehashfakehashfakehashfak",
		"Test User",
	)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return result.LastInsertID
}

var testSeq atomic.Int64

func TestUserListNotifications_Empty(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)
	uid := createTestUser(t, db)

	req := httptest.NewRequest("GET", "/api/user/notifications", nil)
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	items := resp["notifications"].([]any)
	if len(items) != 0 {
		t.Fatalf("expected 0, got %d", len(items))
	}
}

func TestUserListNotifications_WithData(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)
	uid := createTestUser(t, db)

	notifications.NotifyUser(db, uid, "welcome", "Welcome!", "Welcome to the app")
	notifications.NotifyUser(db, uid, "info", "Update", "Your profile was updated")

	// Other user's notification — should not appear.
	otherUID := createTestUser(t, db)
	notifications.NotifyUser(db, otherUID, "info", "Not mine", "nope")

	req := httptest.NewRequest("GET", "/api/user/notifications", nil)
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	items := resp["notifications"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2, got %d", len(items))
	}
	if resp["total"].(float64) != 2 {
		t.Fatalf("expected total=2, got %v", resp["total"])
	}
}

func TestUserUnreadCount(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)
	uid := createTestUser(t, db)

	notifications.NotifyUser(db, uid, "info", "N1", "")
	notifications.NotifyUser(db, uid, "info", "N2", "")

	req := httptest.NewRequest("GET", "/api/user/notifications/unread-count", nil)
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	if resp["unread"].(float64) != 2 {
		t.Fatalf("expected unread=2, got %v", resp["unread"])
	}
}

func TestUserMarkRead(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)
	uid := createTestUser(t, db)

	id, _ := notifications.NotifyUser(db, uid, "info", "To read", "msg")

	req := httptest.NewRequest("POST", "/api/user/notifications/"+itoa(id)+"/read", nil)
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	count := notifications.UnreadCount(db, notifications.EntityUser, uid)
	if count != 0 {
		t.Fatalf("expected 0 unread, got %d", count)
	}
}

func TestUserMarkAllRead(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)
	uid := createTestUser(t, db)

	notifications.NotifyUser(db, uid, "info", "N1", "")
	notifications.NotifyUser(db, uid, "info", "N2", "")

	req := httptest.NewRequest("POST", "/api/user/notifications/read-all", nil)
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	if resp["marked"].(float64) != 2 {
		t.Fatalf("expected marked=2, got %v", resp["marked"])
	}
}

func TestUserDeleteNotification(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)
	uid := createTestUser(t, db)

	id, _ := notifications.NotifyUser(db, uid, "info", "To delete", "msg")

	req := httptest.NewRequest("DELETE", "/api/user/notifications/"+itoa(id), nil)
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM notifications WHERE id = ?", id).Scan(&count)
	if count != 0 {
		t.Fatal("notification should be deleted")
	}
}

func TestUserDeleteNotification_WrongUser(t *testing.T) {
	t.Parallel()
	router, a, db := setup(t)
	uid := createTestUser(t, db)
	otherUID := createTestUser(t, db)

	id, _ := notifications.NotifyUser(db, otherUID, "info", "Not mine", "msg")

	req := httptest.NewRequest("DELETE", "/api/user/notifications/"+itoa(id), nil)
	testutil.AddUserAuth(t, req, a, itoa(uid))
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestUserNotifications_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/user/notifications", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func itoa(id int64) string {
	return fmt.Sprintf("%d", id)
}
