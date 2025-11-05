//go:build !postgres && !sqlite

package main

import "github.com/billingcat/crm/model"

func migrationsDir() string             { panic("build with -tags postgres or -tags sqlite") }
func migrateDSN(_ *model.Config) string { panic("build with -tags postgres or -tags sqlite") }
