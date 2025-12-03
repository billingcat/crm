//go:build sqlite

package model

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// InitDatabase for SQLite (pure Go)
func InitDatabase(cfg *Config) (*Store, error) {
	svr := cfg.Servers[cfg.Mode]
	filename := svr.DBName
	fmt.Println("Use server sqlite and database", filename)

	db, err := gorm.Open(sqlite.Open(filename), gormLoggerFor(cfg, svr))
	if err != nil {
		return nil, err
	}
	return &Store{db: db, Config: cfg}, nil
}
