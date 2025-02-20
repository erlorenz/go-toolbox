package config

import "testing"

func TestScreamingSnake(t *testing.T) {

	table := map[string]string{
		"Port":          "PORT",
		"Host":          "HOST",
		"HTTPPort":      "HTTP_PORT",
		"UserID":        "USER_ID",
		"Test2Test":     "TEST2_TEST",
		"EnableSSLMode": "ENABLE_SSL_MODE",
		"DB":            "DB",
		"Logging.Level": "LOGGING_LEVEL",
	}

	for in, want := range table {
		t.Run(in, func(t *testing.T) {
			got := toScreamingSnakeCase(in)
			if want != got {
				t.Errorf("wanted %s, got %s", want, got)
			}
		})
	}
}

func TestKebab(t *testing.T) {

	table := map[string]string{
		"Port":          "port",
		"Host":          "host",
		"HTTPPort":      "http-port",
		"UserID":        "user-id",
		"Test2Test":     "test2-test",
		"EnableSSLMode": "enable-ssl-mode",
		"DB":            "db",
		"Logging.Level": "logging-level",
	}

	for in, want := range table {
		t.Run(in, func(t *testing.T) {
			got := toKebabCase(in)
			if want != got {
				t.Errorf("wanted %s, got %s", want, got)
			}
		})
	}
}
