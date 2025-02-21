# go-toolbox

A collection of packages for building web servers in Go. It provides common functionality needed when setting up the plumbing for a project.

## Packages

### config

The config package provides a simple and flexible way to handle application configuration through environment variables and command-line flags. It uses struct tags to parse into a configuration struct. Heavily inspired by [github.com/ardanlabs/conf](https://pkg.go.dev/github.com/ardanlabs/conf/v3).

#### Key Features

- Unified handling of environment variables and command-line flags
- Automatic parsing into configuration structs based on naming convention
- Validation of required configuration values
- Clear error messages for missing or invalid configuration
- Support for default values

#### Usage

First, define your configuration structure with field tags:

```go
type Config struct {
    Version string // populated with BuildInfo.Main.Version
    Port     int    `env:"MY_PORT" short:"p" default:"8080"` // also reads flag -p
    DBString string `flag:"dsn" required:"true"` // errors if empty
    // Nested structs are prefixed
    Log struct {
        Level string // env=LOG_LEVEL flag=log-level
    }
}
```

Then load your configuration:

```go
func main() {
    var cfg Config
    opts := config.Options{
        UseBuildInfo: true,
    }
    if err := config.Parse(&cfg, opts); err != nil {
        log.Fatalf("Configuration error: %v", err)
    }

    // Configuration is now ready to use
    log.Printf("Starting server on port %d", cfg.Port)
}
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
