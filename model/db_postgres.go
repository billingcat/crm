//go:build postgres

package model

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// InitDatabase for PostgreSQL
func InitDatabase(cfg *Config) (*Store, error) {
	svr := cfg.Servers[cfg.Mode]
	fmt.Println("Use server postgresql and database", svr.DBName)

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=5432 sslmode=disable TimeZone=UTC",
		svr.DBHost, svr.DBUser, svr.DBPassword, svr.DBName,
	)
	db, err := gorm.Open(postgres.Open(dsn), gormLoggerFor(cfg, svr))
	if err != nil {
		return nil, err
	}
	return &Store{db: db, Config: cfg}, nil
}
