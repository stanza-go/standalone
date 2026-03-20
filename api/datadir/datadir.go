// Package datadir manages the application data directory. All persistent state
// — database, logs, uploads, backups, config — lives under a single directory.
// The default is ~/.stanza/, overridden by the DATA_DIR environment variable.
package datadir

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultName = ".stanza"

// Dir holds resolved paths for the data directory and its subdirectories.
type Dir struct {
	Root    string
	DB      string
	Logs    string
	Uploads string
	Backups string
	Config  string
}

// Resolve determines the data directory root and returns a Dir with all
// paths resolved. It reads DATA_DIR from the environment; if unset, it
// defaults to ~/.stanza/. The directory tree is created if it does not exist.
func Resolve() (*Dir, error) {
	root := os.Getenv("DATA_DIR")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("datadir: home dir: %w", err)
		}
		root = filepath.Join(home, defaultName)
	}

	d := &Dir{
		Root:    root,
		DB:      filepath.Join(root, "database.sqlite"),
		Logs:    filepath.Join(root, "logs"),
		Uploads: filepath.Join(root, "uploads"),
		Backups: filepath.Join(root, "backups"),
		Config:  filepath.Join(root, "config.yaml"),
	}

	dirs := []string{root, d.Logs, d.Uploads, d.Backups}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("datadir: create %s: %w", dir, err)
		}
	}

	return d, nil
}
