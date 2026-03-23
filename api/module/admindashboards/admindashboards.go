// Package admindashboards provides CRUD endpoints for saved metric dashboard
// configurations. Each dashboard stores a set of chart panel configs (metric
// name, time range, aggregation function, label filters) that can be loaded
// in the admin metrics explorer.
package admindashboards

import (
	"encoding/json"

	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
)

// Register mounts the dashboard CRUD routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET    /api/admin/dashboards      — list all dashboards
//	POST   /api/admin/dashboards      — create a dashboard
//	GET    /api/admin/dashboards/{id} — get a single dashboard
//	PUT    /api/admin/dashboards/{id} — update a dashboard
//	DELETE /api/admin/dashboards/{id} — delete a dashboard
func Register(admin *http.Group, db *sqlite.DB) {
	admin.HandleFunc("GET /dashboards", listHandler(db))
	admin.HandleFunc("POST /dashboards", createHandler(db))
	admin.HandleFunc("GET /dashboards/{id}", getHandler(db))
	admin.HandleFunc("PUT /dashboards/{id}", updateHandler(db))
	admin.HandleFunc("DELETE /dashboards/{id}", deleteHandler(db))
}

type dashboard struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	Panels    json.RawMessage `json:"panels"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
}

type dashboardInput struct {
	Name   string          `json:"name"`
	Panels json.RawMessage `json:"panels"`
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sql, args := sqlite.Select("id", "name", "panels", "created_at", "updated_at").
			From("metric_dashboards").
			OrderBy("updated_at", "DESC").
			Build()
		dashboards, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (dashboard, error) {
			var d dashboard
			var panels string
			err := rows.Scan(&d.ID, &d.Name, &panels, &d.CreatedAt, &d.UpdatedAt)
			d.Panels = json.RawMessage(panels)
			return d, err
		})
		if err != nil {
			http.WriteServerError(w, r, "failed to query dashboards", err)
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"dashboards": dashboards,
		})
	}
}

func createHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var input dashboardInput
		if !http.BindJSON(w, r, &input) {
			return
		}

		if !validateInput(w, input) {
			return
		}

		now := sqlite.Now()
		id, err := sqlite.Insert("metric_dashboards").
			Set("name", input.Name).
			Set("panels", string(input.Panels)).
			Set("created_at", now).
			Set("updated_at", now).
			Exec(db)
		if err != nil {
			http.WriteServerError(w, r, "failed to create dashboard", err)
			return
		}

		http.WriteJSON(w, http.StatusCreated, dashboard{
			ID:        id,
			Name:      input.Name,
			Panels:    input.Panels,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
}

func getHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		sql, args := sqlite.Select("id", "name", "panels", "created_at", "updated_at").
			From("metric_dashboards").
			Where("id = ?", id).
			Build()
		var d dashboard
		var panels string
		err := db.QueryRow(sql, args...).Scan(&d.ID, &d.Name, &panels, &d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			http.WriteError(w, http.StatusNotFound, "dashboard not found")
			return
		}
		d.Panels = json.RawMessage(panels)

		http.WriteJSON(w, http.StatusOK, d)
	}
}

func updateHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		var input dashboardInput
		if !http.BindJSON(w, r, &input) {
			return
		}

		if !validateInput(w, input) {
			return
		}

		now := sqlite.Now()
		affected, err := sqlite.Update("metric_dashboards").
			Set("name", input.Name).
			Set("panels", string(input.Panels)).
			Set("updated_at", now).
			Where("id = ?", id).
			Exec(db)
		if err != nil {
			http.WriteServerError(w, r, "failed to update dashboard", err)
			return
		}
		if affected == 0 {
			http.WriteError(w, http.StatusNotFound, "dashboard not found")
			return
		}

		// Read back the full row.
		sql, args := sqlite.Select("id", "name", "panels", "created_at", "updated_at").
			From("metric_dashboards").
			Where("id = ?", id).
			Build()
		var d dashboard
		var panels string
		err = db.QueryRow(sql, args...).Scan(&d.ID, &d.Name, &panels, &d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			http.WriteServerError(w, r, "failed to read updated dashboard", err)
			return
		}
		d.Panels = json.RawMessage(panels)

		http.WriteJSON(w, http.StatusOK, d)
	}
}

func deleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		affected, err := sqlite.Delete("metric_dashboards").
			Where("id = ?", id).
			Exec(db)
		if err != nil {
			http.WriteServerError(w, r, "failed to delete dashboard", err)
			return
		}
		if affected == 0 {
			http.WriteError(w, http.StatusNotFound, "dashboard not found")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"deleted": true,
		})
	}
}

// validateInput checks that name is present and panels is a valid JSON array.
// Returns false and writes the error response if validation fails.
func validateInput(w http.ResponseWriter, input dashboardInput) bool {
	if input.Name == "" {
		http.WriteError(w, http.StatusBadRequest, "name is required")
		return false
	}
	if len(input.Name) > 100 {
		http.WriteError(w, http.StatusBadRequest, "name must be 100 characters or fewer")
		return false
	}

	// Panels must be a valid JSON array.
	if len(input.Panels) == 0 {
		http.WriteError(w, http.StatusBadRequest, "panels is required")
		return false
	}
	var arr []json.RawMessage
	if json.Unmarshal(input.Panels, &arr) != nil {
		http.WriteError(w, http.StatusBadRequest, "panels must be a JSON array")
		return false
	}

	return true
}
