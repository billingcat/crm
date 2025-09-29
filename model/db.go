package model

import (
	"fmt"
	"path/filepath"

	"github.com/glebarez/sqlite"
	// "gorm.io/driver/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// CRMDatenbank ist die Hauptstruktur des Modells
type CRMDatenbank struct {
	db     *gorm.DB
	Config *Config
}

type Config struct {
	Basedir                  string
	CookieSecret             string
	MailAPIKey               string
	MailSecret               string
	Mode                     string
	Port                     int
	PublishingServerAddress  string
	PublishingServerUsername string
	RegistrationAllowed      bool
	Servers                  map[string]server
	SP                       string
	XMLDir                   string
}

type server struct {
	Database   string
	DBName     string
	DBUser     string
	DBPassword string
	DBHost     string
}

func (crmdb *CRMDatenbank) autoMigrate() error {
	// Migrate the schema

	var err error
	if err = crmdb.db.AutoMigrate(&Company{}); err != nil {
		return err
	}
	if err = crmdb.db.AutoMigrate(&ContactInfo{}); err != nil {
		return err
	}
	if err = crmdb.db.AutoMigrate(&Person{}); err != nil {
		return err
	}
	if err = crmdb.db.AutoMigrate(&Invoice{}); err != nil {
		return err
	}
	if err = crmdb.db.AutoMigrate(&InvoicePosition{}); err != nil {
		return err
	}
	if err = crmdb.db.AutoMigrate(&Settings{}); err != nil {
		return err
	}
	if err = crmdb.db.AutoMigrate(&User{}); err != nil {
		return err
	}
	if err = crmdb.db.AutoMigrate(&SignupToken{}); err != nil {
		return err
	}
	if err = crmdb.db.AutoMigrate(&RecentView{}); err != nil {
		return err
	}
	if err = crmdb.db.AutoMigrate(&Note{}); err != nil {
		return err
	}
	if err = crmdb.db.AutoMigrate(&APIToken{}); err != nil {
		return err
	}
	crmdb.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ux_recent_user_entity
         ON recent_views(user_id, entity_type, entity_id)`)
	crmdb.db.Exec(`CREATE INDEX IF NOT EXISTS idx_recent_user_viewed_at
         ON recent_views(user_id, viewed_at DESC)`)
	crmdb.db.Exec(`UPDATE notes SET author_id = owner_id WHERE (author_id IS NULL OR author_id = 0)`)
	return nil
}

// InitDatabase starts the database
func InitDatabase(cfg *Config) (*CRMDatenbank, error) {
	var err error

	crmdb := &CRMDatenbank{Config: cfg}
	svr := cfg.Servers[cfg.Mode]
	gormConfig := &gorm.Config{}
	if cfg.Mode == "development" {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	} else {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	}

	switch svr.Database {
	case "sqlite3":
		filename := filepath.Join("db", svr.DBName)
		fmt.Println("Use server sqlite3 and database", filename)
		crmdb.db, err = gorm.Open(sqlite.Open(filename), gormConfig)
		if err != nil {
			return nil, err
		}
	case "postgresql":
		fmt.Println("Use server postgresql and database", svr.DBName)
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=5432 sslmode=disable TimeZone=UTC",
			svr.DBHost, svr.DBUser, svr.DBPassword, svr.DBName)
		crmdb.db, err = gorm.Open(postgres.Open(dsn), gormConfig)
	default:
		return nil, fmt.Errorf("not implemented yet")
	}
	if err = crmdb.autoMigrate(); err != nil {
		return nil, err
	}
	return crmdb, nil
}
