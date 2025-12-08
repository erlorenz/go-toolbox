package cfgx_test

import (
	"flag"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/erlorenz/go-toolbox/cfgx"
)

// cleanupEnv registers cleanup to unset the given environment variables
func cleanupEnv(t *testing.T, keys ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, key := range keys {
			os.Unsetenv(key)
		}
	})
}

func TestParse(t *testing.T) {
	cfg := struct {
		Version string
		Author  string `env:"PROGRAM_AUTHOR" optional:"true" desc:"The author of the program"`
		Port    int    `default:"5000" short:"p" desc:"The server port"`
		BaseURL string `default:"http://example.com" env:"API_URL" desc:"The API base URL"`
		Debug   bool   `default:"true" short:"d"`
		Logging struct {
			Level string `default:"info" desc:"The minimum log level"`
		}
	}{Version: "v10.0.0"}

	t.Run("Defaults", func(t *testing.T) {

		cfg := cfg
		err := cfgx.Parse(&cfg, cfgx.Options{SkipFlags: true, SkipEnv: true})
		if err != nil {
			t.Fatal(err)
		}

		if cfg.Author != "" {
			t.Errorf("Author: wanted empty string, got %s", cfg.Author)
		}
		if want := "v10.0.0"; cfg.Version != want {
			t.Errorf("Version: wanted %s, got %s", "v10.0.0", cfg.Version)
		}
		if want := 5000; cfg.Port != want {
			t.Errorf("Port: wanted %d, got %d", want, cfg.Port)
		}
		if want := "info"; cfg.Logging.Level != want {
			t.Errorf("Logging.Level: wanted %s, got %s", want, cfg.Logging.Level)
		}
		if want := "http://example.com"; cfg.BaseURL != want {
			t.Errorf("Logging.Level: wanted %s, got %s", want, cfg.BaseURL)
		}
		if want := true; cfg.Debug != want {
			t.Errorf("Debug: wanted %t, got %t", want, cfg.Debug)
		}
	})
	t.Run("EnvsPrefixed", func(t *testing.T) {

		cfg := cfg

		os.Setenv("PROGRAM_AUTHOR", "John Deere") // Should use tag
		os.Setenv("APP_PORT", "5001")
		os.Setenv("APP_LOGGING_LEVEL", "debug")
		os.Setenv("VERSION", "error") // Should skip
		os.Setenv("API_URL", "http://api.example.com")
		cleanupEnv(t, "PROGRAM_AUTHOR", "APP_PORT", "APP_LOGGING_LEVEL", "VERSION", "API_URL")

		err := cfgx.Parse(&cfg, cfgx.Options{SkipFlags: true, EnvPrefix: "APP"})
		if err != nil {
			t.Fatal(err)
		}

		if want := "John Deere"; cfg.Author != want {
			t.Errorf("Author: wanted %s, got %s", want, cfg.Author)
		}
		if want := "v10.0.0"; cfg.Version != want {
			t.Errorf("Version: wanted %s, got %s", want, cfg.Version)
		}
		if want := 5001; cfg.Port != want {
			t.Errorf("Port: wanted %d, got %d", want, cfg.Port)
		}
		if want := "debug"; cfg.Logging.Level != want {
			t.Errorf("Logging.Level: wanted %s, got %s", want, cfg.Logging.Level)
		}
		if want := "http://api.example.com"; cfg.BaseURL != want {
			t.Errorf("BaseURL: wanted %s, got %s", want, cfg.BaseURL)
		}
		if want := true; cfg.Debug != want {
			t.Errorf("Debug: wanted %t, got %t", want, cfg.Debug)
		}
	})

	t.Run("Envs", func(t *testing.T) {

		cfg := cfg

		os.Setenv("PROGRAM_AUTHOR", "John Deere")
		os.Setenv("PORT", "5001")
		os.Setenv("LOGGING_LEVEL", "debug")
		os.Setenv("VERSION", "error")
		os.Setenv("API_URL", "http://api.example.com")
		cleanupEnv(t, "PROGRAM_AUTHOR", "PORT", "LOGGING_LEVEL", "VERSION", "API_URL")

		err := cfgx.Parse(&cfg, cfgx.Options{SkipFlags: true})
		if err != nil {
			t.Fatal(err)
		}

		if want := "John Deere"; cfg.Author != want {
			t.Errorf("Author: wanted %s, got %s", want, cfg.Author)
		}
		if want := "v10.0.0"; cfg.Version != want {
			t.Errorf("Version: wanted %s, got %s", want, cfg.Version)
		}
		if want := 5001; cfg.Port != want {
			t.Errorf("Port: wanted %d, got %d", want, cfg.Port)
		}
		if want := "debug"; cfg.Logging.Level != want {
			t.Errorf("Logging.Level: wanted %s, got %s", want, cfg.Logging.Level)
		}
		if want := "http://api.example.com"; cfg.BaseURL != want {
			t.Errorf("BaseURL: wanted %s, got %s", want, cfg.BaseURL)
		}
		if want := true; cfg.Debug != want {
			t.Errorf("Debug: wanted %t, got %t", want, cfg.Debug)
		}
	})

	t.Run("Flags", func(t *testing.T) {
		cfg := cfg
		os.Setenv("PROGRAM_AUTHOR", "John Deere")
		os.Setenv("PORT", "5001")
		os.Setenv("LOGGING_LEVEL", "debug")
		os.Setenv("API_URL", "http://api.example.com")
		os.Setenv("DEBUG", "true")
		cleanupEnv(t, "PROGRAM_AUTHOR", "PORT", "LOGGING_LEVEL", "API_URL", "DEBUG")

		args := []string{"-port", "3000", "--logging-level=error", "-author=Jack Smith", "-base-url=http://example.com/api"}

		err := cfgx.Parse(&cfg, cfgx.Options{Args: args})
		if err != nil {
			t.Fatal(err)
		}

		if want := "Jack Smith"; cfg.Author != want {
			t.Errorf("Author: wanted %s, got %s", want, cfg.Author)
		}
		if want := "v10.0.0"; cfg.Version != want {
			t.Errorf("Version: wanted %s, got %s", want, cfg.Version)
		}
		if want := 3000; cfg.Port != want {
			t.Errorf("Port: wanted %d, got %d", want, cfg.Port)
		}
		if want := "error"; cfg.Logging.Level != want {
			t.Errorf("Logging.Level: wanted %s, got %s", want, cfg.Logging.Level)
		}
		if want := "http://example.com/api"; cfg.BaseURL != want {
			t.Errorf("BaseURL: wanted %s, got %s", want, cfg.BaseURL)
		}
	})

	t.Run("Flags_Short", func(t *testing.T) {
		cfg := cfg
		os.Setenv("PROGRAM_AUTHOR", "John Deere")
		os.Setenv("PORT", "5001")
		os.Setenv("LOGGING_LEVEL", "debug")
		os.Setenv("API_URL", "http://api.example.com")
		cleanupEnv(t, "PROGRAM_AUTHOR", "PORT", "LOGGING_LEVEL", "API_URL")

		args := []string{"-p", "3000", "--logging-level=error", "-author=Jack Smith", "-base-url=http://example.com/api"}

		err := cfgx.Parse(&cfg, cfgx.Options{Args: args})
		if err != nil {
			t.Fatal(err)
		}

		if want := "Jack Smith"; cfg.Author != want {
			t.Errorf("Author: wanted %s, got %s", want, cfg.Author)
		}
		if want := "v10.0.0"; cfg.Version != want {
			t.Errorf("Version: wanted %s, got %s", want, cfg.Version)
		}
		if want := 3000; cfg.Port != want {
			t.Errorf("Port: wanted %d, got %d", want, cfg.Port)
		}
		if want := "error"; cfg.Logging.Level != want {
			t.Errorf("Logging.Level: wanted %s, got %s", want, cfg.Logging.Level)
		}
		if want := "http://example.com/api"; cfg.BaseURL != want {
			t.Errorf("BaseURL: wanted %s, got %s", want, cfg.BaseURL)
		}
	})

	t.Run("Files", func(t *testing.T) {

		fakeFS := fstest.MapFS{
			"my_secret": &fstest.MapFile{
				Data: []byte("supersecret"),
			},
			"my_secret_int": &fstest.MapFile{
				Data: []byte("5"),
			},
		}

		var cfg struct {
			MySecret    string
			MySecretInt int
		}

		sfc := &cfgx.FileContentSource{
			PriorityLevel: 50,
			Tag:           "file",
			FS:            fakeFS,
		}

		err := cfgx.Parse(&cfg, cfgx.Options{
			SkipFlags: true,
			SkipEnv:   true,
			Sources:   []cfgx.Source{sfc},
		})
		if err != nil {
			t.Fatal(err)
		}

		if want, got := "supersecret", cfg.MySecret; got != want {
			t.Errorf("MySecret: wanted %s, got %s", want, got)
		}
		if want, got := 5, cfg.MySecretInt; want != got {
			t.Errorf("MySecretInt: wanted %d, got %d", want, got)
		}
	})
}

func TestOptions(t *testing.T) {
	t.Parallel()

	type bicfg struct {
		Version string
		Author  string `env:"PROGRAM_AUTHOR" optional:"true" desc:"The author of the program"`
		Port    int    `default:"5000" desc:"The server port"`
		BaseURL string `default:"http://example.com" env:"API_URL" short:"p" desc:"The API base URL"`
		Logging struct {
			Level string `default:"info" desc:"The minimum log level"`
		}
	}
	t.Run("BuildInfo", func(t *testing.T) {
		var cfg bicfg
		cfgx.Parse(&cfg, cfgx.Options{
			ProgramName:   "The program",
			SkipFlags:     true,
			SkipEnv:       true,
			ErrorHandling: flag.PanicOnError,
		})

		if want := "(devel)"; cfg.Version != want {
			t.Errorf("Version: wanted %s, got %s", want, cfg.Version)
		}
	})
}

func TestValidate(t *testing.T) {
	t.Parallel()
	t.Run("OptionalNone", func(t *testing.T) {
		var cfg struct {
			Required string
		}

		err := cfgx.Parse(&cfg, cfgx.Options{SkipFlags: true, SkipEnv: true})
		if err == nil {
			t.Fatal(err)
		}
	})

	t.Run("OptionalTrue", func(t *testing.T) {
		var cfg struct {
			NotRequired string `optional:"true"`
		}

		err := cfgx.Parse(&cfg, cfgx.Options{SkipFlags: true, SkipEnv: true})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("OptionalFalse", func(t *testing.T) {
		var cfg struct {
			NotRequired string `optional:"false"`
		}

		err := cfgx.Parse(&cfg, cfgx.Options{SkipFlags: true, SkipEnv: true})
		if err == nil {
			t.Fatal(err)
		}
	})
}

func TestTypeSupport(t *testing.T) {
	t.Parallel()

	t.Run("Duration_Default", func(t *testing.T) {
		t.Parallel()
		var cfg struct {
			Timeout time.Duration `default:"5s"`
		}

		err := cfgx.Parse(&cfg, cfgx.Options{SkipFlags: true, SkipEnv: true})
		if err != nil {
			t.Fatal(err)
		}

		if want := 5 * time.Second; cfg.Timeout != want {
			t.Errorf("Timeout: wanted %v, got %v", want, cfg.Timeout)
		}
	})

	t.Run("Duration_Env", func(t *testing.T) {
		t.Parallel()
		var cfg struct {
			Timeout time.Duration
		}

		os.Setenv("TIMEOUT", "10m")
		cleanupEnv(t, "TIMEOUT")

		err := cfgx.Parse(&cfg, cfgx.Options{SkipFlags: true})
		if err != nil {
			t.Fatal(err)
		}

		if want := 10 * time.Minute; cfg.Timeout != want {
			t.Errorf("Timeout: wanted %v, got %v", want, cfg.Timeout)
		}
	})

	t.Run("Duration_Flag", func(t *testing.T) {
		t.Parallel()
		var cfg struct {
			Timeout time.Duration `short:"t"`
		}

		args := []string{"-t", "1h30m"}

		err := cfgx.Parse(&cfg, cfgx.Options{Args: args, SkipEnv: true})
		if err != nil {
			t.Fatal(err)
		}

		if want := 90 * time.Minute; cfg.Timeout != want {
			t.Errorf("Timeout: wanted %v, got %v", want, cfg.Timeout)
		}
	})

	t.Run("Int64", func(t *testing.T) {
		t.Parallel()
		var cfg struct {
			BigNum int64 `default:"9223372036854775807"`
		}

		err := cfgx.Parse(&cfg, cfgx.Options{SkipFlags: true, SkipEnv: true})
		if err != nil {
			t.Fatal(err)
		}

		if want := int64(9223372036854775807); cfg.BigNum != want {
			t.Errorf("BigNum: wanted %d, got %d", want, cfg.BigNum)
		}
	})

	t.Run("Uint", func(t *testing.T) {
		t.Parallel()
		var cfg struct {
			Count uint `default:"42"`
		}

		err := cfgx.Parse(&cfg, cfgx.Options{SkipFlags: true, SkipEnv: true})
		if err != nil {
			t.Fatal(err)
		}

		if want := uint(42); cfg.Count != want {
			t.Errorf("Count: wanted %d, got %d", want, cfg.Count)
		}
	})

	t.Run("Float64", func(t *testing.T) {
		t.Parallel()
		var cfg struct {
			Rate float64 `default:"3.14159"`
		}

		err := cfgx.Parse(&cfg, cfgx.Options{SkipFlags: true, SkipEnv: true})
		if err != nil {
			t.Fatal(err)
		}

		if want := 3.14159; cfg.Rate != want {
			t.Errorf("Rate: wanted %f, got %f", want, cfg.Rate)
		}
	})
}
