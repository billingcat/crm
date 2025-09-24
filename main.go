package main

import (
	"log"
	"os"

	"github.com/billingcat/crm/controller"
	"github.com/billingcat/crm/model"

	"github.com/pelletier/go-toml/v2"
)

func dothings() error {
	data, err := os.ReadFile("config.toml")
	if err != nil {
		return err
	}
	cfg := &model.Config{}
	toml.Unmarshal(data, cfg)
	crmdb, err := model.InitDatabase(cfg)
	if err != nil {
		return err
	}
	return controller.NewController(crmdb)
}

func main() {
	if err := dothings(); err != nil {
		log.Fatal(err)
	}
}
