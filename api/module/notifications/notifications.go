// Package notifications provides the notification service and shared types.
// Notifications target a specific entity (admin or user) identified by
// entity_type and entity_id. Other modules use the exported functions to
// create notifications; the admin and user notification endpoints read them.
package notifications

import (
	"time"

	"github.com/stanza-go/framework/pkg/sqlite"
)

// Entity types for the entity_type column.
const (
	EntityAdmin = "admin"
	EntityUser  = "user"
)

// Notification represents a single notification row.
type Notification struct {
	ID         int64  `json:"id"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	Message    string `json:"message"`
	Data       string `json:"data,omitempty"`
	ReadAt     string `json:"read_at,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// Notify creates a notification for a specific entity.
func Notify(db *sqlite.DB, entityType string, entityID int64, notifType, title, message, data string) (int64, error) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	sql, args := sqlite.Insert("notifications").
		Set("entity_type", entityType).
		Set("entity_id", entityID).
		Set("type", notifType).
		Set("title", title).
		Set("message", message).
		Set("data", data).
		Set("created_at", now).
		Build()
	result, err := db.Exec(sql, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertID, nil
}

// NotifyAdmin creates a notification for an admin user.
func NotifyAdmin(db *sqlite.DB, adminID int64, notifType, title, message string) (int64, error) {
	return Notify(db, EntityAdmin, adminID, notifType, title, message, "")
}

// NotifyUser creates a notification for an end user.
func NotifyUser(db *sqlite.DB, userID int64, notifType, title, message string) (int64, error) {
	return Notify(db, EntityUser, userID, notifType, title, message, "")
}

// NotifyAllAdmins creates a notification for every active admin.
func NotifyAllAdmins(db *sqlite.DB, notifType, title, message string) error {
	rows, err := db.Query("SELECT id FROM admins WHERE is_active = 1 AND deleted_at IS NULL")
	if err != nil {
		return err
	}

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		if _, err := NotifyAdmin(db, id, notifType, title, message); err != nil {
			return err
		}
	}
	return nil
}

// UnreadCount returns the number of unread notifications for an entity.
func UnreadCount(db *sqlite.DB, entityType string, entityID int64) int {
	sql, args := sqlite.Count("notifications").
		Where("entity_type = ?", entityType).
		Where("entity_id = ?", entityID).
		Where("read_at IS NULL").
		Build()
	var count int
	_ = db.QueryRow(sql, args...).Scan(&count)
	return count
}
