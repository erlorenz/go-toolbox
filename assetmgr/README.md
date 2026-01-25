# assetmgr

A static asset manager for Go web applications following the "no build" philosophy.

Inspired by [Rails Propshaft](https://github.com/rails/propshaft), this package provides content-based versioning and cache busting without bundling or transpiling. Perfect for use with htmx, Templ, and Deno-style ES modules.

## Installation

```bash
go get github.com/erlorenz/go-toolbox/assetmgr
```

## Quick Start

```go
package main

import (
    "embed"
    "net/http"

    "github.com/erlorenz/go-toolbox/assetmgr"
)

//go:embed static
var staticFS embed.FS

func main() {
    mgr, err := assetmgr.New(
        assetmgr.WithFS("/static", staticFS),
    )
    if err != nil {
        panic(err)
    }

    // Serve assets
    http.Handle("/static/", mgr)

    // Get versioned paths in templates
    asset := mgr.Get("/static/js/app.js")
    // asset.VersionedPath = "/static/js/app.js?v=abc123"
    // asset.ScriptTag = `<script type="module" src="/static/js/app.js?v=abc123"></script>`
}
```

## Design Philosophy

This package is designed for the **"no build" philosophy**:

- **No bundling** - Serve files directly as-is
- **No transpiling** - Use native ES modules
- **Query string versioning** - Files stay unchanged, `?v=hash` handles cache busting
- **Startup processing** - Walk files once at boot, pre-compute everything
- **Deno-compatible** - Works with import maps for ES module resolution

### Why Query String Versioning?

Unlike Rails Propshaft which renames files (`app-abc123.css`), we use query strings (`app.css?v=abc123`):

1. **Automatic rewriting** - CSS `url()` and JS `import` statements are rewritten to versioned paths
2. **Same cache busting** - Browsers treat different query strings as different resources
3. **Simpler** - The original file structure is preserved

### CSS/JS Compilation

Like Propshaft, we automatically rewrite relative references in CSS and JS files:

**CSS:**
```css
/* Input */
@import "./reset.css";
body { background: url("../images/bg.png"); }

/* Output (in production) */
@import "/static/css/reset.css?v=abc123";
body { background: url("/static/images/bg.png?v=def456"); }
```

**JS:**
```js
// Input
import { foo } from "./utils.js";
export * from "./module.js";

// Output (in production)
import { foo } from "/static/js/utils.js?v=abc123";
export * from "/static/js/module.js?v=def456";
```

**What gets rewritten:**
- CSS: `url()`, `@import`
- JS: `import`, `export from`, dynamic `import()`

**What is left unchanged:**
- Remote URLs (`https://...`, `//...`)
- Data URIs (`data:...`)
- Bare specifiers (`import "lodash"`) - handled by import map
- References to unknown files

## Features

### Multiple Filesystem Sources

Combine assets from multiple locations with different prefixes:

```go
//go:embed app/static
var appAssets embed.FS

//go:embed vendor/static
var vendorAssets embed.FS

mgr, _ := assetmgr.New(
    assetmgr.WithFS("/static", appAssets),
    assetmgr.WithFS("/vendor", vendorAssets),
)

// Assets available at:
// /static/js/app.js
// /vendor/htmx.min.js
```

### Development Mode

In development, use `os.DirFS` for live reloading:

```go
// Production: embedded + compiled
//go:embed static
var staticFS embed.FS

// Development: live filesystem, no compilation
devFS := os.DirFS("./static")

mgr, _ := assetmgr.New(
    assetmgr.WithFS("/static", devFS),
    assetmgr.WithDevMode(true),
)
```

**Dev mode behavior:**
- No CSS/JS compilation (files served as-is for hot reload)
- No caching headers
- Files re-read on each request
- Import maps regenerated on each request

Dev mode is automatically enabled when `APP_ENV != "production"`. Customize with `WithEnvVar()`:

```go
assetmgr.WithEnvVar("GO_ENV")  // Check GO_ENV instead of APP_ENV
```

Explicit `WithDevMode()` always wins over the environment variable.

### Import Map Support

For Deno-style ES modules, use import maps with automatic path rewriting:

**importmap.json or deno.json:**
```json
{
    "imports": {
        "app": "/static/js/app.js",
        "lodash": "https://cdn.skypack.dev/lodash"
    }
}
```

**Go code:**
```go
mgr, _ := assetmgr.New(
    assetmgr.WithFS("/static", staticFS),
    assetmgr.WithImportMap("/static/importmap.json"),
)

// In your template:
// mgr.ImportMapTag() outputs:
// <script type="importmap">
// {
//   "imports": {
//     "app": "/static/js/app.js?v=abc123",
//     "lodash": "https://cdn.skypack.dev/lodash"
//   }
// }
// </script>
```

Local paths are automatically rewritten to include version hashes. Remote URLs are preserved.

### Multiple Import Maps (Monorepo)

Multiple import maps can be merged, with later entries overwriting earlier ones:

```go
mgr, _ := assetmgr.New(
    assetmgr.WithFS("/static", staticFS),
    assetmgr.WithFS("/app", appFS),
    assetmgr.WithImportMap("/static/deno.json"),  // Base imports
    assetmgr.WithImportMap("/app/deno.json"),     // App-specific (wins on conflict)
)
```

Both `importmap.json` and `deno.json` formats are supported (same structure - `imports` and `scopes` at the root level).

### Pre-Rendered Tags

Script and link tags are pre-rendered at startup for zero runtime overhead:

```go
asset := mgr.Get("/static/js/app.js")
asset.ScriptTag  // <script type="module" src="/static/js/app.js?v=abc123"></script>

asset = mgr.Get("/static/css/style.css")
asset.LinkTag    // <link rel="stylesheet" href="/static/css/style.css?v=def456">
```

### Batch Tag Generation

Generate all tags for a path prefix:

```go
// All JS scripts under /static/js/
mgr.ScriptTags("/static/js/")
// <script type="module" src="/static/js/app.js?v=abc123"></script>
// <script type="module" src="/static/js/utils.js?v=def456"></script>

// All CSS under /static/css/
mgr.LinkTags("/static/css/")
// <link rel="stylesheet" href="/static/css/main.css?v=ghi789">
// <link rel="stylesheet" href="/static/css/theme.css?v=jkl012">
```

### HTTP Handler with Caching

The Manager implements `http.Handler` with intelligent caching:

**Versioned requests** (`?v=abc123`):
```
Cache-Control: public, max-age=31536000, immutable
```

**Non-versioned requests**:
```
Cache-Control: no-cache
ETag: "abc123"
```

```go
http.Handle("/static/", mgr)
http.Handle("/vendor/", mgr)
```

## API Reference

### Types

```go
// Asset represents a static asset with metadata
type Asset struct {
    Path          string  // "/static/js/app.js"
    VersionedPath string  // "/static/js/app.js?v=abc123"
    Hash          string  // "abc123" (FNV-1a hex)
    ContentType   string  // "text/javascript; charset=utf-8"
    ScriptTag     string  // Pre-rendered <script> tag (JS only)
    LinkTag       string  // Pre-rendered <link> tag (CSS only)
    Size          int64   // File size in bytes
}
```

### Options

| Option | Description |
|--------|-------------|
| `WithFS(prefix, fs.FS)` | Add a filesystem with URL prefix |
| `WithImportMap(path)` | Load import map from asset path |
| `WithDevMode(bool)` | Enable/disable dev mode explicitly |
| `WithEnvVar(name)` | Environment variable for dev mode detection |

### Methods

| Method | Description |
|--------|-------------|
| `Get(path) *Asset` | Get asset by path (nil if not found) |
| `MustGet(path) *Asset` | Get asset by path (panics if not found) |
| `All() []*Asset` | Get all assets sorted by path |
| `ByExtension(ext) []*Asset` | Get assets by extension (e.g., ".js") |
| `ByPrefix(prefix) []*Asset` | Get assets under path prefix |
| `ScriptTags(prefix) string` | All `<script>` tags for JS files under prefix |
| `LinkTags(prefix) string` | All `<link>` tags for CSS files under prefix |
| `ImportMapTag() string` | `<script type="importmap">` with rewritten paths |
| `ImportMapJSON() []byte` | Raw import map JSON with rewritten paths |
| `Reload() error` | Rebuild asset map (for dev mode) |
| `ServeHTTP(w, r)` | Serve assets with caching headers |

## Use with Templ

```go
// components/head.templ
templ Head(mgr *assetmgr.Manager) {
    <head>
        @templ.Raw(mgr.ImportMapTag())
        @templ.Raw(mgr.LinkTags("/static/css/"))
    </head>
}

templ Body(mgr *assetmgr.Manager) {
    <body>
        <!-- content -->
        @templ.Raw(mgr.ScriptTags("/static/js/"))
    </body>
}
```

## Monorepo Setup

For monorepos with assets in multiple locations:

```go
//go:embed shared/static
var sharedAssets embed.FS

//go:embed apps/web/static
var appAssets embed.FS

mgr, _ := assetmgr.New(
    assetmgr.WithFS("/shared", sharedAssets),
    assetmgr.WithFS("/static", appAssets),
)
```

## Technical Details

### Hashing

Uses FNV-1a (64-bit) for content hashing:
- Fast, non-cryptographic hash
- Good distribution for cache keys
- Same algorithm used in the `kv` package

### Hidden Files

Files starting with `.` are automatically skipped (e.g., `.gitignore`, `.DS_Store`).

### Content Types

Determined by file extension using Go's `mime.TypeByExtension()`. Falls back to `application/octet-stream`.

### Concurrency

The Manager is safe for concurrent use. Assets are accessed via read lock.

## Comparison with Other Solutions

| Feature | assetmgr | Propshaft | esbuild |
|---------|----------|-----------|---------|
| No build step | Yes | Yes | No |
| Query string versioning | Yes | No (filename) | N/A |
| Import map support | Yes | No | No |
| Go embed.FS | Yes | N/A | N/A |
| Live reload (dev) | Yes | Yes | Yes |
| Bundling | No | No | Yes |
| CSS url() rewriting | Yes | Yes | Yes |
| JS import rewriting | Yes | No | Yes |

## Sources

Design inspired by:
- [Rails Propshaft](https://github.com/rails/propshaft) - Rails 8 default asset pipeline
- [Laravel Mix Versioning](https://laravel-mix.com/docs/6.0/versioning) - Query string approach
- [go-staticfiles](https://github.com/catcombo/go-staticfiles) - Go asset hashing
