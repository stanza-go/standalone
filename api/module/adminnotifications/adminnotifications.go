// Package adminnotifications provides admin endpoints for managing
// notifications. Admins can list their notifications, get unread counts,
// mark individual or all notifications as read, delete notifications, and
// send notifications (with optional email delivery).
package adminnotifications

import (
	"encoding/json"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/validate"
	"github.com/stanza-go/standalone/module/notifications"
)

// Register mounts admin notification routes on the given group.
// Routes:
//
//	GET    /api/admin/notifications              — list notifications (paginated)
//	GET    /api/admin/notifications/unread-count  — count of unread notifications
//	GET    /api/admin/notifications/stream        — SSE stream for real-time notifications
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
		adminID := claims.IntUID()

		pg := http.ParsePagination(r, 50, 100)
		unreadOnly := r.URL.Query().Get("unread") == "true"

		q := sqlite.Select("id", "type", "title", "message", "data", sqlite.CoalesceEmpty("read_at"), "created_at").
			From("notifications").
			Where("entity_type = ?", notifications.EntityAdmin).
			Where("entity_id = ?", adminID)
		if unreadOnly {
			q.WhereNull("read_at")
		}

		total, _ := db.Count(q)

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "type", "created_at"},
			"id", "DESC")
		sql, args := q.OrderBy(sortCol, sortDir).Limit(pg.Limit).Offset(pg.Offset).Build()
		items, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (notifications.Notification, error) {
			var n notifications.Notification
			var readAt string
			if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Message, &n.Data, &readAt, &n.CreatedAt); err != nil {
				return n, err
			}
			n.ReadAt = readAt
			n.EntityType = notifications.EntityAdmin
			n.EntityID = adminID
			return n, nil
		})
		if err != nil {
			http.WriteServerError(w, r, "failed to list notifications", err)
			return
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
		adminID := claims.IntUID()

		unreadOnly := r.URL.Query().Get("unread") == "true"

		q := sqlite.Select("id", "type", "title", "message", sqlite.CoalesceEmpty("read_at"), "created_at").
			From("notifications").
			Where("entity_type = ?", notifications.EntityAdmin).
			Where("entity_id = ?", adminID)
		if unreadOnly {
			q.WhereNull("read_at")
		}

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "type", "created_at"},
			"id", "DESC")

		sql, args := q.OrderBy(sortCol, sortDir).Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteServerError(w, r, "failed to export notifications", err)
			return
		}
		defer rows.Close()

		http.WriteCSV(w, "notifications", []string{"ID", "Type", "Title", "Message", "Read At", "Created At"}, func() []string {
			if !rows.Next() {
				return nil
			}
			var id int64
			var typ, title, message, readAt, createdAt string
			if err := rows.Scan(&id, &typ, &title, &message, &readAt, &createdAt); err != nil {
				return nil
			}
			return []string{sqlite.FormatID(id), typ, title, message, readAt, createdAt}
		})
	}
}

func unreadCountHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID := claims.IntUID()

		count := notifications.UnreadCount(db, notifications.EntityAdmin, adminID)
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"unread": count,
		})
	}
}

func markReadHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID := claims.IntUID()

		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		now := sqlite.Now()
		n, err := db.Update(sqlite.Update("notifications").
			Set("read_at", now).
			Where("id = ?", id).
			Where("entity_type = ?", notifications.EntityAdmin).
			Where("entity_id = ?", adminID).
			WhereNull("read_at"))
		if err != nil {
			http.WriteServerError(w, r, "failed to mark notification as read", err)
			return
		}

		if n == 0 {
			http.WriteError(w, http.StatusNotFound, "notification not found or already read")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func markAllReadHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID := claims.IntUID()

		now := sqlite.Now()
		n, err := db.Update(sqlite.Update("notifications").
			Set("read_at", now).
			Where("entity_type = ?", notifications.EntityAdmin).
			Where("entity_id = ?", adminID).
			WhereNull("read_at"))
		if err != nil {
			http.WriteServerError(w, r, "failed to mark notifications as read", err)
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"marked":  n,
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
		if !http.BindJSON(w, r, &req) {
			return
		}
		if req.Type == "" {
			req.Type = "info"
		}
		v := validate.Fields(
			validate.Required("entity_type", req.EntityType),
			validate.OneOf("entity_type", req.EntityType, notifications.EntityAdmin, notifications.EntityUser),
			validate.Check("entity_id", req.EntityID > 0, "must be a positive number"),
			validate.Required("title", req.Title),
		)
		if v.HasErrors() {
			v.WriteError(w)
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
			http.WriteServerError(w, r, "failed to create notification", err)
			return
		}

		http.WriteJSON(w, http.StatusCreated, map[string]any{
			"id":         id,
			"email_sent": req.SendEmail,
		})
	}
}

// streamHandler streams real-time notification events to the connected admin
// using Server-Sent Events (SSE). Events include new notifications and
// updated unread counts. The connection stays open until the client
// disconnects or the server shuts down.
//
// SSE is used instead of WebSocket because notifications are server-push
// only (unidirectional). SSE also works over HTTP/2, which is required for
// deployments behind HTTP/2 edge proxies like Railway where WebSocket
// upgrade headers are stripped.
//
// Keepalive comments are sent every 30s to prevent proxy timeouts.
// The retry directive tells the client to reconnect after 5s on failure.
func streamHandler(svc *notifications.Service) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID := claims.IntUID()

		eventCh, unsub := svc.Hub().Subscribe(adminID)
		defer unsub()

		sse := http.NewSSEWriter(w)
		_ = sse.Retry(5000)

		// Send initial unread count so the client syncs immediately.
		initialCount := notifications.UnreadCount(svc.DB(), notifications.EntityAdmin, adminID)
		initMsg, _ := json.Marshal(notifications.Event{
			Type:        "unread_count",
			UnreadCount: initialCount,
		})
		if err := sse.Event("notification", string(initMsg)); err != nil {
			return
		}

		heartbeat := time.NewTicker(30 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case evt, ok := <-eventCh:
				if !ok {
					return
				}
				msg, _ := json.Marshal(evt)
				if err := sse.Event("notification", string(msg)); err != nil {
					return
				}
			case <-heartbeat.C:
				if err := sse.Comment("keepalive"); err != nil {
					return
				}
			}
		}
	}
}

func bulkDeleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID := claims.IntUID()

		var req struct {
			IDs []int64 `json:"ids"`
		}
		if !http.BindJSON(w, r, &req) {
			return
		}
		if !http.CheckBulkIDs(w, req.IDs, 100) {
			return
		}

		ids := make([]any, len(req.IDs))
		for i, id := range req.IDs {
			ids[i] = id
		}

		n, err := db.Delete(sqlite.Delete("notifications").
			Where("entity_type = ?", notifications.EntityAdmin).
			Where("entity_id = ?", adminID).
			WhereIn("id", ids...))
		if err != nil {
			http.WriteServerError(w, r, "failed to bulk delete notifications", err)
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"affected": n,
		})
	}
}

func deleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		adminID := claims.IntUID()

		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		n, err := db.Delete(sqlite.Delete("notifications").
			Where("id = ?", id).
			Where("entity_type = ?", notifications.EntityAdmin).
			Where("entity_id = ?", adminID))
		if err != nil {
			http.WriteServerError(w, r, "failed to delete notification", err)
			return
		}

		if n == 0 {
			http.WriteError(w, http.StatusNotFound, "notification not found")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}
