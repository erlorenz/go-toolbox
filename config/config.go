// Package config provides functionality to parse configuration from multiple sources
// in a predictable precedence order with strong error handling and traceability.
// It is designed to be flexible enough for most applications while providing
// sensible defaults that follow Go idioms and best practices.
// with a defined precedence: command line args > environment variables > yaml files > defaults.
// It uses struct tags to customize field names and validation rules.
package config

import (
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// Config tag constants
const (
	configTag  = "config"  // Main config tag
	envTag     = "env"     // Environment variable name
	flagTag    = "flag"    // Command line flag name
	defaultTag = "default" // Default value
	// descriptionTag = "desc"     // Description for help messages
	optionalTag = "optional" // Mark field as optional
	// validateTag    = "validate" // Validation rules
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// MultiError holds multiple errors that occurred during parsing
type MultiError struct {
	Errors []error
}

func (m MultiError) Error() string {
	if len(m.Errors) == 0 {
		return "no errors"
	}

	errMsgs := make([]string, len(m.Errors))
	for i, err := range m.Errors {
		errMsgs[i] = err.Error()
	}

	return fmt.Sprintf("%d error(s) occurred:\n- %s",
		len(m.Errors), strings.Join(errMsgs, "\n- "))
}

// Options holds options for the Parse function
type Options struct {
	// ProgramName is used in usage messages for command line flags
	ProgramName string
	// EnvPrefix is prefixed to environment variable names (unless overridden by tags)
	EnvPrefix string
	// SkipFlags indicates whether to skip parsing command line flags
	SkipFlags bool
	// SkipEnv indicates whether to skip parsing environment variables
	SkipEnv bool
	// Args provides command line arguments (defaults to os.Args[1:])
	Args []string
	// ErrorHandling determines how parsing errors are handled
	ErrorHandling flag.ErrorHandling
}

// DefaultConfigOptions returns the default configuration options
func DefaultConfigOptions() Options {
	opts := Options{
		ProgramName:   os.Args[0],
		EnvPrefix:     "",
		SkipFlags:     false,
		SkipEnv:       false,
		Args:          os.Args[1:],
		ErrorHandling: flag.ContinueOnError,
	}

	return opts
}

// Parse populates the config struct from different sources
// It follows this precedence order (highest to lowest):
// 1. Command line arguments
// 2. Environment variables
// 3. YAML configuration files
// 4. Default values from struct tags
func Parse(cfg any, options Options) (map[string]configField, error) {

	// opts := setOptions(options)

	// Get reflected value and ensure it's a pointer to a struct
	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return nil, errors.New("config must be a pointer to a struct")
	}
	v = v.Elem()

	var allErrors MultiError

	structMap := walkStruct(v, "")

	if err := applyDefaults(structMap); err != nil {
		return structMap, err
	}

	if err := applyEnvs(structMap); err != nil {
		return structMap, err
	}

	// Parse environment variables (if enabled)
	// if !opts.SkipEnv {
	// 	if err := parseEnv(v, opts.EnvPrefix); err != nil {
	// 		allErrors.Errors = append(allErrors.Errors, fmt.Errorf("parsing environment variables: %w", err))
	// 	}
	// }

	// Parse command line arguments (if enabled, highest precedence)
	// if !opts.SkipFlags {
	// 	if err := parseFlags(v, opts.ProgramName, opts.Args, opts.ErrorHandling); err != nil {
	// 		allErrors.Errors = append(allErrors.Errors, fmt.Errorf("parsing command line flags: %w", err))
	// 	}
	// }

	// Validate the configuration
	// if err := validate(v); err != nil {
	// 	allErrors.Errors = append(allErrors.Errors, fmt.Errorf("validation: %w", err))
	// }

	if len(allErrors.Errors) > 0 {
		return nil, allErrors
	}

	return structMap, nil
}

type configField struct {
	Path        string
	Value       reflect.Value
	Kind        reflect.Kind
	Name        string
	StructField reflect.StructField
	Tag         reflect.StructTag
}

func walkStruct(v reflect.Value, currPath string) map[string]configField {
	fields := map[string]configField{}

	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		// Get values
		field := v.Field(i)
		structField := t.Field(i)
		name := structField.Name
		kind := field.Kind()
		tag := structField.Tag

		// Join the path
		path := name
		if currPath != "" {
			path = strings.Join([]string{currPath, name}, ".")
		}

		// Recursive for structs
		if kind == reflect.Struct {
			nestedFields := walkStruct(field, path)
			maps.Copy(fields, nestedFields)
			continue
		}

		val := field
		fields[path] = configField{
			Path: path, Value: val, Kind: kind, Name: name, StructField: structField, Tag: tag}
	}
	return fields
}

func applyDefaults(fields map[string]configField) error {
	var allErrs MultiError

	for _, field := range fields {
		def, ok := field.Tag.Lookup(defaultTag)
		if !ok {
			continue
		}

		switch field.Kind {
		case reflect.String:
			field.Value.SetString(def)
		case reflect.Int:
			intVal, _ := strconv.ParseInt(def, 10, 64)
			field.Value.SetInt(intVal)
		default:
			allErrs.Errors = append(allErrs.Errors, fmt.Errorf("cannot set %s: unimplemented kind %s", field.Path, field.Kind))
		}
	}
	if len(allErrs.Errors) > 0 {
		return allErrs
	}
	return nil
}

func applyEnvs(fields map[string]configField) error {
	var multErr MultiError

	for _, field := range fields {

		envName := toScreamingSnakeCase(field.Path)

		def, ok := field.Tag.Lookup(envTag)
		if ok {
			envName = def
		}

		val, ok := os.LookupEnv(envName)
		if !ok {
			continue
		}

		// String
		field.Value.SetString(val)
	}

	if len(multErr.Errors) > 0 {
		return multErr
	}
	return nil
}
