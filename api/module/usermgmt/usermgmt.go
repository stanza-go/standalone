// Package usermgmt provides admin endpoints for managing end-users.
// This is the first "app-level" admin module — it manages the actual
// users of the application (not admin accounts). It serves as a second
// CRUD reference alongside adminusers.
package usermgmt

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

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

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pg := http.ParsePagination(r, 50, 100)
		search := r.URL.Query().Get("search")

		selectQ := sqlite.Select("id", "email", "name", "is_active", "created_at", "updated_at").
			From("users").
			Where("deleted_at IS NULL")
		if search != "" {
			like := "%" + sqlite.EscapeLike(search) + "%"
			selectQ.Where("(email LIKE ? ESCAPE '\\' OR name LIKE ? ESCAPE '\\')", like, like)
		}

		var total int
		sql, args := sqlite.CountFrom(selectQ).Build()
		_ = db.QueryRow(sql, args...).Scan(&total)

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "email", "name", "is_active", "created_at", "updated_at"},
			"id", "DESC")
		sql, args = selectQ.OrderBy(sortCol, sortDir).Limit(pg.Limit).Offset(pg.Offset).Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to list users")
			return
		}
		defer rows.Close()

		users := make([]userJSON, 0)
		for rows.Next() {
			var u userJSON
			var isActive int
			if err := rows.Scan(&u.ID, &u.Email, &u.Name, &isActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan user")
				return
			}
			u.IsActive = isActive == 1
			users = append(users, u)
		}
		if err := rows.Err(); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to iterate users")
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
			Where("deleted_at IS NULL")
		if search != "" {
			like := "%" + sqlite.EscapeLike(search) + "%"
			q.Where("(email LIKE ? ESCAPE '\\' OR name LIKE ? ESCAPE '\\')", like, like)
		}

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "email", "name", "is_active", "created_at", "updated_at"},
			"id", "DESC")

		sql, args := q.OrderBy(sortCol, sortDir).Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to export users")
			return
		}
		defer rows.Close()

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=users-%s.csv", time.Now().UTC().Format("20060102")))
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"ID", "Email", "Name", "Active", "Created At", "Updated At"})

		for rows.Next() {
			var id int64
			var email, name, createdAt, updatedAt string
			var isActive int
			if err := rows.Scan(&id, &email, &name, &isActive, &createdAt, &updatedAt); err != nil {
				break
			}
			active := "No"
			if isActive == 1 {
				active = "Yes"
			}
			_ = cw.Write([]string{strconv.FormatInt(id, 10), email, name, active, createdAt, updatedAt})
		}
		cw.Flush()
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
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
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
			http.WriteError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		sql, args := sqlite.Insert("users").
			Set("email", req.Email).
			Set("password", hash).
			Set("name", req.Name).
			Set("created_at", now).
			Set("updated_at", now).
			Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				http.WriteError(w, http.StatusConflict, "email already exists")
				return
			}
			http.WriteError(w, http.StatusInternalServerError, "failed to create user")
			return
		}

		adminaudit.Log(db, r, "user.create", "user", strconv.FormatInt(result.LastInsertID, 10), req.Email)

		_ = wh.Dispatch(r.Context(), "user.created", map[string]any{
			"id":    result.LastInsertID,
			"email": req.Email,
			"name":  req.Name,
		})

		http.WriteJSON(w, http.StatusCreated, map[string]any{
			"user": userJSON{
				ID:        result.LastInsertID,
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
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid user id")
			return
		}

		var u userJSON
		var isActive int
		sql, args := sqlite.Select("id", "email", "name", "is_active", "created_at", "updated_at").
			From("users").
			Where("id = ?", id).
			Where("deleted_at IS NULL").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&u.ID, &u.Email, &u.Name, &isActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			http.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		u.IsActive = isActive == 1

		// Count active sessions for this user.
		var sessionCount int
		sql, args = sqlite.Count("refresh_tokens").
			Where("entity_type = 'user'").
			Where("entity_id = ?", strconv.FormatInt(id, 10)).
			Where("expires_at > ?", time.Now().UTC().Format(time.RFC3339)).
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
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid user id")
			return
		}

		var req updateRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Load current user.
		var currentEmail, currentName, createdAt string
		var currentActive int
		sql, args := sqlite.Select("email", "name", "is_active", "created_at").
			From("users").
			Where("id = ?", id).
			Where("deleted_at IS NULL").
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
			if *req.IsActive {
				isActive = 1
			} else {
				isActive = 0
			}
		}

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

		q := sqlite.Update("users").
			Set("name", name).
			Set("is_active", isActive)
		if req.Password != "" {
			hash, err := auth.HashPassword(req.Password)
			if err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to hash password")
				return
			}
			q.Set("password", hash)
		}
		sql, args = q.Set("updated_at", now).
			Where("id = ?", id).
			Where("deleted_at IS NULL").
			Build()
		if _, err := db.Exec(sql, args...); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to update user")
			return
		}

		// If deactivated, revoke all sessions.
		if req.IsActive != nil && !*req.IsActive {
			sql, args = sqlite.Delete("refresh_tokens").
				Where("entity_type = 'user'").
				Where("entity_id = ?", strconv.FormatInt(id, 10)).
				Build()
			_, _ = db.Exec(sql, args...)
		}

		adminaudit.Log(db, r, "user.update", "user", strconv.FormatInt(id, 10), currentEmail)

		_ = wh.Dispatch(r.Context(), "user.updated", map[string]any{
			"id":        id,
			"email":     currentEmail,
			"name":      name,
			"is_active": isActive == 1,
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"user": userJSON{
				ID:        id,
				Email:     currentEmail,
				Name:      name,
				IsActive:  isActive == 1,
				CreatedAt: createdAt,
				UpdatedAt: now,
			},
		})
	}
}

func deleteHandler(db *sqlite.DB, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid user id")
			return
		}

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		sql, args := sqlite.Update("users").
			Set("deleted_at", now).
			Set("is_active", 0).
			Set("updated_at", now).
			Where("id = ?", id).
			Where("deleted_at IS NULL").
			Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to delete user")
			return
		}
		if result.RowsAffected == 0 {
			http.WriteError(w, http.StatusNotFound, "user not found")
			return
		}

		// Revoke all sessions for this user.
		sql, args = sqlite.Delete("refresh_tokens").
			Where("entity_type = 'user'").
			Where("entity_id = ?", strconv.FormatInt(id, 10)).
			Build()
		_, _ = db.Exec(sql, args...)

		adminaudit.Log(db, r, "user.delete", "user", strconv.FormatInt(id, 10), "")

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
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
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

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		ids := make([]any, len(req.IDs))
		for i, id := range req.IDs {
			ids[i] = id
		}

		query, args := sqlite.Update("users").
			Set("deleted_at", now).
			Set("is_active", 0).
			Set("updated_at", now).
			Where("deleted_at IS NULL").
			WhereIn("id", ids...).
			Build()
		result, err := db.Exec(query, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to bulk delete users")
			return
		}

		// Revoke sessions for deleted users.
		for _, id := range req.IDs {
			idStr := strconv.FormatInt(id, 10)
			sql, a := sqlite.Delete("refresh_tokens").
				Where("entity_type = 'user'").
				Where("entity_id = ?", idStr).
				Build()
			_, _ = db.Exec(sql, a...)
		}

		// Audit log each deletion.
		for _, id := range req.IDs {
			adminaudit.Log(db, r, "user.delete", "user", strconv.FormatInt(id, 10), "bulk")
		}

		_ = wh.Dispatch(r.Context(), "user.bulk_deleted", map[string]any{
			"ids":      req.IDs,
			"affected": result.RowsAffected,
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"affected": result.RowsAffected,
		})
	}
}

// activityHandler returns audit log entries where this user is the target entity.
func activityHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid user id")
			return
		}

		pg := http.ParsePagination(r, 20, 100)
		idStr := strconv.FormatInt(id, 10)

		selectQ := sqlite.Select(
			"al.id", "al.admin_id", "COALESCE(a.email, '')", "COALESCE(a.name, '')",
			"al.action", "al.details", "al.ip_address", "al.created_at",
		).From("audit_log al").
			LeftJoin("admins a", "a.id = CAST(al.admin_id AS INTEGER)").
			Where("al.entity_type = 'user'").
			Where("al.entity_id = ?", idStr)

		var total int
		sql, args := sqlite.CountFrom(selectQ).Build()
		_ = db.QueryRow(sql, args...).Scan(&total)

		sql, args = selectQ.
			OrderBy("al.created_at", "DESC").
			Limit(pg.Limit).Offset(pg.Offset).
			Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to query activity")
			return
		}
		defer rows.Close()

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
		entries := make([]entry, 0)
		for rows.Next() {
			var e entry
			if err := rows.Scan(&e.ID, &e.AdminID, &e.AdminEmail, &e.AdminName,
				&e.Action, &e.Details, &e.IPAddress, &e.CreatedAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan activity")
				return
			}
			entries = append(entries, e)
		}
		if err := rows.Err(); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to iterate activity")
			return
		}

		http.PaginatedResponse(w, "entries", entries, total)
	}
}

// sessionsHandler returns active refresh tokens for this user.
func sessionsHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid user id")
			return
		}

		idStr := strconv.FormatInt(id, 10)
		now := time.Now().UTC().Format(time.RFC3339)

		sql, args := sqlite.Select("id", "created_at", "expires_at").
			From("refresh_tokens").
			Where("entity_type = 'user'").
			Where("entity_id = ?", idStr).
			Where("expires_at > ?", now).
			OrderBy("created_at", "DESC").
			Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to query sessions")
			return
		}
		defer rows.Close()

		type session struct {
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
			ExpiresAt string `json:"expires_at"`
		}
		sessions := make([]session, 0)
		for rows.Next() {
			var s session
			if err := rows.Scan(&s.ID, &s.CreatedAt, &s.ExpiresAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan session")
				return
			}
			sessions = append(sessions, s)
		}
		if err := rows.Err(); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to iterate sessions")
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
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid user id")
			return
		}

		pg := http.ParsePagination(r, 20, 100)
		idStr := strconv.FormatInt(id, 10)

		selectQ := sqlite.Select(
			"id", "uuid", "original_name", "content_type",
			"size_bytes", "has_thumbnail", "created_at",
		).From("uploads").
			Where("entity_type = 'user'").
			Where("entity_id = ?", idStr).
			Where("deleted_at IS NULL")

		var total int
		sql, args := sqlite.CountFrom(selectQ).Build()
		_ = db.QueryRow(sql, args...).Scan(&total)

		sql, args = selectQ.
			OrderBy("created_at", "DESC").
			Limit(pg.Limit).Offset(pg.Offset).
			Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to query uploads")
			return
		}
		defer rows.Close()

		type uploadEntry struct {
			ID           int64  `json:"id"`
			UUID         string `json:"uuid"`
			OriginalName string `json:"original_name"`
			ContentType  string `json:"content_type"`
			SizeBytes    int64  `json:"size_bytes"`
			HasThumbnail bool   `json:"has_thumbnail"`
			CreatedAt    string `json:"created_at"`
		}
		uploads := make([]uploadEntry, 0)
		for rows.Next() {
			var u uploadEntry
			var hasThumbnail int
			if err := rows.Scan(&u.ID, &u.UUID, &u.OriginalName, &u.ContentType,
				&u.SizeBytes, &hasThumbnail, &u.CreatedAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan upload")
				return
			}
			u.HasThumbnail = hasThumbnail == 1
			uploads = append(uploads, u)
		}
		if err := rows.Err(); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to iterate uploads")
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
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid user id")
			return
		}

		// Verify user exists and is active.
		var email, name string
		var isActive int
		sql, args := sqlite.Select("email", "name", "is_active").
			From("users").
			Where("id = ?", id).
			Where("deleted_at IS NULL").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&email, &name, &isActive); err != nil {
			http.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		if isActive == 0 {
			http.WriteError(w, http.StatusBadRequest, "cannot impersonate an inactive user")
			return
		}

		uid := strconv.FormatInt(id, 10)
		token, err := a.IssueAccessToken(uid, []string{"user"})
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to issue token")
			return
		}

		adminaudit.Log(db, r, "user.impersonate", "user", strconv.FormatInt(id, 10), email)

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
