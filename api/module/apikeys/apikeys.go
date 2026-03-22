// Package apikeys provides admin CRUD endpoints for managing API keys.
// Keys are generated server-side, returned once on creation, and stored
// as SHA-256 hashes. Supports scopes, optional expiration, and revocation.
package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
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
	admin.HandleFunc("GET /api-keys/export", exportHandler(db))
	admin.HandleFunc("POST /api-keys", createHandler(db))
	admin.HandleFunc("POST /api-keys/bulk-revoke", bulkRevokeHandler(db))
	admin.HandleFunc("PUT /api-keys/{id}", updateHandler(db))
	admin.HandleFunc("DELETE /api-keys/{id}", deleteHandler(db))
}

type apiKeyJSON struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	KeyPrefix    string `json:"key_prefix"`
	Scopes       string `json:"scopes"`
	EntityType   string `json:"entity_type"`
	EntityID     string `json:"entity_id"`
	CreatedBy    int64  `json:"created_by"`
	RequestCount int64  `json:"request_count"`
	LastUsedAt   string `json:"last_used_at"`
	ExpiresAt    string `json:"expires_at"`
	CreatedAt    string `json:"created_at"`
	RevokedAt    string `json:"revoked_at"`
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pg := http.ParsePagination(r, 50, 100)
		search := r.URL.Query().Get("search")

		selectQ := sqlite.Select("id", "name", "key_prefix", "scopes",
			"entity_type", "COALESCE(entity_id, '')", "created_by",
			"request_count", "COALESCE(last_used_at, '')", "COALESCE(expires_at, '')",
			"created_at", "COALESCE(revoked_at, '')").
			From("api_keys")
		if search != "" {
			like := "%" + escapeLike(search) + "%"
			selectQ.Where("(name LIKE ? ESCAPE '\\' OR key_prefix LIKE ? ESCAPE '\\')", like, like)
		}

		var total int
		sql, args := sqlite.CountFrom(selectQ).Build()
		_ = db.QueryRow(sql, args...).Scan(&total)

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "name", "created_at", "last_used_at", "request_count"},
			"id", "DESC")
		sql, args = selectQ.
			OrderBy(sortCol, sortDir).
			Limit(pg.Limit).
			Offset(pg.Offset).
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
			if err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.Scopes,
				&k.EntityType, &k.EntityID, &k.CreatedBy,
				&k.RequestCount, &k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt, &k.RevokedAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan api key")
				return
			}
			keys = append(keys, k)
		}
		if err := rows.Err(); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to iterate api keys")
			return
		}

		http.PaginatedResponse(w, "api_keys", keys, total)
	}
}

func exportHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")

		q := sqlite.Select("id", "name", "key_prefix", "scopes",
			"entity_type", "COALESCE(entity_id, '')", "request_count",
			"COALESCE(last_used_at, '')", "COALESCE(expires_at, '')",
			"created_at", "COALESCE(revoked_at, '')").
			From("api_keys")
		if search != "" {
			like := "%" + escapeLike(search) + "%"
			q.Where("(name LIKE ? ESCAPE '\\' OR key_prefix LIKE ? ESCAPE '\\')", like, like)
		}

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "name", "created_at", "last_used_at", "request_count"},
			"id", "DESC")

		sql, args := q.OrderBy(sortCol, sortDir).Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to export api keys")
			return
		}
		defer rows.Close()

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=api-keys-%s.csv", time.Now().UTC().Format("20060102")))
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"ID", "Name", "Key Prefix", "Scopes", "Entity Type", "Entity ID", "Request Count", "Last Used At", "Expires At", "Created At", "Revoked At"})

		for rows.Next() {
			var id, requestCount int64
			var name, keyPrefix, scopes, entityType, entityID, lastUsedAt, expiresAt, createdAt, revokedAt string
			if err := rows.Scan(&id, &name, &keyPrefix, &scopes, &entityType, &entityID, &requestCount, &lastUsedAt, &expiresAt, &createdAt, &revokedAt); err != nil {
				break
			}
			_ = cw.Write([]string{strconv.FormatInt(id, 10), name, keyPrefix, scopes, entityType, entityID, strconv.FormatInt(requestCount, 10), lastUsedAt, expiresAt, createdAt, revokedAt})
		}
		cw.Flush()
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
		entityID := strconv.FormatInt(createdBy, 10)
		q := sqlite.Insert("api_keys").
			Set("name", req.Name).
			Set("key_prefix", prefix).
			Set("key_hash", keyHash).
			Set("scopes", req.Scopes).
			Set("entity_type", "admin").
			Set("entity_id", entityID).
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
				"id":          result.LastInsertID,
				"name":        req.Name,
				"key":         fullKey,
				"key_prefix":  prefix,
				"scopes":      req.Scopes,
				"entity_type": "admin",
				"entity_id":   entityID,
				"created_by":  createdBy,
				"expires_at":  req.ExpiresAt,
				"created_at":  now,
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
		sql, args := sqlite.Select("name", "scopes", "key_prefix",
			"entity_type", "COALESCE(entity_id, '')", "created_by",
			"request_count", "COALESCE(last_used_at, '')", "COALESCE(expires_at, '')",
			"created_at", "COALESCE(revoked_at, '')").
			From("api_keys").
			Where("id = ?", id).
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&current.Name, &current.Scopes, &current.KeyPrefix,
			&current.EntityType, &current.EntityID, &current.CreatedBy,
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

func bulkRevokeHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs []int64 `json:"ids"`
		}
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(req.IDs) == 0 {
			http.WriteError(w, http.StatusBadRequest, "ids required")
			return
		}
		if len(req.IDs) > 100 {
			http.WriteError(w, http.StatusBadRequest, "maximum 100 ids per request")
			return
		}

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		ids := make([]any, len(req.IDs))
		for i, id := range req.IDs {
			ids[i] = id
		}

		query, args := sqlite.Update("api_keys").
			Set("revoked_at", now).
			Where("revoked_at IS NULL").
			WhereIn("id", ids...).
			Build()
		result, err := db.Exec(query, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to bulk revoke api keys")
			return
		}

		for _, id := range req.IDs {
			adminaudit.Log(db, r, "api_key.revoke", "api_key", strconv.FormatInt(id, 10), "bulk")
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"affected": result.RowsAffected,
		})
	}
}

// escapeLike escapes LIKE wildcards (% and _) in a search term so they
// are matched literally when used with ESCAPE '\'.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
