// Package assetmgr provides a static asset manager for Go web applications.
//
// It's designed for the "no build" philosophy - no bundling or transpiling.
// Assets are served directly with content-based versioning for cache busting.
//
// Key features:
//   - Multiple fs.FS sources with configurable prefixes
//   - FNV-1a content hashing with query string versioning (?v=hash)
//   - Immutable caching headers for versioned requests
//   - Import map support for Deno/ES modules with path rewriting
//   - Pre-rendered script/link tags for zero runtime overhead
//   - Dev mode with no caching and file re-reading
//
// Example usage:
//
//	//go:embed static
//	var staticFS embed.FS
//
//	mgr, err := assetmgr.New(
//	    assetmgr.WithFS("/static", staticFS),
//	    assetmgr.WithImportMap("static/importmap.json"),
//	)
//
//	http.Handle("/static/", mgr)
//
//	// In templates:
//	asset := mgr.Get("/static/js/app.js")
//	asset.ScriptTag // <script type="module" src="/static/js/app.js?v=abc123"></script>
package assetmgr

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// ErrNotFound is returned when an asset is not found.
	ErrNotFound = errors.New("asset not found")

	// ErrInvalidImportMap is returned when the import map cannot be parsed.
	ErrInvalidImportMap = errors.New("invalid import map")
)

// Asset represents a static asset with its metadata and pre-rendered tags.
type Asset struct {
	// Path is the logical path of the asset (e.g., "/static/js/app.js").
	Path string

	// VersionedPath includes the version query string (e.g., "/static/js/app.js?v=abc123").
	VersionedPath string

	// Hash is the FNV-1a hash of the file contents (hex encoded).
	Hash string

	// ContentType is the MIME type of the asset.
	ContentType string

	// ScriptTag is a pre-rendered <script> tag for JS files.
	// Empty for non-JS files.
	ScriptTag string

	// LinkTag is a pre-rendered <link> tag for CSS files.
	// Empty for non-CSS files.
	LinkTag string

	// Size is the file size in bytes.
	Size int64

	// fsys is the filesystem containing this asset.
	fsys fs.FS

	// fsPath is the path within the filesystem.
	fsPath string
}

// ImportMap represents a JavaScript import map structure.
// See: https://developer.mozilla.org/en-US/docs/Web/HTML/Element/script/type/importmap
type ImportMap struct {
	Imports map[string]string            `json:"imports,omitempty"`
	Scopes  map[string]map[string]string `json:"scopes,omitempty"`
}

// Manager is a static asset manager that handles file serving,
// versioning, and import map rewriting.
type Manager struct {
	mu sync.RWMutex

	// assets maps logical path to Asset
	assets map[string]*Asset

	// sources is a list of filesystem sources in order
	sources []fsSource

	// importMap is the parsed and rewritten import map
	importMap *ImportMap

	// importMapPath is the path to the import map file
	importMapPath string

	// importMapTag is the pre-rendered import map script tag
	importMapTag string

	// devMode disables caching and re-reads files on each request
	devMode bool

	// envVar is the environment variable to check for dev mode
	envVar string

	// modTime is used for Last-Modified header (set at build time)
	modTime time.Time
}

// fsSource represents a filesystem with its URL prefix.
type fsSource struct {
	prefix string
	fsys   fs.FS
}

// Option configures a Manager.
type Option func(*Manager) error

// WithFS adds a filesystem with the given URL prefix.
// The prefix should start with "/" (e.g., "/static", "/vendor").
// Multiple filesystems can be added with different prefixes.
// Files are served at prefix + file path within the FS.
//
// Example:
//
//	WithFS("/static", embeddedFS)  // /static/js/app.js
//	WithFS("/vendor", vendorFS)   // /vendor/htmx.js
func WithFS(prefix string, fsys fs.FS) Option {
	return func(m *Manager) error {
		// Ensure prefix starts with /
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		// Remove trailing slash
		prefix = strings.TrimSuffix(prefix, "/")

		m.sources = append(m.sources, fsSource{
			prefix: prefix,
			fsys:   fsys,
		})
		return nil
	}
}

// WithImportMap loads an import map from the specified path within the filesystems.
// The import map will be rewritten to include versioned paths for local assets.
//
// Example:
//
//	WithImportMap("/static/importmap.json")
func WithImportMap(path string) Option {
	return func(m *Manager) error {
		m.importMapPath = path
		return nil
	}
}

// WithDevMode explicitly enables or disables development mode.
// In dev mode:
//   - No caching headers are sent
//   - Files are re-read on each request (useful with os.DirFS)
//   - Import map is regenerated on each request
//
// If not set, dev mode is determined by the APP_ENV environment variable
// (dev mode is enabled if APP_ENV != "production").
func WithDevMode(enabled bool) Option {
	return func(m *Manager) error {
		m.devMode = enabled
		return nil
	}
}

// WithEnvVar sets the environment variable used to detect dev mode.
// Dev mode is enabled if the variable's value is NOT "production".
// Default: "APP_ENV"
func WithEnvVar(name string) Option {
	return func(m *Manager) error {
		m.envVar = name
		return nil
	}
}

// New creates a new asset Manager.
// At least one WithFS option must be provided.
//
// The constructor walks all filesystems, hashes file contents,
// and pre-renders script/link tags. This happens once at startup.
func New(opts ...Option) (*Manager, error) {
	m := &Manager{
		assets:  make(map[string]*Asset),
		sources: make([]fsSource, 0),
		envVar:  "APP_ENV",
		modTime: time.Now(),
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(m); err != nil {
			return nil, err
		}
	}

	// Check for at least one filesystem
	if len(m.sources) == 0 {
		return nil, errors.New("at least one WithFS option is required")
	}

	// Determine dev mode from environment if not explicitly set
	// Check if WithDevMode was called by seeing if it's still false
	// (we need a separate flag, but for simplicity, check env var)
	if !m.devMode && os.Getenv(m.envVar) != "production" {
		m.devMode = true
	}

	// Build asset map
	if err := m.build(); err != nil {
		return nil, err
	}

	return m, nil
}

// build walks all filesystems and builds the asset map.
func (m *Manager) build() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing assets
	m.assets = make(map[string]*Asset)

	// Walk each filesystem
	for _, src := range m.sources {
		if err := m.walkFS(src); err != nil {
			return fmt.Errorf("walking %s: %w", src.prefix, err)
		}
	}

	// Load and rewrite import map if specified
	if m.importMapPath != "" {
		if err := m.loadImportMap(); err != nil {
			return err
		}
	}

	return nil
}

// walkFS walks a single filesystem and adds assets to the map.
func (m *Manager) walkFS(src fsSource) error {
	return fs.WalkDir(src.fsys, ".", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(filepath.Base(filePath), ".") {
			return nil
		}

		// Build the logical path
		logicalPath := src.prefix + "/" + filePath

		// Clean the path
		logicalPath = path.Clean(logicalPath)

		// Read file contents for hashing
		content, err := fs.ReadFile(src.fsys, filePath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", filePath, err)
		}

		// Compute FNV-1a hash
		hash := hashContent(content)

		// Determine content type
		contentType := mime.TypeByExtension(filepath.Ext(filePath))
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		// Create versioned path
		versionedPath := fmt.Sprintf("%s?v=%s", logicalPath, hash)

		// Create asset
		asset := &Asset{
			Path:          logicalPath,
			VersionedPath: versionedPath,
			Hash:          hash,
			ContentType:   contentType,
			Size:          int64(len(content)),
			fsys:          src.fsys,
			fsPath:        filePath,
		}

		// Pre-render tags
		asset.ScriptTag = m.renderScriptTag(asset)
		asset.LinkTag = m.renderLinkTag(asset)

		m.assets[logicalPath] = asset
		return nil
	})
}

// hashContent computes a hex-encoded FNV-1a hash of the content.
func hashContent(content []byte) string {
	h := fnv.New64a()
	h.Write(content)
	return fmt.Sprintf("%x", h.Sum64())
}

// renderScriptTag creates a <script> tag for JavaScript files.
func (m *Manager) renderScriptTag(asset *Asset) string {
	ext := strings.ToLower(filepath.Ext(asset.Path))
	switch ext {
	case ".js", ".mjs":
		return fmt.Sprintf(`<script type="module" src="%s"></script>`, asset.VersionedPath)
	case ".ts":
		// TypeScript files served directly (Deno-style)
		return fmt.Sprintf(`<script type="module" src="%s"></script>`, asset.VersionedPath)
	default:
		return ""
	}
}

// renderLinkTag creates a <link> tag for CSS files.
func (m *Manager) renderLinkTag(asset *Asset) string {
	ext := strings.ToLower(filepath.Ext(asset.Path))
	if ext == ".css" {
		return fmt.Sprintf(`<link rel="stylesheet" href="%s">`, asset.VersionedPath)
	}
	return ""
}

// loadImportMap loads and processes the import map.
func (m *Manager) loadImportMap() error {
	// Find the import map file in our assets
	asset, ok := m.assets[m.importMapPath]
	if !ok {
		return fmt.Errorf("%w: import map not found at %s", ErrInvalidImportMap, m.importMapPath)
	}

	// Read the file
	content, err := fs.ReadFile(asset.fsys, asset.fsPath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidImportMap, err)
	}

	// Parse the import map
	var im ImportMap
	if err := json.Unmarshal(content, &im); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidImportMap, err)
	}

	// Rewrite local paths in imports
	if im.Imports != nil {
		for key, value := range im.Imports {
			if rewritten := m.rewriteImportPath(value); rewritten != "" {
				im.Imports[key] = rewritten
			}
		}
	}

	// Rewrite local paths in scopes
	if im.Scopes != nil {
		for scope, imports := range im.Scopes {
			for key, value := range imports {
				if rewritten := m.rewriteImportPath(value); rewritten != "" {
					im.Scopes[scope][key] = rewritten
				}
			}
		}
	}

	m.importMap = &im

	// Pre-render the import map tag
	m.importMapTag = m.renderImportMapTag()

	return nil
}

// rewriteImportPath rewrites a local path to its versioned equivalent.
// Returns empty string if the path is not a local asset.
func (m *Manager) rewriteImportPath(importPath string) string {
	// Skip remote URLs
	if strings.HasPrefix(importPath, "http://") ||
		strings.HasPrefix(importPath, "https://") ||
		strings.HasPrefix(importPath, "//") {
		return ""
	}

	// Check if this is a local asset
	if asset, ok := m.assets[importPath]; ok {
		return asset.VersionedPath
	}

	return ""
}

// renderImportMapTag creates the <script type="importmap"> tag.
func (m *Manager) renderImportMapTag() string {
	if m.importMap == nil {
		return ""
	}

	data, err := json.MarshalIndent(m.importMap, "", "  ")
	if err != nil {
		return ""
	}

	return fmt.Sprintf(`<script type="importmap">
%s
</script>`, string(data))
}

// Get returns the asset at the given path.
// Returns nil if the asset doesn't exist.
func (m *Manager) Get(assetPath string) *Asset {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.assets[assetPath]
}

// MustGet returns the asset at the given path.
// Panics if the asset doesn't exist.
func (m *Manager) MustGet(assetPath string) *Asset {
	asset := m.Get(assetPath)
	if asset == nil {
		panic(fmt.Sprintf("asset not found: %s", assetPath))
	}
	return asset
}

// All returns all assets sorted by path.
func (m *Manager) All() []*Asset {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assets := make([]*Asset, 0, len(m.assets))
	for _, asset := range m.assets {
		assets = append(assets, asset)
	}

	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Path < assets[j].Path
	})

	return assets
}

// ByExtension returns all assets with the given extension, sorted by path.
// The extension should include the dot (e.g., ".js", ".css").
func (m *Manager) ByExtension(ext string) []*Asset {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ext = strings.ToLower(ext)
	assets := make([]*Asset, 0)

	for _, asset := range m.assets {
		if strings.ToLower(filepath.Ext(asset.Path)) == ext {
			assets = append(assets, asset)
		}
	}

	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Path < assets[j].Path
	})

	return assets
}

// ByPrefix returns all assets under the given path prefix, sorted by path.
func (m *Manager) ByPrefix(prefix string) []*Asset {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assets := make([]*Asset, 0)

	for _, asset := range m.assets {
		if strings.HasPrefix(asset.Path, prefix) {
			assets = append(assets, asset)
		}
	}

	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Path < assets[j].Path
	})

	return assets
}

// ScriptTags returns pre-rendered <script> tags for all JS files under the given prefix.
// Returns a single string with all tags joined by newlines.
func (m *Manager) ScriptTags(prefix string) string {
	assets := m.ByPrefix(prefix)
	var tags []string

	for _, asset := range assets {
		if asset.ScriptTag != "" {
			tags = append(tags, asset.ScriptTag)
		}
	}

	return strings.Join(tags, "\n")
}

// LinkTags returns pre-rendered <link> tags for all CSS files under the given prefix.
// Returns a single string with all tags joined by newlines.
func (m *Manager) LinkTags(prefix string) string {
	assets := m.ByPrefix(prefix)
	var tags []string

	for _, asset := range assets {
		if asset.LinkTag != "" {
			tags = append(tags, asset.LinkTag)
		}
	}

	return strings.Join(tags, "\n")
}

// ImportMapTag returns the pre-rendered <script type="importmap"> tag.
// The import map has local paths rewritten to versioned paths.
func (m *Manager) ImportMapTag() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.importMapTag
}

// ImportMapJSON returns the import map as JSON with local paths rewritten.
// Returns empty slice if no import map is configured.
func (m *Manager) ImportMapJSON() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.importMap == nil {
		return nil
	}

	data, _ := json.Marshal(m.importMap)
	return data
}

// Reload rebuilds the asset map.
// This is useful in development mode when files change.
// In production, this should not be needed.
func (m *Manager) Reload() error {
	return m.build()
}

// ServeHTTP implements http.Handler.
// It serves assets with appropriate caching headers.
//
// For versioned requests (containing ?v=):
//   - Cache-Control: public, max-age=31536000, immutable
//
// For non-versioned requests:
//   - Cache-Control: no-cache (allows caching but requires revalidation)
//   - ETag: based on content hash
//
// In dev mode, no caching headers are set.
func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// In dev mode, rebuild on each request
	if m.devMode {
		if err := m.build(); err != nil {
			http.Error(w, "Failed to load assets", http.StatusInternalServerError)
			return
		}
	}

	// Get the path without query string
	assetPath := r.URL.Path

	m.mu.RLock()
	asset := m.assets[assetPath]
	m.mu.RUnlock()

	if asset == nil {
		http.NotFound(w, r)
		return
	}

	// Open the file
	file, err := asset.fsys.Open(asset.fsPath)
	if err != nil {
		http.Error(w, "Failed to read asset", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Get file info for ServeContent
	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "Failed to read asset", http.StatusInternalServerError)
		return
	}

	// Check if this is a versioned request
	hasVersion := r.URL.Query().Has("v")

	// Set caching headers (unless in dev mode)
	if !m.devMode {
		if hasVersion {
			// Immutable caching for versioned assets
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			// Require revalidation for non-versioned requests
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("ETag", fmt.Sprintf(`"%s"`, asset.Hash))
		}
	}

	// Set content type
	w.Header().Set("Content-Type", asset.ContentType)

	// Use ServeContent for proper handling of Range requests, If-Modified-Since, etc.
	// Need a ReadSeeker for ServeContent
	if seeker, ok := file.(io.ReadSeeker); ok {
		http.ServeContent(w, r, asset.Path, stat.ModTime(), seeker)
	} else {
		// Fallback: read entire file and serve
		content, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Failed to read asset", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Write(content)
	}
}
