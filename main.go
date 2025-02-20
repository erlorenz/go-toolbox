package main

import (
	"log"
	"os"

	"github.com/erlorenz/go-toolbox/config"
)

func main() {

	var cfg struct {
		Environment string
	}

	os.Setenv("ENVIRONMENT", "development")

	if err := config.Parse(&cfg, config.Options{}); err != nil {
		log.Fatal(err)
	}

	log.Printf("%#v", cfg)
}
