package apikeys

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/sqlite"
)

// NewValidator returns a KeyValidator that looks up API keys in the
// database by their SHA-256 hash. It checks for revocation and
// expiration, and updates last_used_at on each successful lookup.
//
// For admin keys (entity_type="admin"), claims use UID="apikey:<id>"
// with the key's custom scopes. For user keys (entity_type="user"),
// claims use UID=<userID> with the "user" scope, matching JWT auth.
func NewValidator(db *sqlite.DB) auth.KeyValidator {
	return func(keyHash string) (auth.Claims, error) {
		var id int64
		var scopes string
		var entityType string
		var entityID string
		var expiresAt string
		var revokedAt string

		sql, args := sqlite.Select("id", "scopes", "entity_type", "COALESCE(entity_id, '')",
			"COALESCE(expires_at, '')", "COALESCE(revoked_at, '')").
			From("api_keys").
			Where("key_hash = ?", keyHash).
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&id, &scopes, &entityType, &entityID, &expiresAt, &revokedAt); err != nil {
			return auth.Claims{}, errors.New("api key not found")
		}

		if revokedAt != "" {
			return auth.Claims{}, errors.New("api key revoked")
		}

		if expiresAt != "" {
			t, err := time.Parse(time.RFC3339, expiresAt)
			if err == nil && t.Before(time.Now().UTC()) {
				return auth.Claims{}, errors.New("api key expired")
			}
		}

		// Update last_used_at and request_count — don't block the request.
		now := time.Now().UTC().Format(time.RFC3339)
		usageSQL, usageArgs := sqlite.Update("api_keys").
			Set("last_used_at", now).
			SetExpr("request_count", "request_count + 1").
			Where("id = ?", id).
			Build()
		_, _ = db.Exec(usageSQL, usageArgs...)

		// User keys: return the user's ID with "user" scope so the claims
		// match what JWT auth produces — user endpoints work transparently.
		if entityType == "user" && entityID != "" {
			return auth.Claims{
				UID:    entityID,
				Scopes: []string{"user"},
			}, nil
		}

		// Admin / system keys: custom scopes from the key record.
		var scopeList []string
		if scopes != "" {
			for _, s := range strings.Split(scopes, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					scopeList = append(scopeList, s)
				}
			}
		}

		return auth.Claims{
			UID:    "apikey:" + strconv.FormatInt(id, 10),
			Scopes: scopeList,
		}, nil
	}
}
