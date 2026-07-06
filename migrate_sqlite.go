//go:build sqlite

package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/billingcat/crm/model"
	// Local CGO-free migrate driver that reuses glebarez's registered "sqlite"
	// database/sql driver (see internal/sqlitemigrate). Registers scheme "sqlite".
	_ "github.com/billingcat/crm/internal/sqlitemigrate"
)

func migrationsDir() string { return "migrations/sqlite3" }

func migrateDSN(cfg *model.Config) string {
	svr := cfg.Servers[cfg.Mode]
	dbPath := svr.DBName
	if !strings.HasPrefix(dbPath, "/") {
		dbPath = "./" + dbPath
	}
	// glebarez/go-sqlite (modernc-based) uses the _pragma=name(value) query syntax.
	return fmt.Sprintf("sqlite://%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)",
		filepath.ToSlash(dbPath))
}
