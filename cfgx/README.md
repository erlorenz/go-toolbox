# cfgx

A simple and flexible configuration management package for Go applications. Parse configuration from environment variables, command-line flags, and Docker secrets into a struct.

## Features

- **Multiple sources**: Environment variables, command-line flags, Docker secrets, and defaults
- **Priority-based**: Higher priority sources override lower priority ones
- **Struct tags**: Simple, declarative configuration using struct tags
- **Nested structs**: Support for nested configuration with dot notation
- **Validation**: Required field validation with clear error messages
- **Type safe**: Supports `string`, `int`, and `bool` types
- **Auto-generated names**: Environment and flag names generated from field names
- **Version support**: Automatic version field population from build info

## Installation

```bash
go get github.com/erlorenz/go-toolbox/cfgx
```

## Quick Start

Define your configuration structure:

```go
type Config struct {
    Version string // Auto-populated with BuildInfo.Main.Version
    Port    int    `env:"MY_PORT" short:"p" default:"8080"`
    DBString string `flag:"dsn" desc:"Database connection string"`

    // Nested structs use dot notation
    Log struct {
        Level string `default:"info"` // env=LOG_LEVEL flag=log-level
    }
}
```

Load configuration:

```go
import "github.com/erlorenz/go-toolbox/cfgx"

func main() {
    var cfg Config

    if err := cfgx.Parse(&cfg, cfgx.Options{}); err != nil {
        log.Fatalf("Configuration error: %v", err)
    }

    log.Printf("Starting server on port %d", cfg.Port)
}
```

## Configuration Sources

Sources are processed in priority order (highest to lowest):

1. **Command-line flags** (priority: 100)
2. **Docker secrets** (priority: 75)
3. **Environment variables** (priority: 50)
4. **Default struct tags** (priority: 0)

### Environment Variables

```go
type Config struct {
    Port int `env:"MY_PORT" default:"8080"`
}
```

Uses `EnvPrefix` option or field name converted to SCREAMING_SNAKE_CASE:

```bash
# Explicit env name
MY_PORT=9000 ./app

# Auto-generated from field name
PORT=9000 ./app

# With prefix
MY_APP_PORT=9000 ./app  # Options{EnvPrefix: "MY_APP"}
```

### Command-line Flags

```go
type Config struct {
    Port int `flag:"port" short:"p" desc:"Server port"`
}
```

```bash
./app --port=9000
./app -p 9000
```

Auto-generated flag names use kebab-case:

```go
type Config struct {
    DatabaseURL string  // Becomes --database-url
}
```

### Docker Secrets

```go
type Config struct {
    APIKey string `dsec:"api_key"`
}
```

Reads from `/run/secrets/api_key` (or custom path via `os.Root`).

### Default Values

```go
type Config struct {
    LogLevel string `default:"info"`
    Port     int    `default:"8080"`
    Debug    bool   `default:"false"`
}
```

## Struct Tags

| Tag | Description | Example |
|-----|-------------|---------|
| `env:"NAME"` | Override environment variable name | `env:"MY_PORT"` |
| `flag:"name"` | Override flag name | `flag:"port"` |
| `short:"x"` | Short flag alias | `short:"p"` |
| `default:"value"` | Default value | `default:"8080"` |
| `desc:"text"` | Help text description | `desc:"Server port"` |
| `optional:"true"` | Mark field as optional | `optional:"true"` |
| `dsec:"filename"` | Docker secret filename | `dsec:"api_key"` |

## Version Management

The special `Version` field is automatically populated:

```go
type Config struct {
    Version string  // Auto-populated
}
```

### Development

During development with `go run` or `go build`, cfgx uses build info:
- Returns version tag if available (e.g., `v1.2.3`)
- Returns `"(devel)"` for development builds

### Production

Inject version using `-ldflags`:

```bash
VERSION=$(git describe --tags --always --dirty)
go build -ldflags="-X main.Version=$VERSION" -o myapp
```

In your code:

```go
var Version string  // Set via ldflags

func main() {
    cfg := Config{
        Version: Version,  // Highest priority
    }
    cfgx.Parse(&cfg, cfgx.Options{})
}
```

Docker example:

```dockerfile
ARG VERSION=dev
RUN go build -ldflags="-X main.Version=${VERSION}" -o /app
```

## Options

```go
type Options struct {
    EnvPrefix string        // Prefix for environment variables
    SkipEnv   bool          // Skip environment variable parsing
    SkipFlags bool          // Skip command-line flag parsing
    Sources   []Source      // Custom configuration sources
}
```

### Custom Sources

Implement the `Source` interface to add custom configuration sources:

```go
type Source interface {
    Priority() int
    Process(map[string]ConfigField) error
}
```

Example:

```go
type ConsulSource struct{}

func (s *ConsulSource) Priority() int {
    return 60  // Between env vars and docker secrets
}

func (s *ConsulSource) Process(fields map[string]ConfigField) error {
    // Fetch from Consul and set field values
    for path, field := range fields {
        if val, ok := consulGet(path); ok {
            field.Value.SetString(val)
        }
    }
    return nil
}

// Use it
cfgx.Parse(&cfg, cfgx.Options{
    Sources: []cfgx.Source{&ConsulSource{}},
})
```

## Examples

### Web Server Configuration

```go
type Config struct {
    Version string

    Server struct {
        Host string `default:"0.0.0.0"`
        Port int    `env:"PORT" default:"8080"`
    }

    Database struct {
        URL      string `env:"DATABASE_URL"`
        MaxConns int    `default:"10"`
    }

    Log struct {
        Level  string `default:"info"`
        Format string `default:"json"`
    }
}
```

Run with:

```bash
PORT=3000 DATABASE_URL=postgres://... ./app --log-level=debug
```

### Required Fields

```go
type Config struct {
    APIKey string `env:"API_KEY"` // Required by default
    Port   int    `default:"8080" optional:"true"` // Optional
}
```

## Error Handling

cfgx returns a `MultiError` containing all validation errors:

```go
if err := cfgx.Parse(&cfg, cfgx.Options{}); err != nil {
    if multiErr, ok := err.(*cfgx.MultiError); ok {
        for _, e := range multiErr.Errors {
            log.Printf("Config error: %v", e)
        }
    }
    os.Exit(1)
}
```

## Testing

For testing, skip sources you don't need:

```go
func TestConfig(t *testing.T) {
    cfg := Config{Port: 9000}

    err := cfgx.Parse(&cfg, cfgx.Options{
        SkipEnv:   true,  // Don't read environment
        SkipFlags: true,  // Don't parse flags
    })

    if err != nil {
        t.Fatal(err)
    }
}
```

## License

MIT
