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
	"log/slog"
	"maps"
	"os"
	"reflect"
	"runtime/debug"
	"slices"
	"strings"
)

const (
	tagEnv         = "env"
	tagFlag        = "flag"
	tagDefault     = "default"
	tagDescription = "desc"     // Description for help messages
	tagOptional    = "optional" // Mark field as optional
	tagShort       = "short"    // Short flag in addition

	tagDockerSecret = "dsec" // Optional
)

var (
	ErrNotPointerToStruct = errors.New("config must be a pointer to a struct")
)

// Source processes the configField map and applies values to the
// config struct. Choose a priority to process before or after other sources.
type Source interface {
	Priority() int
	Process(map[string]ConfigField) error
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
	// Sources adds additional sources.
	Sources []Source
}

// Parse populates the config struct from different sources.
// It follows this priority order (highest to lowest):
//
// Command line arguments - 100,
// Environment variables - 50,
// Default values from struct tags - 0
//
// To add a source in order, choose a priority in between the
// included sources.
// Add a top level field named Version to read the build info
// into it (as of 1.24 it uses the git tag).
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

	var sources []Source

	// Set default tags source
	sources = append(sources, &defaultSource{priority: 0})

	// Set environment variables source
	if !opts.SkipEnv {
		sources = append(sources, &envSource{
			priority: 50,
			prefix:   options.EnvPrefix,
		})
	}

	//  Set command flags source
	if !opts.SkipFlags {
		sources = append(sources, &flagSource{
			priority: 100,
			opts:     options,
		})
	}

	// Set the optional additional sources
	if len(opts.Sources) > 0 {
		sources = append(sources, opts.Sources...)
	}

	// Set Version if exists in the structMap. Will be overridden
	// if it exists in other sources.
	if version, ok := structMap["Version"]; ok {
		bi, _ := debug.ReadBuildInfo()

		version.Value.SetString(cmp.Or(bi.Main.Version, "(develop)"))
	}

	// Sort and call Process on each source
	slices.SortFunc(sources, func(a, b Source) int {
		return cmp.Compare(a.Priority(), b.Priority())
	})

	for _, source := range sources {
		source.Process(structMap)
	}

	// Validate the required
	if err := validateRequired(structMap); err != nil {
		return handleError(opts.ErrorHandling, fmt.Errorf("validation: %w", err))
	}

	return nil
}

// ConfigField represents a field in the config struct.
type ConfigField struct {
	Path        string
	Value       reflect.Value
	Kind        reflect.Kind
	Name        string
	StructField reflect.StructField
	Tag         reflect.StructTag
	Description string
}

// Gather map of ConfigFields
func walkStruct(v reflect.Value, currPath string) map[string]ConfigField {
	fields := map[string]ConfigField{}

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
		desc := cmp.Or(tag.Get(tagDescription), path)

		fields[path] = ConfigField{
			Path: path, Value: fieldVal, Kind: kind, Name: name, StructField: structField, Tag: tag, Description: desc}
	}
	return fields
}

// Error if required fields are missing
func validateRequired(fields map[string]ConfigField) error {
	var allErrs []error

	for path, field := range fields {
		// Get optional tag
		reqVal, exists := field.Tag.Lookup(tagOptional)

		// Skip if optional
		optional := exists && reqVal != "false"
		if optional {
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

// Handle the errors depending on the strategy
func handleError(errHandling flag.ErrorHandling, err error) error {
	if errHandling == flag.ExitOnError {
		slog.Error("Error parsing config struct.", "error", err)
		os.Exit(1)
	}
	if errHandling == flag.PanicOnError {
		panic(err)
	}

	return err
}
