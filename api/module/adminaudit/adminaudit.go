// Package adminaudit provides admin audit logging and the audit log viewer
// endpoint. It records admin actions (create, update, delete, etc.) and
// exposes them through a paginated, filterable list.
package adminaudit

import (
	nethttp "net/http"
	"strconv"
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
	admin.HandleFunc("GET /audit/export", exportHandler(db))
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

// buildAuditSelect returns a SelectBuilder for audit entries with the admin
// LEFT JOIN and all applicable filters from query parameters.
func buildAuditSelect(r *http.Request) *sqlite.SelectBuilder {
	q := sqlite.Select(
		"audit_log.id", "audit_log.admin_id",
		"COALESCE(admins.email, '')", "COALESCE(admins.name, '')",
		"audit_log.action", "audit_log.entity_type", "audit_log.entity_id",
		"audit_log.details", "audit_log.ip_address", "audit_log.created_at",
	).From("audit_log").
		LeftJoin("admins", "admins.id = CAST(audit_log.admin_id AS INTEGER)")

	if action := r.URL.Query().Get("action"); action != "" {
		q.Where("audit_log.action = ?", action)
	}
	if adminID := r.URL.Query().Get("admin_id"); adminID != "" {
		q.Where("audit_log.admin_id = ?", adminID)
	}
	if search := r.URL.Query().Get("search"); search != "" {
		q.WhereSearch(search, "audit_log.details", "audit_log.action")
	}
	if from := r.URL.Query().Get("from"); from != "" {
		q.Where("audit_log.created_at >= ?", from)
	}
	if to := r.URL.Query().Get("to"); to != "" {
		q.Where("audit_log.created_at <= ?", to)
	}

	return q
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pg := http.ParsePagination(r, 50, 100)
		selectQ := buildAuditSelect(r)

		var total int
		sql, args := sqlite.CountFrom(selectQ).Build()
		_ = db.QueryRow(sql, args...).Scan(&total)

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "action", "entity_type", "created_at", "admin_id"},
			"id", "DESC")
		sql, args = selectQ.OrderBy("audit_log."+sortCol, sortDir).Limit(pg.Limit).Offset(pg.Offset).Build()
		rows, err := db.Query(sql, args...)
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
		if err := rows.Err(); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to iterate audit entries")
			return
		}

		http.PaginatedResponse(w, "entries", entries, total)
	}
}

func exportHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		selectQ := buildAuditSelect(r)

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "action", "entity_type", "created_at", "admin_id"},
			"id", "DESC")
		sql, args := selectQ.OrderBy("audit_log."+sortCol, sortDir).Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to export audit log")
			return
		}
		defer rows.Close()

		http.WriteCSV(w, "audit-log", []string{"ID", "Admin ID", "Admin Email", "Admin Name", "Action", "Entity Type", "Entity ID", "Details", "IP Address", "Created At"}, func() []string {
			if !rows.Next() {
				return nil
			}
			var id int64
			var adminIDVal, adminEmail, adminName, actionVal, entityType, entityID, details, ipAddress, createdAt string
			if err := rows.Scan(&id, &adminIDVal, &adminEmail, &adminName, &actionVal, &entityType, &entityID, &details, &ipAddress, &createdAt); err != nil {
				return nil
			}
			return []string{strconv.FormatInt(id, 10), adminIDVal, adminEmail, adminName, actionVal, entityType, entityID, details, ipAddress, createdAt}
		})
	}
}

// recentHandler returns the last 10 audit entries for the dashboard
// activity feed. Lightweight endpoint — no pagination, no filtering.
func recentHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sql, args := sqlite.Select(
			"audit_log.id", "audit_log.admin_id",
			"COALESCE(admins.email, '')", "COALESCE(admins.name, '')",
			"audit_log.action", "audit_log.entity_type", "audit_log.entity_id",
			"audit_log.details", "audit_log.ip_address", "audit_log.created_at",
		).From("audit_log").
			LeftJoin("admins", "admins.id = CAST(audit_log.admin_id AS INTEGER)").
			OrderBy("audit_log.id", "DESC").
			Limit(10).
			Build()
		rows, err := db.Query(sql, args...)
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
		if err := rows.Err(); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to iterate recent entries")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"entries": entries,
		})
	}
}
