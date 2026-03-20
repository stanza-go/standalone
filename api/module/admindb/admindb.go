// Package admindb provides the database administration endpoints. It exposes
// SQLite statistics, migration history, table inventory, and manual backup.
package admindb

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
)

// Register mounts the database admin routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET  /api/admin/database        — stats, tables, migrations
//	POST /api/admin/database/backup — trigger manual backup
func Register(admin *http.Group, db *sqlite.DB, backupsDir string) {
	admin.HandleFunc("GET /database", infoHandler(db, backupsDir))
	admin.HandleFunc("POST /database/backup", backupHandler(db, backupsDir))
}

func infoHandler(db *sqlite.DB, backupsDir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// File sizes.
		var dbSizeBytes int64
		if info, err := os.Stat(db.Path()); err == nil {
			dbSizeBytes = info.Size()
		}

		var walSizeBytes int64
		if info, err := os.Stat(db.Path() + "-wal"); err == nil {
			walSizeBytes = info.Size()
		}

		var shmSizeBytes int64
		if info, err := os.Stat(db.Path() + "-shm"); err == nil {
			shmSizeBytes = info.Size()
		}

		// PRAGMA stats.
		var pageCount int
		db.QueryRow("PRAGMA page_count").Scan(&pageCount)

		var pageSize int
		db.QueryRow("PRAGMA page_size").Scan(&pageSize)

		var freelistCount int
		db.QueryRow("PRAGMA freelist_count").Scan(&freelistCount)

		var journalMode string
		db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)

		// Collect table names first, then close rows before querying counts.
		var tableNames []string
		rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
		if err == nil {
			for rows.Next() {
				var name string
				if err := rows.Scan(&name); err == nil {
					tableNames = append(tableNames, name)
				}
			}
			rows.Close()
		}

		type tableInfo struct {
			Name     string `json:"name"`
			RowCount int    `json:"row_count"`
		}
		tables := make([]tableInfo, 0, len(tableNames))
		for _, name := range tableNames {
			var count int
			db.QueryRow(fmt.Sprintf("SELECT count(*) FROM [%s]", name)).Scan(&count)
			tables = append(tables, tableInfo{Name: name, RowCount: count})
		}

		// Migrations.
		type migrationInfo struct {
			Version   int64  `json:"version"`
			Name      string `json:"name"`
			AppliedAt string `json:"applied_at"`
		}
		var migrations []migrationInfo
		mrows, err := db.Query("SELECT version, name, applied_at FROM _migrations ORDER BY version ASC")
		if err == nil {
			for mrows.Next() {
				var m migrationInfo
				if err := mrows.Scan(&m.Version, &m.Name, &m.AppliedAt); err == nil {
					migrations = append(migrations, m)
				}
			}
			mrows.Close()
		}

		// Backups listing from the backups directory.
		type backupInfo struct {
			Name      string `json:"name"`
			SizeBytes int64  `json:"size_bytes"`
			CreatedAt string `json:"created_at"`
		}
		var backups []backupInfo
		entries, err := os.ReadDir(backupsDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				backups = append(backups, backupInfo{
					Name:      e.Name(),
					SizeBytes: info.Size(),
					CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
				})
			}
		}

		// Sort backups newest first.
		sort.Slice(backups, func(i, j int) bool {
			return backups[i].CreatedAt > backups[j].CreatedAt
		})

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"files": map[string]any{
				"db_size_bytes":  dbSizeBytes,
				"wal_size_bytes": walSizeBytes,
				"shm_size_bytes": shmSizeBytes,
				"path":           db.Path(),
			},
			"pragmas": map[string]any{
				"page_count":     pageCount,
				"page_size":      pageSize,
				"freelist_count": freelistCount,
				"journal_mode":   journalMode,
			},
			"tables":     tables,
			"migrations": migrations,
			"backups":    backups,
		})
	}
}

func backupHandler(db *sqlite.DB, backupsDir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ts := time.Now().UTC().Format("20060102T150405Z")
		backupName := fmt.Sprintf("database.sqlite.%s.bak", ts)
		backupPath := filepath.Join(backupsDir, backupName)

		src, err := os.Open(db.Path())
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to open database file")
			return
		}
		defer src.Close()

		dst, err := os.Create(backupPath)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to create backup file")
			return
		}
		defer dst.Close()

		written, err := io.Copy(dst, src)
		if err != nil {
			os.Remove(backupPath)
			http.WriteError(w, http.StatusInternalServerError, "failed to write backup")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"name":       backupName,
			"path":       backupPath,
			"size_bytes": written,
			"created_at": time.Now().UTC().Format(time.RFC3339),
		})
	}
}
