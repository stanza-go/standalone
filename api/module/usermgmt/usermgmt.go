// Package usermgmt provides admin endpoints for managing end-users.
// This is the first "app-level" admin module — it manages the actual
// users of the application (not admin accounts). It serves as a second
// CRUD reference alongside adminusers.
package usermgmt

import (
	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/validate"
	"github.com/stanza-go/standalone/module/adminaudit"
	"github.com/stanza-go/standalone/module/webhooks"
)

// Register mounts end-user management routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET    /api/admin/users                     - list users with search and pagination
//	POST   /api/admin/users                     - create a new user
//	GET    /api/admin/users/{id}                - get a single user
//	PUT    /api/admin/users/{id}                - update a user
//	DELETE /api/admin/users/{id}                - soft-delete a user
//	POST   /api/admin/users/{id}/impersonate    - generate access token as this user
//	GET    /api/admin/users/{id}/activity       - audit log entries for this user
//	GET    /api/admin/users/{id}/sessions       - active sessions for this user
//	GET    /api/admin/users/{id}/uploads        - uploads belonging to this user
func Register(admin *http.Group, a *auth.Auth, db *sqlite.DB, wh *webhooks.Dispatcher) {
	admin.HandleFunc("GET /users", listHandler(db))
	admin.HandleFunc("GET /users/export", exportHandler(db))
	admin.HandleFunc("POST /users", createHandler(db, wh))
	admin.HandleFunc("POST /users/bulk-delete", bulkDeleteHandler(db, wh))
	admin.HandleFunc("GET /users/{id}", getHandler(db))
	admin.HandleFunc("PUT /users/{id}", updateHandler(db, wh))
	admin.HandleFunc("DELETE /users/{id}", deleteHandler(db, wh))
	admin.HandleFunc("POST /users/{id}/impersonate", impersonateHandler(a, db))
	admin.HandleFunc("GET /users/{id}/activity", activityHandler(db))
	admin.HandleFunc("GET /users/{id}/sessions", sessionsHandler(db))
	admin.HandleFunc("GET /users/{id}/uploads", uploadsHandler(db))
}

type userJSON struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func scanUser(rows *sqlite.Rows) (userJSON, error) {
	var u userJSON
	if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return u, err
	}
	return u, nil
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pg := http.ParsePagination(r, 50, 100)
		search := r.URL.Query().Get("search")

		selectQ := sqlite.Select("id", "email", "name", "is_active", "created_at", "updated_at").
			From("users").
			WhereNull("deleted_at")
		selectQ.WhereSearch(search, "email", "name")

		total, _ := db.Count(selectQ)

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "email", "name", "is_active", "created_at", "updated_at"},
			"id", "DESC")
		sql, args := selectQ.OrderBy(sortCol, sortDir).Limit(pg.Limit).Offset(pg.Offset).Build()
		users, err := sqlite.QueryAll(db, sql, args, scanUser)
		if err != nil {
			http.WriteServerError(w, r, "failed to list users", err)
			return
		}

		http.PaginatedResponse(w, "users", users, total)
	}
}

func exportHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")

		q := sqlite.Select("id", "email", "name", "is_active", "created_at", "updated_at").
			From("users").
			WhereNull("deleted_at").
			WhereSearch(search, "email", "name")

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "email", "name", "is_active", "created_at", "updated_at"},
			"id", "DESC")

		sql, args := q.OrderBy(sortCol, sortDir).Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteServerError(w, r, "failed to export users", err)
			return
		}
		defer rows.Close()

		http.WriteCSV(w, "users", []string{"ID", "Email", "Name", "Active", "Created At", "Updated At"}, func() []string {
			if !rows.Next() {
				return nil
			}
			var id int64
			var email, name, createdAt, updatedAt string
			var isActive bool
			if err := rows.Scan(&id, &email, &name, &isActive, &createdAt, &updatedAt); err != nil {
				return nil
			}
			active := "No"
			if isActive {
				active = "Yes"
			}
			return []string{sqlite.FormatID(id), email, name, active, createdAt, updatedAt}
		})
	}
}

type createRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func createHandler(db *sqlite.DB, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createRequest
		if !http.BindJSON(w, r, &req) {
			return
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

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			http.WriteServerError(w, r, "failed to hash password", err)
			return
		}

		now := sqlite.Now()
		id, err := db.Insert(sqlite.Insert("users").
			Set("email", req.Email).
			Set("password", hash).
			Set("name", req.Name).
			Set("created_at", now).
			Set("updated_at", now))
		if err != nil {
			if sqlite.IsUniqueConstraintError(err) {
				http.WriteError(w, http.StatusConflict, "email already exists")
				return
			}
			http.WriteServerError(w, r, "failed to create user", err)
			return
		}

		adminaudit.Log(db, r, "user.create", "user", sqlite.FormatID(id), req.Email)

		_ = wh.Dispatch(r.Context(), "user.created", map[string]any{
			"id":    id,
			"email": req.Email,
			"name":  req.Name,
		})

		http.WriteJSON(w, http.StatusCreated, map[string]any{
			"user": userJSON{
				ID:        id,
				Email:     req.Email,
				Name:      req.Name,
				IsActive:  true,
				CreatedAt: now,
				UpdatedAt: now,
			},
		})
	}
}

func getHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		sql, args := sqlite.Select("id", "email", "name", "is_active", "created_at", "updated_at").
			From("users").
			Where("id = ?", id).
			WhereNull("deleted_at").
			Build()
		u, err := sqlite.QueryOne(db, sql, args, scanUser)
		if err != nil {
			http.WriteError(w, http.StatusNotFound, "user not found")
			return
		}

		// Count active sessions for this user.
		var sessionCount int
		sql, args = sqlite.Count("refresh_tokens").
			Where("entity_type = 'user'").
			Where("entity_id = ?", sqlite.FormatID(id)).
			Where("expires_at > ?", sqlite.Now()).
			Build()
		_ = db.QueryRow(sql, args...).Scan(&sessionCount)

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"user":           u,
			"active_sessions": sessionCount,
		})
	}
}

type updateRequest struct {
	Name     string `json:"name"`
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

		// Load current user.
		var currentEmail, currentName, createdAt string
		var currentActive bool
		sql, args := sqlite.Select("email", "name", "is_active", "created_at").
			From("users").
			Where("id = ?", id).
			WhereNull("deleted_at").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&currentEmail, &currentName, &currentActive, &createdAt); err != nil {
			http.WriteError(w, http.StatusNotFound, "user not found")
			return
		}

		// Merge updates.
		name := currentName
		if req.Name != "" {
			name = req.Name
		}
		isActive := currentActive
		if req.IsActive != nil {
			isActive = *req.IsActive
		}

		now := sqlite.Now()

		q := sqlite.Update("users").
			Set("name", name).
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
			http.WriteServerError(w, r, "failed to update user", err)
			return
		}

		// If deactivated, revoke all sessions.
		if req.IsActive != nil && !*req.IsActive {
			_, _ = db.Delete(sqlite.Delete("refresh_tokens").
				Where("entity_type = 'user'").
				Where("entity_id = ?", sqlite.FormatID(id)))
		}

		adminaudit.Log(db, r, "user.update", "user", sqlite.FormatID(id), currentEmail)

		_ = wh.Dispatch(r.Context(), "user.updated", map[string]any{
			"id":        id,
			"email":     currentEmail,
			"name":      name,
			"is_active": isActive,
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"user": userJSON{
				ID:        id,
				Email:     currentEmail,
				Name:      name,
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

		now := sqlite.Now()
		n, err := db.Update(sqlite.Update("users").
			Set("deleted_at", now).
			Set("is_active", false).
			Set("updated_at", now).
			Where("id = ?", id).
			WhereNull("deleted_at"))
		if err != nil {
			http.WriteServerError(w, r, "failed to delete user", err)
			return
		}
		if n == 0 {
			http.WriteError(w, http.StatusNotFound, "user not found")
			return
		}

		// Revoke all sessions for this user.
		_, _ = db.Delete(sqlite.Delete("refresh_tokens").
			Where("entity_type = 'user'").
			Where("entity_id = ?", sqlite.FormatID(id)))

		adminaudit.Log(db, r, "user.delete", "user", sqlite.FormatID(id), "")

		_ = wh.Dispatch(r.Context(), "user.deleted", map[string]any{
			"id": id,
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
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

		now := sqlite.Now()
		ids := make([]any, len(req.IDs))
		for i, id := range req.IDs {
			ids[i] = id
		}

		n, err := db.Update(sqlite.Update("users").
			Set("deleted_at", now).
			Set("is_active", false).
			Set("updated_at", now).
			WhereNull("deleted_at").
			WhereIn("id", ids...))
		if err != nil {
			http.WriteServerError(w, r, "failed to bulk delete users", err)
			return
		}

		// Revoke sessions for deleted users.
		idStrs := make([]any, len(req.IDs))
		for i, id := range req.IDs {
			idStrs[i] = sqlite.FormatID(id)
		}
		_, _ = db.Delete(sqlite.Delete("refresh_tokens").
			Where("entity_type = 'user'").
			WhereIn("entity_id", idStrs...))

		// Audit log each deletion.
		for _, id := range req.IDs {
			adminaudit.Log(db, r, "user.delete", "user", sqlite.FormatID(id), "bulk")
		}

		_ = wh.Dispatch(r.Context(), "user.bulk_deleted", map[string]any{
			"ids":      req.IDs,
			"affected": n,
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"affected": n,
		})
	}
}

// activityHandler returns audit log entries where this user is the target entity.
func activityHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		pg := http.ParsePagination(r, 20, 100)
		idStr := sqlite.FormatID(id)

		selectQ := sqlite.Select(
			"al.id", "al.admin_id", sqlite.CoalesceEmpty("a.email"), sqlite.CoalesceEmpty("a.name"),
			"al.action", "al.details", "al.ip_address", "al.created_at",
		).From("audit_log al").
			LeftJoin("admins a", "a.id = CAST(al.admin_id AS INTEGER)").
			Where("al.entity_type = 'user'").
			Where("al.entity_id = ?", idStr)

		total, _ := db.Count(selectQ)

		sql, args := selectQ.
			OrderBy("al.created_at", "DESC").
			Limit(pg.Limit).Offset(pg.Offset).
			Build()
		type entry struct {
			ID         int64  `json:"id"`
			AdminID    string `json:"admin_id"`
			AdminEmail string `json:"admin_email"`
			AdminName  string `json:"admin_name"`
			Action     string `json:"action"`
			Details    string `json:"details"`
			IPAddress  string `json:"ip_address"`
			CreatedAt  string `json:"created_at"`
		}
		entries, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (entry, error) {
			var e entry
			err := rows.Scan(&e.ID, &e.AdminID, &e.AdminEmail, &e.AdminName,
				&e.Action, &e.Details, &e.IPAddress, &e.CreatedAt)
			return e, err
		})
		if err != nil {
			http.WriteServerError(w, r, "failed to query activity", err)
			return
		}

		http.PaginatedResponse(w, "entries", entries, total)
	}
}

// sessionsHandler returns active refresh tokens for this user.
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
			Where("entity_type = 'user'").
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

// uploadsHandler returns uploads belonging to this user.
func uploadsHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		pg := http.ParsePagination(r, 20, 100)
		idStr := sqlite.FormatID(id)

		selectQ := sqlite.Select(
			"id", "uuid", "original_name", "content_type",
			"size_bytes", "has_thumbnail", "created_at",
		).From("uploads").
			Where("entity_type = 'user'").
			Where("entity_id = ?", idStr).
			WhereNull("deleted_at")

		total, _ := db.Count(selectQ)

		sql, args := selectQ.
			OrderBy("created_at", "DESC").
			Limit(pg.Limit).Offset(pg.Offset).
			Build()
		type uploadEntry struct {
			ID           int64  `json:"id"`
			UUID         string `json:"uuid"`
			OriginalName string `json:"original_name"`
			ContentType  string `json:"content_type"`
			SizeBytes    int64  `json:"size_bytes"`
			HasThumbnail bool   `json:"has_thumbnail"`
			CreatedAt    string `json:"created_at"`
		}
		uploads, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (uploadEntry, error) {
			var u uploadEntry
			if err := rows.Scan(&u.ID, &u.UUID, &u.OriginalName, &u.ContentType,
				&u.SizeBytes, &u.HasThumbnail, &u.CreatedAt); err != nil {
				return u, err
			}
			return u, nil
		})
		if err != nil {
			http.WriteServerError(w, r, "failed to query uploads", err)
			return
		}

		http.PaginatedResponse(w, "uploads", uploads, total)
	}
}

// impersonateHandler generates a short-lived access token as if the
// admin were the target user. This is for debugging purposes — the token
// contains the user's ID and "user" scope. No refresh token is created
// so the impersonation expires naturally.
func impersonateHandler(a *auth.Auth, db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		// Verify user exists and is active.
		var email, name string
		var isActive bool
		sql, args := sqlite.Select("email", "name", "is_active").
			From("users").
			Where("id = ?", id).
			WhereNull("deleted_at").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&email, &name, &isActive); err != nil {
			http.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		if !isActive {
			http.WriteError(w, http.StatusBadRequest, "cannot impersonate an inactive user")
			return
		}

		uid := sqlite.FormatID(id)
		token, err := a.IssueAccessToken(uid, []string{"user"})
		if err != nil {
			http.WriteServerError(w, r, "failed to issue token", err)
			return
		}

		adminaudit.Log(db, r, "user.impersonate", "user", sqlite.FormatID(id), email)

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"token": token,
			"user": map[string]any{
				"id":    id,
				"email": email,
				"name":  name,
			},
		})
	}
}
