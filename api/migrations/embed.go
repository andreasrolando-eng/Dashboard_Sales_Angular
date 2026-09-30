// Package migrations embeds the goose SQL migration files so the server
// binary carries its own schema history — no separate migrations folder
// needs to ship alongside the binary in production.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
