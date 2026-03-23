// Package usersettings implements per-user key-value preferences.
// All routes require a valid JWT with the "user" scope. Each user
// has their own isolated set of settings that other users cannot access.
package usersettings

import (
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
		settings, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (userSetting, error) {
			var s userSetting
			err := rows.Scan(&s.Key, &s.Value, &s.UpdatedAt)
			return s, err
		})
		if err != nil {
			http.WriteServerError(w, r, "failed to query settings", err)
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

		now := sqlite.Now()

		// Upsert each setting.
		for key, value := range req.Settings {
			sql, args := sqlite.Insert("user_settings").
				Set("user_id", claims.UID).
				Set("key", key).
				Set("value", value).
				Set("created_at", now).
				Set("updated_at", now).
				OnConflict([]string{"user_id", "key"}, []string{"value", "updated_at"}).
				Build()
			if _, err := db.Exec(sql, args...); err != nil {
				http.WriteServerError(w, r, "failed to save setting", err)
				return
			}
		}

		// Return the updated settings.
		sql, args := sqlite.Select("key", "value", "updated_at").
			From("user_settings").
			Where("user_id = ?", claims.UID).
			OrderBy("key", "ASC").
			Build()
		settings, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (userSetting, error) {
			var s userSetting
			err := rows.Scan(&s.Key, &s.Value, &s.UpdatedAt)
			return s, err
		})
		if err != nil {
			http.WriteServerError(w, r, "failed to query settings", err)
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

		dsql, dargs := sqlite.Delete("user_settings").
			Where("user_id = ?", claims.UID).
			Where("key = ?", key).
			Build()
		result, err := db.Exec(dsql, dargs...)
		if err != nil {
			http.WriteServerError(w, r, "failed to delete setting", err)
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
