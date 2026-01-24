# casing

A lightweight Go package for converting strings between different casing conventions.

## Features

- **Pure functions** - No state, just simple conversions
- **Handles acronyms** - Correctly processes sequences like "HTTPServer"
- **Dot notation support** - Treats dots as word boundaries (e.g., "User.Name")
- **No dependencies** - Uses only the Go standard library

## Installation

```bash
go get github.com/erlorenz/go-toolbox/casing
```

## Usage

```go
import "github.com/erlorenz/go-toolbox/casing"

// Convert to snake_case
casing.ToSnake("HTTPServer")        // "http_server"
casing.ToSnake("UserName")          // "user_name"
casing.ToSnake("User.Name")         // "user_name"
casing.ToSnake("IOError")           // "io_error"

// Convert to SCREAMING_SNAKE_CASE
casing.ToScreamingSnake("HTTPServer")  // "HTTP_SERVER"
casing.ToScreamingSnake("userName")    // "USER_NAME"

// Convert to kebab-case
casing.ToKebab("HTTPServer")        // "http-server"
casing.ToKebab("userName")          // "user-name"

// Convert to PascalCase
casing.ToPascal("http_server")      // "HttpServer"
casing.ToPascal("user-name")        // "UserName"
casing.ToPascal("user.name")        // "UserName"

// Convert to camelCase
casing.ToCamel("http_server")       // "httpServer"
casing.ToCamel("user-name")         // "userName"
casing.ToCamel("user.name")         // "userName"
```

## Function Reference

### ToSnake(s string) string

Converts a string to `snake_case`.

**Rules:**
- Uppercase letters become lowercase with `_` prefix (except at start)
- Dots (`.`) are replaced with `_`
- Handles acronyms: `HTTPServer` → `http_server`

**Examples:**
```go
ToSnake("HTTPServer")    // "http_server"
ToSnake("UserName")      // "user_name"
ToSnake("User.Name")     // "user_name"
ToSnake("IOError")       // "io_error"
ToSnake("XMLParser")     // "xml_parser"
```

### ToScreamingSnake(s string) string

Converts a string to `SCREAMING_SNAKE_CASE` (uppercase snake_case).

**Examples:**
```go
ToScreamingSnake("HTTPServer")  // "HTTP_SERVER"
ToScreamingSnake("userName")    // "USER_NAME"
```

### ToKebab(s string) string

Converts a string to `kebab-case` (like snake_case but with hyphens).

**Examples:**
```go
ToKebab("HTTPServer")    // "http-server"
ToKebab("userName")      // "user-name"
ToKebab("User.Name")     // "user-name"
```

### ToPascal(s string) string

Converts a string to `PascalCase` (first letter uppercase).

**Rules:**
- Treats `_`, `-`, `.`, and spaces as word boundaries
- Capitalizes first letter of each word
- Removes separators

**Examples:**
```go
ToPascal("http_server")       // "HttpServer"
ToPascal("user-name")         // "UserName"
ToPascal("user.name")         // "UserName"
ToPascal("hello world")       // "HelloWorld"
```

### ToCamel(s string) string

Converts a string to `camelCase` (like PascalCase but first letter lowercase).

**Examples:**
```go
ToCamel("http_server")        // "httpServer"
ToCamel("user-name")          // "userName"
ToCamel("user.name")          // "userName"
ToCamel("hello world")        // "helloWorld"
```

## Common Use Cases

### API/Database Field Mapping

```go
// Convert Go struct field names to database columns
type User struct {
    UserName string
    EmailAddress string
}

// Convert field names to snake_case for database
dbColumn := casing.ToSnake("UserName")  // "user_name"
```

### Configuration Keys

```go
// Normalize configuration keys from different sources
envKey := casing.ToScreamingSnake("databaseUrl")  // "DATABASE_URL"
configKey := casing.ToKebab("databaseUrl")        // "database-url"
```

### Code Generation

```go
// Generate Go identifiers from schema names
goIdentifier := casing.ToPascal("user_profile")  // "UserProfile"
jsonField := casing.ToCamel("user_profile")      // "userProfile"
```

## Implementation Notes

### Acronym Handling

The package intelligently handles acronyms by detecting sequences of uppercase letters:

```go
ToSnake("HTTPServer")     // "http_server" (not "h_t_t_p_server")
ToSnake("IOError")        // "io_error" (not "i_o_error")
ToSnake("XMLParser")      // "xml_parser"
```

**Algorithm:**
- Detects when transitioning from acronym to word
- Treats last letter of acronym as start of next word
- Example: "XMLParser" → "XML" + "Parser" → "xml_parser"

### Dot Notation

Dots are treated as word boundaries, useful for nested configuration:

```go
ToSnake("Database.Connection.URL")  // "database_connection_url"
ToPascal("user.profile.name")       // "UserProfileName"
```

### Edge Cases

```go
// Empty string
ToSnake("")              // ""

// Already in target format
ToSnake("already_snake") // "already_snake"

// Single character
ToSnake("A")             // "a"

// Numbers
ToSnake("User123")       // "user123"
```

## Performance

All functions are implemented with efficient string building using `strings.Builder`:
- Single pass through the input string
- No regular expressions
- Minimal allocations

Suitable for use in hot paths (request handlers, data transformations).

## License

MIT
