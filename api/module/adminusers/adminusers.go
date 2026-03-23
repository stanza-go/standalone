// Package adminusers provides admin CRUD endpoints for managing admin
// accounts. It supports listing, creating, updating, and soft-deleting
// admins with pagination and self-action protection.
package adminusers

import (
	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/validate"
	"github.com/stanza-go/standalone/module/adminaudit"
	"github.com/stanza-go/standalone/module/adminroles"
	"github.com/stanza-go/standalone/module/webhooks"
)

// Register mounts the admin user management routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET    /api/admin/admins               - list admins with pagination
//	POST   /api/admin/admins               - create a new admin
//	GET    /api/admin/admins/{id}          - get a single admin
//	PUT    /api/admin/admins/{id}          - update an admin
//	DELETE /api/admin/admins/{id}          - soft-delete an admin
//	GET    /api/admin/admins/{id}/activity - audit log entries by this admin
//	GET    /api/admin/admins/{id}/sessions - active sessions for this admin
func Register(admin *http.Group, db *sqlite.DB, wh *webhooks.Dispatcher) {
	admin.HandleFunc("GET /admins", listHandler(db))
	admin.HandleFunc("GET /admins/export", exportHandler(db))
	admin.HandleFunc("POST /admins", createHandler(db, wh))
	admin.HandleFunc("POST /admins/bulk-delete", bulkDeleteHandler(db, wh))
	admin.HandleFunc("GET /admins/{id}", getHandler(db))
	admin.HandleFunc("PUT /admins/{id}", updateHandler(db, wh))
	admin.HandleFunc("DELETE /admins/{id}", deleteHandler(db, wh))
	admin.HandleFunc("GET /admins/{id}/activity", activityHandler(db))
	admin.HandleFunc("GET /admins/{id}/sessions", sessionsHandler(db))
}

type adminJSON struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func scanAdmin(rows *sqlite.Rows) (adminJSON, error) {
	var a adminJSON
	if err := rows.Scan(&a.ID, &a.Email, &a.Name, &a.Role, &a.IsActive, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return a, err
	}
	return a, nil
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pg := http.ParsePagination(r, 50, 100)
		search := r.URL.Query().Get("search")

		selectQ := sqlite.Select("id", "email", "name", "role", "is_active", "created_at", "updated_at").
			From("admins").
			WhereNull("deleted_at")
		selectQ.WhereSearch(search, "email", "name")

		total, _ := db.Count(selectQ)

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "email", "name", "role", "is_active", "created_at", "updated_at"},
			"id", "ASC")
		sql, args := selectQ.
			OrderBy(sortCol, sortDir).
			Limit(pg.Limit).
			Offset(pg.Offset).
			Build()
		admins, err := sqlite.QueryAll(db, sql, args, scanAdmin)
		if err != nil {
			http.WriteServerError(w, r, "failed to list admins", err)
			return
		}

		http.PaginatedResponse(w, "admins", admins, total)
	}
}

func exportHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")

		q := sqlite.Select("id", "email", "name", "role", "is_active", "created_at", "updated_at").
			From("admins").
			WhereNull("deleted_at").
			WhereSearch(search, "email", "name")

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "email", "name", "role", "is_active", "created_at", "updated_at"},
			"id", "ASC")

		sql, args := q.OrderBy(sortCol, sortDir).Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteServerError(w, r, "failed to export admins", err)
			return
		}
		defer rows.Close()

		http.WriteCSV(w, "admins", []string{"ID", "Email", "Name", "Role", "Active", "Created At", "Updated At"}, func() []string {
			if !rows.Next() {
				return nil
			}
			var id int64
			var email, name, role, createdAt, updatedAt string
			var isActive bool
			if err := rows.Scan(&id, &email, &name, &role, &isActive, &createdAt, &updatedAt); err != nil {
				return nil
			}
			active := "No"
			if isActive {
				active = "Yes"
			}
			return []string{sqlite.FormatID(id), email, name, role, active, createdAt, updatedAt}
		})
	}
}

type createRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

func createHandler(db *sqlite.DB, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createRequest
		if !http.BindJSON(w, r, &req) {
			return
		}

		if req.Role == "" {
			req.Role = "admin"
		}

		v := validate.Fields(
			validate.Required("email", req.Email),
			validate.Email("email", req.Email),
			validate.Required("password", req.Password),
			validate.MinLen("password", req.Password, 8),
		)
		if v.HasErrors() {
			v.WriteError(w)
			return
		}
		if !adminroles.ValidateRoleExists(db, req.Role) {
			http.WriteError(w, http.StatusUnprocessableEntity, "invalid role: "+req.Role)
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			http.WriteServerError(w, r, "failed to hash password", err)
			return
		}

		now := sqlite.Now()
		id, err := db.Insert(sqlite.Insert("admins").
			Set("email", req.Email).
			Set("password", hash).
			Set("name", req.Name).
			Set("role", req.Role).
			Set("created_at", now).
			Set("updated_at", now))
		if err != nil {
			if sqlite.IsUniqueConstraintError(err) {
				http.WriteError(w, http.StatusConflict, "email already exists")
				return
			}
			http.WriteServerError(w, r, "failed to create admin", err)
			return
		}

		adminaudit.Log(db, r, "admin.create", "admin", sqlite.FormatID(id), req.Email)

		_ = wh.Dispatch(r.Context(), webhooks.EventAdminCreated, map[string]any{
			"id":    id,
			"email": req.Email,
			"name":  req.Name,
			"role":  req.Role,
		})

		http.WriteJSON(w, http.StatusCreated, map[string]any{
			"admin": adminJSON{
				ID:        id,
				Email:     req.Email,
				Name:      req.Name,
				Role:      req.Role,
				IsActive:  true,
				CreatedAt: now,
				UpdatedAt: now,
			},
		})
	}
}

type updateRequest struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	IsActive *bool  `json:"is_active"`
	Password string `json:"password"`
}

func updateHandler(db *sqlite.DB, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		var req updateRequest
		if !http.BindJSON(w, r, &req) {
			return
		}

		if req.Role != "" && !adminroles.ValidateRoleExists(db, req.Role) {
			http.WriteError(w, http.StatusBadRequest, "invalid role: "+req.Role)
			return
		}

		// Prevent self-deactivation.
		if req.IsActive != nil && !*req.IsActive {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if ok && claims.IntUID() == id {
				http.WriteError(w, http.StatusBadRequest, "cannot deactivate your own account")
				return
			}
		}

		// Load current admin.
		var currentEmail, currentName, currentRole, createdAt string
		var currentActive bool
		sql, args := sqlite.Select("email", "name", "role", "is_active", "created_at").
			From("admins").
			Where("id = ?", id).
			WhereNull("deleted_at").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&currentEmail, &currentName, &currentRole, &currentActive, &createdAt); err != nil {
			http.WriteError(w, http.StatusNotFound, "admin not found")
			return
		}

		// Merge updates.
		name := currentName
		if req.Name != "" {
			name = req.Name
		}
		role := currentRole
		if req.Role != "" {
			role = req.Role
		}
		isActive := currentActive
		if req.IsActive != nil {
			isActive = *req.IsActive
		}

		now := sqlite.Now()

		q := sqlite.Update("admins").
			Set("name", name).
			Set("role", role).
			Set("is_active", isActive)
		if req.Password != "" {
			hash, err := auth.HashPassword(req.Password)
			if err != nil {
				http.WriteServerError(w, r, "failed to hash password", err)
				return
			}
			q.Set("password", hash)
		}
		if _, err := db.Update(q.Set("updated_at", now).
			Where("id = ?", id).
			WhereNull("deleted_at")); err != nil {
			http.WriteServerError(w, r, "failed to update admin", err)
			return
		}

		adminaudit.Log(db, r, "admin.update", "admin", sqlite.FormatID(id), currentEmail)

		_ = wh.Dispatch(r.Context(), webhooks.EventAdminUpdated, map[string]any{
			"id":        id,
			"email":     currentEmail,
			"name":      name,
			"role":      role,
			"is_active": isActive,
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"admin": adminJSON{
				ID:        id,
				Email:     currentEmail,
				Name:      name,
				Role:      role,
				IsActive:  isActive,
				CreatedAt: createdAt,
				UpdatedAt: now,
			},
		})
	}
}

func deleteHandler(db *sqlite.DB, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		// Prevent self-deletion.
		claims, ok := auth.ClaimsFromContext(r.Context())
		if ok && claims.IntUID() == id {
			http.WriteError(w, http.StatusBadRequest, "cannot delete your own account")
			return
		}

		now := sqlite.Now()
		n, err := db.Update(sqlite.Update("admins").
			Set("deleted_at", now).
			Set("is_active", false).
			Set("updated_at", now).
			Where("id = ?", id).
			WhereNull("deleted_at"))
		if err != nil {
			http.WriteServerError(w, r, "failed to delete admin", err)
			return
		}
		if n == 0 {
			http.WriteError(w, http.StatusNotFound, "admin not found")
			return
		}

		// Revoke all sessions for this admin.
		_, _ = db.Delete(sqlite.Delete("refresh_tokens").
			Where("entity_type = 'admin'").
			Where("entity_id = ?", sqlite.FormatID(id)))

		adminaudit.Log(db, r, "admin.delete", "admin", sqlite.FormatID(id), "")

		_ = wh.Dispatch(r.Context(), webhooks.EventAdminDeleted, map[string]any{
			"id": id,
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}

// getHandler returns a single admin by ID with active session count.
func getHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		sql, args := sqlite.Select("id", "email", "name", "role", "is_active", "created_at", "updated_at").
			From("admins").
			Where("id = ?", id).
			WhereNull("deleted_at").
			Build()
		a, err := sqlite.QueryOne(db, sql, args, scanAdmin)
		if err != nil {
			http.WriteError(w, http.StatusNotFound, "admin not found")
			return
		}

		var sessionCount int
		sql, args = sqlite.Count("refresh_tokens").
			Where("entity_type = 'admin'").
			Where("entity_id = ?", sqlite.FormatID(id)).
			Where("expires_at > ?", sqlite.Now()).
			Build()
		_ = db.QueryRow(sql, args...).Scan(&sessionCount)

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"admin":           a,
			"active_sessions": sessionCount,
		})
	}
}

// activityHandler returns audit log entries performed by this admin.
func activityHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		pg := http.ParsePagination(r, 20, 100)
		idStr := sqlite.FormatID(id)

		selectQ := sqlite.Select(
			"id", "action", "entity_type", "entity_id", "details", "ip_address", "created_at",
		).From("audit_log").
			Where("admin_id = ?", idStr)

		total, _ := db.Count(selectQ)

		sql, args := selectQ.
			OrderBy("created_at", "DESC").
			Limit(pg.Limit).Offset(pg.Offset).
			Build()
		type entry struct {
			ID         int64  `json:"id"`
			Action     string `json:"action"`
			EntityType string `json:"entity_type"`
			EntityID   string `json:"entity_id"`
			Details    string `json:"details"`
			IPAddress  string `json:"ip_address"`
			CreatedAt  string `json:"created_at"`
		}
		entries, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (entry, error) {
			var e entry
			err := rows.Scan(&e.ID, &e.Action, &e.EntityType, &e.EntityID,
				&e.Details, &e.IPAddress, &e.CreatedAt)
			return e, err
		})
		if err != nil {
			http.WriteServerError(w, r, "failed to query activity", err)
			return
		}

		http.PaginatedResponse(w, "entries", entries, total)
	}
}

// sessionsHandler returns active refresh tokens for this admin.
func sessionsHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		idStr := sqlite.FormatID(id)
		now := sqlite.Now()

		sql, args := sqlite.Select("id", "created_at", "expires_at").
			From("refresh_tokens").
			Where("entity_type = 'admin'").
			Where("entity_id = ?", idStr).
			Where("expires_at > ?", now).
			OrderBy("created_at", "DESC").
			Build()
		type session struct {
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
			ExpiresAt string `json:"expires_at"`
		}
		sessions, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (session, error) {
			var s session
			err := rows.Scan(&s.ID, &s.CreatedAt, &s.ExpiresAt)
			return s, err
		})
		if err != nil {
			http.WriteServerError(w, r, "failed to query sessions", err)
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"sessions": sessions,
			"total":    len(sessions),
		})
	}
}

func bulkDeleteHandler(db *sqlite.DB, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs []int64 `json:"ids"`
		}
		if !http.BindJSON(w, r, &req) {
			return
		}
		if !http.CheckBulkIDs(w, req.IDs, 100) {
			return
		}

		// Prevent self-deletion.
		claims, ok := auth.ClaimsFromContext(r.Context())
		if ok {
			for _, id := range req.IDs {
				if claims.IntUID() == id {
					http.WriteError(w, http.StatusBadRequest, "cannot delete your own account")
					return
				}
			}
		}

		now := sqlite.Now()
		ids := make([]any, len(req.IDs))
		for i, id := range req.IDs {
			ids[i] = id
		}

		n, err := db.Update(sqlite.Update("admins").
			Set("deleted_at", now).
			Set("is_active", false).
			Set("updated_at", now).
			WhereNull("deleted_at").
			WhereIn("id", ids...))
		if err != nil {
			http.WriteServerError(w, r, "failed to bulk delete admins", err)
			return
		}

		// Revoke sessions for deleted admins.
		idStrs := make([]any, len(req.IDs))
		for i, id := range req.IDs {
			idStrs[i] = sqlite.FormatID(id)
		}
		_, _ = db.Delete(sqlite.Delete("refresh_tokens").
			Where("entity_type = 'admin'").
			WhereIn("entity_id", idStrs...))

		for _, id := range req.IDs {
			adminaudit.Log(db, r, "admin.delete", "admin", sqlite.FormatID(id), "bulk")
		}

		_ = wh.Dispatch(r.Context(), webhooks.EventAdminBulkDeleted, map[string]any{
			"ids":      req.IDs,
			"affected": n,
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"affected": n,
		})
	}
}


