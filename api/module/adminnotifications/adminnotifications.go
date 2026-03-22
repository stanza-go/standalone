// Package adminnotifications provides admin endpoints for managing
// notifications. Admins can list their notifications, get unread counts,
// mark individual or all notifications as read, delete notifications, and
// send notifications (with optional email delivery).
package adminnotifications

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
//	GET    /api/admin/notifications/stream        — WebSocket stream for real-time notifications
//	POST   /api/admin/notifications/send          — send a notification (with optional email)
//	POST   /api/admin/notifications/{id}/read     — mark one notification as read
//	POST   /api/admin/notifications/read-all      — mark all notifications as read
//	DELETE /api/admin/notifications/{id}           — delete a notification
func Register(admin *http.Group, db *sqlite.DB, svc *notifications.Service) {
	admin.HandleFunc("GET /notifications", listHandler(db))
	admin.HandleFunc("GET /notifications/export", exportHandler(db))
	admin.HandleFunc("GET /notifications/unread-count", unreadCountHandler(db))
	admin.HandleFunc("GET /notifications/stream", streamHandler(svc))
	admin.HandleFunc("POST /notifications/send", sendHandler(svc))
	admin.HandleFunc("POST /notifications/bulk-delete", bulkDeleteHandler(db))
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

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "type", "created_at"},
			"id", "DESC")

		// Fetch page.
		q := sqlite.Select("id", "type", "title", "message", "data", "COALESCE(read_at, '')", "created_at").
			From("notifications").
			Where("entity_type = ?", notifications.EntityAdmin).
			Where("entity_id = ?", adminID).
			OrderBy(sortCol, sortDir).
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

func exportHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID, _ := strconv.ParseInt(claims.UID, 10, 64)

		unreadOnly := r.URL.Query().Get("unread") == "true"

		q := sqlite.Select("id", "type", "title", "message", "COALESCE(read_at, '')", "created_at").
			From("notifications").
			Where("entity_type = ?", notifications.EntityAdmin).
			Where("entity_id = ?", adminID)
		if unreadOnly {
			q.Where("read_at IS NULL")
		}

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "type", "created_at"},
			"id", "DESC")

		sql, args := q.OrderBy(sortCol, sortDir).Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to export notifications")
			return
		}
		defer rows.Close()

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=notifications-%s.csv", time.Now().UTC().Format("20060102")))
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"ID", "Type", "Title", "Message", "Read At", "Created At"})

		for rows.Next() {
			var id int64
			var typ, title, message, readAt, createdAt string
			if err := rows.Scan(&id, &typ, &title, &message, &readAt, &createdAt); err != nil {
				break
			}
			_ = cw.Write([]string{strconv.FormatInt(id, 10), typ, title, message, readAt, createdAt})
		}
		cw.Flush()
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

// streamHandler upgrades to a WebSocket connection and streams real-time
// notification events to the connected admin. Events include new notifications
// and updated unread counts. The connection stays open until the client
// disconnects or the server shuts down.
//
// Ping frames are sent every 30s to detect dead connections. The client can
// close the connection at any time.
func streamHandler(svc *notifications.Service) func(http.ResponseWriter, *http.Request) {
	upgrader := http.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID, _ := strconv.ParseInt(claims.UID, 10, 64)

		conn, err := upgrader.Upgrade(w, r)
		if err != nil {
			return
		}
		defer conn.Close()

		eventCh, unsub := svc.Hub().Subscribe(adminID)
		defer unsub()

		// Read goroutine — detects client disconnection.
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
			}
		}()

		// Send initial unread count so the client syncs immediately.
		initialCount := notifications.UnreadCount(svc.DB(), notifications.EntityAdmin, adminID)
		initMsg, _ := json.Marshal(notifications.Event{
			Type:        "unread_count",
			UnreadCount: initialCount,
		})
		if err := conn.WriteMessage(http.TextMessage, initMsg); err != nil {
			return
		}

		pingTicker := time.NewTicker(30 * time.Second)
		defer pingTicker.Stop()

		for {
			select {
			case <-done:
				return
			case evt, ok := <-eventCh:
				if !ok {
					return
				}
				msg, _ := json.Marshal(evt)
				if err := conn.WriteMessage(http.TextMessage, msg); err != nil {
					return
				}
			case <-pingTicker.C:
				if err := conn.WritePing(nil); err != nil {
					return
				}
			}
		}
	}
}

func bulkDeleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID, _ := strconv.ParseInt(claims.UID, 10, 64)

		var req struct {
			IDs []int64 `json:"ids"`
		}
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(req.IDs) == 0 {
			http.WriteError(w, http.StatusBadRequest, "ids required")
			return
		}
		if len(req.IDs) > 100 {
			http.WriteError(w, http.StatusBadRequest, "maximum 100 ids per request")
			return
		}

		placeholders := make([]string, len(req.IDs))
		args := make([]any, 0, len(req.IDs)+2)
		args = append(args, notifications.EntityAdmin, adminID)
		for i, id := range req.IDs {
			placeholders[i] = "?"
			args = append(args, id)
		}

		query := fmt.Sprintf(
			"DELETE FROM notifications WHERE entity_type = ? AND entity_id = ? AND id IN (%s)",
			strings.Join(placeholders, ","),
		)
		result, err := db.Exec(query, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to bulk delete notifications")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"affected": result.RowsAffected,
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
