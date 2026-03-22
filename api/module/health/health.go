// Package health provides the health check endpoint. It reports application
// status, uptime, build metadata, and database connectivity.
package health

import (
	"runtime"
	"time"

	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
)

var startTime = time.Now()

// BuildInfo holds compile-time metadata injected via -ldflags. Fields
// are empty strings in development mode (go run .).
type BuildInfo struct {
	Version   string // semantic version tag, e.g. "v0.1.0"
	Commit    string // short git commit hash, e.g. "a20cce5"
	BuildTime string // UTC build timestamp, e.g. "2026-03-22T10:30:00Z"
}

// Register mounts the health check routes on the given router group.
func Register(api *http.Group, db *sqlite.DB, bi BuildInfo) {
	ver := bi.Version
	if ver == "" {
		ver = "dev"
	}
	commitVal := bi.Commit
	if commitVal == "" {
		commitVal = "unknown"
	}

	api.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		dbOK := true
		var dbErr string
		row := db.QueryRow("SELECT 1")
		var one int
		if err := row.Scan(&one); err != nil {
			dbOK = false
			dbErr = err.Error()
		}

		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		status := http.StatusOK
		if !dbOK {
			status = http.StatusServiceUnavailable
		}

		resp := map[string]any{
			"status":     statusText(dbOK),
			"version":    ver,
			"commit":     commitVal,
			"uptime":     time.Since(startTime).Round(time.Second).String(),
			"go":         runtime.Version(),
			"goroutines": runtime.NumGoroutine(),
			"memory_mb":  mem.Alloc / 1024 / 1024,
			"database": map[string]any{
				"ok":    dbOK,
				"error": dbErr,
			},
		}
		if bi.BuildTime != "" {
			resp["build_time"] = bi.BuildTime
		}

		http.WriteJSON(w, status, resp)
	})
}

func statusText(ok bool) string {
	if ok {
		return "ok"
	}
	return "degraded"
}
