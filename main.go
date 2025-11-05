package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/billingcat/crm/controller"
	"github.com/billingcat/crm/model"
	"github.com/pelletier/go-toml/v2"

	// migrate imports
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func loadConfig() (*model.Config, error) {
	data, err := os.ReadFile("config.toml")
	if err != nil {
		return nil, err
	}
	cfg := &model.Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// runMigrations applies all pending migrations.
// In development it runs automatically; in production only when explicitly requested.
func runMigrations(cfg *model.Config) {

	src := "file://" + filepath.ToSlash(migrationsDir())
	dsn := migrateDSN(cfg)

	m, err := migrate.New(src, dsn)
	if err != nil {
		log.Fatalf("migration setup failed: %v", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migration failed: %v", err)
	}
	log.Println("migrations applied")
}

func main() {
	var maintenance bool
	var migrateOnly bool
	flag.BoolVar(&maintenance, "maintenance", false, "run maintenance tasks and exit")
	flag.BoolVar(&migrateOnly, "migrate", false, "run database migrations and exit")
	flag.Parse()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Automatically migrate in dev; explicit in prod
	if cfg.Mode == "development" && !migrateOnly {
		runMigrations(cfg)
	} else if migrateOnly {
		runMigrations(cfg)
		return
	}

	crmdb, err := model.InitDatabase(cfg)
	if err != nil {
		log.Fatal(err)
	}

	if maintenance {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if err := model.RunMaintenance(ctx, crmdb); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := controller.NewController(crmdb); err != nil {
		log.Fatal(err)
	}
}
