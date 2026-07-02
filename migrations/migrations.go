// Package migrations embeds the ordered SQL migration files for the full
// UIGraph stack (auth, rbac, graph, billing schemas).
// The migrate package reads this FS on startup and applies any pending files.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
