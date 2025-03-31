package cfgx

import "testing"

func TestSnake(t *testing.T) {

	table := map[string]string{
		"Port":                  "port",
		"Host":                  "host",
		"HTTPPort":              "http_port",
		"UserID":                "user_id",
		"Test2Test":             "test2_test",
		"EnableSSLMode":         "enable_ssl_mode",
		"DB":                    "db",
		"DB.PasswordFile":       "db_password_file",
		"Logging.Level":         "logging_level",
		"First.SecondACR.Third": "first_second_acr_third",
	}

	for in, want := range table {
		t.Run(in, func(t *testing.T) {
			got := toSnakeCase(in)
			if want != got {
				t.Errorf("wanted %s, got %s", want, got)
			}
		})
	}
}

func TestScreamingSnake(t *testing.T) {
	table := map[string]string{
		"Port":                  "PORT",
		"Host":                  "HOST",
		"HTTPPort":              "HTTP_PORT",
		"UserID":                "USER_ID",
		"Test2Test":             "TEST2_TEST",
		"EnableSSLMode":         "ENABLE_SSL_MODE",
		"DB":                    "DB",
		"DB.PasswordFile":       "DB_PASSWORD_FILE",
		"Logging.Level":         "LOGGING_LEVEL",
		"First.SecondACR.Third": "FIRST_SECOND_ACR_THIRD",
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
		"Port":                  "port",
		"Host":                  "host",
		"HTTPPort":              "http-port",
		"UserID":                "user-id",
		"Test2Test":             "test2-test",
		"EnableSSLMode":         "enable-ssl-mode",
		"DB":                    "db",
		"DB.PasswordFile":       "db-password-file",
		"Logging.Level":         "logging-level",
		"First.SecondACR.Third": "first-second-acr-third"}

	for in, want := range table {
		t.Run(in, func(t *testing.T) {
			got := toKebabCase(in)
			if want != got {
				t.Errorf("wanted %s, got %s", want, got)
			}
		})
	}
}
