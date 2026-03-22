// Package usersettings implements per-user key-value preferences.
// All routes require a valid JWT with the "user" scope. Each user
// has their own isolated set of settings that other users cannot access.
package usersettings

import (
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/framework/pkg/validate"
)

// Register mounts the user settings routes on the given group.
// The group should already have RequireAuth + RequireScope("user") applied.
//
// Routes:
//
//	GET    /settings      — list all settings for the authenticated user
//	GET    /settings/{key} — get a specific setting by key
//	PUT    /settings      — batch upsert settings (accepts {"settings": {"key": "value", ...}})
//	DELETE /settings/{key} — delete a specific setting
func Register(user *http.Group, db *sqlite.DB) {
	user.HandleFunc("GET /settings", listSettings(db))
	user.HandleFunc("GET /settings/{key}", getSetting(db))
	user.HandleFunc("PUT /settings", batchUpsert(db))
	user.HandleFunc("DELETE /settings/{key}", deleteSetting(db))
}

type userSetting struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"`
}

// listSettings returns all settings for the authenticated user.
func listSettings(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		sql, args := sqlite.Select("key", "value", "updated_at").
			From("user_settings").
			Where("user_id = ?", claims.UID).
			OrderBy("key", "ASC").
			Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to query settings")
			return
		}
		defer rows.Close()

		settings := make([]userSetting, 0)
		for rows.Next() {
			var s userSetting
			if err := rows.Scan(&s.Key, &s.Value, &s.UpdatedAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan setting")
				return
			}
			settings = append(settings, s)
		}
		if err := rows.Err(); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to iterate settings")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"settings": settings,
		})
	}
}

// getSetting returns a single setting by key for the authenticated user.
func getSetting(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		key := r.PathValue("key")
		if key == "" {
			http.WriteError(w, http.StatusBadRequest, "key is required")
			return
		}

		var s userSetting
		sql, args := sqlite.Select("key", "value", "updated_at").
			From("user_settings").
			Where("user_id = ?", claims.UID).
			Where("key = ?", key).
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&s.Key, &s.Value, &s.UpdatedAt); err != nil {
			http.WriteError(w, http.StatusNotFound, "setting not found")
			return
		}

		http.WriteJSON(w, http.StatusOK, s)
	}
}

// batchRequest is the expected JSON body for PUT /settings.
type batchRequest struct {
	Settings map[string]string `json:"settings"`
}

// batchUpsert creates or updates multiple settings at once.
func batchUpsert(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		var req batchRequest
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		v := validate.Fields(
			validate.Check("settings", len(req.Settings) > 0, "at least one setting is required"),
			validate.Check("settings", len(req.Settings) <= 50, "maximum 50 settings per request"),
		)
		if v.HasErrors() {
			v.WriteError(w)
			return
		}

		// Validate all keys are non-empty and reasonable length.
		for key := range req.Settings {
			kv := validate.Fields(
				validate.Check("key", len(key) >= 1 && len(key) <= 255, "key must be 1-255 characters"),
			)
			if kv.HasErrors() {
				kv.WriteError(w)
				return
			}
		}

		now := time.Now().UTC().Format(time.RFC3339)

		// Upsert each setting using INSERT OR REPLACE.
		for key, value := range req.Settings {
			_, err := db.Exec(
				`INSERT INTO user_settings (user_id, key, value, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?)
				 ON CONFLICT(user_id, key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
				claims.UID, key, value, now, now,
			)
			if err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to save setting")
				return
			}
		}

		// Return the updated settings.
		sql, args := sqlite.Select("key", "value", "updated_at").
			From("user_settings").
			Where("user_id = ?", claims.UID).
			OrderBy("key", "ASC").
			Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to query settings")
			return
		}
		defer rows.Close()

		settings := make([]userSetting, 0)
		for rows.Next() {
			var s userSetting
			if err := rows.Scan(&s.Key, &s.Value, &s.UpdatedAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan setting")
				return
			}
			settings = append(settings, s)
		}
		if err := rows.Err(); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to iterate settings")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"settings": settings,
		})
	}
}

// deleteSetting removes a specific setting by key.
func deleteSetting(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			http.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		key := r.PathValue("key")
		if key == "" {
			http.WriteError(w, http.StatusBadRequest, "key is required")
			return
		}

		result, err := db.Exec(
			"DELETE FROM user_settings WHERE user_id = ? AND key = ?",
			claims.UID, key,
		)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to delete setting")
			return
		}

		if result.RowsAffected == 0 {
			http.WriteError(w, http.StatusNotFound, "setting not found")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "setting deleted",
		})
	}
}
