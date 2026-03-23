// Package userauth implements end-user authentication endpoints:
// register, login, status check (with token refresh), and logout.
// It mirrors the admin auth flow but uses the users table and
// cookie paths scoped to /api (not /api/admin).
package userauth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/validate"
	"github.com/stanza-go/standalone/module/webhooks"
)

// Register mounts the user auth routes on the given router group.
// Routes:
//
//	POST /api/auth/register — create a new user account
//	POST /api/auth/login    — authenticate with email + password
//	GET  /api/auth          — status check, refresh access token
//	POST /api/auth/logout   — revoke session, clear cookies
func Register(api *http.Group, a *auth.Auth, db *sqlite.DB, wh *webhooks.Dispatcher) {
	g := api.Group("/auth")

	g.HandleFunc("POST /register", registerHandler(a, db, wh))
	g.HandleFunc("POST /login", loginHandler(a, db))
	g.HandleFunc("GET /", statusHandler(a, db))
	g.HandleFunc("POST /logout", logoutHandler(a, db))
}

// registerRequest is the expected JSON body for POST /register.
type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// registerHandler creates a new user account, issues tokens, and logs
// them in automatically.
func registerHandler(a *auth.Auth, db *sqlite.DB, wh *webhooks.Dispatcher) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		l := log.FromContext(r.Context())

		var req registerRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		req.Name = strings.TrimSpace(req.Name)

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

		passwordHash, err := auth.HashPassword(req.Password)
		if err != nil {
			http.WriteServerError(w, r, "internal error", err)
			return
		}

		now := sqlite.Now()
		id, err := db.Insert(sqlite.Insert("users").
			Set("email", req.Email).
			Set("password", passwordHash).
			Set("name", req.Name).
			Set("is_active", true).
			Set("created_at", now).
			Set("updated_at", now))
		if err != nil {
			if sqlite.IsUniqueConstraintError(err) {
				http.WriteError(w, http.StatusConflict, "email already registered")
				return
			}
			http.WriteServerError(w, r, "internal error", err)
			return
		}
		uid := sqlite.FormatID(id)

		// Auto-login: issue tokens.
		if err := issueSession(w, a, db, l, uid); err != nil {
			return
		}

		l.Info("user registered", log.String("email", req.Email), log.String("uid", uid))

		_ = wh.Dispatch(r.Context(), "user.registered", map[string]any{
			"id":    id,
			"email": req.Email,
			"name":  req.Name,
		})

		http.WriteJSON(w, http.StatusCreated, map[string]any{
			"user": map[string]any{
				"id":    id,
				"email": req.Email,
				"name":  req.Name,
			},
		})
	}
}

// loginRequest is the expected JSON body for POST /login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginHandler authenticates a user by email and password, issues
// access and refresh tokens, and sets them as cookies.
func loginHandler(a *auth.Auth, db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		l := log.FromContext(r.Context())

		var req loginRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		v := validate.Fields(
			validate.Required("email", req.Email),
			validate.Required("password", req.Password),
		)
		if v.HasErrors() {
			v.WriteError(w)
			return
		}

		var id int64
		var passwordHash, name string
		sql, args := sqlite.Select("id", "password", "name").
			From("users").
			Where("email = ?", strings.TrimSpace(strings.ToLower(req.Email))).
			WhereNull("deleted_at").
			Where("is_active = 1").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&id, &passwordHash, &name); err != nil {
			http.WriteError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		if !auth.VerifyPassword(passwordHash, req.Password) {
			http.WriteError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		uid := sqlite.FormatID(id)

		if err := issueSession(w, a, db, l, uid); err != nil {
			return
		}

		l.Info("user login", log.String("email", req.Email), log.String("uid", uid))

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"user": map[string]any{
				"id":    id,
				"email": req.Email,
				"name":  name,
			},
		})
	}
}

// statusHandler validates the refresh token, checks if the user is
// still active, and issues a fresh access token with up-to-date
// scopes. The frontend polls this endpoint every ~1 minute.
func statusHandler(a *auth.Auth, db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		refreshToken, err := auth.ReadRefreshToken(r)
		if err != nil {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		tokenHash := auth.HashToken(refreshToken)

		var entityID, expiresAtStr string
		sql, args := sqlite.Select("entity_id", "expires_at").
			From("refresh_tokens").
			Where("token_hash = ?", tokenHash).
			Where("entity_type = 'user'").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&entityID, &expiresAtStr); err != nil {
			http.WriteError(w, http.StatusUnauthorized, "invalid session")
			return
		}

		expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
		if err != nil || time.Now().After(expiresAt) {
			_, _ = db.Delete(sqlite.Delete("refresh_tokens").Where("token_hash = ?", tokenHash))
			a.ClearAllCookies(w)
			http.WriteError(w, http.StatusUnauthorized, "session expired")
			return
		}

		var id int64
		var email, name string
		sql, args = sqlite.Select("id", "email", "name").
			From("users").
			Where("id = ?", entityID).
			WhereNull("deleted_at").
			Where("is_active = 1").
			Build()
		row = db.QueryRow(sql, args...)
		if err := row.Scan(&id, &email, &name); err != nil {
			_, _ = db.Delete(sqlite.Delete("refresh_tokens").Where("token_hash = ?", tokenHash))
			a.ClearAllCookies(w)
			http.WriteError(w, http.StatusUnauthorized, "account deactivated")
			return
		}

		uid := sqlite.FormatID(id)

		accessToken, err := a.IssueAccessToken(uid, []string{"user"})
		if err != nil {
			http.WriteServerError(w, r, "internal error", err)
			return
		}

		a.SetAccessTokenCookie(w, accessToken)

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"user": map[string]any{
				"id":    id,
				"email": email,
				"name":  name,
			},
		})
	}
}

// logoutHandler revokes the refresh token and clears all cookies.
func logoutHandler(a *auth.Auth, db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		refreshToken, err := auth.ReadRefreshToken(r)
		if err == nil {
			tokenHash := auth.HashToken(refreshToken)
			_, _ = db.Delete(sqlite.Delete("refresh_tokens").Where("token_hash = ?", tokenHash))
		}

		a.ClearAllCookies(w)

		http.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "logged out",
		})
	}
}

// issueSession creates access and refresh tokens, stores the refresh
// token hash in the database, and sets both as cookies. Shared by
// register and login handlers.
func issueSession(w http.ResponseWriter, a *auth.Auth, db *sqlite.DB, logger *log.Logger, uid string) error {
	accessToken, err := a.IssueAccessToken(uid, []string{"user"})
	if err != nil {
		logger.Error("issue access token", log.String("error", err.Error()))
		http.WriteError(w, http.StatusInternalServerError, "internal error")
		return err
	}

	refreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		logger.Error("generate refresh token", log.String("error", err.Error()))
		http.WriteError(w, http.StatusInternalServerError, "internal error")
		return err
	}

	tokenID, err := randomID()
	if err != nil {
		logger.Error("generate token id", log.String("error", err.Error()))
		http.WriteError(w, http.StatusInternalServerError, "internal error")
		return err
	}

	expiresAt := sqlite.FormatTime(time.Now().Add(a.RefreshTokenTTL()))
	_, err = db.Insert(sqlite.Insert("refresh_tokens").
		Set("id", tokenID).
		Set("entity_type", "user").
		Set("entity_id", uid).
		Set("token_hash", auth.HashToken(refreshToken)).
		Set("expires_at", expiresAt))
	if err != nil {
		logger.Error("store refresh token", log.String("error", err.Error()))
		http.WriteError(w, http.StatusInternalServerError, "internal error")
		return err
	}

	a.SetAccessTokenCookie(w, accessToken)
	a.SetRefreshTokenCookie(w, refreshToken)
	return nil
}

// randomID generates a 16-byte hex-encoded random ID (32 characters).
func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
