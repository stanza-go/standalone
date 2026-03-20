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
	db.AddMigration(1742428802, "create_refresh_tokens", createRefreshTokensUp, createRefreshTokensDown)
	db.AddMigration(1742428803, "create_users", createUsersUp, createUsersDown)
	db.AddMigration(1742428804, "create_api_keys", createAPIKeysUp, createAPIKeysDown)
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

func createRefreshTokensUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE refresh_tokens (
		id          TEXT PRIMARY KEY,
		entity_type TEXT NOT NULL,
		entity_id   TEXT NOT NULL,
		token_hash  TEXT NOT NULL,
		expires_at  TEXT NOT NULL,
		created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_refresh_tokens_entity ON refresh_tokens(entity_type, entity_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash)`)
	return err
}

func createRefreshTokensDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS refresh_tokens`)
	return err
}

func createUsersUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE users (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		email      TEXT    NOT NULL UNIQUE,
		password   TEXT    NOT NULL,
		name       TEXT    NOT NULL DEFAULT '',
		is_active  INTEGER NOT NULL DEFAULT 1,
		created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		deleted_at TEXT
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL`)
	return err
}

func createUsersDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS users`)
	return err
}

func createAPIKeysUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE api_keys (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		name         TEXT    NOT NULL,
		key_prefix   TEXT    NOT NULL,
		key_hash     TEXT    NOT NULL,
		scopes       TEXT    NOT NULL DEFAULT '',
		created_by   INTEGER NOT NULL,
		last_used_at TEXT,
		expires_at   TEXT,
		created_at   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		revoked_at   TEXT
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_api_keys_hash ON api_keys(key_hash) WHERE revoked_at IS NULL`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_api_keys_created_by ON api_keys(created_by)`)
	return err
}

func createAPIKeysDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS api_keys`)
	return err
}
