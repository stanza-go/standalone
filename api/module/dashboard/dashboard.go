// Package dashboard provides the admin dashboard endpoint. It returns
// system health metrics, database stats, and application-level counters
// for display in the admin panel.
package dashboard

import (
	"os"
	"runtime"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/cache"
	"github.com/stanza-go/framework/pkg/cron"
	"github.com/stanza-go/framework/pkg/email"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/queue"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/webhooks"
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
// Register mounts the dashboard routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET /api/admin/dashboard        — system, database, queue, cron, and app stats
//	GET /api/admin/dashboard/charts — time-series data for dashboard charts
func Register(admin *http.Group, db *sqlite.DB, q *queue.Queue, s *cron.Scheduler, m *http.Metrics, wh *webhooks.Dispatcher, a *auth.Auth, ec *email.Client) {
	statsCache := cache.New[*dbStats](
		cache.WithTTL[*dbStats](30 * time.Second),
		cache.WithMaxSize[*dbStats](1),
	)
	chartsCache := cache.New[*chartsData](
		cache.WithTTL[*chartsData](5 * time.Minute),
		cache.WithMaxSize[*chartsData](3),
	)
	admin.HandleFunc("GET /dashboard", statsHandler(db, q, s, m, wh, a, ec, statsCache, chartsCache))
	admin.HandleFunc("GET /dashboard/charts", chartsHandler(db, chartsCache))
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

	sql, args := sqlite.Count("admins").WhereNull("deleted_at").Build()
	_ = db.QueryRow(sql, args...).Scan(&st.TotalAdmins)

	sql, args = sqlite.Count("users").WhereNull("deleted_at").Build()
	_ = db.QueryRow(sql, args...).Scan(&st.TotalUsers)

	sql, args = sqlite.Count("refresh_tokens").Where("expires_at > ?", sqlite.Now()).Build()
	_ = db.QueryRow(sql, args...).Scan(&st.ActiveSessions)

	sql, args = sqlite.Count("api_keys").Where("revoked_at IS NULL").Build()
	_ = db.QueryRow(sql, args...).Scan(&st.ActiveAPIKeys)

	row = db.QueryRow(`SELECT count(*) FROM _migrations`)
	_ = row.Scan(&st.Migrations)

	return st, nil
}

func statsHandler(db *sqlite.DB, q *queue.Queue, s *cron.Scheduler, m *http.Metrics, wh *webhooks.Dispatcher, a *auth.Auth, ec *email.Client, statsCache *cache.Cache[*dbStats], chartsCache *cache.Cache[*chartsData]) func(http.ResponseWriter, *http.Request) {
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
		cronStats := s.Stats()
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

		// Cache stats — aggregate across all caches.
		sc := statsCache.Stats()
		cc := chartsCache.Stats()

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
				"total":     len(entries),
				"enabled":   cronEnabled,
				"running":   cronRunning,
				"next_run":  cronNextRun,
				"completed": cronStats.Completed,
				"failed":    cronStats.Failed,
				"skipped":   cronStats.Skipped,
			},
			"cache": map[string]any{
				"entries":   sc.Size + cc.Size,
				"hits":      sc.Hits + cc.Hits,
				"misses":    sc.Misses + cc.Misses,
				"evictions": sc.Evictions + cc.Evictions,
			},
			"http":    m.Stats(),
			"webhook": wh.Stats(),
			"auth":    a.Stats(),
			"email":   ec.Stats(),
			"stats": map[string]any{
				"total_admins":    st.TotalAdmins,
				"total_users":     st.TotalUsers,
				"active_sessions": st.ActiveSessions,
				"active_api_keys": st.ActiveAPIKeys,
			},
		})
	}
}

// chartsData holds time-series data for dashboard charts.
type chartsData struct {
	Users    []dayCount    `json:"users"`
	Activity []dayCount    `json:"activity"`
	Jobs     []dayJobCount `json:"jobs"`
}

type dayCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type dayJobCount struct {
	Date      string `json:"date"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
}

func chartsHandler(db *sqlite.DB, chartsCache *cache.Cache[*chartsData]) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		period := r.URL.Query().Get("period")
		days := 7
		switch period {
		case "30d":
			days = 30
		case "90d":
			days = 90
		default:
			period = "7d"
		}

		data, _ := chartsCache.GetOrSet(period, func() (*chartsData, error) {
			return queryCharts(db, days)
		})
		if data == nil {
			data = &chartsData{}
		}

		http.WriteJSON(w, http.StatusOK, data)
	}
}

func queryCharts(db *sqlite.DB, days int) (*chartsData, error) {
	since := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	result := &chartsData{}

	// Generate all dates in range for gap-filling.
	dateMap := make(map[string]bool, days+1)
	var dates []string
	for i := days; i >= 0; i-- {
		d := time.Now().UTC().AddDate(0, 0, -i).Format("2006-01-02")
		dates = append(dates, d)
		dateMap[d] = true
	}

	// Users created per day.
	userCounts := make(map[string]int, days+1)
	userSQL, userArgs := sqlite.Select("date(created_at) as day", "COUNT(*) as cnt").
		From("users").
		Where("created_at >= ?", since).
		WhereNull("deleted_at").
		GroupBy("day").
		OrderBy("day", "ASC").
		Build()
	userRows, _ := sqlite.QueryAll(db, userSQL, userArgs, func(rows *sqlite.Rows) (dayCount, error) {
		var dc dayCount
		err := rows.Scan(&dc.Date, &dc.Count)
		return dc, err
	})
	for _, dc := range userRows {
		userCounts[dc.Date] = dc.Count
	}
	for _, d := range dates {
		result.Users = append(result.Users, dayCount{Date: d, Count: userCounts[d]})
	}

	// Audit log activity per day.
	activityCounts := make(map[string]int, days+1)
	activitySQL, activityArgs := sqlite.Select("date(created_at) as day", "COUNT(*) as cnt").
		From("audit_log").
		Where("created_at >= ?", since).
		GroupBy("day").
		OrderBy("day", "ASC").
		Build()
	activityRows, _ := sqlite.QueryAll(db, activitySQL, activityArgs, func(rows *sqlite.Rows) (dayCount, error) {
		var dc dayCount
		err := rows.Scan(&dc.Date, &dc.Count)
		return dc, err
	})
	for _, dc := range activityRows {
		activityCounts[dc.Date] = dc.Count
	}
	for _, d := range dates {
		result.Activity = append(result.Activity, dayCount{Date: d, Count: activityCounts[d]})
	}

	// Queue jobs per day (completed vs failed).
	jobCounts := make(map[string]dayJobCount, days+1)
	jobSQL, jobArgs := sqlite.Select(
		"date(created_at) as day",
		"SUM(CASE WHEN status IN ('completed') THEN 1 ELSE 0 END) as completed",
		"SUM(CASE WHEN status IN ('failed','dead') THEN 1 ELSE 0 END) as failed",
	).
		From("_queue_jobs").
		Where("created_at >= ?", since).
		GroupBy("day").
		OrderBy("day", "ASC").
		Build()
	jobRows, _ := sqlite.QueryAll(db, jobSQL, jobArgs, func(rows *sqlite.Rows) (dayJobCount, error) {
		var djc dayJobCount
		err := rows.Scan(&djc.Date, &djc.Completed, &djc.Failed)
		return djc, err
	})
	for _, djc := range jobRows {
		jobCounts[djc.Date] = djc
	}
	for _, d := range dates {
		jd := jobCounts[d]
		result.Jobs = append(result.Jobs, dayJobCount{Date: d, Completed: jd.Completed, Failed: jd.Failed})
	}

	return result, nil
}
