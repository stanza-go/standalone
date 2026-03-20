// Package seed creates initial data on first boot. It checks if data
// already exists before inserting to be safely re-runnable.
package seed

import (
	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/sqlite"
)

// Run creates seed data if the database is empty. Safe to call on
// every boot — it only inserts when no admins exist.
func Run(db *sqlite.DB, logger *log.Logger) error {
	var count int
	row := db.QueryRow(`SELECT COUNT(*) FROM admins`)
	if err := row.Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	password, err := auth.HashPassword("admin")
	if err != nil {
		return err
	}

	_, err = db.Exec(
		`INSERT INTO admins (email, password, name, role) VALUES (?, ?, ?, ?)`,
		"admin@stanza.dev", password, "Admin", "superadmin",
	)
	if err != nil {
		return err
	}

	logger.Info("seed: created default admin",
		log.String("email", "admin@stanza.dev"),
		log.String("password", "admin"),
	)

	return nil
}
