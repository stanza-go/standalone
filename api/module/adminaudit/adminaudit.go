// Package adminaudit provides admin audit logging and the audit log viewer
// endpoint. It records admin actions (create, update, delete, etc.) and
// exposes them through a paginated, filterable list.
package adminaudit

import (
	nethttp "net/http"
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
)

// Register mounts the audit log routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET /api/admin/audit         — list audit entries with pagination and filtering
//	GET /api/admin/audit/recent  — last 10 entries for the dashboard activity feed
func Register(admin *http.Group, db *sqlite.DB) {
	admin.HandleFunc("GET /audit", listHandler(db))
	admin.HandleFunc("GET /audit/recent", recentHandler(db))
}

// Log records an admin action in the audit log. It extracts the admin ID
// from the request context (JWT claims) and the client IP from the request.
// This function is intentionally fire-and-forget — errors are silently
// ignored because audit logging should never block the primary operation.
func Log(db *sqlite.DB, r *nethttp.Request, action, entityType, entityID, details string) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	ip := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip = strings.TrimSpace(parts[0])
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	sql, args := sqlite.Insert("audit_log").
		Set("admin_id", claims.UID).
		Set("action", action).
		Set("entity_type", entityType).
		Set("entity_id", entityID).
		Set("details", details).
		Set("ip_address", ip).
		Set("created_at", now).
		Build()
	_, _ = db.Exec(sql, args...)
}

type entryJSON struct {
	ID         int64  `json:"id"`
	AdminID    string `json:"admin_id"`
	AdminEmail string `json:"admin_email"`
	AdminName  string `json:"admin_name"`
	Action     string `json:"action"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Details    string `json:"details"`
	IPAddress  string `json:"ip_address"`
	CreatedAt  string `json:"created_at"`
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := http.QueryParamInt(r, "limit", 50)
		offset := http.QueryParamInt(r, "offset", 0)
		action := r.URL.Query().Get("action")
		adminID := r.URL.Query().Get("admin_id")
		search := r.URL.Query().Get("search")
		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")

		// Count query.
		countSQL := "SELECT count(*) FROM audit_log"
		var countArgs []any
		var conditions []string

		if action != "" {
			conditions = append(conditions, "audit_log.action = ?")
			countArgs = append(countArgs, action)
		}
		if adminID != "" {
			conditions = append(conditions, "audit_log.admin_id = ?")
			countArgs = append(countArgs, adminID)
		}
		if search != "" {
			conditions = append(conditions, "(audit_log.details LIKE ? ESCAPE '\\' OR audit_log.action LIKE ? ESCAPE '\\')")
			like := "%" + escapeLike(search) + "%"
			countArgs = append(countArgs, like, like)
		}
		if from != "" {
			conditions = append(conditions, "audit_log.created_at >= ?")
			countArgs = append(countArgs, from)
		}
		if to != "" {
			conditions = append(conditions, "audit_log.created_at <= ?")
			countArgs = append(countArgs, to)
		}

		if len(conditions) > 0 {
			countSQL += " WHERE " + strings.Join(conditions, " AND ")
		}

		var total int
		_ = db.QueryRow(countSQL, countArgs...).Scan(&total)

		// Select query with LEFT JOIN for admin info.
		selectSQL := `SELECT audit_log.id, audit_log.admin_id, COALESCE(admins.email, ''), COALESCE(admins.name, ''),
			audit_log.action, audit_log.entity_type, audit_log.entity_id,
			audit_log.details, audit_log.ip_address, audit_log.created_at
			FROM audit_log
			LEFT JOIN admins ON admins.id = CAST(audit_log.admin_id AS INTEGER)`

		var selectArgs []any
		if len(conditions) > 0 {
			selectSQL += " WHERE " + strings.Join(conditions, " AND ")
			selectArgs = append(selectArgs, countArgs...)
		}

		selectSQL += " ORDER BY audit_log.id DESC LIMIT ? OFFSET ?"
		selectArgs = append(selectArgs, limit, offset)

		rows, err := db.Query(selectSQL, selectArgs...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to list audit entries")
			return
		}
		defer rows.Close()

		entries := make([]entryJSON, 0)
		for rows.Next() {
			var e entryJSON
			if err := rows.Scan(&e.ID, &e.AdminID, &e.AdminEmail, &e.AdminName,
				&e.Action, &e.EntityType, &e.EntityID,
				&e.Details, &e.IPAddress, &e.CreatedAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan audit entry")
				return
			}
			entries = append(entries, e)
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"entries": entries,
			"total":   total,
		})
	}
}

// escapeLike escapes LIKE wildcards (% and _) in a search term so they
// are matched literally when used with ESCAPE '\'.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// recentHandler returns the last 10 audit entries for the dashboard
// activity feed. Lightweight endpoint — no pagination, no filtering.
func recentHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`SELECT audit_log.id, audit_log.admin_id, COALESCE(admins.email, ''), COALESCE(admins.name, ''),
			audit_log.action, audit_log.entity_type, audit_log.entity_id,
			audit_log.details, audit_log.ip_address, audit_log.created_at
			FROM audit_log
			LEFT JOIN admins ON admins.id = CAST(audit_log.admin_id AS INTEGER)
			ORDER BY audit_log.id DESC LIMIT 10`)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to list recent activity")
			return
		}
		defer rows.Close()

		entries := make([]entryJSON, 0)
		for rows.Next() {
			var e entryJSON
			if err := rows.Scan(&e.ID, &e.AdminID, &e.AdminEmail, &e.AdminName,
				&e.Action, &e.EntityType, &e.EntityID,
				&e.Details, &e.IPAddress, &e.CreatedAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan audit entry")
				return
			}
			entries = append(entries, e)
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"entries": entries,
		})
	}
}
