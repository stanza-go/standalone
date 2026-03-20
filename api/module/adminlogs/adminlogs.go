// Package adminlogs provides admin endpoints for viewing structured log files.
// It reads JSON-line log files from the data directory and supports filtering
// by level, keyword search, and file selection.
package adminlogs

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/stanza-go/framework/pkg/http"
)

// Register mounts the log viewer routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET /api/admin/logs       — list log entries with filtering
//	GET /api/admin/logs/files — list available log files
func Register(admin *http.Group, logsDir string) {
	admin.HandleFunc("GET /logs", entriesHandler(logsDir))
	admin.HandleFunc("GET /logs/files", filesHandler(logsDir))
}

func entriesHandler(logsDir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		if file == "" {
			file = "stanza.log"
		}

		if !isValidLogFile(file) {
			http.WriteJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid log file name",
			})
			return
		}

		level := strings.ToLower(r.URL.Query().Get("level"))
		search := strings.ToLower(r.URL.Query().Get("search"))

		limit := 200
		if s := r.URL.Query().Get("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}

		path := filepath.Join(logsDir, file)
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				http.WriteJSON(w, http.StatusOK, map[string]any{
					"entries": []any{},
					"file":    file,
					"total":   0,
				})
				return
			}
			http.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error": "failed to open log file",
			})
			return
		}
		defer f.Close()

		var lines []string
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}

		// Iterate backwards (newest first), filter, collect up to limit.
		entries := make([]map[string]any, 0, limit)
		for i := len(lines) - 1; i >= 0 && len(entries) < limit; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}

			var entry map[string]any
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}

			if level != "" {
				if entryLevel, ok := entry["level"].(string); !ok || entryLevel != level {
					continue
				}
			}

			if search != "" && !matchesSearch(entry, search) {
				continue
			}

			entries = append(entries, entry)
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"entries": entries,
			"file":    file,
			"total":   len(lines),
		})
	}
}

func filesHandler(logsDir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		dirEntries, err := os.ReadDir(logsDir)
		if err != nil {
			http.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error": "failed to read logs directory",
			})
			return
		}

		type fileInfo struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		}

		var files []fileInfo
		for _, e := range dirEntries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasPrefix(name, "stanza") || !strings.HasSuffix(name, ".log") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			files = append(files, fileInfo{
				Name: name,
				Size: info.Size(),
			})
		}

		// Current log first, then rotated files newest first.
		sort.Slice(files, func(i, j int) bool {
			if files[i].Name == "stanza.log" {
				return true
			}
			if files[j].Name == "stanza.log" {
				return false
			}
			return files[i].Name > files[j].Name
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"files": files,
		})
	}
}

// isValidLogFile prevents path traversal by ensuring the name has no
// directory separators and matches the stanza log naming convention.
func isValidLogFile(name string) bool {
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return false
	}
	return strings.HasPrefix(name, "stanza") && strings.HasSuffix(name, ".log")
}

// matchesSearch checks if any string value in the entry contains the
// search term (case-insensitive).
func matchesSearch(entry map[string]any, search string) bool {
	for _, v := range entry {
		if s, ok := v.(string); ok {
			if strings.Contains(strings.ToLower(s), search) {
				return true
			}
		}
	}
	return false
}
