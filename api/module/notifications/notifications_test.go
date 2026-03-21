package notifications_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stanza-go/framework/pkg/email"
	"github.com/stanza-go/standalone/module/notifications"
	"github.com/stanza-go/standalone/testutil"
)

func TestNotify(t *testing.T) {
	t.Parallel()
	db := testutil.SetupDB(t)

	id, err := notifications.Notify(db, "admin", 1, "info", "Test", "Test message", `{"key":"val"}`)
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	// Verify row.
	var title, data string
	err = db.QueryRow("SELECT title, data FROM notifications WHERE id = ?", id).Scan(&title, &data)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if title != "Test" {
		t.Errorf("title = %q, want Test", title)
	}
	if data != `{"key":"val"}` {
		t.Errorf("data = %q, want {\"key\":\"val\"}", data)
	}
}

func TestNotifyAdmin(t *testing.T) {
	t.Parallel()
	db := testutil.SetupDB(t)

	id, err := notifications.NotifyAdmin(db, 1, "alert", "Admin Alert", "Something happened")
	if err != nil {
		t.Fatalf("NotifyAdmin: %v", err)
	}
	if id <= 0 {
		t.Fatal("expected positive ID")
	}

	var entityType string
	var entityID int64
	err = db.QueryRow("SELECT entity_type, entity_id FROM notifications WHERE id = ?", id).Scan(&entityType, &entityID)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if entityType != "admin" {
		t.Errorf("entity_type = %q, want admin", entityType)
	}
	if entityID != 1 {
		t.Errorf("entity_id = %d, want 1", entityID)
	}
}

func TestNotifyUser(t *testing.T) {
	t.Parallel()
	db := testutil.SetupDB(t)

	id, err := notifications.NotifyUser(db, 42, "welcome", "Welcome!", "msg")
	if err != nil {
		t.Fatalf("NotifyUser: %v", err)
	}

	var entityType string
	var entityID int64
	err = db.QueryRow("SELECT entity_type, entity_id FROM notifications WHERE id = ?", id).Scan(&entityType, &entityID)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if entityType != "user" {
		t.Errorf("entity_type = %q, want user", entityType)
	}
	if entityID != 42 {
		t.Errorf("entity_id = %d, want 42", entityID)
	}
}

func TestNotifyAllAdmins(t *testing.T) {
	t.Parallel()
	db := testutil.SetupDB(t)

	// Seed has 1 admin (admin@stanza.dev). Insert another.
	_, err := db.Exec("INSERT INTO admins (email, password, name, role) VALUES (?, ?, ?, ?)",
		"admin2@stanza.dev", "$2a$10$fakehash", "Admin 2", "admin")
	if err != nil {
		t.Fatalf("insert admin: %v", err)
	}

	err = notifications.NotifyAllAdmins(db, "system", "System Update", "Maintenance scheduled")
	if err != nil {
		t.Fatalf("NotifyAllAdmins: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE type = 'system'").Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 notifications (one per admin), got %d", count)
	}
}

func TestUnreadCount(t *testing.T) {
	t.Parallel()
	db := testutil.SetupDB(t)

	notifications.NotifyAdmin(db, 1, "info", "N1", "")
	notifications.NotifyAdmin(db, 1, "info", "N2", "")
	id3, _ := notifications.NotifyAdmin(db, 1, "info", "N3", "")

	count := notifications.UnreadCount(db, notifications.EntityAdmin, 1)
	if count != 3 {
		t.Fatalf("expected 3 unread, got %d", count)
	}

	// Mark one as read.
	_, _ = db.Exec("UPDATE notifications SET read_at = '2026-01-01T00:00:00Z' WHERE id = ?", id3)

	count = notifications.UnreadCount(db, notifications.EntityAdmin, 1)
	if count != 2 {
		t.Fatalf("expected 2 unread after marking one, got %d", count)
	}
}

// --- Service tests ---

// mockResend starts an httptest server that captures email requests and
// returns a Resend-compatible success response.
func mockResend(t *testing.T, calls *atomic.Int32) (*httptest.Server, *email.Client) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"id": "test-msg-id"})
	}))
	t.Cleanup(srv.Close)
	client := email.New("test-api-key",
		email.WithFrom("test@stanza.dev"),
		email.WithEndpoint(srv.URL),
	)
	return srv, client
}

func TestServiceNotifyAdmin_NoEmail(t *testing.T) {
	t.Parallel()
	db := testutil.SetupDB(t)
	svc := notifications.NewService(db, nil, nil)

	id, err := svc.NotifyAdmin(1, "info", "Service Test", "no email")
	if err != nil {
		t.Fatalf("NotifyAdmin: %v", err)
	}
	if id <= 0 {
		t.Fatal("expected positive ID")
	}

	var title string
	err = db.QueryRow("SELECT title FROM notifications WHERE id = ?", id).Scan(&title)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if title != "Service Test" {
		t.Errorf("title = %q, want Service Test", title)
	}
}

func TestServiceNotifyAdmin_WithEmail(t *testing.T) {
	t.Parallel()
	db := testutil.SetupDB(t)

	var calls atomic.Int32
	_, client := mockResend(t, &calls)
	logger := testutil.NewLogger(t)
	svc := notifications.NewService(db, client, logger)

	id, err := svc.NotifyAdmin(1, "warning", "Deploy Alert", "Deployment started",
		notifications.WithEmail(context.Background()),
	)
	if err != nil {
		t.Fatalf("NotifyAdmin: %v", err)
	}
	if id <= 0 {
		t.Fatal("expected positive ID")
	}

	// Notification created in DB.
	var title string
	err = db.QueryRow("SELECT title FROM notifications WHERE id = ?", id).Scan(&title)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if title != "Deploy Alert" {
		t.Errorf("title = %q, want Deploy Alert", title)
	}

	// Email was sent (seed admin is admin@stanza.dev with ID 1).
	if calls.Load() != 1 {
		t.Errorf("expected 1 email call, got %d", calls.Load())
	}
}

func TestServiceNotifyUser_WithEmail(t *testing.T) {
	t.Parallel()
	db := testutil.SetupDB(t)

	// Insert a user to look up email from.
	_, err := db.Exec("INSERT INTO users (email, password, name) VALUES (?, ?, ?)",
		"user@example.com", "$2a$10$fakehash", "Test User")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// Get the user ID.
	var userID int64
	err = db.QueryRow("SELECT id FROM users WHERE email = 'user@example.com'").Scan(&userID)
	if err != nil {
		t.Fatalf("get user id: %v", err)
	}

	var calls atomic.Int32
	_, client := mockResend(t, &calls)
	logger := testutil.NewLogger(t)
	svc := notifications.NewService(db, client, logger)

	id, err := svc.NotifyUser(userID, "welcome", "Welcome!", "Welcome to Stanza",
		notifications.WithEmail(context.Background()),
	)
	if err != nil {
		t.Fatalf("NotifyUser: %v", err)
	}
	if id <= 0 {
		t.Fatal("expected positive ID")
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 email call, got %d", calls.Load())
	}
}

func TestServiceNotifyAllAdmins_WithEmail(t *testing.T) {
	t.Parallel()
	db := testutil.SetupDB(t)

	// Seed has 1 admin. Add another.
	_, err := db.Exec("INSERT INTO admins (email, password, name, role) VALUES (?, ?, ?, ?)",
		"admin2@stanza.dev", "$2a$10$fakehash", "Admin 2", "admin")
	if err != nil {
		t.Fatalf("insert admin: %v", err)
	}

	var calls atomic.Int32
	_, client := mockResend(t, &calls)
	logger := testutil.NewLogger(t)
	svc := notifications.NewService(db, client, logger)

	err = svc.NotifyAllAdmins("system", "System Alert", "Maintenance scheduled",
		notifications.WithEmail(context.Background()),
	)
	if err != nil {
		t.Fatalf("NotifyAllAdmins: %v", err)
	}

	// 2 notifications created.
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE type = 'system'").Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 notifications, got %d", count)
	}

	// 2 emails sent.
	if calls.Load() != 2 {
		t.Errorf("expected 2 email calls, got %d", calls.Load())
	}
}

func TestServiceNotifyAdmin_EmailFailure_StillCreatesNotification(t *testing.T) {
	t.Parallel()
	db := testutil.SetupDB(t)

	// Mock server that always returns 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	t.Cleanup(srv.Close)
	client := email.New("test-api-key",
		email.WithFrom("test@stanza.dev"),
		email.WithEndpoint(srv.URL),
	)
	logger := testutil.NewLogger(t)
	svc := notifications.NewService(db, client, logger)

	id, err := svc.NotifyAdmin(1, "error", "Failure Test", "Email will fail",
		notifications.WithEmail(context.Background()),
	)
	if err != nil {
		t.Fatalf("NotifyAdmin should succeed even when email fails: %v", err)
	}
	if id <= 0 {
		t.Fatal("expected positive ID")
	}

	// Notification still exists.
	var title string
	err = db.QueryRow("SELECT title FROM notifications WHERE id = ?", id).Scan(&title)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if title != "Failure Test" {
		t.Errorf("title = %q, want Failure Test", title)
	}
}

func TestServiceNotifyAdmin_NoEmailClient(t *testing.T) {
	t.Parallel()
	db := testutil.SetupDB(t)

	// Service with nil email client — email silently skipped.
	svc := notifications.NewService(db, nil, nil)

	id, err := svc.NotifyAdmin(1, "info", "No Client", "should work",
		notifications.WithEmail(context.Background()),
	)
	if err != nil {
		t.Fatalf("NotifyAdmin: %v", err)
	}
	if id <= 0 {
		t.Fatal("expected positive ID")
	}
}

func TestServiceNotifyAdmin_UnconfiguredClient(t *testing.T) {
	t.Parallel()
	db := testutil.SetupDB(t)

	// Client with empty API key — email silently skipped.
	client := email.New("")
	svc := notifications.NewService(db, client, nil)

	id, err := svc.NotifyAdmin(1, "info", "Unconfigured", "should work",
		notifications.WithEmail(context.Background()),
	)
	if err != nil {
		t.Fatalf("NotifyAdmin: %v", err)
	}
	if id <= 0 {
		t.Fatal("expected positive ID")
	}
}
