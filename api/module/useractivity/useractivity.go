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

		// Build select query — no LEFT JOIN on admins, users don't see admin identity.
		sb := sqlite.Select("id", "action", "details", "ip_address", "created_at").
			From("audit_log").
			Where("entity_type = 'user'").
			Where("entity_id = ?", claims.UID)

		q := r.URL.Query()
		if action := q.Get("action"); action != "" {
			sb.Where("action = ?", action)
		}
		if from := q.Get("from"); from != "" {
			sb.Where("created_at >= ?", from)
		}
		if to := q.Get("to"); to != "" {
			sb.Where("created_at <= ?", to)
		}

		total, _ := db.Count(sb)

		selectSQL, selectArgs := sb.OrderBy("created_at", "DESC").
			Limit(pg.Limit).Offset(pg.Offset).
			Build()

		entries, err := sqlite.QueryAll(db, selectSQL, selectArgs, func(rows *sqlite.Rows) (activityEntry, error) {
			var e activityEntry
			err := rows.Scan(&e.ID, &e.Action, &e.Details, &e.IPAddress, &e.CreatedAt)
			return e, err
		})
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to query activity")
			return
		}

		http.PaginatedResponse(w, "entries", entries, total)
	}
}
