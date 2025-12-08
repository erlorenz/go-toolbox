# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Structure

This is a **single-module Go toolbox** repository designed to house multiple independent utility packages. Users can install individual packages via:

```bash
go get github.com/erlorenz/go-toolbox/<package-name>
```

**Current packages:**
- `cfgx` - Configuration management from multiple sources (env vars, flags, docker secrets, defaults)

## Development Commands

**Run tests:**
```bash
# Run all tests
go test -v ./...

# Run tests with race detection
go test --race -v ./...

# Run specific package tests
go test -v ./cfgx

# Run specific test
go test -v ./cfgx -run TestParse
```

**Lint:**
```bash
go vet ./...
```

**Run example:**
```bash
go run ./examples/config
```

**Note:** The Makefile references `./config` but the actual package is `./cfgx`. The Makefile may be outdated.

## Architecture: cfgx Package

The `cfgx` package provides configuration parsing from multiple sources with a priority-based system.

### Core Design

**Source-based architecture:** Configuration values come from "sources" (defaults, environment variables, command-line flags, docker secrets, etc.) processed in priority order. Higher priority sources override lower priority ones.

**Built-in source priorities:**
- Command-line flags: 100 (highest)
- Docker secrets: 75
- Environment variables: 50
- Default struct tags: 0 (lowest)

### Key Components

**Source interface** ([cfgx.go](cfgx/cfgx.go:38-43)):
```go
type Source interface {
    Priority() int
    Process(map[string]ConfigField) error
}
```

All configuration sources implement this interface. Users can add custom sources by implementing it and passing via `Options.Sources`.

**ConfigField** ([cfgx.go](cfgx/cfgx.go:140-149)): Represents a parsed struct field with metadata (path, value, tags, reflection info). The `walkStruct` function recursively walks the config struct to build a `map[string]ConfigField` using dot notation for nested paths.

**Built-in sources** ([sources.go](cfgx/sources.go)):
- `defaultSource`: Reads `default:"value"` struct tags
- `envSource`: Reads environment variables (auto-generates SCREAMING_SNAKE_CASE names from field paths, supports `env:"NAME"` tag override and `EnvPrefix` option)
- `flagSource`: Reads command-line flags (auto-generates kebab-case names, supports `flag:"name"` and `short:"x"` tags)
- `DockerSecretsSource`: Reads files from `/run/secrets/` (uses `os.Root` for security, supports `dsec:"filename"` tag)

**Struct tag system:**
- `env:"NAME"` - Override environment variable name
- `flag:"name"` - Override flag name
- `short:"x"` - Add short flag alias (e.g., `-p` for `--port`)
- `default:"value"` - Default value
- `desc:"text"` - Description for help text
- `optional:"true"` - Mark field as optional (otherwise required)
- `dsec:"filename"` - Docker secret filename

**Supported types:** `string`, `int`, `bool`. Nested structs are supported (field paths use dot notation).

### Processing Flow

1. Validate input is pointer to struct
2. Walk struct recursively to build `map[string]ConfigField`
3. Set `Version` field from build info if present
4. Sort sources by priority (low to high)
5. Call `Process()` on each source in order
6. Validate required fields (fields without `optional:"true"` must be non-zero)

### Testing Patterns

Tests use table-driven subtests with `t.Run()`. See [config_test.go](cfgx/config_test.go) for examples:
- Tests use the `_test` package suffix (`package cfgx_test`) for black-box testing
- Common pattern: define a base config struct, copy it in subtests to avoid mutation
- Use `Options.SkipFlags` and `Options.SkipEnv` to test sources in isolation
- Use `testing/fstest` for testing file-based sources

### Adding New Sources

To add a custom configuration source:
1. Implement the `Source` interface
2. Choose an appropriate priority (0-100+ scale)
3. In `Process()`, iterate over the `map[string]ConfigField` and set values using `field.Value.Set*()`
4. Return `&MultiError{errs}` if multiple errors occur
5. Pass the source via `Options.Sources` when calling `Parse()`

Example: The `DockerSecretsSource` wraps `FileContentSource` and opens `/run/secrets` using `os.Root.FS()` for security.
