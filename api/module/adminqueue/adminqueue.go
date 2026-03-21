// Package adminqueue provides admin endpoints for viewing and managing
// the job queue. It exposes queue stats, job listing with filters,
// and actions to retry or cancel individual jobs.
package adminqueue

import (
	"strconv"

	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/queue"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminaudit"
)

// Register mounts the queue admin routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET  /api/admin/queue/stats       — queue stats by status
//	GET  /api/admin/queue/jobs        — list jobs (query: status, type, limit, offset)
//	POST /api/admin/queue/jobs/{id}/retry  — retry a failed/dead job
//	POST /api/admin/queue/jobs/{id}/cancel — cancel a pending job
func Register(admin *http.Group, q *queue.Queue, db *sqlite.DB) {
	admin.HandleFunc("GET /queue/stats", statsHandler(q))
	admin.HandleFunc("GET /queue/jobs", jobsHandler(q))
	admin.HandleFunc("POST /queue/jobs/{id}/retry", retryHandler(q, db))
	admin.HandleFunc("POST /queue/jobs/{id}/cancel", cancelHandler(q, db))
}

func statsHandler(q *queue.Queue) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := q.Stats()
		if err != nil {
			http.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error": "failed to get queue stats",
			})
			return
		}
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"pending":   stats.Pending,
			"running":   stats.Running,
			"completed": stats.Completed,
			"failed":    stats.Failed,
			"dead":      stats.Dead,
			"cancelled": stats.Cancelled,
		})
	}
}

func jobsHandler(q *queue.Queue) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		f := queue.Filter{
			Status: r.URL.Query().Get("status"),
			Type:   r.URL.Query().Get("type"),
			Queue:  r.URL.Query().Get("queue"),
		}

		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				f.Limit = n
			}
		}
		if f.Limit == 0 {
			f.Limit = 50
		}

		if v := r.URL.Query().Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				f.Offset = n
			}
		}

		jobs, err := q.Jobs(f)
		if err != nil {
			http.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error": "failed to list jobs",
			})
			return
		}

		type jobJSON struct {
			ID          int64  `json:"id"`
			Queue       string `json:"queue"`
			Type        string `json:"type"`
			Payload     string `json:"payload"`
			Status      string `json:"status"`
			Attempts    int    `json:"attempts"`
			MaxAttempts int    `json:"max_attempts"`
			LastError   string `json:"last_error"`
			RunAt       string `json:"run_at"`
			StartedAt   string `json:"started_at"`
			CompletedAt string `json:"completed_at"`
			CreatedAt   string `json:"created_at"`
			UpdatedAt   string `json:"updated_at"`
		}

		result := make([]jobJSON, len(jobs))
		for i, j := range jobs {
			var runAt, startedAt, completedAt string
			if !j.RunAt.IsZero() {
				runAt = j.RunAt.Format("2006-01-02T15:04:05Z")
			}
			if !j.StartedAt.IsZero() {
				startedAt = j.StartedAt.Format("2006-01-02T15:04:05Z")
			}
			if !j.CompletedAt.IsZero() {
				completedAt = j.CompletedAt.Format("2006-01-02T15:04:05Z")
			}
			result[i] = jobJSON{
				ID:          j.ID,
				Queue:       j.Queue,
				Type:        j.Type,
				Payload:     string(j.Payload),
				Status:      j.Status,
				Attempts:    j.Attempts,
				MaxAttempts: j.MaxAttempts,
				LastError:   j.LastError,
				RunAt:       runAt,
				StartedAt:   startedAt,
				CompletedAt: completedAt,
				CreatedAt:   j.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt:   j.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			}
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"jobs": result,
		})
	}
}

func retryHandler(q *queue.Queue, db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid job id",
			})
			return
		}
		if err := q.Retry(id); err != nil {
			http.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": err.Error(),
			})
			return
		}
		adminaudit.Log(db, r, "job.retry", "job", strconv.FormatInt(id, 10), "")
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}

func cancelHandler(q *queue.Queue, db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid job id",
			})
			return
		}
		if err := q.Cancel(id); err != nil {
			http.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": err.Error(),
			})
			return
		}
		adminaudit.Log(db, r, "job.cancel", "job", strconv.FormatInt(id, 10), "")
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}
