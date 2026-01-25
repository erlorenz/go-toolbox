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

	// CSSTag is a pre-rendered <link rel="stylesheet"> tag for CSS files.
	// Empty for non-CSS files.
	CSSTag string

	// Size is the file size in bytes.
	Size int64

	// fsys is the filesystem containing this asset.
	fsys fs.FS

	// fsPath is the path within the filesystem.
	fsPath string

	// compiled holds the compiled content for CSS/JS files.
	// nil for non-compiled assets (images, fonts, etc.).
	compiled []byte
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

	// importMap is the merged import map from all sources
	importMap *ImportMap

	// importMapPaths is the list of import map paths to load (in order)
	importMapPaths []string

	// importMapTag is the pre-rendered import map script tag
	importMapTag string

	// devMode disables caching, compilation, and re-reads files on each request
	devMode bool

	// devModeSet tracks if WithDevMode was explicitly called
	devModeSet bool

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
// Multiple import maps can be specified by calling WithImportMap multiple times.
// Maps are merged in order, with later entries overwriting earlier ones.
// Both importmap.json and deno.json formats are supported (same structure).
//
// Example:
//
//	WithImportMap("/static/deno.json")        // Base imports
//	WithImportMap("/app/importmap.json")      // App-specific (overwrites)
func WithImportMap(path string) Option {
	return func(m *Manager) error {
		m.importMapPaths = append(m.importMapPaths, path)
		return nil
	}
}

// WithDevMode explicitly enables or disables development mode.
// In dev mode:
//   - No caching headers are sent
//   - No CSS/JS compilation (files served as-is for hot reload)
//   - Files are re-read on each request (useful with os.DirFS)
//   - Import map is regenerated on each request
//
// If not set, dev mode is determined by the APP_ENV environment variable
// (dev mode is enabled if APP_ENV != "production").
func WithDevMode(enabled bool) Option {
	return func(m *Manager) error {
		m.devMode = enabled
		m.devModeSet = true
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
// compiles CSS/JS (unless in dev mode), and pre-renders script/link tags.
// This happens once at startup.
func New(opts ...Option) (*Manager, error) {
	m := &Manager{
		assets:         make(map[string]*Asset),
		sources:        make([]fsSource, 0),
		importMapPaths: make([]string, 0),
		envVar:         "APP_ENV",
		modTime:        time.Now(),
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
	if !m.devModeSet {
		m.devMode = os.Getenv(m.envVar) != "production"
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

	// Compile CSS/JS files (skip in dev mode)
	if !m.devMode {
		m.compileAssets()
	}

	// Load and merge import maps
	if len(m.importMapPaths) > 0 {
		if err := m.loadImportMaps(); err != nil {
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
		asset.CSSTag = m.renderCSSTag(asset)

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

// compileAssets compiles CSS and JS files, rewriting relative imports.
func (m *Manager) compileAssets() {
	// Create a resolver function that looks up versioned paths
	resolve := func(logicalPath string) string {
		if asset, ok := m.assets[logicalPath]; ok {
			return asset.VersionedPath
		}
		return ""
	}

	for _, asset := range m.assets {
		ext := strings.ToLower(filepath.Ext(asset.Path))

		switch ext {
		case ".css":
			// Read and compile CSS
			content, err := fs.ReadFile(asset.fsys, asset.fsPath)
			if err != nil {
				continue
			}
			compiled := compileCSS(content, asset.Path, resolve)
			// Only store if content changed
			if string(compiled) != string(content) {
				asset.compiled = compiled
				// Update hash and versioned path based on compiled content
				asset.Hash = hashContent(compiled)
				asset.VersionedPath = fmt.Sprintf("%s?v=%s", asset.Path, asset.Hash)
				asset.CSSTag = m.renderCSSTag(asset)
			}

		case ".js", ".mjs", ".ts":
			// Read and compile JS
			content, err := fs.ReadFile(asset.fsys, asset.fsPath)
			if err != nil {
				continue
			}
			compiled := compileJS(content, asset.Path, resolve)
			// Only store if content changed
			if string(compiled) != string(content) {
				asset.compiled = compiled
				// Update hash and versioned path based on compiled content
				asset.Hash = hashContent(compiled)
				asset.VersionedPath = fmt.Sprintf("%s?v=%s", asset.Path, asset.Hash)
				asset.ScriptTag = m.renderScriptTag(asset)
			}
		}
	}
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

// renderCSSTag creates a <link rel="stylesheet"> tag for CSS files.
func (m *Manager) renderCSSTag(asset *Asset) string {
	ext := strings.ToLower(filepath.Ext(asset.Path))
	if ext == ".css" {
		return fmt.Sprintf(`<link rel="stylesheet" href="%s">`, asset.VersionedPath)
	}
	return ""
}

// loadImportMaps loads and merges all import maps.
func (m *Manager) loadImportMaps() error {
	// Initialize merged import map
	m.importMap = &ImportMap{
		Imports: make(map[string]string),
		Scopes:  make(map[string]map[string]string),
	}

	// Load and merge each import map in order
	for _, importMapPath := range m.importMapPaths {
		if err := m.loadAndMergeImportMap(importMapPath); err != nil {
			return err
		}
	}

	// Pre-render the import map tag
	m.importMapTag = m.renderImportMapTag()

	return nil
}

// loadAndMergeImportMap loads a single import map and merges it into the existing one.
func (m *Manager) loadAndMergeImportMap(importMapPath string) error {
	// Find the import map file in our assets
	asset, ok := m.assets[importMapPath]
	if !ok {
		return fmt.Errorf("%w: import map not found at %s", ErrInvalidImportMap, importMapPath)
	}

	// Read the file
	content, err := fs.ReadFile(asset.fsys, asset.fsPath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidImportMap, err)
	}

	// Parse the import map (works for both importmap.json and deno.json)
	var im ImportMap
	if err := json.Unmarshal(content, &im); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidImportMap, err)
	}

	// Merge imports (later wins)
	if im.Imports != nil {
		for key, value := range im.Imports {
			// Rewrite local paths to versioned paths
			if rewritten := m.rewriteImportPath(value); rewritten != "" {
				m.importMap.Imports[key] = rewritten
			} else {
				m.importMap.Imports[key] = value
			}
		}
	}

	// Merge scopes (later wins per scope)
	if im.Scopes != nil {
		for scope, imports := range im.Scopes {
			if m.importMap.Scopes[scope] == nil {
				m.importMap.Scopes[scope] = make(map[string]string)
			}
			for key, value := range imports {
				if rewritten := m.rewriteImportPath(value); rewritten != "" {
					m.importMap.Scopes[scope][key] = rewritten
				} else {
					m.importMap.Scopes[scope][key] = value
				}
			}
		}
	}

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

// CSSTags returns pre-rendered <link rel="stylesheet"> tags for all CSS files under the given prefix.
// Returns a single string with all tags joined by newlines.
func (m *Manager) CSSTags(prefix string) string {
	assets := m.ByPrefix(prefix)
	var tags []string

	for _, asset := range assets {
		if asset.CSSTag != "" {
			tags = append(tags, asset.CSSTag)
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

// ModulePreloadTag returns a <link rel="modulepreload"> tag for the given import map key.
// The importKey should be a key in the import map (e.g., "app", "utils").
// Returns empty string if the import map is not configured or the key doesn't exist.
//
// Example:
//
//	mgr.ModulePreloadTag("app")
//	// <link rel="modulepreload" href="/static/js/app.js?v=abc123">
func (m *Manager) ModulePreloadTag(importKey string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.importMap == nil || m.importMap.Imports == nil {
		return ""
	}

	href, ok := m.importMap.Imports[importKey]
	if !ok {
		return ""
	}

	return fmt.Sprintf(`<link rel="modulepreload" href="%s">`, href)
}

// ModulePreloadTags returns <link rel="modulepreload"> tags for multiple import map keys.
// Returns a single string with all tags joined by newlines.
// Keys that don't exist in the import map are silently skipped.
//
// Example:
//
//	mgr.ModulePreloadTags("app", "utils", "htmx")
//	// <link rel="modulepreload" href="/static/js/app.js?v=abc123">
//	// <link rel="modulepreload" href="/static/js/utils.js?v=def456">
//	// <link rel="modulepreload" href="https://cdn.example.com/htmx.js">
func (m *Manager) ModulePreloadTags(importKeys ...string) string {
	var tags []string

	for _, key := range importKeys {
		if tag := m.ModulePreloadTag(key); tag != "" {
			tags = append(tags, tag)
		}
	}

	return strings.Join(tags, "\n")
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
// In dev mode, no caching headers are set and files are re-read on each request.
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

	// If we have compiled content, serve that
	if asset.compiled != nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(asset.compiled)))
		w.Write(asset.compiled)
		return
	}

	// Otherwise, serve from filesystem
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
