// Package userapikeys provides user-facing endpoints for managing personal
// API keys. Users can create keys for programmatic access to their own
// data via the /api/user/* endpoints. All queries are scoped to the
// authenticated user via entity_type="user" and entity_id=userID.
package userapikeys

import (
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/validate"
)

const entityType = "user"

// Register mounts the user API key management routes on the given group.
// The group should already have user auth middleware applied.
// Routes:
//
//	GET    /api/user/api-keys      - list user's API keys
//	POST   /api/user/api-keys      - create a new API key (returns key once)
//	PUT    /api/user/api-keys/{id} - update name
//	DELETE /api/user/api-keys/{id} - revoke an API key
func Register(user *http.Group, db *sqlite.DB) {
	user.HandleFunc("GET /api-keys", listHandler(db))
	user.HandleFunc("POST /api-keys", createHandler(db))
	user.HandleFunc("PUT /api-keys/{id}", updateHandler(db))
	user.HandleFunc("DELETE /api-keys/{id}", deleteHandler(db))
}

type apiKeyJSON struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	KeyPrefix    string `json:"key_prefix"`
	RequestCount int64  `json:"request_count"`
	LastUsedAt   string `json:"last_used_at"`
	ExpiresAt    string `json:"expires_at"`
	CreatedAt    string `json:"created_at"`
	RevokedAt    string `json:"revoked_at"`
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.UID

		pg := http.ParsePagination(r, 50, 100)
		search := r.URL.Query().Get("search")

		selectQ := sqlite.Select("id", "name", "key_prefix",
			"request_count", "COALESCE(last_used_at, '')", "COALESCE(expires_at, '')",
			"created_at", "COALESCE(revoked_at, '')").
			From("api_keys").
			Where("entity_type = ?", entityType).
			Where("entity_id = ?", userID)
		selectQ.WhereSearch(search, "name", "key_prefix")

		total, _ := db.Count(selectQ)

		sql, args := selectQ.
			OrderBy("id", "DESC").
			Limit(pg.Limit).
			Offset(pg.Offset).
			Build()
		keys, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (apiKeyJSON, error) {
			var k apiKeyJSON
			err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix,
				&k.RequestCount, &k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt, &k.RevokedAt)
			return k, err
		})
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to list api keys")
			return
		}

		http.PaginatedResponse(w, "api_keys", keys, total)
	}
}

type createRequest struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
}

func createHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.UID

		var req createRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Validate expiration if provided.
		var expiresOK bool
		if req.ExpiresAt == "" {
			expiresOK = true
		} else {
			t, err := time.Parse(time.RFC3339, req.ExpiresAt)
			expiresOK = err == nil && !t.Before(time.Now().UTC())
		}

		v := validate.Fields(
			validate.Required("name", req.Name),
			validate.Check("expires_at", expiresOK, "must be a valid ISO 8601 date in the future"),
		)
		if v.HasErrors() {
			v.WriteError(w)
			return
		}

		// Generate API key with prefix, display prefix, and hash.
		fullKey, prefix, keyHash, err := auth.GenerateAPIKey("stza_")
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to generate key")
			return
		}

		createdBy := claims.IntUID()
		now := time.Now().UTC().Format(time.RFC3339)

		q := sqlite.Insert("api_keys").
			Set("name", req.Name).
			Set("key_prefix", prefix).
			Set("key_hash", keyHash).
			Set("scopes", "user").
			Set("entity_type", entityType).
			Set("entity_id", userID).
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

		http.WriteJSON(w, http.StatusCreated, map[string]any{
			"api_key": map[string]any{
				"id":         result.LastInsertID,
				"name":       req.Name,
				"key":        fullKey,
				"key_prefix": prefix,
				"expires_at": req.ExpiresAt,
				"created_at": now,
			},
		})
	}
}

type updateRequest struct {
	Name string `json:"name"`
}

func updateHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.UID

		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		var req updateRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Load current key scoped to user.
		var current apiKeyJSON
		sql, args := sqlite.Select("name", "key_prefix",
			"request_count", "COALESCE(last_used_at, '')", "COALESCE(expires_at, '')",
			"created_at", "COALESCE(revoked_at, '')").
			From("api_keys").
			Where("id = ?", id).
			Where("entity_type = ?", entityType).
			Where("entity_id = ?", userID).
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&current.Name, &current.KeyPrefix,
			&current.RequestCount, &current.LastUsedAt, &current.ExpiresAt,
			&current.CreatedAt, &current.RevokedAt); err != nil {
			http.WriteError(w, http.StatusNotFound, "api key not found")
			return
		}

		if current.RevokedAt != "" {
			http.WriteError(w, http.StatusBadRequest, "cannot update a revoked key")
			return
		}

		name := current.Name
		if req.Name != "" {
			name = req.Name
		}

		sql, args = sqlite.Update("api_keys").
			Set("name", name).
			Where("id = ?", id).
			Where("entity_type = ?", entityType).
			Where("entity_id = ?", userID).
			Build()
		if _, err := db.Exec(sql, args...); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to update api key")
			return
		}

		current.ID = id
		current.Name = name

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"api_key": current,
		})
	}
}

func deleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.UID

		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		now := time.Now().UTC().Format(time.RFC3339)
		sql, args := sqlite.Update("api_keys").
			Set("revoked_at", now).
			Where("id = ?", id).
			Where("entity_type = ?", entityType).
			Where("entity_id = ?", userID).
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

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}

