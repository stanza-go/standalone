// Package migration registers all database migrations for the standalone app.
// Migrations are Go functions, not SQL files. Each migration runs in its own
// transaction with automatic rollback on failure.
package migration

import (
	"github.com/stanza-go/framework/pkg/sqlite"
)

// Register adds all migrations to the database. Call this before db.Migrate().
func Register(db *sqlite.DB) {
	db.AddMigration(1742428800, "create_settings", createSettingsUp, createSettingsDown)
	db.AddMigration(1742428801, "create_admins", createAdminsUp, createAdminsDown)
}

func createSettingsUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		group_name TEXT NOT NULL DEFAULT 'general',
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_settings_group ON settings(group_name)`)
	return err
}

func createSettingsDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS settings`)
	return err
}

func createAdminsUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE admins (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		email      TEXT    NOT NULL UNIQUE,
		password   TEXT    NOT NULL,
		name       TEXT    NOT NULL DEFAULT '',
		role       TEXT    NOT NULL DEFAULT 'admin',
		is_active  INTEGER NOT NULL DEFAULT 1,
		created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		deleted_at TEXT
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_admins_email ON admins(email) WHERE deleted_at IS NULL`)
	return err
}

func createAdminsDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS admins`)
	return err
}
