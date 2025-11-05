//go:build sqlite

package model

import (
	"fmt"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// InitDatabase for SQLite (pure Go)
func InitDatabase(cfg *Config) (*CRMDatabase, error) {
	svr := cfg.Servers[cfg.Mode]
	filename := filepath.Join("db", svr.DBName)
	fmt.Println("Use server sqlite and database", filename)

	db, err := gorm.Open(sqlite.Open(filename), gormLoggerFor(cfg, svr))
	if err != nil {
		return nil, err
	}
	return &CRMDatabase{db: db, Config: cfg}, nil
}
