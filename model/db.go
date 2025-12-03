//go:build !postgres && !sqlite

package model

import "fmt"

// InitDatabase for SQLite (pure Go)
func InitDatabase(_ *Config) (*Store, error) {
	return nil, fmt.Errorf("no build tags specified, use either sqlite or postgres")
}
