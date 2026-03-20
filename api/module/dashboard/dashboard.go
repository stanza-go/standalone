// Package dashboard provides the admin dashboard endpoint. It returns
// system health metrics, database stats, and application-level counters
// for display in the admin panel.
package dashboard

import (
	"os"
	"runtime"
	"time"

	"github.com/stanza-go/framework/pkg/cron"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/queue"
	"github.com/stanza-go/framework/pkg/sqlite"
)

var startTime = time.Now()

// Register mounts the dashboard routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET /api/admin/dashboard — system, database, queue, cron, and app stats
func Register(admin *http.Group, db *sqlite.DB, q *queue.Queue, s *cron.Scheduler) {
	admin.HandleFunc("GET /dashboard", statsHandler(db, q, s))
}

func statsHandler(db *sqlite.DB, q *queue.Queue, s *cron.Scheduler) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		// Database file size.
		var dbSizeBytes int64
		if info, err := os.Stat(db.Path()); err == nil {
			dbSizeBytes = info.Size()
		}

		// WAL file size.
		var walSizeBytes int64
		if info, err := os.Stat(db.Path() + "-wal"); err == nil {
			walSizeBytes = info.Size()
		}

		// Table count.
		var tableCount int
		row := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
		row.Scan(&tableCount)

		// Admin count.
		var totalAdmins int
		row = db.QueryRow(`SELECT count(*) FROM admins WHERE deleted_at IS NULL`)
		row.Scan(&totalAdmins)

		// Active session count (non-expired refresh tokens).
		var activeSessions int
		row = db.QueryRow(`SELECT count(*) FROM refresh_tokens WHERE expires_at > ?`, time.Now().UTC().Format(time.RFC3339))
		row.Scan(&activeSessions)

		// Migration count.
		var appliedMigrations int
		row = db.QueryRow(`SELECT count(*) FROM _migrations`)
		row.Scan(&appliedMigrations)

		// Queue stats.
		queueStats := map[string]any{
			"pending":   0,
			"running":   0,
			"completed": 0,
			"failed":    0,
			"dead":      0,
			"cancelled": 0,
		}
		if qs, err := q.Stats(); err == nil {
			queueStats["pending"] = qs.Pending
			queueStats["running"] = qs.Running
			queueStats["completed"] = qs.Completed
			queueStats["failed"] = qs.Failed
			queueStats["dead"] = qs.Dead
			queueStats["cancelled"] = qs.Cancelled
		}

		// Cron stats.
		entries := s.Entries()
		var cronEnabled, cronRunning int
		var cronNextRun string
		var earliest time.Time
		for _, e := range entries {
			if e.Enabled {
				cronEnabled++
			}
			if e.Running {
				cronRunning++
			}
			if e.Enabled && !e.NextRun.IsZero() && (earliest.IsZero() || e.NextRun.Before(earliest)) {
				earliest = e.NextRun
			}
		}
		if !earliest.IsZero() {
			cronNextRun = earliest.UTC().Format(time.RFC3339)
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"system": map[string]any{
				"uptime_seconds":  int(time.Since(startTime).Seconds()),
				"uptime":          time.Since(startTime).Round(time.Second).String(),
				"go_version":      runtime.Version(),
				"goroutines":      runtime.NumGoroutine(),
				"memory_alloc_mb": float64(mem.Alloc) / 1024 / 1024,
				"memory_sys_mb":   float64(mem.Sys) / 1024 / 1024,
			},
			"database": map[string]any{
				"size_bytes":     dbSizeBytes,
				"wal_size_bytes": walSizeBytes,
				"tables":         tableCount,
				"migrations":     appliedMigrations,
			},
			"queue": queueStats,
			"cron": map[string]any{
				"total":    len(entries),
				"enabled":  cronEnabled,
				"running":  cronRunning,
				"next_run": cronNextRun,
			},
			"stats": map[string]any{
				"total_admins":    totalAdmins,
				"active_sessions": activeSessions,
			},
		})
	}
}
