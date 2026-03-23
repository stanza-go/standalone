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
	db.AddMigration(1742428805, "create_audit_log", createAuditLogUp, createAuditLogDown)
	db.AddMigration(1742428806, "create_cron_runs", createCronRunsUp, createCronRunsDown)
	db.AddMigration(1742428807, "add_api_key_request_count", addAPIKeyRequestCountUp, addAPIKeyRequestCountDown)
	db.AddMigration(1742428808, "create_uploads", createUploadsUp, createUploadsDown)
	db.AddMigration(1742428809, "create_password_reset_tokens", createPasswordResetTokensUp, createPasswordResetTokensDown)
	db.AddMigration(1742428810, "create_roles", createRolesUp, createRolesDown)
	db.AddMigration(1742428811, "create_notifications", createNotificationsUp, createNotificationsDown)
	db.AddMigration(1742428812, "add_api_key_entity", addAPIKeyEntityUp, addAPIKeyEntityDown)
	db.AddMigration(1742428813, "create_user_settings", createUserSettingsUp, createUserSettingsDown)
	db.AddMigration(1742428814, "create_webhooks", createWebhooksUp, createWebhooksDown)
	db.AddMigration(1742428816, "create_metric_dashboards", createMetricDashboardsUp, createMetricDashboardsDown)
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

func createAuditLogUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE audit_log (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		admin_id    TEXT    NOT NULL,
		action      TEXT    NOT NULL,
		entity_type TEXT    NOT NULL DEFAULT '',
		entity_id   TEXT    NOT NULL DEFAULT '',
		details     TEXT    NOT NULL DEFAULT '',
		ip_address  TEXT    NOT NULL DEFAULT '',
		created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_audit_log_admin_id ON audit_log(admin_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_audit_log_action ON audit_log(action)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_audit_log_created_at ON audit_log(created_at)`)
	return err
}

func createAuditLogDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS audit_log`)
	return err
}

func createCronRunsUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE cron_runs (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT    NOT NULL,
		started_at  TEXT    NOT NULL,
		duration_ms INTEGER NOT NULL,
		status      TEXT    NOT NULL DEFAULT 'success',
		error       TEXT    NOT NULL DEFAULT '',
		created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_cron_runs_name ON cron_runs(name)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_cron_runs_started_at ON cron_runs(started_at)`)
	return err
}

func createCronRunsDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS cron_runs`)
	return err
}

func addAPIKeyRequestCountUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`ALTER TABLE api_keys ADD COLUMN request_count INTEGER NOT NULL DEFAULT 0`)
	return err
}

func addAPIKeyRequestCountDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`ALTER TABLE api_keys DROP COLUMN request_count`)
	return err
}

func createUploadsUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE uploads (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		uuid          TEXT    NOT NULL UNIQUE,
		original_name TEXT    NOT NULL,
		stored_name   TEXT    NOT NULL,
		content_type  TEXT    NOT NULL DEFAULT '',
		size_bytes    INTEGER NOT NULL DEFAULT 0,
		storage_path  TEXT    NOT NULL,
		has_thumbnail INTEGER NOT NULL DEFAULT 0,
		uploaded_by   TEXT    NOT NULL DEFAULT '',
		entity_type   TEXT    NOT NULL DEFAULT '',
		entity_id     TEXT    NOT NULL DEFAULT '',
		created_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		deleted_at    TEXT
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_uploads_uuid ON uploads(uuid)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_uploads_entity ON uploads(entity_type, entity_id) WHERE deleted_at IS NULL`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_uploads_created_at ON uploads(created_at)`)
	return err
}

func createUploadsDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS uploads`)
	return err
}

func createPasswordResetTokensUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE password_reset_tokens (
		id         TEXT PRIMARY KEY,
		email      TEXT NOT NULL,
		token_hash TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		used_at    TEXT,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_password_reset_tokens_hash ON password_reset_tokens(token_hash) WHERE used_at IS NULL`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_password_reset_tokens_email ON password_reset_tokens(email)`)
	return err
}

func createPasswordResetTokensDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS password_reset_tokens`)
	return err
}

func createRolesUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE roles (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT    NOT NULL UNIQUE,
		description TEXT    NOT NULL DEFAULT '',
		is_system   INTEGER NOT NULL DEFAULT 0,
		created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE role_scopes (
		id      INTEGER PRIMARY KEY AUTOINCREMENT,
		role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
		scope   TEXT    NOT NULL,
		UNIQUE(role_id, scope)
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_role_scopes_role_id ON role_scopes(role_id)`)
	if err != nil {
		return err
	}

	// Seed built-in roles.
	_, err = tx.Exec(`INSERT INTO roles (name, description, is_system) VALUES
		('superadmin', 'Full access to all admin features', 1),
		('admin', 'Standard admin with user and settings access', 1),
		('viewer', 'Read-only admin access', 1)`)
	if err != nil {
		return err
	}

	// superadmin scopes (role_id=1).
	_, err = tx.Exec(`INSERT INTO role_scopes (role_id, scope) VALUES
		(1, 'admin'), (1, 'admin:users'), (1, 'admin:settings'),
		(1, 'admin:jobs'), (1, 'admin:logs'), (1, 'admin:audit'),
		(1, 'admin:uploads'), (1, 'admin:database'), (1, 'admin:roles'),
		(1, 'admin:notifications')`)
	if err != nil {
		return err
	}

	// admin scopes (role_id=2).
	_, err = tx.Exec(`INSERT INTO role_scopes (role_id, scope) VALUES
		(2, 'admin'), (2, 'admin:users'), (2, 'admin:settings')`)
	if err != nil {
		return err
	}

	// viewer scopes (role_id=3).
	_, err = tx.Exec(`INSERT INTO role_scopes (role_id, scope) VALUES
		(3, 'admin')`)
	return err
}

func createRolesDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS role_scopes`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`DROP TABLE IF EXISTS roles`)
	return err
}

func createNotificationsUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE notifications (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		entity_type TEXT    NOT NULL,
		entity_id   INTEGER NOT NULL,
		type        TEXT    NOT NULL,
		title       TEXT    NOT NULL,
		message     TEXT    NOT NULL DEFAULT '',
		data        TEXT    NOT NULL DEFAULT '',
		read_at     TEXT,
		created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_notifications_entity ON notifications(entity_type, entity_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_notifications_unread ON notifications(entity_type, entity_id, read_at) WHERE read_at IS NULL`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_notifications_created_at ON notifications(created_at)`)
	return err
}

func createNotificationsDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS notifications`)
	return err
}

func addAPIKeyEntityUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`ALTER TABLE api_keys ADD COLUMN entity_type TEXT NOT NULL DEFAULT 'admin'`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE api_keys ADD COLUMN entity_id TEXT NOT NULL DEFAULT ''`)
	if err != nil {
		return err
	}

	// Backfill: existing keys were created by admins.
	_, err = tx.Exec(`UPDATE api_keys SET entity_id = CAST(created_by AS TEXT) WHERE entity_id = ''`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_api_keys_entity ON api_keys(entity_type, entity_id)`)
	return err
}

func addAPIKeyEntityDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP INDEX IF EXISTS idx_api_keys_entity`)
	if err != nil {
		return err
	}
	// SQLite doesn't support DROP COLUMN before 3.35.0; recreate table if needed.
	// For simplicity, just drop the index — the columns become unused.
	return nil
}

func createUserSettingsUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE user_settings (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id    INTEGER NOT NULL,
		key        TEXT    NOT NULL,
		value      TEXT    NOT NULL DEFAULT '',
		created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		UNIQUE(user_id, key)
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_user_settings_user_id ON user_settings(user_id)`)
	return err
}

func createUserSettingsDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS user_settings`)
	return err
}

func createWebhooksUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE webhooks (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		url         TEXT    NOT NULL,
		secret      TEXT    NOT NULL,
		description TEXT    NOT NULL DEFAULT '',
		events      TEXT    NOT NULL DEFAULT '["*"]',
		is_active   INTEGER NOT NULL DEFAULT 1,
		created_by  INTEGER NOT NULL DEFAULT 0,
		created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		updated_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_webhooks_active ON webhooks(is_active)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE webhook_deliveries (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		webhook_id    INTEGER NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
		delivery_id   TEXT    NOT NULL DEFAULT '',
		event         TEXT    NOT NULL,
		payload       TEXT    NOT NULL DEFAULT '{}',
		status        TEXT    NOT NULL DEFAULT 'pending',
		status_code   INTEGER NOT NULL DEFAULT 0,
		response_body TEXT    NOT NULL DEFAULT '',
		attempts      INTEGER NOT NULL DEFAULT 0,
		created_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		completed_at  TEXT
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status)`)
	if err != nil {
		return err
	}

	// Add admin:webhooks scope to superadmin role.
	_, err = tx.Exec(`INSERT INTO role_scopes (role_id, scope) VALUES (1, 'admin:webhooks')`)
	return err
}

func createWebhooksDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS webhook_deliveries`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`DROP TABLE IF EXISTS webhooks`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`DELETE FROM role_scopes WHERE scope = 'admin:webhooks'`)
	return err
}

func createMetricDashboardsUp(tx *sqlite.Tx) error {
	_, err := tx.Exec(`CREATE TABLE metric_dashboards (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT    NOT NULL,
		panels     TEXT    NOT NULL DEFAULT '[]',
		created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`)
	return err
}

func createMetricDashboardsDown(tx *sqlite.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS metric_dashboards`)
	return err
}
