// Package dashboard provides the admin dashboard endpoint. It returns
// system health metrics, database stats, and application-level counters
// for display in the admin panel.
package dashboard

import (
	"os"
	"runtime"
	"time"

	"github.com/stanza-go/framework/pkg/cache"
	"github.com/stanza-go/framework/pkg/cron"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/queue"
	"github.com/stanza-go/framework/pkg/sqlite"
)

var startTime = time.Now()

// dbStats holds cached database and application counters.
type dbStats struct {
	DBSizeBytes       int64 `json:"db_size_bytes"`
	WALSizeBytes      int64 `json:"wal_size_bytes"`
	Tables            int   `json:"tables"`
	Migrations        int   `json:"migrations"`
	TotalAdmins       int   `json:"total_admins"`
	TotalUsers        int   `json:"total_users"`
	ActiveSessions    int   `json:"active_sessions"`
	ActiveAPIKeys     int   `json:"active_api_keys"`
}

// Register mounts the dashboard routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET /api/admin/dashboard — system, database, queue, cron, and app stats
func Register(admin *http.Group, db *sqlite.DB, q *queue.Queue, s *cron.Scheduler) {
	statsCache := cache.New[*dbStats](
		cache.WithTTL[*dbStats](30 * time.Second),
		cache.WithMaxSize[*dbStats](1),
	)
	admin.HandleFunc("GET /dashboard", statsHandler(db, q, s, statsCache))
}

func queryDBStats(db *sqlite.DB) (*dbStats, error) {
	st := &dbStats{}

	if info, err := os.Stat(db.Path()); err == nil {
		st.DBSizeBytes = info.Size()
	}
	if info, err := os.Stat(db.Path() + "-wal"); err == nil {
		st.WALSizeBytes = info.Size()
	}

	row := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	_ = row.Scan(&st.Tables)

	sql, args := sqlite.Count("admins").Where("deleted_at IS NULL").Build()
	_ = db.QueryRow(sql, args...).Scan(&st.TotalAdmins)

	sql, args = sqlite.Count("users").Where("deleted_at IS NULL").Build()
	_ = db.QueryRow(sql, args...).Scan(&st.TotalUsers)

	sql, args = sqlite.Count("refresh_tokens").Where("expires_at > ?", time.Now().UTC().Format(time.RFC3339)).Build()
	_ = db.QueryRow(sql, args...).Scan(&st.ActiveSessions)

	sql, args = sqlite.Count("api_keys").Where("revoked_at IS NULL").Build()
	_ = db.QueryRow(sql, args...).Scan(&st.ActiveAPIKeys)

	row = db.QueryRow(`SELECT count(*) FROM _migrations`)
	_ = row.Scan(&st.Migrations)

	return st, nil
}

func statsHandler(db *sqlite.DB, q *queue.Queue, s *cron.Scheduler, statsCache *cache.Cache[*dbStats]) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		// Database and app counters — cached for 30s to reduce DB queries.
		st, _ := statsCache.GetOrSet("stats", func() (*dbStats, error) {
			return queryDBStats(db)
		})
		if st == nil {
			st = &dbStats{}
		}

		// Queue stats — live (in-memory, cheap).
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

		// Cron stats — live (in-memory, cheap).
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
				"size_bytes":     st.DBSizeBytes,
				"wal_size_bytes": st.WALSizeBytes,
				"tables":         st.Tables,
				"migrations":     st.Migrations,
			},
			"queue": queueStats,
			"cron": map[string]any{
				"total":    len(entries),
				"enabled":  cronEnabled,
				"running":  cronRunning,
				"next_run": cronNextRun,
			},
			"stats": map[string]any{
				"total_admins":    st.TotalAdmins,
				"total_users":     st.TotalUsers,
				"active_sessions": st.ActiveSessions,
				"active_api_keys": st.ActiveAPIKeys,
			},
		})
	}
}
