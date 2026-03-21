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
func NewValidator(db *sqlite.DB) auth.KeyValidator {
	return func(keyHash string) (auth.Claims, error) {
		var id int64
		var scopes string
		var expiresAt string
		var revokedAt string

		sql, args := sqlite.Select("id", "scopes", "COALESCE(expires_at, '')", "COALESCE(revoked_at, '')").
			From("api_keys").
			Where("key_hash = ?", keyHash).
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&id, &scopes, &expiresAt, &revokedAt); err != nil {
			return auth.Claims{}, errors.New("api key not found")
		}

		if revokedAt != "" {
			return auth.Claims{}, errors.New("api key revoked")
		}

		if expiresAt != "" {
			t, err := time.Parse("2006-01-02T15:04:05Z", expiresAt)
			if err == nil && t.Before(time.Now().UTC()) {
				return auth.Claims{}, errors.New("api key expired")
			}
		}

		// Update last_used_at and request_count — don't block the request.
		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		db.Exec(`UPDATE api_keys SET last_used_at = ?, request_count = request_count + 1 WHERE id = ?`, now, id)

		// Build claims from key scopes.
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
