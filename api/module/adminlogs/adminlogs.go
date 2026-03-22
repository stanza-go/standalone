// Package adminlogs provides admin endpoints for viewing structured log files.
// It reads JSON-line log files from the data directory and supports filtering
// by level, keyword search, and file selection. Includes a WebSocket endpoint
// for real-time log streaming.
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
//	GET /api/admin/logs/stream — WebSocket: stream new log entries in real-time
func Register(admin *http.Group, logsDir string) {
	admin.HandleFunc("GET /logs", entriesHandler(logsDir))
	admin.HandleFunc("GET /logs/files", filesHandler(logsDir))
	admin.HandleFunc("GET /logs/stream", streamHandler(logsDir))
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

// streamHandler upgrades to WebSocket and tails stanza.log in real-time.
// Query params: ?level=info&search=keyword (optional server-side filters).
// The server sends each new JSON log line as a WebSocket text message.
// The client can send a JSON message to update filters mid-stream:
//
//	{"level":"error","search":"timeout"}
func streamHandler(logsDir string) func(http.ResponseWriter, *http.Request) {
	upgrader := http.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r)
		if err != nil {
			return
		}
		defer conn.Close()

		level := strings.ToLower(r.URL.Query().Get("level"))
		search := strings.ToLower(r.URL.Query().Get("search"))

		// Read client messages in a separate goroutine for filter updates
		// and to detect disconnection.
		done := make(chan struct{})
		filterCh := make(chan streamFilter, 1)
		go func() {
			defer close(done)
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				var f streamFilter
				if json.Unmarshal(msg, &f) == nil {
					select {
					case filterCh <- f:
					default:
					}
				}
			}
		}()

		path := filepath.Join(logsDir, "stanza.log")

		// Open the file and seek to the end.
		f, err := os.Open(path)
		if err != nil {
			_ = conn.CloseWithMessage(http.CloseGoingAway, "log file not available")
			return
		}
		defer f.Close()

		_, _ = f.Seek(0, io.SeekEnd)
		reader := bufio.NewReader(f)

		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()

		pingTicker := time.NewTicker(30 * time.Second)
		defer pingTicker.Stop()

		for {
			select {
			case <-done:
				return
			case f, ok := <-filterCh:
				if !ok {
					return
				}
				level = strings.ToLower(f.Level)
				search = strings.ToLower(f.Search)
			case <-pingTicker.C:
				if err := conn.WritePing(nil); err != nil {
					return
				}
			case <-ticker.C:
				if err := sendNewLines(conn, reader, level, search); err != nil {
					return
				}
			}
		}
	}
}

type streamFilter struct {
	Level  string `json:"level"`
	Search string `json:"search"`
}

// sendNewLines reads any new lines from the reader and sends matching
// entries to the WebSocket connection.
func sendNewLines(conn *http.Conn, reader *bufio.Reader, level, search string) error {
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			if err != nil {
				return nil
			}
			continue
		}

		// Validate JSON and apply filters.
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

		if writeErr := conn.WriteMessage(http.TextMessage, []byte(line)); writeErr != nil {
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
			http.WriteJSON(w, http.StatusInternalServerError, map[string]any{
				"error": "failed to read logs directory",
			})
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
