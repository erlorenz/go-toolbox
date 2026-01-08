package casing

import "testing"

func TestSnake(t *testing.T) {

	table := map[string]string{
		// Basic cases
		"Port":                  "port",
		"Host":                  "host",
		"UserID":                "user_id",

		// Single acronyms
		"DB":                    "db",
		"API":                   "api",

		// Acronyms in middle
		"HTTPPort":              "http_port",
		"EnableSSLMode":         "enable_ssl_mode",

		// Acronym at start
		"HTTPSConnection":       "https_connection",

		// Acronym at end
		"ConnectionHTTPS":       "connection_https",

		// Numbers
		"Test2Test":             "test2_test",
		"OAuth2Client":          "o_auth2_client",

		// Single letter cases
		"AProvider":             "a_provider",

		// Dots (nested struct fields)
		"DB.PasswordFile":       "db_password_file",
		"Logging.Level":         "logging_level",
		"First.SecondACR.Third": "first_second_acr_third",

		// CamelCase (starting lowercase)
		"myFieldName":           "my_field_name",
		"someAPIKey":            "some_api_key",

		// Edge cases
		"":                      "",
		"a":                     "a",
		"A":                     "a",
	}

	for in, want := range table {
		t.Run(in, func(t *testing.T) {
			got := ToSnake(in)
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
			got := ToScreamingSnake(in)
			if want != got {
				t.Errorf("wanted %s, got %s", want, got)
			}
		})
	}

}

func TestKebab(t *testing.T) {
	table := map[string]string{
		// Basic cases
		"Port":                  "port",
		"Host":                  "host",
		"UserID":                "user-id",

		// Single acronyms
		"DB":                    "db",
		"API":                   "api",

		// Acronyms in middle
		"HTTPPort":              "http-port",
		"EnableSSLMode":         "enable-ssl-mode",

		// Acronym at start
		"HTTPSConnection":       "https-connection",

		// Acronym at end
		"ConnectionHTTPS":       "connection-https",

		// Numbers
		"Test2Test":             "test2-test",
		"OAuth2Client":          "o-auth2-client",

		// Single letter cases
		"AProvider":             "a-provider",

		// Dots (nested struct fields)
		"DB.PasswordFile":       "db-password-file",
		"Logging.Level":         "logging-level",
		"First.SecondACR.Third": "first-second-acr-third",

		// CamelCase (starting lowercase)
		"myFieldName":           "my-field-name",
		"someAPIKey":            "some-api-key",

		// Edge cases
		"":                      "",
		"a":                     "a",
		"A":                     "a",
	}

	for in, want := range table {
		t.Run(in, func(t *testing.T) {
			got := ToKebab(in)
			if want != got {
				t.Errorf("wanted %s, got %s", want, got)
			}
		})
	}
}

func TestPascal(t *testing.T) {
	table := map[string]string{
		// snake_case input
		"port":                  "Port",
		"host":                  "Host",
		"user_id":               "UserId",
		"http_port":             "HttpPort",
		"enable_ssl_mode":       "EnableSslMode",
		"db":                    "Db",
		"api":                   "Api",
		"db_password_file":      "DbPasswordFile",
		"logging_level":         "LoggingLevel",
		"my_field_name":         "MyFieldName",

		// kebab-case input
		"http-port":             "HttpPort",
		"user-id":               "UserId",
		"enable-ssl-mode":       "EnableSslMode",
		"db-password-file":      "DbPasswordFile",

		// dot notation
		"db.password_file":      "DbPasswordFile",
		"logging.level":         "LoggingLevel",

		// Already PascalCase
		"Port":                  "Port",
		"HTTPPort":              "Httpport",
		"UserID":                "Userid",

		// Already camelCase
		"myFieldName":           "Myfieldname",

		// Mixed separators
		"my-field_name":         "MyFieldName",
		"test.value_here":       "TestValueHere",

		// Edge cases
		"":                      "",
		"a":                     "A",
		"A":                     "A",
		"_":                     "",
		"__test__":              "Test",
		"test_":                 "Test",
		"_test":                 "Test",
	}

	for in, want := range table {
		t.Run(in, func(t *testing.T) {
			got := ToPascal(in)
			if want != got {
				t.Errorf("wanted %s, got %s", want, got)
			}
		})
	}
}

func TestCamel(t *testing.T) {
	table := map[string]string{
		// snake_case input
		"port":                  "port",
		"host":                  "host",
		"user_id":               "userId",
		"http_port":             "httpPort",
		"enable_ssl_mode":       "enableSslMode",
		"db":                    "db",
		"api":                   "api",
		"db_password_file":      "dbPasswordFile",
		"logging_level":         "loggingLevel",
		"my_field_name":         "myFieldName",

		// kebab-case input
		"http-port":             "httpPort",
		"user-id":               "userId",
		"enable-ssl-mode":       "enableSslMode",
		"db-password-file":      "dbPasswordFile",

		// dot notation
		"db.password_file":      "dbPasswordFile",
		"logging.level":         "loggingLevel",

		// Already PascalCase
		"Port":                  "port",
		"HTTPPort":              "httpport",
		"UserID":                "userid",

		// Already camelCase
		"myFieldName":           "myfieldname",

		// Mixed separators
		"my-field_name":         "myFieldName",
		"test.value_here":       "testValueHere",

		// Edge cases
		"":                      "",
		"a":                     "a",
		"A":                     "a",
		"_":                     "",
		"__test__":              "test",
		"test_":                 "test",
		"_test":                 "test",
	}

	for in, want := range table {
		t.Run(in, func(t *testing.T) {
			got := ToCamel(in)
			if want != got {
				t.Errorf("wanted %s, got %s", want, got)
			}
		})
	}
}
