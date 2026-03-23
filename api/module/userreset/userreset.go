// Package userreset implements password reset endpoints for end users.
// It provides a two-step flow: request a reset (sends email with token)
// and confirm the reset (validates token, updates password).
//
// Routes:
//
//	POST /api/auth/forgot-password — request password reset email
//	POST /api/auth/reset-password  — confirm reset with token + new password
package userreset

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/email"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/validate"
)

// tokenTTL is how long a password reset token remains valid.
const tokenTTL = 30 * time.Minute

// Register mounts password reset routes on the given router group.
func Register(api *http.Group, db *sqlite.DB, emailClient *email.Client) {
	g := api.Group("/auth")

	g.HandleFunc("POST /forgot-password", forgotPasswordHandler(db, emailClient))
	g.HandleFunc("POST /reset-password", resetPasswordHandler(db))
}

// forgotPasswordRequest is the expected JSON body for POST /forgot-password.
type forgotPasswordRequest struct {
	Email string `json:"email"`
}

// forgotPasswordHandler generates a reset token, stores its hash, and
// sends a reset email. Always returns 200 regardless of whether the
// email exists — prevents email enumeration.
func forgotPasswordHandler(db *sqlite.DB, emailClient *email.Client) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		l := log.FromContext(r.Context())

		var req forgotPasswordRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		v := validate.Fields(
			validate.Required("email", req.Email),
			validate.Email("email", req.Email),
		)
		if v.HasErrors() {
			v.WriteError(w)
			return
		}

		// Always return success to prevent email enumeration.
		successResponse := map[string]string{
			"status": "If an account with that email exists, a password reset link has been sent.",
		}

		// Check if user exists.
		var userID int64
		sql, args := sqlite.Select("id").
			From("users").
			Where("email = ?", req.Email).
			WhereNull("deleted_at").
			Where("is_active = 1").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&userID); err != nil {
			// User not found — return success anyway to prevent enumeration.
			l.Debug("password reset requested for unknown email", log.String("email", req.Email))
			http.WriteJSON(w, http.StatusOK, successResponse)
			return
		}

		// Invalidate any existing unused reset tokens for this email.
		now := sqlite.Now()
		sql, args = sqlite.Update("password_reset_tokens").
			Set("used_at", now).
			Where("email = ?", req.Email).
			Where("used_at IS NULL").
			Build()
		_, _ = db.Exec(sql, args...)

		// Generate reset token (32 bytes = 64 hex chars).
		token, err := generateToken()
		if err != nil {
			http.WriteServerError(w, r, "internal error", err)
			return
		}

		tokenID, err := randomID()
		if err != nil {
			http.WriteServerError(w, r, "internal error", err)
			return
		}

		expiresAt := sqlite.FormatTime(time.Now().Add(tokenTTL))
		tokenHash := auth.HashToken(token)

		sql, args = sqlite.Insert("password_reset_tokens").
			Set("id", tokenID).
			Set("email", req.Email).
			Set("token_hash", tokenHash).
			Set("expires_at", expiresAt).
			Build()
		_, err = db.Exec(sql, args...)
		if err != nil {
			http.WriteServerError(w, r, "internal error", err)
			return
		}

		// Send the reset email.
		if emailClient.Configured() {
			if err := sendResetEmail(r.Context(), emailClient, req.Email, token); err != nil {
				l.Error("send reset email",
					log.String("email", req.Email),
					log.String("error", err.Error()),
				)
				// Don't fail the request — the token is stored and can be
				// retried. Log the error for observability.
			} else {
				l.Info("password reset email sent", log.String("email", req.Email))
			}
		} else {
			l.Warn("email not configured — reset token generated but not sent",
				log.String("email", req.Email),
				log.String("token", token),
			)
		}

		http.WriteJSON(w, http.StatusOK, successResponse)
	}
}

// resetPasswordRequest is the expected JSON body for POST /reset-password.
type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// resetPasswordHandler validates the reset token and updates the user's
// password. The token is marked as used after a successful reset.
func resetPasswordHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		l := log.FromContext(r.Context())

		var req resetPasswordRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		v := validate.Fields(
			validate.Required("token", req.Token),
			validate.Required("password", req.Password),
			validate.MinLen("password", req.Password, 8),
		)
		if v.HasErrors() {
			v.WriteError(w)
			return
		}

		tokenHash := auth.HashToken(req.Token)

		// Look up the token.
		var tokenID, tokenEmail, expiresAtStr string
		sql, args := sqlite.Select("id", "email", "expires_at").
			From("password_reset_tokens").
			Where("token_hash = ?", tokenHash).
			Where("used_at IS NULL").
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&tokenID, &tokenEmail, &expiresAtStr); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid or expired reset token")
			return
		}

		// Check expiration.
		expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
		if err != nil || time.Now().After(expiresAt) {
			// Mark expired token as used.
			now := sqlite.Now()
			sql, args = sqlite.Update("password_reset_tokens").
				Set("used_at", now).
				Where("id = ?", tokenID).
				Build()
			_, _ = db.Exec(sql, args...)

			http.WriteError(w, http.StatusBadRequest, "reset token has expired")
			return
		}

		// Hash the new password.
		passwordHash, err := auth.HashPassword(req.Password)
		if err != nil {
			http.WriteServerError(w, r, "internal error", err)
			return
		}

		// Update the user's password.
		now := sqlite.Now()
		sql, args = sqlite.Update("users").
			Set("password", passwordHash).
			Set("updated_at", now).
			Where("email = ?", tokenEmail).
			WhereNull("deleted_at").
			Where("is_active = 1").
			Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			http.WriteServerError(w, r, "internal error", err)
			return
		}
		if result.RowsAffected == 0 {
			http.WriteError(w, http.StatusBadRequest, "account not found or deactivated")
			return
		}

		// Mark the token as used.
		sql, args = sqlite.Update("password_reset_tokens").
			Set("used_at", now).
			Where("id = ?", tokenID).
			Build()
		_, _ = db.Exec(sql, args...)

		// Revoke all existing refresh tokens for this user so they must
		// log in again with the new password.
		var userID string
		sql, args = sqlite.Select("id").
			From("users").
			Where("email = ?", tokenEmail).
			Build()
		row = db.QueryRow(sql, args...)
		if err := row.Scan(&userID); err == nil {
			sql, args = sqlite.Delete("refresh_tokens").
				Where("entity_type = 'user'").
				Where("entity_id = ?", userID).
				Build()
			_, _ = db.Exec(sql, args...)
		}

		l.Info("password reset completed", log.String("email", tokenEmail))

		http.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "password has been reset",
		})
	}
}

// sendResetEmail sends the password reset email via Resend.
func sendResetEmail(ctx context.Context, client *email.Client, toEmail, token string) error {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
  <h2 style="color: #1a1a1a;">Password Reset</h2>
  <p>You requested a password reset. Use the token below to reset your password. This token expires in 30 minutes.</p>
  <div style="background: #f5f5f5; border: 1px solid #e0e0e0; border-radius: 6px; padding: 16px; margin: 20px 0; font-family: monospace; font-size: 14px; word-break: break-all;">
    %s
  </div>
  <p style="color: #666; font-size: 13px;">If you did not request this reset, you can safely ignore this email.</p>
</body>
</html>`, token)

	text := fmt.Sprintf("Password Reset\n\nYou requested a password reset. Use this token to reset your password (expires in 30 minutes):\n\n%s\n\nIf you did not request this reset, you can safely ignore this email.", token)

	_, err := client.Send(ctx, email.Message{
		To:      []string{toEmail},
		Subject: "Password Reset",
		HTML:    html,
		Text:    text,
	})
	return err
}

// generateToken creates a cryptographically random 32-byte token (64 hex chars).
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// randomID generates a 16-byte hex-encoded random ID (32 characters).
func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
