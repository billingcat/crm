package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/billingcat/crm/controller"
	"github.com/billingcat/crm/model"
	"github.com/pelletier/go-toml/v2"
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

func main() {
	var maintenance bool
	flag.BoolVar(&maintenance, "maintenance", false, "run maintenance tasks and exit")
	flag.Parse()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	crmdb, err := model.InitDatabase(cfg)
	if err != nil {
		log.Fatal(err)
	}

	if maintenance {
		// bounded runtime & cancellation support
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if err := model.RunMaintenance(ctx, crmdb); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Regular web server mode
	if err := controller.NewController(crmdb); err != nil {
		log.Fatal(err)
	}
}
