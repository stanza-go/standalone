// Package apikeys provides admin CRUD endpoints for managing API keys.
// Keys are generated server-side, returned once on creation, and stored
// as SHA-256 hashes. Supports scopes, optional expiration, and revocation.
package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/validate"
	"github.com/stanza-go/standalone/module/adminaudit"
)

// Register mounts the API key management routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET    /api/admin/api-keys      - list all API keys
//	POST   /api/admin/api-keys      - create a new API key (returns key once)
//	PUT    /api/admin/api-keys/{id} - update name or scopes
//	DELETE /api/admin/api-keys/{id} - revoke an API key
func Register(admin *http.Group, db *sqlite.DB) {
	admin.HandleFunc("GET /api-keys", listHandler(db))
	admin.HandleFunc("POST /api-keys", createHandler(db))
	admin.HandleFunc("PUT /api-keys/{id}", updateHandler(db))
	admin.HandleFunc("DELETE /api-keys/{id}", deleteHandler(db))
}

type apiKeyJSON struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	KeyPrefix    string `json:"key_prefix"`
	Scopes       string `json:"scopes"`
	CreatedBy    int64  `json:"created_by"`
	RequestCount int64  `json:"request_count"`
	LastUsedAt   string `json:"last_used_at"`
	ExpiresAt    string `json:"expires_at"`
	CreatedAt    string `json:"created_at"`
	RevokedAt    string `json:"revoked_at"`
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := http.QueryParamInt(r, "limit", 50)
		offset := http.QueryParamInt(r, "offset", 0)

		var total int
		sql, args := sqlite.Count("api_keys").Build()
		db.QueryRow(sql, args...).Scan(&total)

		sql, args = sqlite.Select("id", "name", "key_prefix", "scopes", "created_by",
			"request_count", "COALESCE(last_used_at, '')", "COALESCE(expires_at, '')",
			"created_at", "COALESCE(revoked_at, '')").
			From("api_keys").
			OrderBy("id", "DESC").
			Limit(limit).
			Offset(offset).
			Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to list api keys")
			return
		}
		defer rows.Close()

		keys := make([]apiKeyJSON, 0)
		for rows.Next() {
			var k apiKeyJSON
			if err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.Scopes, &k.CreatedBy,
				&k.RequestCount, &k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt, &k.RevokedAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan api key")
				return
			}
			keys = append(keys, k)
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"api_keys": keys,
			"total":    total,
		})
	}
}

type createRequest struct {
	Name      string `json:"name"`
	Scopes    string `json:"scopes"`
	ExpiresAt string `json:"expires_at"`
}

func createHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Validate scopes format if provided.
		scopesOK := true
		if req.Scopes != "" {
			for _, s := range strings.Split(req.Scopes, ",") {
				if strings.TrimSpace(s) == "" {
					scopesOK = false
					break
				}
			}
		}

		// Validate expiration if provided.
		var expiresOK bool
		if req.ExpiresAt == "" {
			expiresOK = true
		} else {
			t, err := time.Parse("2006-01-02T15:04:05Z", req.ExpiresAt)
			expiresOK = err == nil && !t.Before(time.Now().UTC())
		}

		v := validate.Fields(
			validate.Required("name", req.Name),
			validate.Check("scopes", scopesOK, "invalid format, use comma-separated values"),
			validate.Check("expires_at", expiresOK, "must be a valid ISO 8601 date in the future"),
		)
		if v.HasErrors() {
			v.WriteError(w)
			return
		}

		// Generate API key: 32 random bytes -> hex.
		keyBytes := make([]byte, 32)
		if _, err := rand.Read(keyBytes); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to generate key")
			return
		}
		rawKey := hex.EncodeToString(keyBytes)
		fullKey := "stza_" + rawKey
		prefix := fullKey[:13] // "stza_" + 8 hex chars

		// Hash for storage.
		hash := sha256.Sum256([]byte(fullKey))
		keyHash := hex.EncodeToString(hash[:])

		// Get the creating admin's ID from JWT claims.
		var createdBy int64
		claims, ok := auth.ClaimsFromContext(r.Context())
		if ok {
			createdBy, _ = strconv.ParseInt(claims.UID, 10, 64)
		}

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		q := sqlite.Insert("api_keys").
			Set("name", req.Name).
			Set("key_prefix", prefix).
			Set("key_hash", keyHash).
			Set("scopes", req.Scopes).
			Set("created_by", createdBy).
			Set("created_at", now)
		if req.ExpiresAt != "" {
			q.Set("expires_at", req.ExpiresAt)
		}

		sql, args := q.Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to create api key")
			return
		}

		adminaudit.Log(db, r, "api_key.create", "api_key", strconv.FormatInt(result.LastInsertID, 10), req.Name)

		http.WriteJSON(w, http.StatusCreated, map[string]any{
			"api_key": map[string]any{
				"id":         result.LastInsertID,
				"name":       req.Name,
				"key":        fullKey,
				"key_prefix": prefix,
				"scopes":     req.Scopes,
				"created_by": createdBy,
				"expires_at": req.ExpiresAt,
				"created_at": now,
			},
		})
	}
}

type updateRequest struct {
	Name   string `json:"name"`
	Scopes string `json:"scopes"`
}

func updateHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid api key id")
			return
		}

		var req updateRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Load current key.
		var current apiKeyJSON
		sql, args := sqlite.Select("name", "scopes", "key_prefix", "created_by",
			"request_count", "COALESCE(last_used_at, '')", "COALESCE(expires_at, '')",
			"created_at", "COALESCE(revoked_at, '')").
			From("api_keys").
			Where("id = ?", id).
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&current.Name, &current.Scopes, &current.KeyPrefix, &current.CreatedBy,
			&current.RequestCount, &current.LastUsedAt, &current.ExpiresAt, &current.CreatedAt, &current.RevokedAt); err != nil {
			http.WriteError(w, http.StatusNotFound, "api key not found")
			return
		}

		if current.RevokedAt != "" {
			http.WriteError(w, http.StatusBadRequest, "cannot update a revoked key")
			return
		}

		// Merge updates.
		name := current.Name
		if req.Name != "" {
			name = req.Name
		}
		scopes := current.Scopes
		if req.Scopes != "" {
			scopes = req.Scopes
		}

		sql, args = sqlite.Update("api_keys").
			Set("name", name).
			Set("scopes", scopes).
			Where("id = ?", id).
			Build()
		if _, err := db.Exec(sql, args...); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to update api key")
			return
		}

		current.ID = id
		current.Name = name
		current.Scopes = scopes

		adminaudit.Log(db, r, "api_key.update", "api_key", strconv.FormatInt(id, 10), name)

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"api_key": current,
		})
	}
}

func deleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid api key id")
			return
		}

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		sql, args := sqlite.Update("api_keys").
			Set("revoked_at", now).
			Where("id = ?", id).
			Where("revoked_at IS NULL").
			Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to revoke api key")
			return
		}
		if result.RowsAffected == 0 {
			http.WriteError(w, http.StatusNotFound, "api key not found or already revoked")
			return
		}

		adminaudit.Log(db, r, "api_key.revoke", "api_key", strconv.FormatInt(id, 10), "")

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}
