package model

import (
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// CRMDatabase wraps the GORM database connection and holds the configuration
type CRMDatabase struct {
	db     *gorm.DB
	Config *Config
}

// Config holds the application configuration, it is read from config.toml
type Config struct {
	Basedir                  string
	CookieSecret             string
	MailAPIKey               string
	MailSecret               string
	Mode                     string
	UseInvitationCodes       bool
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
	DBLogger   string
}

// ParentType defines the type of parent entity for certain records (notes, contacts)
type ParentType string

const (
	// ParentTypeCompany is a customer.
	ParentTypeCompany ParentType = "company"
	// ParentTypePerson is a contact person.
	ParentTypePerson ParentType = "person"
)

func (p ParentType) String() string { return string(p) }

// IsValid checks if the ParentType is valid (= either company or person)
func (p ParentType) IsValid() bool {
	switch p {
	case ParentTypeCompany, ParentTypePerson:
		return true
	default:
		return false
	}
}

// shared helper for GORM logger
func gormLoggerFor(cfg *Config, svr server) *gorm.Config {
	gormConfig := &gorm.Config{}
	switch svr.DBLogger {
	case "info":
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	case "silent":
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	default:
		if cfg.Mode == "development" {
			gormConfig.Logger = logger.Default.LogMode(logger.Info)
		} else {
			gormConfig.Logger = logger.Default.LogMode(logger.Silent)
		}
	}
	return gormConfig
}
