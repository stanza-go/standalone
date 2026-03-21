// Package adminsettings provides the settings management endpoints. Settings
// are key-value pairs stored in SQLite, grouped by category, and editable
// in-place through the admin panel.
package adminsettings

import (
	"time"

	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminaudit"
)

// Register mounts the settings admin routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET /api/admin/settings      — list all settings grouped by category
//	PUT /api/admin/settings/{key} — update a single setting value
func Register(admin *http.Group, db *sqlite.DB) {
	admin.HandleFunc("GET /settings", listHandler(db))
	admin.HandleFunc("PUT /settings/{key}", updateHandler(db))
}

type setting struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	GroupName string `json:"group_name"`
	UpdatedAt string `json:"updated_at"`
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sql, args := sqlite.Select("key", "value", "group_name", "updated_at").
			From("settings").
			OrderBy("group_name", "ASC").
			OrderBy("key", "ASC").
			Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to query settings")
			return
		}
		defer rows.Close()

		var settings []setting
		for rows.Next() {
			var s setting
			if err := rows.Scan(&s.Key, &s.Value, &s.GroupName, &s.UpdatedAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan setting")
				return
			}
			settings = append(settings, s)
		}

		if settings == nil {
			settings = []setting{}
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"settings": settings,
		})
	}
}

func updateHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		key := http.PathParam(r, "key")
		if key == "" {
			http.WriteError(w, http.StatusBadRequest, "key is required")
			return
		}

		var req struct {
			Value string `json:"value"`
		}
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		now := time.Now().UTC().Format("2006-01-02T15:04:05Z")

		sql, args := sqlite.Update("settings").
			Set("value", req.Value).
			Set("updated_at", now).
			Where("key = ?", key).
			Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to update setting")
			return
		}

		if result.RowsAffected == 0 {
			http.WriteError(w, http.StatusNotFound, "setting not found")
			return
		}

		var s setting
		sql, args = sqlite.Select("key", "value", "group_name", "updated_at").
			From("settings").
			Where("key = ?", key).
			Build()
		row := db.QueryRow(sql, args...)
		if err := row.Scan(&s.Key, &s.Value, &s.GroupName, &s.UpdatedAt); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to read updated setting")
			return
		}

		adminaudit.Log(db, r, "setting.update", "setting", key, req.Value)

		http.WriteJSON(w, http.StatusOK, s)
	}
}
