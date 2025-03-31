package main

import "testing"

func TestDeriveOutputFilename(t *testing.T) {
	var table = map[string]string{
		"colorMap":             "color.go",
		"statusCodeMap":        "statuscode.go",
		"option":               "option.go",
		"somethingMapToString": "something.go",
	}

	for name, want := range table {
		t.Run(name, func(t *testing.T) {

			if got := deriveOutputFileName(name); want != got {
				t.Errorf("wanted %s, got %s", want, got)
			}
		})
	}
}
func TestDeriveTypeName(t *testing.T) {
	var table = map[string]string{
		"colorMap":             "Color",
		"statusCodeMap":        "StatusCode",
		"option":               "Option",
		"somethingMapToString": "Something",
	}

	for name, want := range table {
		t.Run(name, func(t *testing.T) {
			suffixes := []string{"Map", "MapToString", "ToString", "ToDisplay"}

			if got := deriveTypeName(name, suffixes); want != got {
				t.Errorf("wanted %s, got %s", want, got)
			}
		})
	}
}

func TestToPascalCase(t *testing.T) {
	var table = map[string]string{
		"colors":     "Colors",
		"statusCode": "StatusCode",
		"dbHost":     "DbHost", // cant avoid this
		"DBHost":     "DBHost",
		"something":  "Something",
		"Two Words":  "TwoWords",
		"two words":  "TwoWords",
	}

	for name, want := range table {
		t.Run(name, func(t *testing.T) {

			if got := toPascalCase(name); want != got {
				t.Errorf("wanted %s, got %s", want, got)
			}
		})
	}
}
