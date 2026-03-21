// Package usermgmt provides admin endpoints for managing end-users.
// This is the first "app-level" admin module — it manages the actual
// users of the application (not admin accounts). It serves as a second
// CRUD reference alongside adminusers.
package usermgmt

import (
	"strconv"
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/validate"
	"github.com/stanza-go/standalone/module/adminaudit"
)

// Register mounts end-user management routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET    /api/admin/users                - list users with search and pagination
//	POST   /api/admin/users                - create a new user
//	GET    /api/admin/users/{id}           - get a single user
//	PUT    /api/admin/users/{id}           - update a user
//	DELETE /api/admin/users/{id}           - soft-delete a user
//	POST   /api/admin/users/{id}/impersonate - generate access token as this user
func Register(admin *http.Group, a *auth.Auth, db *sqlite.DB) {
	admin.HandleFunc("GET /users", listHandler(db))
	admin.HandleFunc("POST /users", createHandler(db))
	admin.HandleFunc("GET /users/{id}", getHandler(db))
	admin.HandleFunc("PUT /users/{id}", updateHandler(db))
	admin.HandleFunc("DELETE /users/{id}", deleteHandler(db))
	admin.HandleFunc("POST /users/{id}/impersonate", impersonateHandler(a, db))
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
		limit := http.QueryParamInt(r, "limit", 50)
		offset := http.QueryParamInt(r, "offset", 0)
		search := r.URL.Query().Get("search")

		countQ := sqlite.Count("users").Where("deleted_at IS NULL")
		selectQ := sqlite.Select("id", "email", "name", "is_active", "created_at", "updated_at").
			From("users").
			Where("deleted_at IS NULL")
		if search != "" {
			like := "%" + escapeLike(search) + "%"
			countQ.Where("(email LIKE ? ESCAPE '\\' OR name LIKE ? ESCAPE '\\')", like, like)
			selectQ.Where("(email LIKE ? ESCAPE '\\' OR name LIKE ? ESCAPE '\\')", like, like)
		}

		var total int
		sql, args := countQ.Build()
		_ = db.QueryRow(sql, args...).Scan(&total)

		sql, args = selectQ.OrderBy("id", "DESC").Limit(limit).Offset(offset).Build()
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

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"users": users,
			"total": total,
		})
	}
}

type createRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func createHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
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

func updateHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
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

func deleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
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

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
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
