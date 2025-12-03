package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
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
	fmt.Println("Read config file: config.toml")
	data, err := os.ReadFile("config.toml")
	if err != nil {
		return nil, err
	}
	cfg := &model.Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	// if base directory is not a directory, return an error
	fi, err := os.Stat(cfg.Basedir)

	// fallback to current working directory
	if err != nil {
		cfg.Basedir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	} else if !fi.IsDir() {
		cfg.Basedir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	fmt.Println("Use base dir:", cfg.Basedir)
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

	for {
		v, dirty, verr := m.Version()
		if verr == migrate.ErrNilVersion {
			v = 0
			dirty = false
		} else if verr != nil {
			log.Fatalf("read migration version failed: %v", verr)
		}
		log.Printf("▶ applying next migration (current version=%d, dirty=%v)", v, dirty)

		err := m.Steps(1)
		if err == migrate.ErrNoChange {
			log.Println("migrations applied")
			return
		}

		// Some versions of golang-migrate report "no more migrations"
		// as something like "file does not exist" / ErrUnknownVersion instead.
		// You can treat that as "we're done" as well:
		if errors.Is(err, os.ErrNotExist) {
			log.Println("no further migrations – done")
			return
		}

		if err != nil {
			log.Fatalf("❌ migration step starting from version %d failed: %v", v, err)
		}
	}
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

	s, err := model.InitDatabase(cfg)
	if err != nil {
		log.Fatal(err)
	}

	if maintenance {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if err := model.RunMaintenance(ctx, s); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := controller.NewController(s); err != nil {
		log.Fatal(err)
	}
}
