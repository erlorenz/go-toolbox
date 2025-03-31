// Package cfgx provides functionality to parse configuration from multiple sources
// in a predictable precedence order with strong error handling and traceability.
// It is designed to be flexible enough for most applications while providing
// sensible defaults that follow Go idioms and best practices.
// with a defined precedence: command line args > environment variables > yaml files > defaults.
// It uses struct tags to customize field names and validation rules.
package cfgx

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"log"
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

var (
	ErrNotPointerToStruct = errors.New("config must be a pointer to a struct")
)

// Source processes the configField map and applies values to the
// config struct. Choose a priority to process before or after other sources.
type Source interface {
	Priority() int
	Process(map[string]configField) error
}

// Options holds options for the Parse function.
type Options struct {
	// ProgramName is the name of the running program (defaults to os.Args[0]).
	ProgramName string
	// EnvPrefix looks adds a prefix to environment variable lookups.
	EnvPrefix string
	// SkipFlags ignores command line flags.
	SkipFlags bool
	// SkipEnv ignores environment variables.
	SkipEnv bool
	// Args provides command line arguments (defaults to os.Args[1:]).
	Args []string
	// ErrorHandling determines how parsing errors are handled.
	ErrorHandling flag.ErrorHandling
	// UseBuildInfo uses debug.BuildInfo to set the Version property to the git tag.
	UseBuildInfo bool
	// Sources adds additional sources.
	Sources []Source
}

// Parse populates the config struct from different sources.
// It follows this precedence order (highest to lowest):
// 1. Command line arguments
// 2. Environment variables
// 3. YAML configuration files
// 4. Default values from struct tags
func Parse(cfg any, options Options) error {

	// Set default options and override if non-zero
	opts := setOptions(options)

	// Make sure it is pointer to struct
	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return handleError(opts.ErrorHandling, ErrNotPointerToStruct)
	}

	// Walk the struct and get map of paths with dot notation
	// Skips any fields that are already populated
	structMap := walkStruct(v.Elem(), "")

	// 1. Use the default tags
	if err := applyDefaults(structMap); err != nil {
		return handleError(opts.ErrorHandling, err)
	}

	// 2. Override with env vars
	if !opts.SkipEnv {
		err := applyEnvs(structMap, opts.EnvPrefix)
		if err != nil {
			return handleError(opts.ErrorHandling, err)
		}
	}

	// 3. Parse flags and override with values
	if !opts.SkipFlags {
		err := applyFlags(structMap, opts)
		if err != nil {
			return handleError(opts.ErrorHandling, err)
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

	// Validate the required
	if err := validateRequired(structMap); err != nil {
		return handleError(opts.ErrorHandling, fmt.Errorf("validation: %w", err))
	}

	return nil
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

	for i := range v.NumField() {
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
			intVal, err := strconv.ParseInt(defVal, 10, 64)
			if err != nil {
				allErrs = append(allErrs, fmt.Errorf("cannot set %s: %w", field.Path, err))
				break
			}
			field.Value.SetInt(intVal)
		case reflect.Bool:
			boolVal, _ := strconv.ParseBool(defVal)
			field.Value.SetBool(boolVal)
		default:
			allErrs = append(allErrs, fmt.Errorf("cannot set %s: unimplemented kind %s", field.Path, field.Kind))
		}
	}
	if len(allErrs) > 0 {
		return &MultiError{allErrs}
	}
	return nil
}

func applyEnvs(fields map[string]configField, prefix string) error {
	var allErrs []error

	for _, field := range fields {

		envName := toScreamingSnakeCase(field.Path)
		// Add prefix
		if prefix != "" {
			envName = prefix + "_" + envName
		}

		// Overwrite with tag
		tagVal, ok := field.Tag.Lookup(envTag)
		if ok {
			envName = tagVal
		}

		// Get value from env
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
		// Bool
		case reflect.Bool:
			boolVal, _ := strconv.ParseBool(envVal)
			field.Value.SetBool(boolVal)
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
	var allErrs []error

	flags := flag.NewFlagSet(opts.ProgramName, opts.ErrorHandling)

	// Temporary flag map of pointers to values
	flagValues := map[string]any{}

	// Load the flagValues map with the flag values
	for path, field := range fields {
		flagName := toKebabCase(field.Path)
		shortFlagName := field.Tag.Get(shortTag)

		// Overwrite with tag
		if tagVal, ok := field.Tag.Lookup(flagTag); ok {
			flagName = tagVal
		}

		switch field.Kind {
		case reflect.String:
			flagValues[path] = flags.String(flagName, "", field.Description)
			if shortFlagName != "" {
				flagValues[path+"-short"] = flags.String(shortFlagName, "", field.Description)
			}
		case reflect.Int:
			flagValues[path] = flags.Int(flagName, 0, field.Description)
			if shortFlagName != "" {
				flagValues[path+"-short"] = flags.Int(shortFlagName, 0, field.Description)
			}
		case reflect.Bool:
			flagValues[path] = flags.Bool(flagName, false, field.Description)
			if shortFlagName != "" {
				flagValues[path+"-short"] = flags.Bool(shortFlagName, false, field.Description)
			}
		}

	}

	// Parse flags
	if err := flags.Parse(opts.Args); err != nil {
		return fmt.Errorf("failed parsing flags: %w", err)
	}

	// Now set the values to the fields
	for path, flagValPtr := range flagValues {
		// Skip the default
		flagVal := reflect.ValueOf(flagValPtr).Elem()
		if flagVal.IsZero() {
			continue
		}

		// Make short use same field
		path = strings.TrimSuffix(path, "-short")

		field := fields[path]
		field.Value.Set(flagVal)
	}

	if len(allErrs) > 0 {
		return &MultiError{allErrs}
	}
	return nil
}

func validateRequired(fields map[string]configField) error {
	var allErrs []error

	for path, field := range fields {
		// Get required tag
		reqVal, exists := field.Tag.Lookup("required")

		// Skip if not required
		notRequired := !exists || (exists && reqVal == "false")
		if notRequired {
			continue
		}

		// If it is required and zero value add error
		if field.Value.IsZero() {
			allErrs = append(allErrs, fmt.Errorf("%s is required", path))
		}
	}

	if len(allErrs) > 0 {
		return &MultiError{allErrs}
	}
	return nil
}

func handleError(errHandling flag.ErrorHandling, err error) error {
	if errHandling == flag.ExitOnError {
		log.Fatal(err)
	}
	if errHandling == flag.PanicOnError {
		panic(err)
	}

	return err
}
