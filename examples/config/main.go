package main

import (
	"log"
	"os"

	"github.com/erlorenz/go-toolbox/config"
)

func main() {

	var cfg struct {
		Version     string
		Environment string
		Log         struct {
			Level string `default:"info"`
		}
		URL   string `required:"true" env:"BASE_URL"`
		Debug bool   `short:"d"`
	}

	os.Setenv("APP_ENVIRONMENT", "development") //  prefix APP
	os.Setenv("BASE_URL", "http://example.com") // comment out for required error
	args := []string{"-d"}                      // short flag

	if err := config.Parse(&cfg, config.Options{
		UseBuildInfo: true,
		EnvPrefix:    "APP",
		Args:         args,
	}); err != nil {
		log.Fatal(err)
	}

	log.Printf("%s: %s", "Version", cfg.Version)         // (develop) if using go run
	log.Printf("%s: %s", "Environment", cfg.Environment) // development
	log.Printf("%s: %v", "Log.Level", cfg.Log.Level)     // info
	log.Printf("%s: %v", "URL", cfg.URL)                 // http://example.com
	log.Printf("%s: %v", "Debug", cfg.Debug)             // true
}
