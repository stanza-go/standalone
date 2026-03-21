package notifications_test

import (
	"testing"

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
