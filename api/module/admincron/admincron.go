// Package admincron provides admin endpoints for viewing and managing
// cron jobs. It exposes the scheduler's entries and allows triggering,
// enabling, and disabling individual jobs.
package admincron

import (
	"strconv"

	"github.com/stanza-go/framework/pkg/cron"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminaudit"
)

// Register mounts the cron admin routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET  /api/admin/cron                    — list all cron entries
//	GET  /api/admin/cron/{name}/runs        — get run history for a job
//	POST /api/admin/cron/{name}/trigger     — trigger a job immediately
//	POST /api/admin/cron/{name}/enable      — enable a job
//	POST /api/admin/cron/{name}/disable     — disable a job
func Register(admin *http.Group, s *cron.Scheduler, db *sqlite.DB) {
	admin.HandleFunc("GET /cron", listHandler(s))
	admin.HandleFunc("GET /cron/{name}/runs", runsHandler(db))
	admin.HandleFunc("POST /cron/{name}/trigger", triggerHandler(s, db))
	admin.HandleFunc("POST /cron/{name}/enable", enableHandler(s, db))
	admin.HandleFunc("POST /cron/{name}/disable", disableHandler(s, db))
}

func listHandler(s *cron.Scheduler) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		entries := s.Entries()

		type entryJSON struct {
			Name     string `json:"name"`
			Schedule string `json:"schedule"`
			Enabled  bool   `json:"enabled"`
			Running  bool   `json:"running"`
			LastRun  string `json:"last_run"`
			NextRun  string `json:"next_run"`
			LastErr  string `json:"last_err"`
		}

		result := make([]entryJSON, len(entries))
		for i, e := range entries {
			var lastErr string
			if e.LastErr != nil {
				lastErr = e.LastErr.Error()
			}
			var lastRun, nextRun string
			if !e.LastRun.IsZero() {
				lastRun = e.LastRun.Format("2006-01-02T15:04:05Z")
			}
			if !e.NextRun.IsZero() {
				nextRun = e.NextRun.Format("2006-01-02T15:04:05Z")
			}
			result[i] = entryJSON{
				Name:     e.Name,
				Schedule: e.Schedule,
				Enabled:  e.Enabled,
				Running:  e.Running,
				LastRun:  lastRun,
				NextRun:  nextRun,
				LastErr:  lastErr,
			}
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"entries": result,
		})
	}
}

func runsHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")

		limit := 50
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}
		offset := 0
		if v := r.URL.Query().Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				offset = n
			}
		}

		type runJSON struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			StartedAt  string `json:"started_at"`
			DurationMs int64  `json:"duration_ms"`
			Status     string `json:"status"`
			Error      string `json:"error"`
		}

		var runs []runJSON
		func() {
			rows, err := db.Query(
				"SELECT id, name, started_at, duration_ms, status, error FROM cron_runs WHERE name = ? ORDER BY started_at DESC LIMIT ? OFFSET ?",
				name, limit, offset,
			)
			if err != nil {
				http.WriteError(w, http.StatusInternalServerError, "database error")
				return
			}
			defer rows.Close()

			for rows.Next() {
				var run runJSON
				if err := rows.Scan(&run.ID, &run.Name, &run.StartedAt, &run.DurationMs, &run.Status, &run.Error); err != nil {
					http.WriteError(w, http.StatusInternalServerError, "scan error")
					return
				}
				runs = append(runs, run)
			}
		}()
		if runs == nil {
			runs = []runJSON{}
		}

		// Get total count for pagination (rows must be closed first — single mutex).
		var total int64
		_ = db.QueryRow("SELECT COUNT(*) FROM cron_runs WHERE name = ?", name).Scan(&total)

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"runs":  runs,
			"total": total,
		})
	}
}

func triggerHandler(s *cron.Scheduler, db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if err := s.Trigger(name); err != nil {
			http.WriteJSON(w, http.StatusNotFound, map[string]any{
				"error": err.Error(),
			})
			return
		}
		adminaudit.Log(db, r, "cron.trigger", "cron", name, "")
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}

func enableHandler(s *cron.Scheduler, db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if err := s.Enable(name); err != nil {
			http.WriteJSON(w, http.StatusNotFound, map[string]any{
				"error": err.Error(),
			})
			return
		}
		adminaudit.Log(db, r, "cron.enable", "cron", name, "")
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}

func disableHandler(s *cron.Scheduler, db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if err := s.Disable(name); err != nil {
			http.WriteJSON(w, http.StatusNotFound, map[string]any{
				"error": err.Error(),
			})
			return
		}
		adminaudit.Log(db, r, "cron.disable", "cron", name, "")
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}
