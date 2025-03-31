package main

import (
	"flag"
	"log"
	"os"

	"github.com/erlorenz/go-toolbox/cfgx"
)

func main() {

	var cfg struct {
		Version     string
		Environment string
		Log         struct {
			Level string `default:"info"`
		}
		DB struct {
			Name     string
			HostAddr int    `flag:"db-host"`
			Password string `optional:"true"`
		}
		URL   string `env:"BASE_URL"`
		Debug bool   `optional:"true" short:"d"`
	}

	os.Setenv("APP_ENVIRONMENT", "development") // prefixed env
	os.Setenv("BASE_URL", "http://example.com") // specified env name
	args := []string{"-d"}                      // short flag
	args = append(args, "--db-name=postgres")   // regular flag
	args = append(args, "--db-host=5432")       // specified flag name

	if err := cfgx.Parse(&cfg, cfgx.Options{
		EnvPrefix:     "APP",
		Args:          args,
		ErrorHandling: flag.ContinueOnError, // change to see behavior on error
	}); err != nil {
		log.Fatal(err)
	}

	// Set optional later
	if cfg.DB.Password == "" {
		cfg.DB.Password = "postgres"
	}

	log.Printf("%s: %s", "Version", cfg.Version)         // (develop) if using go run
	log.Printf("%s: %s", "Environment", cfg.Environment) // development
	log.Printf("%s: %v", "Log.Level", cfg.Log.Level)     // info
	log.Printf("%s: %v", "URL", cfg.URL)                 // http://example.com
	log.Printf("%s: %v", "Debug", cfg.Debug)             // true
	log.Printf("%s: %v", "DB.Name", cfg.DB.Name)         // postgres
	log.Printf("%s: %v", "DB.HostAddr", cfg.DB.HostAddr) // 5432
	log.Printf("%s: %v", "DB.Password", cfg.DB.Password) // postgres
}
