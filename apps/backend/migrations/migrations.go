// Package migrations embeds PostgreSQL schema migration SQL files.
package migrations

import "embed"

// Files embeds all .sql migration files in this directory.
//
//go:embed *.sql
var Files embed.FS
