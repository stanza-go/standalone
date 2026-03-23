// Package adminprofile implements the authenticated admin's profile
// endpoints: view profile and change password. All routes require
// a valid JWT with the "admin" scope.
package adminprofile

import (
	"strings"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/validate"
	"github.com/stanza-go/standalone/module/adminaudit"
)

// Register mounts the admin profile routes on the given group.
// The group should already have RequireAuth + RequireScope("admin") applied.
//
// Routes:
//
//	GET  /profile          — get authenticated admin's profile
//	PUT  /profile          — update name
//	PUT  /profile/password — change password
func Register(admin *http.Group, db *sqlite.DB) {
	admin.HandleFunc("GET /profile", getProfile(db))
	admin.HandleFunc("PUT /profile", updateProfile(db))
	admin.HandleFunc("PUT /profile/password", changePassword(db))
	admin.HandleFunc("GET /profile/sessions", getSessions(db))
	admin.HandleFunc("DELETE /profile/sessions/{id}", revokeSession(db))
}

// getProfile returns the authenticated admin's profile.
func getProfile(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		var id int64
		var email, name, role, createdAt, updatedAt string
		sql, args := sqlite.Select("id", "email", "name", "role", "created_at", "updated_at").
			From("admins").
			Where("id = ?", claims.UID).
			WhereNull("deleted_at").
			Where("is_active = 1").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&id, &email, &name, &role, &createdAt, &updatedAt); err != nil {
			http.WriteError(w, http.StatusNotFound, "admin not found")
			return
		}

		// Get scopes from the admin's role.
		scopeSQL, scopeArgs := sqlite.Select("rs.scope").
			From("role_scopes rs").
			Join("roles r", "r.id = rs.role_id").
			Where("r.name = ?", role).
			Build()
		scopes, _ := sqlite.QueryAll(db, scopeSQL, scopeArgs, func(rows *sqlite.Rows) (string, error) {
			var scope string
			err := rows.Scan(&scope)
			return scope, err
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"admin": map[string]any{
				"id":         id,
				"email":      email,
				"name":       name,
				"role":       role,
				"scopes":     scopes,
				"created_at": createdAt,
				"updated_at": updatedAt,
			},
		})
	}
}

// updateRequest is the expected JSON body for PUT /profile.
type updateRequest struct {
	Name string `json:"name"`
}

// updateProfile updates the authenticated admin's name.
func updateProfile(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		var req updateRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		req.Name = strings.TrimSpace(req.Name)

		v := validate.Fields(
			validate.Required("name", req.Name),
		)
		if v.HasErrors() {
			v.WriteError(w)
			return
		}

		now := sqlite.Now()
		n, err := db.Update(sqlite.Update("admins").
			Set("name", req.Name).
			Set("updated_at", now).
			Where("id = ?", claims.UID).
			WhereNull("deleted_at").
			Where("is_active = 1"))
		if err != nil {
			http.WriteServerError(w, r, "internal error", err)
			return
		}

		if n == 0 {
			http.WriteError(w, http.StatusNotFound, "admin not found")
			return
		}

		adminaudit.Log(db, r, "admin.update_profile", "admin", claims.UID, "updated name")

		// Return the updated profile.
		var id int64
		var email, name, role, createdAt, updatedAt string
		sql, args := sqlite.Select("id", "email", "name", "role", "created_at", "updated_at").
			From("admins").
			Where("id = ?", claims.UID).
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&id, &email, &name, &role, &createdAt, &updatedAt); err != nil {
			http.WriteServerError(w, r, "internal error", err)
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"admin": map[string]any{
				"id":         id,
				"email":      email,
				"name":       name,
				"role":       role,
				"created_at": createdAt,
				"updated_at": updatedAt,
			},
		})
	}
}

// passwordRequest is the expected JSON body for PUT /profile/password.
type passwordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// changePassword verifies the current password and updates to a new one.
func changePassword(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		var req passwordRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		v := validate.Fields(
			validate.Required("current_password", req.CurrentPassword),
			validate.Required("new_password", req.NewPassword),
			validate.MinLen("new_password", req.NewPassword, 8),
		)
		if v.HasErrors() {
			v.WriteError(w)
			return
		}

		// Verify current password.
		var passwordHash string
		sql, args := sqlite.Select("password").
			From("admins").
			Where("id = ?", claims.UID).
			WhereNull("deleted_at").
			Where("is_active = 1").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&passwordHash); err != nil {
			http.WriteError(w, http.StatusNotFound, "admin not found")
			return
		}

		if !auth.VerifyPassword(passwordHash, req.CurrentPassword) {
			http.WriteError(w, http.StatusUnauthorized, "current password is incorrect")
			return
		}

		// Hash and store new password.
		newHash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			http.WriteServerError(w, r, "internal error", err)
			return
		}

		now := sqlite.Now()
		_, err = db.Update(sqlite.Update("admins").
			Set("password", newHash).
			Set("updated_at", now).
			Where("id = ?", claims.UID))
		if err != nil {
			http.WriteServerError(w, r, "internal error", err)
			return
		}

		adminaudit.Log(db, r, "admin.change_password", "admin", claims.UID, "changed own password")

		// Revoke all other sessions for this admin (keep current session).
		refreshToken, _ := auth.ReadRefreshToken(r)
		if refreshToken != "" {
			_, _ = db.Delete(sqlite.Delete("refresh_tokens").
				Where("entity_type = ?", "admin").
				Where("entity_id = ?", claims.UID).
				Where("token_hash != ?", auth.HashToken(refreshToken)))
		} else {
			_, _ = db.Delete(sqlite.Delete("refresh_tokens").
				Where("entity_type = ?", "admin").
				Where("entity_id = ?", claims.UID))
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"status":           "password updated",
			"sessions_revoked": true,
		})
	}
}

// ActiveSession represents a session for the admin's sessions list.
type ActiveSession struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
	Current   bool   `json:"current"`
}

// getSessions returns the authenticated admin's active sessions.
func getSessions(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		now := sqlite.Now()
		sql, args := sqlite.Select("id", "created_at", "expires_at", "token_hash").
			From("refresh_tokens").
			Where("entity_type = 'admin'").
			Where("entity_id = ?", claims.UID).
			Where("expires_at > ?", now).
			OrderBy("created_at", "DESC").
			Build()
		currentTokenHash := ""
		refreshToken, _ := auth.ReadRefreshToken(r)
		if refreshToken != "" {
			currentTokenHash = auth.HashToken(refreshToken)
		}

		sessions, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (ActiveSession, error) {
			var s ActiveSession
			var tokenHash string
			if err := rows.Scan(&s.ID, &s.CreatedAt, &s.ExpiresAt, &tokenHash); err != nil {
				return s, err
			}
			s.Current = tokenHash == currentTokenHash
			return s, nil
		})
		if err != nil {
			http.WriteServerError(w, r, "failed to list sessions", err)
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"sessions": sessions,
		})
	}
}

// revokeSession revokes a specific session by ID.
func revokeSession(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		sessionID := r.PathValue("id")
		if sessionID == "" {
			http.WriteError(w, http.StatusBadRequest, "invalid session ID")
			return
		}

		n, err := db.Delete(sqlite.Delete("refresh_tokens").
			Where("id = ?", sessionID).
			Where("entity_type = ?", "admin").
			Where("entity_id = ?", claims.UID))
		if err != nil {
			http.WriteServerError(w, r, "internal error", err)
			return
		}

		if n == 0 {
			http.WriteError(w, http.StatusNotFound, "session not found")
			return
		}

		adminaudit.Log(db, r, "admin.revoke_session", "session", sessionID, "revoked own session")

		http.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "session revoked",
		})
	}
}
