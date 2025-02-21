// Package config provides functionality to parse configuration from multiple sources
// in a predictable precedence order with strong error handling and traceability.
// It is designed to be flexible enough for most applications while providing
// sensible defaults that follow Go idioms and best practices.
// with a defined precedence: command line args > environment variables > yaml files > defaults.
// It uses struct tags to customize field names and validation rules.
package config

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
)

const (
	configTag      = "config"
	envTag         = "env"
	flagTag        = "flag"
	defaultTag     = "default"
	descriptionTag = "desc"     // Description for help messages
	optionalTag    = "optional" // Mark field as optional
	shortTag       = "short"    // Short flag in addition
	// validateTag    = "validate" // Validation rules
)

// Options holds options for the Parse function.
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
	// UseBuildInfo uses debug.BuildInfo to set the Version property to the git tag.
	UseBuildInfo bool
}

// Parse populates the config struct from different sources.
// It follows this precedence order (highest to lowest):
// 1. Command line arguments
// 2. Environment variables
// 3. YAML configuration files
// 4. Default values from struct tags
func Parse(cfg any, options Options) (map[string]configField, error) {

	// Make sure it is pointer to struct
	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return nil, errors.New("config must be a pointer to a struct")
	}

	// Set default options and override if non-zero
	opts := setOptions(options)

	// Walk the concrete struct
	// Skips any fields that are already populated
	structMap := walkStruct(v.Elem(), "")

	// 1. Use the default tags
	if err := applyDefaults(structMap); err != nil {
		return structMap, err
	}

	// 2. Override with env vars
	if !opts.SkipEnv {
		err := applyEnvs(structMap)
		if err != nil {
			return structMap, err
		}
	}

	// 3. Parse flags and override with values
	if !opts.SkipFlags {
		err := applyFlags(structMap, opts)
		if err != nil {
			return structMap, err
		}
	}

	// Set Version if opts.UseBuildInfo == true
	if opts.UseBuildInfo {
		bi, _ := debug.ReadBuildInfo()

		version, ok := structMap["Version"]
		if ok {
			version.Value.SetString(cmp.Or(bi.Main.Version, "(develop)"))
		}
	}

	// Validate the configuration
	// if err := validate(v); err != nil {
	// 	allErrors.Errors = append(allErrors.Errors, fmt.Errorf("validation: %w", err))
	// }

	return structMap, nil
}

type configField struct {
	Path        string
	Value       reflect.Value
	Kind        reflect.Kind
	Name        string
	StructField reflect.StructField
	Tag         reflect.StructTag
	Description string
}

func walkStruct(v reflect.Value, currPath string) map[string]configField {
	fields := map[string]configField{}

	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		// Get values
		fieldVal := v.Field(i)
		structField := t.Field(i)
		name := structField.Name
		kind := fieldVal.Kind()
		tag := structField.Tag

		// Skip fields already filled
		if !fieldVal.IsZero() {
			continue
		}

		// Join the path
		path := name
		if currPath != "" {
			path = strings.Join([]string{currPath, name}, ".")
		}

		// Recursive for structs
		if kind == reflect.Struct {
			nestedFields := walkStruct(fieldVal, path)
			maps.Copy(fields, nestedFields)
			continue
		}
		desc := cmp.Or(tag.Get(descriptionTag), path)

		fields[path] = configField{
			Path: path, Value: fieldVal, Kind: kind, Name: name, StructField: structField, Tag: tag, Description: desc}
	}
	return fields
}

func applyDefaults(fields map[string]configField) error {
	var allErrs []error

	for _, field := range fields {
		defVal, ok := field.Tag.Lookup(defaultTag)
		if !ok {
			continue
		}

		switch field.Kind {
		// String
		case reflect.String:
			field.Value.SetString(defVal)
		// Int
		case reflect.Int:
			intVal, _ := strconv.ParseInt(defVal, 10, 64)
			field.Value.SetInt(intVal)
		default:
			allErrs = append(allErrs, fmt.Errorf("cannot set %s: unimplemented kind %s", field.Path, field.Kind))
		}
	}
	if len(allErrs) > 0 {
		return &MultiError{allErrs}
	}
	return nil
}

func applyEnvs(fields map[string]configField) error {
	var allErrs []error

	for _, field := range fields {
		envName := toScreamingSnakeCase(field.Path)

		// Overwrite with tag
		tagVal, ok := field.Tag.Lookup(envTag)
		if ok {
			envName = tagVal
		}

		envVal, ok := os.LookupEnv(envName)
		if !ok {
			continue
		}

		switch field.Kind {
		// String
		case reflect.String:
			field.Value.SetString(envVal)
		// Int
		case reflect.Int:
			intVal, err := strconv.ParseInt(envVal, 10, 64)
			if err != nil {
				allErrs = append(allErrs, fmt.Errorf("cannot set %s: %w", field.Path, err))
				break
			}
			field.Value.SetInt(intVal)
		default:
			allErrs = append(allErrs, fmt.Errorf("cannot set %s: unimplemented kind %s", field.Path, field.Kind))
		}
	}

	if len(allErrs) > 0 {
		return &MultiError{allErrs}
	}
	return nil
}

func applyFlags(fields map[string]configField, opts Options) error {
	allErrs := &MultiError{}

	flags := flag.NewFlagSet(opts.ProgramName, opts.ErrorHandling)

	// Temporary flag map of string values
	flagValues := map[string]*string{}

	// Load the flagValues map with the flag values
	for path, field := range fields {
		flagName := toKebabCase(field.Path)
		shortFlagName := field.Tag.Get(shortTag)

		// Overwrite with tag
		if tagVal, ok := field.Tag.Lookup(flagTag); ok {
			flagName = tagVal
		}

		flagValues[path] = flags.String(flagName, "", field.Description)
		if shortFlagName != "" {
			flagValues[path+"-short"] = flags.String(shortFlagName, "", field.Description)
		}

	}

	// Parse flags
	if err := flags.Parse(opts.Args); err != nil {
		return fmt.Errorf("failed parsing flags: %w", err)
	}

	// Now set the values to the fields
	for path, flagVal := range flagValues {
		// Skip the default
		if *flagVal == "" {
			continue
		}
		// Make short use same field
		path = strings.TrimSuffix(path, "-short")

		field := fields[path]

		switch field.Kind {
		case reflect.String:
			field.Value.SetString(*flagVal)
		case reflect.Int:
			intVal, err := strconv.ParseInt(*flagVal, 10, 64)
			if err != nil {
				allErrs.Errors = append(allErrs.Errors, fmt.Errorf("cannot set %s: %w", field.Path, err))
				break
			}
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
