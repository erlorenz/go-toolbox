# go-toolbox

A collection of Go packages for building web servers. Currently contains utilities for configuration management.

## Installation

```bash
go get github.com/erlorenz/go-toolbox/cfgx
```

## Packages

### cfgx

The cfgx package provides a simple and flexible way to handle application configuration through environment variables and command-line flags. It uses struct tags to parse into a configuration struct. Heavily inspired by [github.com/ardanlabs/conf](https://pkg.go.dev/github.com/ardanlabs/conf/v3).

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
import "github.com/erlorenz/go-toolbox/cfgx"

// Optional: Set via -ldflags at build time for production
var Version string

func main() {
    cfg := Config{
        Version: Version, // Highest priority - set via ldflags
    }

    if err := cfgx.Parse(&cfg, cfgx.Options{}); err != nil {
        log.Fatalf("Configuration error: %v", err)
    }

    // Configuration is now ready to use
    log.Printf("Version %s: Starting server on port %d", cfg.Version, cfg.Port)
}
```

#### Setting Version for Production

For production builds, inject the version using `-ldflags`:

```bash
# Local build
VERSION=$(git describe --tags --always --dirty)
go build -ldflags="-X main.Version=$VERSION" -o myapp

# Docker build
docker build --build-arg VERSION=$(git describe --tags --always) -t myapp .
```

Example Dockerfile:
```dockerfile
FROM golang:1.24 AS builder
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN go build -ldflags="-X main.Version=${VERSION}" -o /app

FROM gcr.io/distroless/base-debian12
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
```

The `Version` field will automatically use build info from `go build` during local development (returns version tag or `"(devel)"`). For production, use `-ldflags` as shown above for reliable version tracking.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
