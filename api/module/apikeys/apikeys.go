// Package apikeys provides admin CRUD endpoints for managing API keys.
// Keys are generated server-side, returned once on creation, and stored
// as SHA-256 hashes. Supports scopes, optional expiration, and revocation.
package apikeys

import (
	"strconv"
	"strings"

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
			"entity_type", sqlite.CoalesceEmpty("entity_id"), "created_by",
			"request_count", sqlite.CoalesceEmpty("last_used_at"), sqlite.CoalesceEmpty("expires_at"),
			"created_at", sqlite.CoalesceEmpty("revoked_at")).
			From("api_keys")
		selectQ.WhereSearch(search, "name", "key_prefix")

		total, _ := db.Count(selectQ)

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "name", "created_at", "last_used_at", "request_count"},
			"id", "DESC")
		sql, args := selectQ.
			OrderBy(sortCol, sortDir).
			Limit(pg.Limit).
			Offset(pg.Offset).
			Build()
		keys, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (apiKeyJSON, error) {
			var k apiKeyJSON
			err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.Scopes,
				&k.EntityType, &k.EntityID, &k.CreatedBy,
				&k.RequestCount, &k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt, &k.RevokedAt)
			return k, err
		})
		if err != nil {
			http.WriteServerError(w, r, "failed to list api keys", err)
			return
		}

		http.PaginatedResponse(w, "api_keys", keys, total)
	}
}

func exportHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")

		q := sqlite.Select("id", "name", "key_prefix", "scopes",
			"entity_type", sqlite.CoalesceEmpty("entity_id"), "request_count",
			sqlite.CoalesceEmpty("last_used_at"), sqlite.CoalesceEmpty("expires_at"),
			"created_at", sqlite.CoalesceEmpty("revoked_at")).
			From("api_keys").
			WhereSearch(search, "name", "key_prefix")

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "name", "created_at", "last_used_at", "request_count"},
			"id", "DESC")

		sql, args := q.OrderBy(sortCol, sortDir).Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteServerError(w, r, "failed to export api keys", err)
			return
		}
		defer rows.Close()

		http.WriteCSV(w, "api-keys", []string{"ID", "Name", "Key Prefix", "Scopes", "Entity Type", "Entity ID", "Request Count", "Last Used At", "Expires At", "Created At", "Revoked At"}, func() []string {
			if !rows.Next() {
				return nil
			}
			var id, requestCount int64
			var name, keyPrefix, scopes, entityType, entityID, lastUsedAt, expiresAt, createdAt, revokedAt string
			if err := rows.Scan(&id, &name, &keyPrefix, &scopes, &entityType, &entityID, &requestCount, &lastUsedAt, &expiresAt, &createdAt, &revokedAt); err != nil {
				return nil
			}
			return []string{sqlite.FormatID(id), name, keyPrefix, scopes, entityType, entityID, strconv.FormatInt(requestCount, 10), lastUsedAt, expiresAt, createdAt, revokedAt}
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

		v := validate.Fields(
			validate.Required("name", req.Name),
			validate.Check("scopes", scopesOK, "invalid format, use comma-separated values"),
			validate.FutureDate("expires_at", req.ExpiresAt),
		)
		if v.HasErrors() {
			v.WriteError(w)
			return
		}

		// Generate API key with prefix, display prefix, and hash.
		fullKey, prefix, keyHash, err := auth.GenerateAPIKey("stza_")
		if err != nil {
			http.WriteServerError(w, r, "failed to generate key", err)
			return
		}

		// Get the creating admin's ID from JWT claims.
		var createdBy int64
		claims, ok := auth.ClaimsFromContext(r.Context())
		if ok {
			createdBy = claims.IntUID()
		}

		now := sqlite.Now()
		entityID := sqlite.FormatID(createdBy)
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

		id, err := db.Insert(q)
		if err != nil {
			http.WriteServerError(w, r, "failed to create api key", err)
			return
		}

		adminaudit.Log(db, r, "api_key.create", "api_key", sqlite.FormatID(id), req.Name)

		http.WriteJSON(w, http.StatusCreated, map[string]any{
			"api_key": map[string]any{
				"id":          id,
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
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
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
			"entity_type", sqlite.CoalesceEmpty("entity_id"), "created_by",
			"request_count", sqlite.CoalesceEmpty("last_used_at"), sqlite.CoalesceEmpty("expires_at"),
			"created_at", sqlite.CoalesceEmpty("revoked_at")).
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

		if _, err := db.Update(sqlite.Update("api_keys").
			Set("name", name).
			Set("scopes", scopes).
			Where("id = ?", id)); err != nil {
			http.WriteServerError(w, r, "failed to update api key", err)
			return
		}

		current.ID = id
		current.Name = name
		current.Scopes = scopes

		adminaudit.Log(db, r, "api_key.update", "api_key", sqlite.FormatID(id), name)

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"api_key": current,
		})
	}
}

func deleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		now := sqlite.Now()
		n, err := db.Update(sqlite.Update("api_keys").
			Set("revoked_at", now).
			Where("id = ?", id).
			Where("revoked_at IS NULL"))
		if err != nil {
			http.WriteServerError(w, r, "failed to revoke api key", err)
			return
		}
		if n == 0 {
			http.WriteError(w, http.StatusNotFound, "api key not found or already revoked")
			return
		}

		adminaudit.Log(db, r, "api_key.revoke", "api_key", sqlite.FormatID(id), "")

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
		if !http.CheckBulkIDs(w, req.IDs, 100) {
			return
		}

		now := sqlite.Now()
		ids := make([]any, len(req.IDs))
		for i, id := range req.IDs {
			ids[i] = id
		}

		n, err := db.Update(sqlite.Update("api_keys").
			Set("revoked_at", now).
			Where("revoked_at IS NULL").
			WhereIn("id", ids...))
		if err != nil {
			http.WriteServerError(w, r, "failed to bulk revoke api keys", err)
			return
		}

		for _, id := range req.IDs {
			adminaudit.Log(db, r, "api_key.revoke", "api_key", sqlite.FormatID(id), "bulk")
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"affected": n,
		})
	}
}

