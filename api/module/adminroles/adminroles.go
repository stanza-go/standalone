// Package adminroles provides CRUD endpoints for managing admin roles
// and their associated scopes. Roles define what permissions an admin
// has within the admin panel. System roles (superadmin, admin, viewer)
// cannot be deleted but their scopes can be modified.
package adminroles

import (
	"strconv"
	"strings"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/validate"
	"github.com/stanza-go/standalone/module/adminaudit"
	"github.com/stanza-go/standalone/module/webhooks"
)

// Scope defines an admin permission scope with a human-readable label.
// The backend is the single source of truth — the admin panel fetches
// scope names and labels from the /admin/roles/scopes endpoint.
type Scope struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// KnownScopes lists all valid admin scopes. Used for validation when
// assigning scopes to roles, and served to the admin panel via the
// scopes endpoint.
var KnownScopes = []Scope{
	{Name: "admin", Label: "Base Access"},
	{Name: "admin:users", Label: "User Management"},
	{Name: "admin:settings", Label: "Settings"},
	{Name: "admin:jobs", Label: "Jobs & Cron"},
	{Name: "admin:logs", Label: "Log Viewer"},
	{Name: "admin:audit", Label: "Audit Log"},
	{Name: "admin:uploads", Label: "Uploads"},
	{Name: "admin:database", Label: "Database"},
	{Name: "admin:roles", Label: "Role Management"},
	{Name: "admin:notifications", Label: "Notifications"},
	{Name: "admin:webhooks", Label: "Webhooks"},
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

func scanRole(rows *sqlite.Rows) (roleJSON, error) {
	var role roleJSON
	if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt); err != nil {
		return role, err
	}
	return role, nil
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sql, args := sqlite.Select("id", "name", "description", "is_system", "created_at", "updated_at").
			From("roles").
			OrderBy("id", "ASC").
			Build()
		roles, err := sqlite.QueryAll(db, sql, args, scanRole)
		if err != nil {
			http.WriteServerError(w, r, "failed to list roles", err)
			return
		}

		// Load scopes and admin counts for each role.
		for i := range roles {
			roles[i].Scopes = loadScopes(db, roles[i].ID)

			var count int
			cq, ca := sqlite.Count("admins").
				Where("role = ?", roles[i].Name).
				WhereNull("deleted_at").
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
		if !http.BindJSON(w, r, &req) {
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

		now := sqlite.Now()
		roleID, err := db.Insert(sqlite.Insert("roles").
			Set("name", req.Name).
			Set("description", req.Description).
			Set("created_at", now).
			Set("updated_at", now))
		if err != nil {
			if sqlite.IsUniqueConstraintError(err) {
				http.WriteError(w, http.StatusConflict, "role name already exists")
				return
			}
			http.WriteServerError(w, r, "failed to create role", err)
			return
		}
		if err := saveScopes(db, roleID, req.Scopes); err != nil {
			http.WriteServerError(w, r, "failed to save scopes", err)
			return
		}

		adminaudit.Log(db, r, "role.create", "role", sqlite.FormatID(roleID), req.Name)

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
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		var req updateRequest
		if !http.BindJSON(w, r, &req) {
			return
		}

		// Load current role.
		var currentName, currentDesc, createdAt string
		var isSystem bool
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
		if req.Name != "" && !isSystem {
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

		now := sqlite.Now()
		q := sqlite.Update("roles").
			Set("description", desc).
			Set("updated_at", now)
		if !isSystem {
			q.Set("name", name)
		}
		if _, err := db.Update(q.Where("id = ?", id)); err != nil {
			if sqlite.IsUniqueConstraintError(err) {
				http.WriteError(w, http.StatusConflict, "role name already exists")
				return
			}
			http.WriteServerError(w, r, "failed to update role", err)
			return
		}

		// Update scopes if provided.
		if req.Scopes != nil {
			// Delete existing scopes.
			if _, err := db.Delete(sqlite.Delete("role_scopes").Where("role_id = ?", id)); err != nil {
				http.WriteServerError(w, r, "failed to update scopes", err)
				return
			}
			if err := saveScopes(db, id, req.Scopes); err != nil {
				http.WriteServerError(w, r, "failed to save scopes", err)
				return
			}
		}

		// If role was renamed and it's not a system role, update admins.
		if !isSystem && name != currentName {
			_, _ = db.Update(sqlite.Update("admins").
				Set("role", name).
				Where("role = ?", currentName))
		}

		scopes := loadScopes(db, id)

		adminaudit.Log(db, r, "role.update", "role", sqlite.FormatID(id), name)

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
				IsSystem:    isSystem,
				Scopes:      scopes,
				CreatedAt:   createdAt,
				UpdatedAt:   now,
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

		// Check if role exists and is not a system role.
		var name string
		var isSystem bool
		sql, args := sqlite.Select("name", "is_system").From("roles").Where("id = ?", id).Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&name, &isSystem); err != nil {
			http.WriteError(w, http.StatusNotFound, "role not found")
			return
		}

		if isSystem {
			http.WriteError(w, http.StatusBadRequest, "cannot delete a system role")
			return
		}

		// Check if any admins are using this role.
		var count int
		cq, ca := sqlite.Count("admins").
			Where("role = ?", name).
			WhereNull("deleted_at").
			Build()
		_ = db.QueryRow(cq, ca...).Scan(&count)
		if count > 0 {
			http.WriteError(w, http.StatusConflict, "role is assigned to "+strconv.Itoa(count)+" admin(s)")
			return
		}

		// Delete scopes first (FK cascade should handle this, but be explicit).
		_, _ = db.Delete(sqlite.Delete("role_scopes").Where("role_id = ?", id))

		if _, err := db.Delete(sqlite.Delete("roles").Where("id = ?", id)); err != nil {
			http.WriteServerError(w, r, "failed to delete role", err)
			return
		}

		adminaudit.Log(db, r, "role.delete", "role", sqlite.FormatID(id), name)

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
	scopes, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (string, error) {
		var s string
		err := rows.Scan(&s)
		return s, err
	})
	if err != nil {
		return []string{}
	}
	return scopes
}

// saveScopes inserts scope rows for a role.
func saveScopes(db *sqlite.DB, roleID int64, scopes []string) error {
	for _, s := range scopes {
		if _, err := db.Insert(sqlite.Insert("role_scopes").
			Set("role_id", roleID).
			Set("scope", s)); err != nil {
			return err
		}
	}
	return nil
}

// validateScopes checks that all provided scopes are known.
func validateScopes(scopes []string) string {
	known := make(map[string]bool, len(KnownScopes))
	for _, s := range KnownScopes {
		known[s.Name] = true
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
	names, _ := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (string, error) {
		var name string
		err := rows.Scan(&name)
		return name, err
	})
	return names
}
