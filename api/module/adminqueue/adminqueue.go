// Package adminqueue provides admin endpoints for viewing and managing
// the job queue. It exposes queue stats, job listing with filters,
// and actions to retry or cancel individual jobs.
package adminqueue

import (
	"time"

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
//	POST /api/admin/queue/jobs/{id}/cancel — cancel a pending or running job
func Register(admin *http.Group, q *queue.Queue, db *sqlite.DB) {
	admin.HandleFunc("GET /queue/stats", statsHandler(q))
	admin.HandleFunc("GET /queue/jobs", jobsHandler(q))
	admin.HandleFunc("POST /queue/jobs/bulk-retry", bulkRetryHandler(q, db))
	admin.HandleFunc("POST /queue/jobs/bulk-cancel", bulkCancelHandler(q, db))
	admin.HandleFunc("POST /queue/jobs/{id}/retry", retryHandler(q, db))
	admin.HandleFunc("POST /queue/jobs/{id}/cancel", cancelHandler(q, db))
}

func statsHandler(q *queue.Queue) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := q.Stats()
		if err != nil {
			http.WriteServerError(w, r, "failed to get queue stats", err)
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

		pg := http.ParsePagination(r, 50, 100)
		f.Limit = pg.Limit
		f.Offset = pg.Offset

		// SQLite single-mutex: close rows before issuing count query.
		// Use function scope to ensure rows.Close() runs before JobCount.
		var jobs []queue.Job
		var jobsErr error
		func() {
			jobs, jobsErr = q.Jobs(f)
		}()
		if jobsErr != nil {
			http.WriteServerError(w, r, "failed to list jobs", jobsErr)
			return
		}

		total, err := q.JobCount(f)
		if err != nil {
			http.WriteServerError(w, r, "failed to count jobs", err)
			return
		}

		type jobJSON struct {
			ID             int64  `json:"id"`
			Queue          string `json:"queue"`
			Type           string `json:"type"`
			Payload        string `json:"payload"`
			Status         string `json:"status"`
			Attempts       int    `json:"attempts"`
			MaxAttempts    int    `json:"max_attempts"`
			TimeoutSeconds int    `json:"timeout_seconds"`
			LastError      string `json:"last_error"`
			RunAt          string `json:"run_at"`
			StartedAt      string `json:"started_at"`
			CompletedAt    string `json:"completed_at"`
			CreatedAt      string `json:"created_at"`
			UpdatedAt      string `json:"updated_at"`
		}

		result := make([]jobJSON, len(jobs))
		for i, j := range jobs {
			var runAt, startedAt, completedAt string
			if !j.RunAt.IsZero() {
				runAt = j.RunAt.Format(time.RFC3339)
			}
			if !j.StartedAt.IsZero() {
				startedAt = j.StartedAt.Format(time.RFC3339)
			}
			if !j.CompletedAt.IsZero() {
				completedAt = j.CompletedAt.Format(time.RFC3339)
			}
			result[i] = jobJSON{
				ID:             j.ID,
				Queue:          j.Queue,
				Type:           j.Type,
				Payload:        string(j.Payload),
				Status:         j.Status,
				Attempts:       j.Attempts,
				MaxAttempts:    j.MaxAttempts,
				TimeoutSeconds: int(j.Timeout.Seconds()),
				LastError:      j.LastError,
				RunAt:          runAt,
				StartedAt:      startedAt,
				CompletedAt:    completedAt,
				CreatedAt:      j.CreatedAt.Format(time.RFC3339),
				UpdatedAt:      j.UpdatedAt.Format(time.RFC3339),
			}
		}

		http.PaginatedResponse(w, "jobs", result, total)
	}
}

func bulkRetryHandler(q *queue.Queue, db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
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

		var affected int64
		for _, id := range req.IDs {
			if err := q.Retry(id); err == nil {
				affected++
				adminaudit.Log(db, r, "job.retry", "job", sqlite.FormatID(id), "bulk")
			}
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"affected": affected,
		})
	}
}

func bulkCancelHandler(q *queue.Queue, db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
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

		var affected int64
		for _, id := range req.IDs {
			if err := q.Cancel(id); err == nil {
				affected++
				adminaudit.Log(db, r, "job.cancel", "job", sqlite.FormatID(id), "bulk")
			}
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"affected": affected,
		})
	}
}

func retryHandler(q *queue.Queue, db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}
		if err := q.Retry(id); err != nil {
			http.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		adminaudit.Log(db, r, "job.retry", "job", sqlite.FormatID(id), "")
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}

func cancelHandler(q *queue.Queue, db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}
		if err := q.Cancel(id); err != nil {
			http.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		adminaudit.Log(db, r, "job.cancel", "job", sqlite.FormatID(id), "")
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}
