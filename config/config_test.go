package config_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/erlorenz/go-toolbox/config"
)

const (
	tagEnv     = "env"
	tagDefault = "default"
)

func TestParse(t *testing.T) {
	cfg := struct {
		Version string
		Author  string `env:"AUTH"`
		Port    int    `default:"5000"`
		Logging struct {
			Level string `default:"info"`
		}
	}{Version: "10.0.0"}

	args := []string{"--author", "John Doe", "-logger-level=v1.0.0"}
	os.Setenv("AUTH", "John Deere")
	os.Setenv("LOGGING_LEVEL", "debug")
	// os.Setenv("PORT", "5000")

	m, err := config.Parse(&cfg, config.Options{Args: args})
	if err != nil {
		t.Fatal(err)
	}

	for name, field := range m {
		t.Logf("%s: %v", name, field.Value)
	}
	t.Fatalf("%+v", cfg)

	cfgv := reflect.ValueOf(cfg)
	ver := cfgv.FieldByName("Version")
	t.Fatal(ver)

	wantAuthor := "John Doe"
	if cfg.Author != wantAuthor {
		t.Errorf("wanted %s, got %s", wantAuthor, cfg.Author)
	}

	if cfg.Version != "v1.0.0" {
		t.Errorf("wanted %s, got %s", "v1.0.0", cfg.Version)
	}

}
