// Package seed creates initial data on first boot. It checks if data
// already exists before inserting to be safely re-runnable.
package seed

import (
	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/sqlite"
)

// Run creates seed data if the database is empty. Safe to call on
// every boot — admins are only created when none exist, settings use
// INSERT OR IGNORE so they are always safe to re-run.
func Run(db *sqlite.DB, logger *log.Logger) error {
	var count int
	cSQL, cArgs := sqlite.Count("admins").Build()
	row := db.QueryRow(cSQL, cArgs...)
	if err := row.Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		password, err := auth.HashPassword("admin")
		if err != nil {
			return err
		}

		iSQL, iArgs := sqlite.Insert("admins").
			Set("email", "admin@stanza.dev").
			Set("password", password).
			Set("name", "Admin").
			Set("role", "superadmin").
			Build()
		_, err = db.Exec(iSQL, iArgs...)
		if err != nil {
			return err
		}

		logger.Info("seed: created default admin",
			log.String("email", "admin@stanza.dev"),
			log.String("password", "admin"),
		)
	}

	if err := seedSettings(db, logger); err != nil {
		return err
	}

	return nil
}

func seedSettings(db *sqlite.DB, logger *log.Logger) error {
	defaults := []struct {
		key   string
		value string
		group string
	}{
		{"app.name", "Stanza", "general"},
		{"app.url", "http://localhost:23710", "general"},
		{"app.timezone", "UTC", "general"},
		{"auth.access_token_ttl", "300", "auth"},
		{"auth.refresh_token_ttl", "86400", "auth"},
		{"auth.max_sessions_per_user", "10", "auth"},
	}

	for _, s := range defaults {
		sql, args := sqlite.Insert("settings").
			OrIgnore().
			Set("key", s.key).
			Set("value", s.value).
			Set("group_name", s.group).
			Build()
		_, err := db.Exec(sql, args...)
		if err != nil {
			return err
		}
	}

	logger.Info("seed: default settings created")
	return nil
}
