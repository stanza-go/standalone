// Package adminwebhooks provides the admin webhook management endpoints.
// Webhooks allow external systems to receive HTTP callbacks when events
// occur in the application.
package adminwebhooks

import (
	"encoding/json"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/validate"
	"github.com/stanza-go/framework/pkg/webhook"
	"github.com/stanza-go/standalone/module/adminaudit"
	"github.com/stanza-go/standalone/module/webhooks"
)

// Register mounts the webhook admin routes on the given admin group.
// Routes:
//
//	GET    /api/admin/webhooks              — list all webhooks
//	POST   /api/admin/webhooks              — create a new webhook
//	GET    /api/admin/webhooks/{id}         — get webhook detail
//	PUT    /api/admin/webhooks/{id}         — update a webhook
//	DELETE /api/admin/webhooks/{id}         — delete a webhook
//	GET    /api/admin/webhooks/{id}/deliveries — list deliveries for a webhook
//	POST   /api/admin/webhooks/{id}/test    — send a test event
func Register(admin *http.Group, db *sqlite.DB, dispatcher *webhooks.Dispatcher) {
	admin.HandleFunc("GET /webhooks", listHandler(db))
	admin.HandleFunc("GET /webhooks/export", exportHandler(db))
	admin.HandleFunc("POST /webhooks", createHandler(db))
	admin.HandleFunc("POST /webhooks/bulk-delete", bulkDeleteHandler(db))
	admin.HandleFunc("GET /webhooks/{id}", getHandler(db))
	admin.HandleFunc("PUT /webhooks/{id}", updateHandler(db))
	admin.HandleFunc("DELETE /webhooks/{id}", deleteHandler(db))
	admin.HandleFunc("GET /webhooks/{id}/deliveries", deliveriesHandler(db))
	admin.HandleFunc("POST /webhooks/{id}/test", testHandler(db, dispatcher))
}

type webhookJSON struct {
	ID          int64    `json:"id"`
	URL         string   `json:"url"`
	Secret      string   `json:"secret"`
	Description string   `json:"description"`
	Events      []string `json:"events"`
	IsActive    bool     `json:"is_active"`
	CreatedBy   int64    `json:"created_by"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type deliveryJSON struct {
	ID           int64  `json:"id"`
	WebhookID    int64  `json:"webhook_id"`
	DeliveryID   string `json:"delivery_id"`
	Event        string `json:"event"`
	Payload      string `json:"payload"`
	Status       string `json:"status"`
	StatusCode   int    `json:"status_code"`
	ResponseBody string `json:"response_body"`
	Attempts     int    `json:"attempts"`
	CreatedAt    string `json:"created_at"`
	CompletedAt  string `json:"completed_at"`
}

func scanWebhook(rows *sqlite.Rows) (webhookJSON, error) {
	var wh webhookJSON
	var eventsStr string
	if err := rows.Scan(&wh.ID, &wh.URL, &wh.Secret, &wh.Description, &eventsStr, &wh.IsActive, &wh.CreatedBy, &wh.CreatedAt, &wh.UpdatedAt); err != nil {
		return wh, err
	}
	_ = json.Unmarshal([]byte(eventsStr), &wh.Events)
	if wh.Events == nil {
		wh.Events = []string{}
	}
	return wh, nil
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pg := http.ParsePagination(r, 50, 100)
		search := r.URL.Query().Get("search")

		qb := sqlite.Select("id", "url", "secret", "description", "events", "is_active", "created_by", "created_at", "updated_at").
			From("webhooks")
		qb.WhereSearch(search, "url", "description")

		total, _ := db.Count(qb)

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "url", "is_active", "created_at", "updated_at"},
			"created_at", "DESC")
		sql, args := qb.OrderBy(sortCol, sortDir).Limit(pg.Limit).Offset(pg.Offset).Build()
		items, err := sqlite.QueryAll(db, sql, args, scanWebhook)
		if err != nil {
			http.WriteServerError(w, r, "failed to query webhooks", err)
			return
		}

		http.PaginatedResponse(w, "webhooks", items, total)
	}
}

func exportHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")

		qb := sqlite.Select("id", "url", "description", "events", "is_active", "created_by", "created_at", "updated_at").
			From("webhooks")
		qb.WhereSearch(search, "url", "description")

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "url", "is_active", "created_at", "updated_at"},
			"created_at", "DESC")

		sql, args := qb.OrderBy(sortCol, sortDir).Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteServerError(w, r, "failed to export webhooks", err)
			return
		}
		defer rows.Close()

		http.WriteCSV(w, "webhooks", []string{"ID", "URL", "Description", "Events", "Active", "Created By", "Created At", "Updated At"}, func() []string {
			if !rows.Next() {
				return nil
			}
			var id, createdBy int64
			var url, description, eventsStr, createdAt, updatedAt string
			var active bool
			if err := rows.Scan(&id, &url, &description, &eventsStr, &active, &createdBy, &createdAt, &updatedAt); err != nil {
				return nil
			}
			isActive := "No"
			if active {
				isActive = "Yes"
			}
			return []string{sqlite.FormatID(id), url, description, eventsStr, isActive, sqlite.FormatID(createdBy), createdAt, updatedAt}
		})
	}
}

func createHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			URL         string   `json:"url"`
			Description string   `json:"description"`
			Events      []string `json:"events"`
		}
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		v := validate.Fields(
			validate.Required("url", req.URL),
			validate.URL("url", req.URL),
		)
		if v.HasErrors() {
			v.WriteError(w)
			return
		}

		if len(req.Events) == 0 {
			req.Events = []string{"*"}
		}

		eventsJSON, _ := json.Marshal(req.Events)
		secret := webhooks.GenerateSecret()
		now := sqlite.Now()

		claims, _ := auth.ClaimsFromContext(r.Context())
		createdBy := claims.IntUID()

		id, err := db.Insert(sqlite.Insert("webhooks").
			Set("url", req.URL).
			Set("secret", secret).
			Set("description", req.Description).
			Set("events", string(eventsJSON)).
			Set("is_active", true).
			Set("created_by", createdBy).
			Set("created_at", now).
			Set("updated_at", now))
		if err != nil {
			http.WriteServerError(w, r, "failed to create webhook", err)
			return
		}

		adminaudit.Log(db, r, "webhook.create", "webhook", sqlite.FormatID(id), req.URL)

		http.WriteJSON(w, http.StatusCreated, webhookJSON{
			ID:          id,
			URL:         req.URL,
			Secret:      secret,
			Description: req.Description,
			Events:      req.Events,
			IsActive:    true,
			CreatedBy:   createdBy,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
}

func getHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		sql, args := sqlite.Select("id", "url", "secret", "description", "events", "is_active", "created_by", "created_at", "updated_at").
			From("webhooks").
			Where("id = ?", id).
			Build()
		wh, err := sqlite.QueryOne(db, sql, args, scanWebhook)
		if err != nil {
			http.WriteError(w, http.StatusNotFound, "webhook not found")
			return
		}

		// Include recent delivery stats.
		var totalDeliveries, successCount, failedCount int
		sq, sa := sqlite.Select(
			"COUNT(*)",
			"COALESCE(SUM(CASE WHEN status='success' THEN 1 ELSE 0 END), 0)",
			"COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END), 0)").
			From("webhook_deliveries").
			Where("webhook_id = ?", id).
			Build()
		_ = db.QueryRow(sq, sa...).Scan(&totalDeliveries, &successCount, &failedCount)

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"webhook":          wh,
			"total_deliveries": totalDeliveries,
			"success_count":    successCount,
			"failed_count":     failedCount,
		})
	}
}

func updateHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			URL         *string  `json:"url"`
			Description *string  `json:"description"`
			Events      []string `json:"events"`
			IsActive    *bool    `json:"is_active"`
		}
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		now := sqlite.Now()
		ub := sqlite.Update("webhooks").Set("updated_at", now).Where("id = ?", id)

		if req.URL != nil {
			if fe := validate.URL("url", *req.URL); fe != nil {
				http.WriteError(w, http.StatusBadRequest, fe.Message)
				return
			}
			ub = ub.Set("url", *req.URL)
		}
		if req.Description != nil {
			ub = ub.Set("description", *req.Description)
		}
		if req.Events != nil {
			eventsJSON, _ := json.Marshal(req.Events)
			ub = ub.Set("events", string(eventsJSON))
		}
		if req.IsActive != nil {
			ub = ub.Set("is_active", *req.IsActive)
		}

		n, err := db.Update(ub)
		if err != nil {
			http.WriteServerError(w, r, "failed to update webhook", err)
			return
		}
		if n == 0 {
			http.WriteError(w, http.StatusNotFound, "webhook not found")
			return
		}

		adminaudit.Log(db, r, "webhook.update", "webhook", id, "")

		// Return updated webhook.
		sql, args := sqlite.Select("id", "url", "secret", "description", "events", "is_active", "created_by", "created_at", "updated_at").
			From("webhooks").
			Where("id = ?", id).
			Build()
		wh, err := sqlite.QueryOne(db, sql, args, scanWebhook)
		if err != nil {
			http.WriteServerError(w, r, "failed to read updated webhook", err)
			return
		}

		http.WriteJSON(w, http.StatusOK, wh)
	}
}

func bulkDeleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs []int64 `json:"ids"`
		}
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if !http.CheckBulkIDs(w, req.IDs, 100) {
			return
		}

		ids := make([]any, len(req.IDs))
		for i, id := range req.IDs {
			ids[i] = id
		}

		// Delete deliveries first (FK constraint).
		_, _ = db.Delete(sqlite.Delete("webhook_deliveries").
			WhereIn("webhook_id", ids...))

		n, err := db.Delete(sqlite.Delete("webhooks").
			WhereIn("id", ids...))
		if err != nil {
			http.WriteServerError(w, r, "failed to bulk delete webhooks", err)
			return
		}

		for _, id := range req.IDs {
			adminaudit.Log(db, r, "webhook.delete", "webhook", sqlite.FormatID(id), "bulk")
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"affected": n,
		})
	}
}

func deleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		// Delete deliveries first (FK constraint).
		_, _ = db.Delete(sqlite.Delete("webhook_deliveries").Where("webhook_id = ?", id))

		n, err := db.Delete(sqlite.Delete("webhooks").Where("id = ?", id))
		if err != nil {
			http.WriteServerError(w, r, "failed to delete webhook", err)
			return
		}
		if n == 0 {
			http.WriteError(w, http.StatusNotFound, "webhook not found")
			return
		}

		adminaudit.Log(db, r, "webhook.delete", "webhook", id, "")

		http.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

func deliveriesHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		pg := http.ParsePagination(r, 50, 100)
		status := r.URL.Query().Get("status")

		// Verify webhook exists.
		vq, va := sqlite.Select("id").From("webhooks").Where("id = ?", id).Build()
		var whID int64
		if err := db.QueryRow(vq, va...).Scan(&whID); err != nil {
			http.WriteError(w, http.StatusNotFound, "webhook not found")
			return
		}

		qb := sqlite.Select("id", "webhook_id", "delivery_id", "event", "payload", "status", "status_code", "response_body", "attempts", "created_at", sqlite.CoalesceEmpty("completed_at")).
			From("webhook_deliveries").
			Where("webhook_id = ?", id)
		if status != "" {
			qb.Where("status = ?", status)
		}

		total, _ := db.Count(qb)

		sql, args := qb.OrderBy("created_at", "DESC").Limit(pg.Limit).Offset(pg.Offset).Build()
		items, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (deliveryJSON, error) {
			var d deliveryJSON
			err := rows.Scan(&d.ID, &d.WebhookID, &d.DeliveryID, &d.Event, &d.Payload, &d.Status, &d.StatusCode, &d.ResponseBody, &d.Attempts, &d.CreatedAt, &d.CompletedAt)
			return d, err
		})
		if err != nil {
			http.WriteServerError(w, r, "failed to query deliveries", err)
			return
		}

		http.PaginatedResponse(w, "deliveries", items, total)
	}
}

func testHandler(db *sqlite.DB, dispatcher *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		// Verify webhook exists and get its URL.
		vq, va := sqlite.Select("url").From("webhooks").Where("id = ?", id).Build()
		var url string
		if err := db.QueryRow(vq, va...).Scan(&url); err != nil {
			http.WriteError(w, http.StatusNotFound, "webhook not found")
			return
		}

		// Send a test event through the dispatcher.
		testPayload := map[string]any{
			"event":     "webhook.test",
			"timestamp": sqlite.Now(),
			"data": map[string]string{
				"message": "This is a test webhook delivery from Stanza.",
			},
		}

		if err := dispatcher.Dispatch(r.Context(), "webhook.test", testPayload); err != nil {
			http.WriteServerError(w, r, "failed to dispatch test event", err)
			return
		}

		adminaudit.Log(db, r, "webhook.test", "webhook", id, url)

		http.WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "queued",
			"message": "Test event has been queued for delivery",
		})
	}
}

// Verify verifies a webhook signature. This is a convenience wrapper around
// the framework's webhook.Verify function, useful for documenting in admin
// endpoints or API docs how recipients should verify signatures.
func Verify(secret, id, timestamp, signature string, body []byte) bool {
	return webhook.Verify(secret, id, timestamp, signature, body)
}

