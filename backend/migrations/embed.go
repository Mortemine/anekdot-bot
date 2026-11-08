// Package migrations embeds the SQL migration files.
package migrations

import "embed"

// FS holds every SQL migration shipped with the service.
//
//go:embed *.sql
var FS embed.FS
