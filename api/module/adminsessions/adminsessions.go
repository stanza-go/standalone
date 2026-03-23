// Package adminsessions provides admin endpoints for viewing and managing
// active sessions (refresh tokens). It allows listing all active sessions
// across entity types and revoking individual sessions.
package adminsessions

import (
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminaudit"
	"github.com/stanza-go/standalone/module/webhooks"
)

// Register mounts the session management routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET    /api/admin/sessions      - list active sessions
//	DELETE /api/admin/sessions/{id} - revoke a session
func Register(admin *http.Group, db *sqlite.DB, wh *webhooks.Dispatcher) {
	admin.HandleFunc("GET /sessions", listHandler(db))
	admin.HandleFunc("GET /sessions/export", exportHandler(db))
	admin.HandleFunc("POST /sessions/bulk-revoke", bulkRevokeHandler(db, wh))
	admin.HandleFunc("DELETE /sessions/{id}", revokeHandler(db, wh))
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		now := sqlite.Now()

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"created_at", "expires_at", "entity_type"},
			"created_at", "DESC")

		sql, args := sqlite.Select(
			"rt.id", "rt.entity_type", "rt.entity_id", "rt.created_at", "rt.expires_at",
			sqlite.CoalesceEmpty("a.email"), sqlite.CoalesceEmpty("a.name")).
			From("refresh_tokens rt").
			LeftJoin("admins a", "rt.entity_type = 'admin' AND rt.entity_id = CAST(a.id AS TEXT)").
			Where("rt.expires_at > ?", now).
			OrderBy("rt."+sortCol, sortDir).
			Build()
		type sessionJSON struct {
			ID         string `json:"id"`
			EntityType string `json:"entity_type"`
			EntityID   string `json:"entity_id"`
			Email      string `json:"email"`
			Name       string `json:"name"`
			CreatedAt  string `json:"created_at"`
			ExpiresAt  string `json:"expires_at"`
		}
		sessions, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (sessionJSON, error) {
			var s sessionJSON
			err := rows.Scan(&s.ID, &s.EntityType, &s.EntityID, &s.CreatedAt, &s.ExpiresAt, &s.Email, &s.Name)
			return s, err
		})
		if err != nil {
			http.WriteServerError(w, r, "failed to list sessions", err)
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"sessions": sessions,
		})
	}
}

func exportHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		now := sqlite.Now()

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"created_at", "expires_at", "entity_type"},
			"created_at", "DESC")

		sql, args := sqlite.Select(
			"rt.id", "rt.entity_type", "rt.entity_id", "rt.created_at", "rt.expires_at",
			sqlite.CoalesceEmpty("a.email"), sqlite.CoalesceEmpty("a.name")).
			From("refresh_tokens rt").
			LeftJoin("admins a", "rt.entity_type = 'admin' AND rt.entity_id = CAST(a.id AS TEXT)").
			Where("rt.expires_at > ?", now).
			OrderBy("rt."+sortCol, sortDir).
			Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteServerError(w, r, "failed to export sessions", err)
			return
		}
		defer rows.Close()

		http.WriteCSV(w, "sessions", []string{"ID", "Entity Type", "Entity ID", "Email", "Name", "Created At", "Expires At"}, func() []string {
			if !rows.Next() {
				return nil
			}
			var id, entityType, entityID, email, name, createdAt, expiresAt string
			if err := rows.Scan(&id, &entityType, &entityID, &createdAt, &expiresAt, &email, &name); err != nil {
				return nil
			}
			return []string{id, entityType, entityID, email, name, createdAt, expiresAt}
		})
	}
}

func bulkRevokeHandler(db *sqlite.DB, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs []string `json:"ids"`
		}
		if !http.BindJSON(w, r, &req) {
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

		ids := make([]any, len(req.IDs))
		for i, id := range req.IDs {
			ids[i] = id
		}

		n, err := db.Delete(sqlite.Delete("refresh_tokens").
			WhereIn("id", ids...))
		if err != nil {
			http.WriteServerError(w, r, "failed to bulk revoke sessions", err)
			return
		}

		for _, id := range req.IDs {
			adminaudit.Log(db, r, "session.revoke", "session", id, "bulk")
		}

		_ = wh.Dispatch(r.Context(), "session.bulk_revoked", map[string]any{
			"ids":      req.IDs,
			"affected": n,
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"affected": n,
		})
	}
}

func revokeHandler(db *sqlite.DB, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.WriteError(w, http.StatusBadRequest, "session id required")
			return
		}

		n, err := db.Delete(sqlite.Delete("refresh_tokens").Where("id = ?", id))
		if err != nil {
			http.WriteServerError(w, r, "failed to revoke session", err)
			return
		}

		if n == 0 {
			http.WriteError(w, http.StatusNotFound, "session not found")
			return
		}

		adminaudit.Log(db, r, "session.revoke", "session", id, "")

		_ = wh.Dispatch(r.Context(), "session.revoked", map[string]any{
			"session_id": id,
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}
