//go:build postgres

package main

import (
	"fmt"

	"github.com/billingcat/crm/model"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
)

func migrationsDir() string { return "migrations/postgres" }

func migrateDSN(cfg *model.Config) string {
	svr := cfg.Servers[cfg.Mode]
	// postgres://user:pass@host:port/db?sslmode=disable&timezone=UTC
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable&timezone=UTC",
		svr.DBUser, svr.DBPassword, svr.DBHost, 5432, svr.DBName)
}
