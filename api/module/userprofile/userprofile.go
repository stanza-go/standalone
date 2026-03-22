// Package userprofile implements the authenticated user's profile
// endpoints: view profile, update profile, change password, and
// session management. All routes require a valid JWT with the "user" scope.
package userprofile

import (
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/validate"
	"github.com/stanza-go/standalone/module/webhooks"
)

// Register mounts the user profile routes on the given group.
// The group should already have RequireAuth + RequireScope("user") applied.
//
// Routes:
//
//	GET    /profile              — get authenticated user's profile
//	PUT    /profile              — update name and/or email
//	PUT    /profile/password     — change password
//	GET    /profile/sessions     — list own active sessions
//	DELETE /profile/sessions/{id} — revoke a specific session
func Register(user *http.Group, db *sqlite.DB, logger *log.Logger, wh *webhooks.Dispatcher) {
	user.HandleFunc("GET /profile", getProfile(db))
	user.HandleFunc("PUT /profile", updateProfile(db, logger, wh))
	user.HandleFunc("PUT /profile/password", changePassword(db, logger))
	user.HandleFunc("GET /profile/sessions", getSessions(db))
	user.HandleFunc("DELETE /profile/sessions/{id}", revokeSession(db))
}

// getProfile returns the authenticated user's profile.
func getProfile(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		var id int64
		var email, name, createdAt, updatedAt string
		sql, args := sqlite.Select("id", "email", "name", "created_at", "updated_at").
			From("users").
			Where("id = ?", claims.UID).
			Where("deleted_at IS NULL").
			Where("is_active = 1").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&id, &email, &name, &createdAt, &updatedAt); err != nil {
			http.WriteError(w, http.StatusNotFound, "user not found")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"user": map[string]any{
				"id":         id,
				"email":      email,
				"name":       name,
				"created_at": createdAt,
				"updated_at": updatedAt,
			},
		})
	}
}

// updateRequest is the expected JSON body for PUT /profile.
type updateRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// updateProfile updates the authenticated user's name and/or email.
func updateProfile(db *sqlite.DB, logger *log.Logger, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
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
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		v := validate.Fields(
			validate.Check("name", req.Name != "" || req.Email != "", "at least one field (name or email) is required"),
			validate.Email("email", req.Email),
		)
		if v.HasErrors() {
			v.WriteError(w)
			return
		}

		now := time.Now().UTC().Format(time.RFC3339)
		b := sqlite.Update("users").
			Set("updated_at", now).
			Where("id = ?", claims.UID).
			Where("deleted_at IS NULL").
			Where("is_active = 1")

		if req.Name != "" {
			b = b.Set("name", req.Name)
		}
		if req.Email != "" {
			b = b.Set("email", req.Email)
		}

		sql, args := b.Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			if sqlite.IsUniqueConstraintError(err) {
				http.WriteError(w, http.StatusConflict, "email already in use")
				return
			}
			logger.Error("update user profile", log.String("error", err.Error()))
			http.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if result.RowsAffected == 0 {
			http.WriteError(w, http.StatusNotFound, "user not found")
			return
		}

		// Return the updated profile.
		var id int64
		var email, name, createdAt, updatedAt string
		sql, args = sqlite.Select("id", "email", "name", "created_at", "updated_at").
			From("users").
			Where("id = ?", claims.UID).
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&id, &email, &name, &createdAt, &updatedAt); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}

		_ = wh.Dispatch(r.Context(), "user.updated", map[string]any{
			"id":    id,
			"email": email,
			"name":  name,
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"user": map[string]any{
				"id":         id,
				"email":      email,
				"name":       name,
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
func changePassword(db *sqlite.DB, logger *log.Logger) func(http.ResponseWriter, *http.Request) {
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
			From("users").
			Where("id = ?", claims.UID).
			Where("deleted_at IS NULL").
			Where("is_active = 1").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&passwordHash); err != nil {
			http.WriteError(w, http.StatusNotFound, "user not found")
			return
		}

		if !auth.VerifyPassword(passwordHash, req.CurrentPassword) {
			http.WriteError(w, http.StatusUnauthorized, "current password is incorrect")
			return
		}

		// Hash and store new password.
		newHash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			logger.Error("hash password", log.String("error", err.Error()))
			http.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}

		now := time.Now().UTC().Format(time.RFC3339)
		sql, args = sqlite.Update("users").
			Set("password", newHash).
			Set("updated_at", now).
			Where("id = ?", claims.UID).
			Build()
		_, err = db.Exec(sql, args...)
		if err != nil {
			logger.Error("update password", log.String("error", err.Error()))
			http.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Revoke all other sessions for this user (keep current session).
		refreshToken, _ := auth.ReadRefreshToken(r)
		if refreshToken != "" {
			sql, args := sqlite.Delete("refresh_tokens").
				Where("entity_type = 'user'").
				Where("entity_id = ?", claims.UID).
				Where("token_hash != ?", auth.HashToken(refreshToken)).
				Build()
			_, _ = db.Exec(sql, args...)
		} else {
			sql, args := sqlite.Delete("refresh_tokens").
				Where("entity_type = 'user'").
				Where("entity_id = ?", claims.UID).
				Build()
			_, _ = db.Exec(sql, args...)
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"status":           "password updated",
			"sessions_revoked": true,
		})
	}
}

// ActiveSession represents a session for the user's sessions list.
type ActiveSession struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
	Current   bool   `json:"current"`
}

// getSessions returns the authenticated user's active sessions.
func getSessions(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		now := time.Now().UTC().Format(time.RFC3339)
		sql, args := sqlite.Select("id", "created_at", "expires_at", "token_hash").
			From("refresh_tokens").
			Where("entity_type = 'user'").
			Where("entity_id = ?", claims.UID).
			Where("expires_at > ?", now).
			OrderBy("created_at", "DESC").
			Build()

		currentTokenHash := ""
		refreshToken, _ := auth.ReadRefreshToken(r)
		if refreshToken != "" {
			currentTokenHash = auth.HashToken(refreshToken)
		}

		type sessionRow struct {
			ActiveSession
			TokenHash string
		}
		rows, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (sessionRow, error) {
			var sr sessionRow
			err := rows.Scan(&sr.ID, &sr.CreatedAt, &sr.ExpiresAt, &sr.TokenHash)
			return sr, err
		})
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}

		sessions := make([]ActiveSession, len(rows))
		for i, sr := range rows {
			sr.Current = sr.TokenHash == currentTokenHash
			sessions[i] = sr.ActiveSession
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

		sql, args := sqlite.Delete("refresh_tokens").
			Where("id = ?", sessionID).
			Where("entity_type = 'user'").
			Where("entity_id = ?", claims.UID).
			Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if result.RowsAffected == 0 {
			http.WriteError(w, http.StatusNotFound, "session not found")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "session revoked",
		})
	}
}
