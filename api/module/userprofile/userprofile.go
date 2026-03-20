// Package userprofile implements the authenticated user's profile
// endpoints: view profile, update profile, and change password.
// All routes require a valid JWT with the "user" scope.
package userprofile

import (
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/sqlite"
)

// Register mounts the user profile routes on the given group.
// The group should already have RequireAuth + RequireScope("user") applied.
//
// Routes:
//
//	GET  /profile          — get authenticated user's profile
//	PUT  /profile          — update name and/or email
//	PUT  /profile/password — change password
func Register(user *http.Group, db *sqlite.DB, logger *log.Logger) {
	user.HandleFunc("GET /profile", getProfile(db))
	user.HandleFunc("PUT /profile", updateProfile(db, logger))
	user.HandleFunc("PUT /profile/password", changePassword(db, logger))
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
func updateProfile(db *sqlite.DB, logger *log.Logger) func(http.ResponseWriter, *http.Request) {
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

		if req.Name == "" && req.Email == "" {
			http.WriteError(w, http.StatusBadRequest, "at least one field (name or email) is required")
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
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
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

		if req.CurrentPassword == "" || req.NewPassword == "" {
			http.WriteError(w, http.StatusBadRequest, "current_password and new_password are required")
			return
		}
		if len(req.NewPassword) < 8 {
			http.WriteError(w, http.StatusBadRequest, "new password must be at least 8 characters")
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

		http.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "password updated",
		})
	}
}
