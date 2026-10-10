// Package migrations embeds immutable, ordered database migrations.
package migrations

import "embed"

// Files is bundled with the admin command so it works independently of cwd.
//
//go:embed *.sql
var Files embed.FS
