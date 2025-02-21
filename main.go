package main

import (
	"log"
	"os"

	"github.com/erlorenz/go-toolbox/config"
)

func main() {

	var cfg struct {
		Environment string
		Version     string
	}

	os.Setenv("ENVIRONMENT", "development")

	if _, err := config.Parse(&cfg, config.Options{
		UseBuildInfo: true,
	}); err != nil {
		log.Fatal(err)
	}

	log.Printf("%s: %s", "Version", cfg.Version)
	log.Printf("%s: %s", "Environment", cfg.Environment)
}
