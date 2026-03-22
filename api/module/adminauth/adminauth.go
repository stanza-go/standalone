// Package adminauth implements the admin authentication endpoints:
// login, status check (with token refresh), and logout. It follows the
// stateless JWT strategy described in STATELESS_AUTH_REF.md.
package adminauth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/sqlite"
)

// Register mounts the admin auth routes on the given router group.
// Routes:
//
//	POST /api/admin/auth/login  — authenticate with email + password
//	GET  /api/admin/auth        — status check, refresh access token
//	POST /api/admin/auth/logout — revoke session, clear cookies
func Register(api *http.Group, a *auth.Auth, db *sqlite.DB, logger *log.Logger) {
	g := api.Group("/admin/auth")

	g.HandleFunc("POST /login", loginHandler(a, db, logger))
	g.HandleFunc("GET /", statusHandler(a, db, logger))
	g.HandleFunc("POST /logout", logoutHandler(a, db, logger))
}

// loginRequest is the expected JSON body for POST /login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginHandler authenticates an admin by email and password, issues
// access and refresh tokens, and sets them as cookies.
func loginHandler(a *auth.Auth, db *sqlite.DB, logger *log.Logger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Email == "" || req.Password == "" {
			http.WriteError(w, http.StatusBadRequest, "email and password are required")
			return
		}

		// Look up admin by email (not soft-deleted, active).
		var id int64
		var passwordHash, name, role string
		sql, args := sqlite.Select("id", "password", "name", "role").
			From("admins").
			Where("email = ?", req.Email).
			Where("deleted_at IS NULL").
			Where("is_active = 1").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&id, &passwordHash, &name, &role); err != nil {
			// Don't reveal whether the email exists.
			http.WriteError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		if !auth.VerifyPassword(passwordHash, req.Password) {
			http.WriteError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		uid := strconv.FormatInt(id, 10)
		scopes := scopesForRole(db, role)

		// Issue access token.
		accessToken, err := a.IssueAccessToken(uid, scopes)
		if err != nil {
			logger.Error("issue access token", log.String("error", err.Error()))
			http.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Generate and store refresh token.
		refreshToken, err := auth.GenerateRefreshToken()
		if err != nil {
			logger.Error("generate refresh token", log.String("error", err.Error()))
			http.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}

		tokenID, err := randomID()
		if err != nil {
			logger.Error("generate token id", log.String("error", err.Error()))
			http.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}

		expiresAt := time.Now().Add(a.RefreshTokenTTL()).UTC().Format(time.RFC3339)
		sql, args = sqlite.Insert("refresh_tokens").
			Set("id", tokenID).
			Set("entity_type", "admin").
			Set("entity_id", uid).
			Set("token_hash", auth.HashToken(refreshToken)).
			Set("expires_at", expiresAt).
			Build()
		_, err = db.Exec(sql, args...)
		if err != nil {
			logger.Error("store refresh token", log.String("error", err.Error()))
			http.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Set cookies.
		a.SetAccessTokenCookie(w, accessToken)
		a.SetRefreshTokenCookie(w, refreshToken)

		logger.Info("admin login", log.String("email", req.Email), log.String("uid", uid))

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"admin": map[string]any{
				"id":    id,
				"email": req.Email,
				"name":  name,
				"role":  role,
			},
		})
	}
}

// statusHandler validates the refresh token, checks if the admin is
// still active, and issues a fresh access token with up-to-date scopes.
// The frontend polls this endpoint every ~1 minute.
func statusHandler(a *auth.Auth, db *sqlite.DB, logger *log.Logger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		refreshToken, err := auth.ReadRefreshToken(r)
		if err != nil {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		tokenHash := auth.HashToken(refreshToken)

		// Validate refresh token against DB.
		var entityID, expiresAtStr string
		sql, args := sqlite.Select("entity_id", "expires_at").
			From("refresh_tokens").
			Where("token_hash = ?", tokenHash).
			Where("entity_type = 'admin'").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&entityID, &expiresAtStr); err != nil {
			http.WriteError(w, http.StatusUnauthorized, "invalid session")
			return
		}

		expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
		if err != nil || time.Now().After(expiresAt) {
			// Expired — clean up.
			sql, args = sqlite.Delete("refresh_tokens").Where("token_hash = ?", tokenHash).Build()
			_, _ = db.Exec(sql, args...)
			a.ClearAllCookies(w)
			http.WriteError(w, http.StatusUnauthorized, "session expired")
			return
		}

		// Check admin is still active and not deleted.
		var id int64
		var email, name, role string
		sql, args = sqlite.Select("id", "email", "name", "role").
			From("admins").
			Where("id = ?", entityID).
			Where("deleted_at IS NULL").
			Where("is_active = 1").
			Build()
		row = db.QueryRow(sql, args...)
		if err := row.Scan(&id, &email, &name, &role); err != nil {
			// Admin deactivated or deleted — revoke session.
			sql, args = sqlite.Delete("refresh_tokens").Where("token_hash = ?", tokenHash).Build()
			_, _ = db.Exec(sql, args...)
			a.ClearAllCookies(w)
			http.WriteError(w, http.StatusUnauthorized, "account deactivated")
			return
		}

		uid := strconv.FormatInt(id, 10)
		scopes := scopesForRole(db, role)

		// Issue fresh access token with current scopes.
		accessToken, err := a.IssueAccessToken(uid, scopes)
		if err != nil {
			logger.Error("issue access token", log.String("error", err.Error()))
			http.WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}

		a.SetAccessTokenCookie(w, accessToken)

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"admin": map[string]any{
				"id":    id,
				"email": email,
				"name":  name,
				"role":  role,
			},
		})
	}
}

// logoutHandler revokes the refresh token and clears all cookies.
func logoutHandler(a *auth.Auth, db *sqlite.DB, logger *log.Logger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		refreshToken, err := auth.ReadRefreshToken(r)
		if err == nil {
			tokenHash := auth.HashToken(refreshToken)
			sql, args := sqlite.Delete("refresh_tokens").Where("token_hash = ?", tokenHash).Build()
			_, _ = db.Exec(sql, args...)
		}

		a.ClearAllCookies(w)

		http.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "logged out",
		})
	}
}

// scopesForRole loads the scopes for a role from the database. Falls
// back to a minimal "admin" scope if the role is not found.
func scopesForRole(db *sqlite.DB, role string) []string {
	sql, args := sqlite.Select("rs.scope").
		From("role_scopes rs").
		Join("roles r", "r.id = rs.role_id").
		Where("r.name = ?", role).
		Build()
	rows, err := db.Query(sql, args...)
	if err != nil {
		return []string{"admin"}
	}
	defer rows.Close()

	var scopes []string
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			continue
		}
		scopes = append(scopes, scope)
	}
	if err := rows.Err(); err != nil {
		return []string{"admin"}
	}
	if len(scopes) == 0 {
		return []string{"admin"}
	}
	return scopes
}

// randomID generates a 16-byte hex-encoded random ID (32 characters).
func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
