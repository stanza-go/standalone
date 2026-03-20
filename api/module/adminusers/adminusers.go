// Package adminusers provides admin CRUD endpoints for managing admin
// accounts. It supports listing, creating, updating, and soft-deleting
// admins with pagination and self-action protection.
package adminusers

import (
	"strconv"
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
)

// Register mounts the admin user management routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET    /api/admin/admins      - list admins with pagination
//	POST   /api/admin/admins      - create a new admin
//	PUT    /api/admin/admins/{id} - update an admin
//	DELETE /api/admin/admins/{id} - soft-delete an admin
func Register(admin *http.Group, db *sqlite.DB) {
	admin.HandleFunc("GET /admins", listHandler(db))
	admin.HandleFunc("POST /admins", createHandler(db))
	admin.HandleFunc("PUT /admins/{id}", updateHandler(db))
	admin.HandleFunc("DELETE /admins/{id}", deleteHandler(db))
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

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := http.QueryParamInt(r, "limit", 50)
		offset := http.QueryParamInt(r, "offset", 0)

		var total int
		row := db.QueryRow(`SELECT COUNT(*) FROM admins WHERE deleted_at IS NULL`)
		row.Scan(&total)

		rows, err := db.Query(
			`SELECT id, email, name, role, is_active, created_at, updated_at
			 FROM admins WHERE deleted_at IS NULL
			 ORDER BY id ASC LIMIT ? OFFSET ?`,
			limit, offset,
		)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to list admins")
			return
		}
		defer rows.Close()

		admins := make([]adminJSON, 0)
		for rows.Next() {
			var a adminJSON
			var isActive int
			if err := rows.Scan(&a.ID, &a.Email, &a.Name, &a.Role, &isActive, &a.CreatedAt, &a.UpdatedAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan admin")
				return
			}
			a.IsActive = isActive == 1
			admins = append(admins, a)
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"admins": admins,
			"total":  total,
		})
	}
}

type createRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

func createHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Email == "" || req.Password == "" {
			http.WriteError(w, http.StatusBadRequest, "email and password are required")
			return
		}

		if req.Role == "" {
			req.Role = "admin"
		}
		if !validRole(req.Role) {
			http.WriteError(w, http.StatusBadRequest, "role must be superadmin, admin, or viewer")
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		result, err := db.Exec(
			`INSERT INTO admins (email, password, name, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			req.Email, hash, req.Name, req.Role, now, now,
		)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				http.WriteError(w, http.StatusConflict, "email already exists")
				return
			}
			http.WriteError(w, http.StatusInternalServerError, "failed to create admin")
			return
		}

		http.WriteJSON(w, http.StatusCreated, map[string]any{
			"admin": adminJSON{
				ID:        result.LastInsertID,
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

func updateHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid admin id")
			return
		}

		var req updateRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Role != "" && !validRole(req.Role) {
			http.WriteError(w, http.StatusBadRequest, "role must be superadmin, admin, or viewer")
			return
		}

		// Prevent self-deactivation.
		if req.IsActive != nil && !*req.IsActive {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if ok && claims.UID == strconv.FormatInt(id, 10) {
				http.WriteError(w, http.StatusBadRequest, "cannot deactivate your own account")
				return
			}
		}

		// Load current admin.
		var currentEmail, currentName, currentRole, createdAt string
		var currentActive int
		row := db.QueryRow(
			`SELECT email, name, role, is_active, created_at FROM admins WHERE id = ? AND deleted_at IS NULL`,
			id,
		)
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
			if *req.IsActive {
				isActive = 1
			} else {
				isActive = 0
			}
		}

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

		if req.Password != "" {
			hash, err := auth.HashPassword(req.Password)
			if err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to hash password")
				return
			}
			_, err = db.Exec(
				`UPDATE admins SET name = ?, role = ?, is_active = ?, password = ?, updated_at = ?
				 WHERE id = ? AND deleted_at IS NULL`,
				name, role, isActive, hash, now, id,
			)
			if err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to update admin")
				return
			}
		} else {
			_, err = db.Exec(
				`UPDATE admins SET name = ?, role = ?, is_active = ?, updated_at = ?
				 WHERE id = ? AND deleted_at IS NULL`,
				name, role, isActive, now, id,
			)
			if err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to update admin")
				return
			}
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"admin": adminJSON{
				ID:        id,
				Email:     currentEmail,
				Name:      name,
				Role:      role,
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
			http.WriteError(w, http.StatusBadRequest, "invalid admin id")
			return
		}

		// Prevent self-deletion.
		claims, ok := auth.ClaimsFromContext(r.Context())
		if ok && claims.UID == strconv.FormatInt(id, 10) {
			http.WriteError(w, http.StatusBadRequest, "cannot delete your own account")
			return
		}

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		result, err := db.Exec(
			`UPDATE admins SET deleted_at = ?, is_active = 0, updated_at = ?
			 WHERE id = ? AND deleted_at IS NULL`,
			now, now, id,
		)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to delete admin")
			return
		}
		if result.RowsAffected == 0 {
			http.WriteError(w, http.StatusNotFound, "admin not found")
			return
		}

		// Revoke all sessions for this admin.
		_, _ = db.Exec(
			`DELETE FROM refresh_tokens WHERE entity_type = 'admin' AND entity_id = ?`,
			strconv.FormatInt(id, 10),
		)

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}

func validRole(role string) bool {
	return role == "superadmin" || role == "admin" || role == "viewer"
}
