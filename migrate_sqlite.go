//go:build sqlite

package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/billingcat/crm/model"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3" // CGO!
)

func migrationsDir() string { return "migrations/sqlite3" }

func migrateDSN(cfg *model.Config) string {
	svr := cfg.Servers[cfg.Mode]
	dbPath := svr.DBName
	if !strings.HasPrefix(dbPath, "/") {
		dbPath = "./" + dbPath
	}
	return fmt.Sprintf("sqlite3://%s?_foreign_keys=on&_journal_mode=WAL",
		filepath.ToSlash(dbPath))
}
