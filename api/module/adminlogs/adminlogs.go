// Package adminlogs provides admin endpoints for viewing structured log files.
// It reads JSON-line log files from the data directory and supports filtering
// by level, keyword search, and file selection. Includes an SSE endpoint for
// real-time log streaming (works over HTTP/2, unlike WebSocket).
package adminlogs

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/http"
)

// Register mounts the log viewer routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET /api/admin/logs        — list log entries with filtering
//	GET /api/admin/logs/files  — list available log files
//	GET /api/admin/logs/sse    — SSE: stream new log entries in real-time
func Register(admin *http.Group, logsDir string) {
	admin.HandleFunc("GET /logs", entriesHandler(logsDir))
	admin.HandleFunc("GET /logs/files", filesHandler(logsDir))
	admin.HandleFunc("GET /logs/sse", sseHandler(logsDir))
}

func entriesHandler(logsDir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		if file == "" {
			file = "stanza.log"
		}

		if !isValidLogFile(file) {
			http.WriteError(w, http.StatusBadRequest, "invalid log file name")
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
			http.WriteServerError(w, r, "failed to open log file", err)
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

// sseHandler streams new log entries via Server-Sent Events.
// Query params: ?level=info&search=keyword (optional server-side filters).
// Unlike the WebSocket endpoint, filters cannot be updated mid-stream —
// the client must reconnect with new query parameters.
func sseHandler(logsDir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		level := strings.ToLower(r.URL.Query().Get("level"))
		search := strings.ToLower(r.URL.Query().Get("search"))

		path := filepath.Join(logsDir, "stanza.log")
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				http.WriteError(w, http.StatusNotFound, "log file not available")
				return
			}
			http.WriteServerError(w, r, "failed to open log file", err)
			return
		}
		defer f.Close()

		_, _ = f.Seek(0, io.SeekEnd)
		reader := bufio.NewReader(f)

		sse := http.NewSSEWriter(w)
		_ = sse.Retry(5000)

		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()

		heartbeat := time.NewTicker(30 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-heartbeat.C:
				if err := sse.Comment("keepalive"); err != nil {
					return
				}
			case <-ticker.C:
				if err := sendSSELines(sse, reader, level, search); err != nil {
					return
				}
			}
		}
	}
}

// sendSSELines reads new lines from the reader and sends matching
// entries as SSE events.
func sendSSELines(sse *http.SSEWriter, reader *bufio.Reader, level, search string) error {
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			if err != nil {
				return nil
			}
			continue
		}

		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) != nil {
			if err != nil {
				return nil
			}
			continue
		}

		if level != "" {
			if entryLevel, ok := entry["level"].(string); !ok || entryLevel != level {
				if err != nil {
					return nil
				}
				continue
			}
		}

		if search != "" && !matchesSearch(entry, search) {
			if err != nil {
				return nil
			}
			continue
		}

		if writeErr := sse.Event("log", line); writeErr != nil {
			return writeErr
		}

		if err != nil {
			return nil
		}
	}
}

func filesHandler(logsDir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		dirEntries, err := os.ReadDir(logsDir)
		if err != nil {
			http.WriteServerError(w, r, "failed to read logs directory", err)
			return
		}

		type fileInfo struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		}

		files := make([]fileInfo, 0)
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
