// Package adminsessions provides admin endpoints for viewing and managing
// active sessions (refresh tokens). It allows listing all active sessions
// across entity types and revoking individual sessions.
package adminsessions

import (
	"encoding/csv"
	"fmt"
	"time"

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
	admin.HandleFunc("DELETE /sessions/{id}", revokeHandler(db, wh))
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC().Format(time.RFC3339)

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"created_at", "expires_at", "entity_type"},
			"created_at", "DESC")

		sql, args := sqlite.Select(
			"rt.id", "rt.entity_type", "rt.entity_id", "rt.created_at", "rt.expires_at",
			"COALESCE(a.email, '')", "COALESCE(a.name, '')").
			From("refresh_tokens rt").
			LeftJoin("admins a", "rt.entity_type = 'admin' AND rt.entity_id = CAST(a.id AS TEXT)").
			Where("rt.expires_at > ?", now).
			OrderBy("rt."+sortCol, sortDir).
			Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to list sessions")
			return
		}
		defer rows.Close()

		type sessionJSON struct {
			ID         string `json:"id"`
			EntityType string `json:"entity_type"`
			EntityID   string `json:"entity_id"`
			Email      string `json:"email"`
			Name       string `json:"name"`
			CreatedAt  string `json:"created_at"`
			ExpiresAt  string `json:"expires_at"`
		}

		sessions := make([]sessionJSON, 0)
		for rows.Next() {
			var s sessionJSON
			if err := rows.Scan(&s.ID, &s.EntityType, &s.EntityID, &s.CreatedAt, &s.ExpiresAt, &s.Email, &s.Name); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan session")
				return
			}
			sessions = append(sessions, s)
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"sessions": sessions,
		})
	}
}

func exportHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC().Format(time.RFC3339)

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"created_at", "expires_at", "entity_type"},
			"created_at", "DESC")

		sql, args := sqlite.Select(
			"rt.id", "rt.entity_type", "rt.entity_id", "rt.created_at", "rt.expires_at",
			"COALESCE(a.email, '')", "COALESCE(a.name, '')").
			From("refresh_tokens rt").
			LeftJoin("admins a", "rt.entity_type = 'admin' AND rt.entity_id = CAST(a.id AS TEXT)").
			Where("rt.expires_at > ?", now).
			OrderBy("rt."+sortCol, sortDir).
			Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to export sessions")
			return
		}
		defer rows.Close()

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=sessions-%s.csv", time.Now().UTC().Format("20060102")))
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"ID", "Entity Type", "Entity ID", "Email", "Name", "Created At", "Expires At"})

		for rows.Next() {
			var id, entityType, entityID, email, name, createdAt, expiresAt string
			if err := rows.Scan(&id, &entityType, &entityID, &createdAt, &expiresAt, &email, &name); err != nil {
				break
			}
			_ = cw.Write([]string{id, entityType, entityID, email, name, createdAt, expiresAt})
		}
		cw.Flush()
	}
}

func revokeHandler(db *sqlite.DB, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.WriteError(w, http.StatusBadRequest, "session id required")
			return
		}

		sql, args := sqlite.Delete("refresh_tokens").Where("id = ?", id).Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to revoke session")
			return
		}

		if result.RowsAffected == 0 {
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
