// Package useractivity provides a read-only activity log for authenticated
// users. It exposes audit log entries where the user is the target entity,
// giving users visibility into admin actions on their account (e.g.,
// profile updates, password resets, deactivation).
//
// All routes require a valid JWT or API key with the "user" scope.
package useractivity

import (
	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
)

// Register mounts the user activity routes on the given group.
// The group should already have RequireAuth + RequireScope("user") applied.
//
// Routes:
//
//	GET /activity — list audit log entries targeting the authenticated user
func Register(user *http.Group, db *sqlite.DB) {
	user.HandleFunc("GET /activity", listActivity(db))
}

type activityEntry struct {
	ID        int64  `json:"id"`
	Action    string `json:"action"`
	Details   string `json:"details"`
	IPAddress string `json:"ip_address"`
	CreatedAt string `json:"created_at"`
}

// listActivity returns audit log entries where the authenticated user is the
// target entity. Supports pagination (limit/offset), action filter, and
// date range (from/to). Admin identity is intentionally omitted from the
// response — users see what happened, not who did it.
func listActivity(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		pg := http.ParsePagination(r, 20, 100)

		// Build count query.
		cb := sqlite.Count("audit_log").
			Where("entity_type = 'user'").
			Where("entity_id = ?", claims.UID)

		q := r.URL.Query()
		if action := q.Get("action"); action != "" {
			cb = cb.Where("action = ?", action)
		}
		if from := q.Get("from"); from != "" {
			cb = cb.Where("created_at >= ?", from)
		}
		if to := q.Get("to"); to != "" {
			cb = cb.Where("created_at <= ?", to)
		}

		var total int
		countSQL, countArgs := cb.Build()
		_ = db.QueryRow(countSQL, countArgs...).Scan(&total)

		// Build select query — no LEFT JOIN on admins, users don't see admin identity.
		sb := sqlite.Select("id", "action", "details", "ip_address", "created_at").
			From("audit_log").
			Where("entity_type = 'user'").
			Where("entity_id = ?", claims.UID)

		if action := q.Get("action"); action != "" {
			sb = sb.Where("action = ?", action)
		}
		if from := q.Get("from"); from != "" {
			sb = sb.Where("created_at >= ?", from)
		}
		if to := q.Get("to"); to != "" {
			sb = sb.Where("created_at <= ?", to)
		}

		selectSQL, selectArgs := sb.OrderBy("created_at", "DESC").
			Limit(pg.Limit).Offset(pg.Offset).
			Build()

		rows, err := db.Query(selectSQL, selectArgs...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to query activity")
			return
		}
		defer rows.Close()

		entries := make([]activityEntry, 0)
		for rows.Next() {
			var e activityEntry
			if err := rows.Scan(&e.ID, &e.Action, &e.Details, &e.IPAddress, &e.CreatedAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan activity")
				return
			}
			entries = append(entries, e)
		}

		http.PaginatedResponse(w, "entries", entries, total)
	}
}
