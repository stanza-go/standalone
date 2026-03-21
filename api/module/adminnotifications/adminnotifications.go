// Package adminnotifications provides admin endpoints for managing
// notifications. Admins can list their notifications, get unread counts,
// mark individual or all notifications as read, delete notifications, and
// send notifications (with optional email delivery).
package adminnotifications

import (
	"strconv"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/notifications"
)

// Register mounts admin notification routes on the given group.
// Routes:
//
//	GET    /api/admin/notifications              — list notifications (paginated)
//	GET    /api/admin/notifications/unread-count  — count of unread notifications
//	POST   /api/admin/notifications/send          — send a notification (with optional email)
//	POST   /api/admin/notifications/{id}/read     — mark one notification as read
//	POST   /api/admin/notifications/read-all      — mark all notifications as read
//	DELETE /api/admin/notifications/{id}           — delete a notification
func Register(admin *http.Group, db *sqlite.DB, svc *notifications.Service) {
	admin.HandleFunc("GET /notifications", listHandler(db))
	admin.HandleFunc("GET /notifications/unread-count", unreadCountHandler(db))
	admin.HandleFunc("POST /notifications/send", sendHandler(svc))
	admin.HandleFunc("POST /notifications/{id}/read", markReadHandler(db))
	admin.HandleFunc("POST /notifications/read-all", markAllReadHandler(db))
	admin.HandleFunc("DELETE /notifications/{id}", deleteHandler(db))
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID, _ := strconv.ParseInt(claims.UID, 10, 64)

		limit := http.QueryParamInt(r, "limit", 50)
		offset := http.QueryParamInt(r, "offset", 0)
		unreadOnly := r.URL.Query().Get("unread") == "true"

		// Count total.
		countQ := sqlite.Count("notifications").
			Where("entity_type = ?", notifications.EntityAdmin).
			Where("entity_id = ?", adminID)
		if unreadOnly {
			countQ.Where("read_at IS NULL")
		}
		sql, args := countQ.Build()
		var total int
		_ = db.QueryRow(sql, args...).Scan(&total)

		// Fetch page.
		q := sqlite.Select("id", "type", "title", "message", "data", "COALESCE(read_at, '')", "created_at").
			From("notifications").
			Where("entity_type = ?", notifications.EntityAdmin).
			Where("entity_id = ?", adminID).
			OrderBy("id", "DESC").
			Limit(limit).
			Offset(offset)
		if unreadOnly {
			q.Where("read_at IS NULL")
		}
		sql, args = q.Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to list notifications")
			return
		}

		var items []notifications.Notification
		for rows.Next() {
			var n notifications.Notification
			var readAt string
			if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Message, &n.Data, &readAt, &n.CreatedAt); err != nil {
				rows.Close()
				http.WriteError(w, http.StatusInternalServerError, "failed to scan notification")
				return
			}
			n.ReadAt = readAt
			n.EntityType = notifications.EntityAdmin
			n.EntityID = adminID
			items = append(items, n)
		}
		rows.Close()

		if items == nil {
			items = []notifications.Notification{}
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"notifications": items,
			"total":         total,
			"unread":        notifications.UnreadCount(db, notifications.EntityAdmin, adminID),
		})
	}
}

func unreadCountHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID, _ := strconv.ParseInt(claims.UID, 10, 64)

		count := notifications.UnreadCount(db, notifications.EntityAdmin, adminID)
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"unread": count,
		})
	}
}

func markReadHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID, _ := strconv.ParseInt(claims.UID, 10, 64)

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid notification id")
			return
		}

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		sql, args := sqlite.Update("notifications").
			Set("read_at", now).
			Where("id = ?", id).
			Where("entity_type = ?", notifications.EntityAdmin).
			Where("entity_id = ?", adminID).
			Where("read_at IS NULL").
			Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to mark notification as read")
			return
		}

		if result.RowsAffected == 0 {
			http.WriteError(w, http.StatusNotFound, "notification not found or already read")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func markAllReadHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID, _ := strconv.ParseInt(claims.UID, 10, 64)

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		sql, args := sqlite.Update("notifications").
			Set("read_at", now).
			Where("entity_type = ?", notifications.EntityAdmin).
			Where("entity_id = ?", adminID).
			Where("read_at IS NULL").
			Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to mark notifications as read")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"marked":  result.RowsAffected,
		})
	}
}

// sendHandler creates a notification for a target entity with optional email
// delivery. Request body:
//
//	{
//	  "entity_type": "admin" | "user",
//	  "entity_id": 1,
//	  "type": "info",
//	  "title": "Hello",
//	  "message": "World",
//	  "send_email": true
//	}
func sendHandler(svc *notifications.Service) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			EntityType string `json:"entity_type"`
			EntityID   int64  `json:"entity_id"`
			Type       string `json:"type"`
			Title      string `json:"title"`
			Message    string `json:"message"`
			SendEmail  bool   `json:"send_email"`
		}
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.EntityType == "" || req.EntityID <= 0 || req.Title == "" {
			http.WriteError(w, http.StatusUnprocessableEntity, "entity_type, entity_id, and title are required")
			return
		}
		if req.Type == "" {
			req.Type = "info"
		}
		if req.EntityType != notifications.EntityAdmin && req.EntityType != notifications.EntityUser {
			http.WriteError(w, http.StatusUnprocessableEntity, "entity_type must be 'admin' or 'user'")
			return
		}

		var opts []notifications.Option
		if req.SendEmail {
			opts = append(opts, notifications.WithEmail(r.Context()))
		}

		var id int64
		var err error
		switch req.EntityType {
		case notifications.EntityAdmin:
			id, err = svc.NotifyAdmin(req.EntityID, req.Type, req.Title, req.Message, opts...)
		case notifications.EntityUser:
			id, err = svc.NotifyUser(req.EntityID, req.Type, req.Title, req.Message, opts...)
		}
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to create notification")
			return
		}

		http.WriteJSON(w, http.StatusCreated, map[string]any{
			"id":         id,
			"email_sent": req.SendEmail,
		})
	}
}

func deleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID, _ := strconv.ParseInt(claims.UID, 10, 64)

		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid notification id")
			return
		}

		sql, args := sqlite.Delete("notifications").
			Where("id = ?", id).
			Where("entity_type = ?", notifications.EntityAdmin).
			Where("entity_id = ?", adminID).
			Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to delete notification")
			return
		}

		if result.RowsAffected == 0 {
			http.WriteError(w, http.StatusNotFound, "notification not found")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}
