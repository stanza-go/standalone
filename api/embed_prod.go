//go:build prod

package main

import (
	"embed"
	"io/fs"
)

//go:embed ui/dist
var rawUIFS embed.FS

//go:embed admin/dist
var rawAdminFS embed.FS

func embeddedUI() fs.FS {
	sub, _ := fs.Sub(rawUIFS, "ui/dist")
	return sub
}

func embeddedAdmin() fs.FS {
	sub, _ := fs.Sub(rawAdminFS, "admin/dist")
	return sub
}
