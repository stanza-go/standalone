// Package health provides the health check endpoint. It reports application
// status, uptime, and database connectivity.
package health

import (
	"runtime"
	"time"

	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
)

var startTime = time.Now()

// Register mounts the health check routes on the given router group.
func Register(api *http.Group, db *sqlite.DB) {
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

		http.WriteJSON(w, status, map[string]any{
			"status":    statusText(dbOK),
			"uptime":    time.Since(startTime).Round(time.Second).String(),
			"go":        runtime.Version(),
			"goroutines": runtime.NumGoroutine(),
			"memory_mb": mem.Alloc / 1024 / 1024,
			"database": map[string]any{
				"ok":    dbOK,
				"error": dbErr,
			},
		})
	})
}

func statusText(ok bool) string {
	if ok {
		return "ok"
	}
	return "degraded"
}
