package model

import (
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type CRMDatabase struct {
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
	DBLogger   string
}

type ParentType string

const (
	ParentTypeCompany ParentType = "company"
	ParentTypePerson  ParentType = "person"
)

func (p ParentType) String() string { return string(p) }

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
