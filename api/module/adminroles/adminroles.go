// Package adminroles provides CRUD endpoints for managing admin roles
// and their associated scopes. Roles define what permissions an admin
// has within the admin panel. System roles (superadmin, admin, viewer)
// cannot be deleted but their scopes can be modified.
package adminroles

import (
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

// KnownScopes lists all valid admin scopes. Used for validation when
// assigning scopes to roles.
var KnownScopes = []string{
	"admin",
	"admin:users",
	"admin:settings",
	"admin:jobs",
	"admin:logs",
	"admin:audit",
	"admin:uploads",
	"admin:database",
	"admin:roles",
	"admin:notifications",
}

// Register mounts the role management routes on the given admin group.
// Routes:
//
//	GET    /api/admin/roles      - list all roles with scopes
//	POST   /api/admin/roles      - create a new role
//	PUT    /api/admin/roles/{id} - update a role and its scopes
//	DELETE /api/admin/roles/{id} - delete a non-system role
//	GET    /api/admin/scopes     - list all known scopes
func Register(admin *http.Group, db *sqlite.DB, wh *webhooks.Dispatcher) {
	// Role names endpoint — accessible to any authenticated admin
	// (base "admin" scope). Used by the admin create/edit form.
	admin.HandleFunc("GET /role-names", roleNamesHandler(db))

	// Full roles CRUD — requires admin:roles scope.
	g := admin.Group("/roles")
	g.Use(auth.RequireScope("admin:roles"))
	g.HandleFunc("GET /", listHandler(db))
	g.HandleFunc("POST /", createHandler(db, wh))
	g.HandleFunc("PUT /{id}", updateHandler(db, wh))
	g.HandleFunc("DELETE /{id}", deleteHandler(db, wh))
	g.HandleFunc("GET /scopes", scopesHandler())
}

type roleJSON struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IsSystem    bool     `json:"is_system"`
	Scopes      []string `json:"scopes"`
	AdminCount  int      `json:"admin_count"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sql, args := sqlite.Select("id", "name", "description", "is_system", "created_at", "updated_at").
			From("roles").
			OrderBy("id", "ASC").
			Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to list roles")
			return
		}

		var roles []roleJSON
		for rows.Next() {
			var role roleJSON
			var isSystem int
			if err := rows.Scan(&role.ID, &role.Name, &role.Description, &isSystem, &role.CreatedAt, &role.UpdatedAt); err != nil {
				rows.Close()
				http.WriteError(w, http.StatusInternalServerError, "failed to scan role")
				return
			}
			role.IsSystem = isSystem == 1
			roles = append(roles, role)
		}
		rows.Close() // Close before issuing more queries (SQLite single-mutex).

		if roles == nil {
			roles = []roleJSON{}
		}

		// Load scopes and admin counts for each role.
		for i := range roles {
			roles[i].Scopes = loadScopes(db, roles[i].ID)

			var count int
			cq, ca := sqlite.Count("admins").
				Where("role = ?", roles[i].Name).
				Where("deleted_at IS NULL").
				Build()
			_ = db.QueryRow(cq, ca...).Scan(&count)
			roles[i].AdminCount = count
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"roles": roles,
		})
	}
}

type createRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scopes      []string `json:"scopes"`
}

func createHandler(db *sqlite.DB, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		req.Name = strings.TrimSpace(strings.ToLower(req.Name))

		v := validate.Fields(
			validate.Required("name", req.Name),
			validate.MinLen("name", req.Name, 2),
			validate.MaxLen("name", req.Name, 50),
		)
		if v.HasErrors() {
			v.WriteError(w)
			return
		}

		if err := validateScopes(req.Scopes); err != "" {
			http.WriteError(w, http.StatusBadRequest, err)
			return
		}

		// Ensure "admin" base scope is always included.
		req.Scopes = ensureBaseScope(req.Scopes)

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		sql, args := sqlite.Insert("roles").
			Set("name", req.Name).
			Set("description", req.Description).
			Set("created_at", now).
			Set("updated_at", now).
			Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				http.WriteError(w, http.StatusConflict, "role name already exists")
				return
			}
			http.WriteError(w, http.StatusInternalServerError, "failed to create role")
			return
		}

		roleID := result.LastInsertID
		if err := saveScopes(db, roleID, req.Scopes); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to save scopes")
			return
		}

		adminaudit.Log(db, r, "role.create", "role", strconv.FormatInt(roleID, 10), req.Name)

		_ = wh.Dispatch(r.Context(), "role.created", map[string]any{
			"id":     roleID,
			"name":   req.Name,
			"scopes": req.Scopes,
		})

		http.WriteJSON(w, http.StatusCreated, map[string]any{
			"role": roleJSON{
				ID:          roleID,
				Name:        req.Name,
				Description: req.Description,
				Scopes:      req.Scopes,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		})
	}
}

type updateRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scopes      []string `json:"scopes"`
}

func updateHandler(db *sqlite.DB, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid role id")
			return
		}

		var req updateRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Load current role.
		var currentName, currentDesc, createdAt string
		var isSystem int
		sql, args := sqlite.Select("name", "description", "is_system", "created_at").
			From("roles").
			Where("id = ?", id).
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&currentName, &currentDesc, &isSystem, &createdAt); err != nil {
			http.WriteError(w, http.StatusNotFound, "role not found")
			return
		}

		// System roles cannot be renamed.
		name := currentName
		if req.Name != "" && isSystem == 0 {
			name = strings.TrimSpace(strings.ToLower(req.Name))
			v := validate.Fields(
				validate.MinLen("name", name, 2),
				validate.MaxLen("name", name, 50),
			)
			if v.HasErrors() {
				v.WriteError(w)
				return
			}
		}

		desc := currentDesc
		if req.Description != "" {
			desc = req.Description
		}

		if req.Scopes != nil {
			if err := validateScopes(req.Scopes); err != "" {
				http.WriteError(w, http.StatusBadRequest, err)
				return
			}
			req.Scopes = ensureBaseScope(req.Scopes)
		}

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		q := sqlite.Update("roles").
			Set("description", desc).
			Set("updated_at", now)
		if isSystem == 0 {
			q.Set("name", name)
		}
		sql, args = q.Where("id = ?", id).Build()
		if _, err := db.Exec(sql, args...); err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				http.WriteError(w, http.StatusConflict, "role name already exists")
				return
			}
			http.WriteError(w, http.StatusInternalServerError, "failed to update role")
			return
		}

		// Update scopes if provided.
		if req.Scopes != nil {
			// Delete existing scopes.
			sql, args = sqlite.Delete("role_scopes").Where("role_id = ?", id).Build()
			if _, err := db.Exec(sql, args...); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to update scopes")
				return
			}
			if err := saveScopes(db, id, req.Scopes); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to save scopes")
				return
			}
		}

		// If role was renamed and it's not a system role, update admins.
		if isSystem == 0 && name != currentName {
			sql, args = sqlite.Update("admins").
				Set("role", name).
				Where("role = ?", currentName).
				Build()
			_, _ = db.Exec(sql, args...)
		}

		scopes := loadScopes(db, id)

		adminaudit.Log(db, r, "role.update", "role", strconv.FormatInt(id, 10), name)

		_ = wh.Dispatch(r.Context(), "role.updated", map[string]any{
			"id":     id,
			"name":   name,
			"scopes": scopes,
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"role": roleJSON{
				ID:          id,
				Name:        name,
				Description: desc,
				IsSystem:    isSystem == 1,
				Scopes:      scopes,
				CreatedAt:   createdAt,
				UpdatedAt:   now,
			},
		})
	}
}

func deleteHandler(db *sqlite.DB, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid role id")
			return
		}

		// Check if role exists and is not a system role.
		var name string
		var isSystem int
		sql, args := sqlite.Select("name", "is_system").From("roles").Where("id = ?", id).Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&name, &isSystem); err != nil {
			http.WriteError(w, http.StatusNotFound, "role not found")
			return
		}

		if isSystem == 1 {
			http.WriteError(w, http.StatusBadRequest, "cannot delete a system role")
			return
		}

		// Check if any admins are using this role.
		var count int
		cq, ca := sqlite.Count("admins").
			Where("role = ?", name).
			Where("deleted_at IS NULL").
			Build()
		_ = db.QueryRow(cq, ca...).Scan(&count)
		if count > 0 {
			http.WriteError(w, http.StatusConflict, "role is assigned to "+strconv.Itoa(count)+" admin(s)")
			return
		}

		// Delete scopes first (FK cascade should handle this, but be explicit).
		sql, args = sqlite.Delete("role_scopes").Where("role_id = ?", id).Build()
		_, _ = db.Exec(sql, args...)

		sql, args = sqlite.Delete("roles").Where("id = ?", id).Build()
		if _, err := db.Exec(sql, args...); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to delete role")
			return
		}

		adminaudit.Log(db, r, "role.delete", "role", strconv.FormatInt(id, 10), name)

		_ = wh.Dispatch(r.Context(), "role.deleted", map[string]any{
			"id":   id,
			"name": name,
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}

func roleNamesHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		names := RoleNames(db)
		if names == nil {
			names = []string{}
		}
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"roles": names,
		})
	}
}

func scopesHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"scopes": KnownScopes,
		})
	}
}

// loadScopes returns the scope names for a given role ID.
func loadScopes(db *sqlite.DB, roleID int64) []string {
	sql, args := sqlite.Select("scope").From("role_scopes").Where("role_id = ?", roleID).Build()
	rows, err := db.Query(sql, args...)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	var scopes []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			continue
		}
		scopes = append(scopes, s)
	}
	if scopes == nil {
		return []string{}
	}
	return scopes
}

// saveScopes inserts scope rows for a role.
func saveScopes(db *sqlite.DB, roleID int64, scopes []string) error {
	for _, s := range scopes {
		sql, args := sqlite.Insert("role_scopes").
			Set("role_id", roleID).
			Set("scope", s).
			Build()
		if _, err := db.Exec(sql, args...); err != nil {
			return err
		}
	}
	return nil
}

// validateScopes checks that all provided scopes are known.
func validateScopes(scopes []string) string {
	known := make(map[string]bool, len(KnownScopes))
	for _, s := range KnownScopes {
		known[s] = true
	}
	for _, s := range scopes {
		if !known[s] {
			return "unknown scope: " + s
		}
	}
	return ""
}

// ensureBaseScope makes sure the "admin" scope is always included.
func ensureBaseScope(scopes []string) []string {
	for _, s := range scopes {
		if s == "admin" {
			return scopes
		}
	}
	return append([]string{"admin"}, scopes...)
}

// ValidateRoleExists checks if a role name exists in the database.
// Exported for use by other modules (e.g., adminusers).
func ValidateRoleExists(db *sqlite.DB, role string) bool {
	var count int
	sql, args := sqlite.Count("roles").Where("name = ?", role).Build()
	_ = db.QueryRow(sql, args...).Scan(&count)
	return count > 0
}

// RoleNames returns all role names from the database.
func RoleNames(db *sqlite.DB) []string {
	sql, args := sqlite.Select("name").From("roles").OrderBy("id", "ASC").Build()
	rows, err := db.Query(sql, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		names = append(names, name)
	}
	return names
}
