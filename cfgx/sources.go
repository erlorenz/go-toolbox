package cfgx

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const (
	dockerPath = "/run/secrets"
)

// Default ===================================================================
type defaultSource struct {
	priority int
}

func (s *defaultSource) Priority() int {
	return s.priority
}

func (s *defaultSource) Process(fields map[string]ConfigField) error {
	var allErrs []error

	for _, field := range fields {
		defVal, ok := field.Tag.Lookup(tagDefault)
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

// Env ====================================================================
type envSource struct {
	priority int
	prefix   string
}

func (s *envSource) Priority() int {
	return s.priority
}

func (s *envSource) Process(fields map[string]ConfigField) error {
	var allErrs []error

	for _, field := range fields {

		envName := toScreamingSnakeCase(field.Path)
		// Add prefix
		if s.prefix != "" {
			envName = s.prefix + "_" + envName
		}

		// Overwrite with tag
		tagVal, ok := field.Tag.Lookup(tagEnv)
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

// Flag ===================================================================
type flagSource struct {
	priority int
	opts     Options
}

func (s *flagSource) Priority() int {
	return s.priority
}
func (s *flagSource) Process(fields map[string]ConfigField) error {
	var allErrs []error

	flags := flag.NewFlagSet(s.opts.ProgramName, s.opts.ErrorHandling)

	// Temporary flag map of pointers to values
	flagValues := map[string]any{}

	// Load the flagValues map with the flag values
	for path, field := range fields {
		flagName := toKebabCase(field.Path)
		shortFlagName := field.Tag.Get(tagShort)

		// Overwrite with tag
		if tagVal, ok := field.Tag.Lookup(tagFlag); ok {
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
	if err := flags.Parse(s.opts.Args); err != nil {
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

// ====================================================================
// Docker Secrets

// DockerSecretsSource wraps a [FileContentSource].
// It reads the docker secret file at “/run/secrets/<secret_name>“.
// It defaults to snake case based on the struct path.
// Override the name with the tag "dsec".
type DockerSecretsSource struct {
	SecretsPath string
	FileContentSource
}

// Process opens an [os.Root] and calls the underlying [FileContentSource]'s
// Process method with the [os.Root.FS].
func (s *DockerSecretsSource) Process(structMap map[string]ConfigField) error {
	root, err := os.OpenRoot(s.SecretsPath)
	if err != nil {
		return fmt.Errorf("open docker path: %w", err)
	}
	defer root.Close()

	s.FileContentSource.FS = root.FS()
	return s.FileContentSource.Process(structMap)
}

// NewDockerSecretsSource sets a priority of 75, a tag of "dsec",
// and a secrets path of `/run/secrets`.
func NewDockerSecretsSource() *DockerSecretsSource {
	return &DockerSecretsSource{
		SecretsPath: dockerPath,
		FileContentSource: FileContentSource{
			PriorityLevel: 75,
			Tag:           tagDockerSecret,
			// Assign the fs.FS in the Process method so we can use os.Root.
		},
	}
}

// FileContentSource reads individual files.
// It can be used for any files that live in the same directory
// or have explicit file locations.
// Do not use this directly, use one of the
// implementations, e.g. DockerSecretsSource.
// This allows to use an [os.Root.FS] in the implementation's
// Process method.
type FileContentSource struct {
	PriorityLevel int
	Tag           string
	FS            fs.FS
}

// Priority implements [Source].
func (s *FileContentSource) Priority() int {
	return s.PriorityLevel
}

// Process implements [Source].
func (s *FileContentSource) Process(structMap map[string]ConfigField) error {
	if s.FS == nil {
		return fmt.Errorf("process SourceFileContent: fs.FS cannot be nil")
	}

	var allErrs []error

	for name, field := range structMap {

		secretName := toSnakeCase(name)

		// override name
		tagVal, ok := field.Tag.Lookup(s.Tag)
		if ok {
			secretName = tagVal
		}

		// skip if it doesn't exist
		file, err := s.FS.Open(secretName)
		if err != nil {
			continue
		}
		defer file.Close()

		b, err := io.ReadAll(file)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("cannot read file %s: %w", secretName, err))
			continue
		}
		secretVal := string(b)

		switch field.Kind {
		// String
		case reflect.String:
			field.Value.SetString(secretVal)
		// Int
		case reflect.Int:
			intVal, err := strconv.ParseInt(secretVal, 10, 64)
			if err != nil {
				allErrs = append(allErrs, fmt.Errorf("cannot set %s: %w", field.Path, err))
				break
			}
			field.Value.SetInt(intVal)
		// Bool
		case reflect.Bool:
			boolVal, _ := strconv.ParseBool(secretVal)
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
