// Package usernotifications provides user-facing endpoints for managing
// notifications. Users can list their notifications, get unread counts,
// mark individual or all notifications as read, and delete notifications.
package usernotifications

import (
	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/notifications"
)

// Register mounts user notification routes on the given group.
// Routes:
//
//	GET    /api/user/notifications              — list notifications (paginated)
//	GET    /api/user/notifications/unread-count  — count of unread notifications
//	POST   /api/user/notifications/{id}/read     — mark one notification as read
//	POST   /api/user/notifications/read-all      — mark all notifications as read
//	DELETE /api/user/notifications/{id}           — delete a notification
func Register(user *http.Group, db *sqlite.DB) {
	user.HandleFunc("GET /notifications", listHandler(db))
	user.HandleFunc("GET /notifications/unread-count", unreadCountHandler(db))
	user.HandleFunc("POST /notifications/{id}/read", markReadHandler(db))
	user.HandleFunc("POST /notifications/read-all", markAllReadHandler(db))
	user.HandleFunc("DELETE /notifications/{id}", deleteHandler(db))
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.IntUID()

		pg := http.ParsePagination(r, 50, 100)
		unreadOnly := r.URL.Query().Get("unread") == "true"

		q := sqlite.Select("id", "type", "title", "message", "data", sqlite.CoalesceEmpty("read_at"), "created_at").
			From("notifications").
			Where("entity_type = ?", notifications.EntityUser).
			Where("entity_id = ?", userID)
		if unreadOnly {
			q.Where("read_at IS NULL")
		}

		total, _ := db.Count(q)

		sql, args := q.OrderBy("id", "DESC").Limit(pg.Limit).Offset(pg.Offset).Build()
		items, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (notifications.Notification, error) {
			var n notifications.Notification
			var readAt string
			if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Message, &n.Data, &readAt, &n.CreatedAt); err != nil {
				return n, err
			}
			n.ReadAt = readAt
			n.EntityType = notifications.EntityUser
			n.EntityID = userID
			return n, nil
		})
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to list notifications")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"notifications": items,
			"total":         total,
			"unread":        notifications.UnreadCount(db, notifications.EntityUser, userID),
		})
	}
}

func unreadCountHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.IntUID()

		count := notifications.UnreadCount(db, notifications.EntityUser, userID)
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"unread": count,
		})
	}
}

func markReadHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.IntUID()

		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		now := sqlite.Now()
		sql, args := sqlite.Update("notifications").
			Set("read_at", now).
			Where("id = ?", id).
			Where("entity_type = ?", notifications.EntityUser).
			Where("entity_id = ?", userID).
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
		userID := claims.IntUID()

		now := sqlite.Now()
		sql, args := sqlite.Update("notifications").
			Set("read_at", now).
			Where("entity_type = ?", notifications.EntityUser).
			Where("entity_id = ?", userID).
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

func deleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.IntUID()

		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		sql, args := sqlite.Delete("notifications").
			Where("id = ?", id).
			Where("entity_type = ?", notifications.EntityUser).
			Where("entity_id = ?", userID).
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
