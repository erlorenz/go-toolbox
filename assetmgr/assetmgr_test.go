package assetmgr_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/erlorenz/go-toolbox/assetmgr"
)

func TestNew(t *testing.T) {
	t.Run("requires at least one filesystem", func(t *testing.T) {
		_, err := assetmgr.New()
		if err == nil {
			t.Error("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "at least one WithFS") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("creates manager with filesystem", func(t *testing.T) {
		fs := fstest.MapFS{
			"app.js": &fstest.MapFile{Data: []byte("console.log('hello')")},
		}

		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		asset := mgr.Get("/static/app.js")
		if asset == nil {
			t.Fatal("expected asset, got nil")
		}
		if asset.Path != "/static/app.js" {
			t.Errorf("expected path /static/app.js, got %s", asset.Path)
		}
	})

	t.Run("prefix without leading slash", func(t *testing.T) {
		fs := fstest.MapFS{
			"app.js": &fstest.MapFile{Data: []byte("console.log('hello')")},
		}

		mgr, err := assetmgr.New(assetmgr.WithFS("static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		asset := mgr.Get("/static/app.js")
		if asset == nil {
			t.Fatal("expected asset, got nil")
		}
	})
}

func TestAssetHashing(t *testing.T) {
	t.Run("same content produces same hash", func(t *testing.T) {
		content := []byte("console.log('hello')")
		fs1 := fstest.MapFS{
			"a.js": &fstest.MapFile{Data: content},
		}
		fs2 := fstest.MapFS{
			"b.js": &fstest.MapFile{Data: content},
		}

		mgr, err := assetmgr.New(
			assetmgr.WithFS("/one", fs1),
			assetmgr.WithFS("/two", fs2),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		a := mgr.Get("/one/a.js")
		b := mgr.Get("/two/b.js")

		if a.Hash != b.Hash {
			t.Errorf("expected same hash, got %s and %s", a.Hash, b.Hash)
		}
	})

	t.Run("different content produces different hash", func(t *testing.T) {
		fs := fstest.MapFS{
			"a.js": &fstest.MapFile{Data: []byte("console.log('a')")},
			"b.js": &fstest.MapFile{Data: []byte("console.log('b')")},
		}

		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		a := mgr.Get("/static/a.js")
		b := mgr.Get("/static/b.js")

		if a.Hash == b.Hash {
			t.Error("expected different hashes")
		}
	})

	t.Run("versioned path includes hash", func(t *testing.T) {
		fs := fstest.MapFS{
			"app.js": &fstest.MapFile{Data: []byte("console.log('hello')")},
		}

		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		asset := mgr.Get("/static/app.js")
		expected := "/static/app.js?v=" + asset.Hash
		if asset.VersionedPath != expected {
			t.Errorf("expected %s, got %s", expected, asset.VersionedPath)
		}
	})
}

func TestContentType(t *testing.T) {
	tests := []struct {
		file        string
		contentType string
	}{
		{"app.js", "text/javascript"},
		{"style.css", "text/css"},
		{"image.png", "image/png"},
		{"data.json", "application/json"},
		{"page.html", "text/html"},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			fs := fstest.MapFS{
				tt.file: &fstest.MapFile{Data: []byte("content")},
			}

			mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			asset := mgr.Get("/static/" + tt.file)
			// Content types may include charset, so just check prefix
			if !strings.HasPrefix(asset.ContentType, tt.contentType) {
				t.Errorf("expected content type starting with %s, got %s", tt.contentType, asset.ContentType)
			}
		})
	}
}

func TestPreRenderedTags(t *testing.T) {
	t.Run("script tag for JS files", func(t *testing.T) {
		fs := fstest.MapFS{
			"app.js": &fstest.MapFile{Data: []byte("console.log('hello')")},
		}

		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		asset := mgr.Get("/static/app.js")
		if !strings.Contains(asset.ScriptTag, `<script type="module"`) {
			t.Errorf("expected script tag with module type, got %s", asset.ScriptTag)
		}
		if !strings.Contains(asset.ScriptTag, asset.VersionedPath) {
			t.Errorf("expected versioned path in script tag")
		}
		if asset.CSSTag != "" {
			t.Errorf("expected no link tag for JS file, got %s", asset.CSSTag)
		}
	})

	t.Run("link tag for CSS files", func(t *testing.T) {
		fs := fstest.MapFS{
			"style.css": &fstest.MapFile{Data: []byte("body { color: red }")},
		}

		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		asset := mgr.Get("/static/style.css")
		if !strings.Contains(asset.CSSTag, `<link rel="stylesheet"`) {
			t.Errorf("expected link tag, got %s", asset.CSSTag)
		}
		if !strings.Contains(asset.CSSTag, asset.VersionedPath) {
			t.Errorf("expected versioned path in link tag")
		}
		if asset.ScriptTag != "" {
			t.Errorf("expected no script tag for CSS file, got %s", asset.ScriptTag)
		}
	})

	t.Run("no tags for other files", func(t *testing.T) {
		fs := fstest.MapFS{
			"image.png": &fstest.MapFile{Data: []byte{0x89, 0x50, 0x4E, 0x47}},
		}

		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		asset := mgr.Get("/static/image.png")
		if asset.ScriptTag != "" {
			t.Errorf("expected no script tag, got %s", asset.ScriptTag)
		}
		if asset.CSSTag != "" {
			t.Errorf("expected no link tag, got %s", asset.CSSTag)
		}
	})
}

func TestByExtension(t *testing.T) {
	fs := fstest.MapFS{
		"app.js":    &fstest.MapFile{Data: []byte("js")},
		"util.js":   &fstest.MapFile{Data: []byte("js")},
		"style.css": &fstest.MapFile{Data: []byte("css")},
	}

	mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	jsFiles := mgr.ByExtension(".js")
	if len(jsFiles) != 2 {
		t.Errorf("expected 2 JS files, got %d", len(jsFiles))
	}

	cssFiles := mgr.ByExtension(".css")
	if len(cssFiles) != 1 {
		t.Errorf("expected 1 CSS file, got %d", len(cssFiles))
	}

	// Sorted by path
	if jsFiles[0].Path > jsFiles[1].Path {
		t.Error("expected files to be sorted by path")
	}
}

func TestByPrefix(t *testing.T) {
	fs := fstest.MapFS{
		"js/app.js":    &fstest.MapFile{Data: []byte("js")},
		"js/util.js":   &fstest.MapFile{Data: []byte("js")},
		"css/style.css": &fstest.MapFile{Data: []byte("css")},
	}

	mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	jsFiles := mgr.ByPrefix("/static/js")
	if len(jsFiles) != 2 {
		t.Errorf("expected 2 files under /static/js, got %d", len(jsFiles))
	}

	cssFiles := mgr.ByPrefix("/static/css")
	if len(cssFiles) != 1 {
		t.Errorf("expected 1 file under /static/css, got %d", len(cssFiles))
	}
}

func TestScriptTags(t *testing.T) {
	fs := fstest.MapFS{
		"js/app.js":  &fstest.MapFile{Data: []byte("app")},
		"js/util.js": &fstest.MapFile{Data: []byte("util")},
	}

	mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tags := mgr.ScriptTags("/static/js")
	lines := strings.Split(tags, "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 script tags, got %d", len(lines))
	}

	for _, line := range lines {
		if !strings.Contains(line, "<script") {
			t.Errorf("expected script tag, got %s", line)
		}
	}
}

func TestCSSTags(t *testing.T) {
	fs := fstest.MapFS{
		"css/main.css":  &fstest.MapFile{Data: []byte("main")},
		"css/theme.css": &fstest.MapFile{Data: []byte("theme")},
	}

	mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tags := mgr.CSSTags("/static/css")
	lines := strings.Split(tags, "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 link tags, got %d", len(lines))
	}

	for _, line := range lines {
		if !strings.Contains(line, "<link") {
			t.Errorf("expected link tag, got %s", line)
		}
	}
}

func TestImportMap(t *testing.T) {
	t.Run("rewrites local paths", func(t *testing.T) {
		fs := fstest.MapFS{
			"importmap.json": &fstest.MapFile{Data: []byte(`{
				"imports": {
					"app": "/static/js/app.js",
					"lodash": "https://cdn.example.com/lodash.js"
				}
			}`)},
			"js/app.js": &fstest.MapFile{Data: []byte("export default {}")},
		}

		mgr, err := assetmgr.New(
			assetmgr.WithFS("/static", fs),
			assetmgr.WithImportMap("/static/importmap.json"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tag := mgr.ImportMapTag()
		if !strings.Contains(tag, `<script type="importmap">`) {
			t.Errorf("expected importmap script tag, got %s", tag)
		}
		// Should contain versioned path for local asset
		if !strings.Contains(tag, "?v=") {
			t.Errorf("expected versioned path in import map")
		}
		// Should preserve remote URL
		if !strings.Contains(tag, "https://cdn.example.com/lodash.js") {
			t.Errorf("expected remote URL to be preserved")
		}
	})

	t.Run("handles missing import map", func(t *testing.T) {
		fs := fstest.MapFS{
			"app.js": &fstest.MapFile{Data: []byte("console.log('hello')")},
		}

		_, err := assetmgr.New(
			assetmgr.WithFS("/static", fs),
			assetmgr.WithImportMap("/static/importmap.json"),
		)
		if err == nil {
			t.Error("expected error for missing import map")
		}
	})
}

func TestModulePreloadTag(t *testing.T) {
	fs := fstest.MapFS{
		"importmap.json": &fstest.MapFile{Data: []byte(`{
			"imports": {
				"app": "/static/js/app.js",
				"utils": "/static/js/utils.js",
				"lodash": "https://cdn.example.com/lodash.js"
			}
		}`)},
		"js/app.js":   &fstest.MapFile{Data: []byte("export default {}")},
		"js/utils.js": &fstest.MapFile{Data: []byte("export const foo = 1")},
	}

	t.Setenv("APP_ENV", "production")
	mgr, err := assetmgr.New(
		assetmgr.WithFS("/static", fs),
		assetmgr.WithImportMap("/static/importmap.json"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("returns modulepreload tag for local asset", func(t *testing.T) {
		tag := mgr.ModulePreloadTag("app")
		if !strings.Contains(tag, `<link rel="modulepreload"`) {
			t.Errorf("expected modulepreload tag, got %s", tag)
		}
		if !strings.Contains(tag, "/static/js/app.js?v=") {
			t.Errorf("expected versioned path, got %s", tag)
		}
	})

	t.Run("returns modulepreload tag for remote URL", func(t *testing.T) {
		tag := mgr.ModulePreloadTag("lodash")
		if !strings.Contains(tag, `<link rel="modulepreload"`) {
			t.Errorf("expected modulepreload tag, got %s", tag)
		}
		if !strings.Contains(tag, "https://cdn.example.com/lodash.js") {
			t.Errorf("expected remote URL, got %s", tag)
		}
	})

	t.Run("returns empty for unknown key", func(t *testing.T) {
		tag := mgr.ModulePreloadTag("unknown")
		if tag != "" {
			t.Errorf("expected empty string, got %s", tag)
		}
	})

	t.Run("ModulePreloadTags returns multiple tags", func(t *testing.T) {
		tags := mgr.ModulePreloadTags("app", "utils", "lodash")
		lines := strings.Split(tags, "\n")
		if len(lines) != 3 {
			t.Errorf("expected 3 tags, got %d", len(lines))
		}
		for _, line := range lines {
			if !strings.Contains(line, `<link rel="modulepreload"`) {
				t.Errorf("expected modulepreload tag, got %s", line)
			}
		}
	})

	t.Run("ModulePreloadTags skips unknown keys", func(t *testing.T) {
		tags := mgr.ModulePreloadTags("app", "unknown", "utils")
		lines := strings.Split(tags, "\n")
		if len(lines) != 2 {
			t.Errorf("expected 2 tags (unknown skipped), got %d", len(lines))
		}
	})
}

func TestHTTPHandler(t *testing.T) {
	fs := fstest.MapFS{
		"app.js": &fstest.MapFile{Data: []byte("console.log('hello')")},
	}

	// Use explicit dev mode false to test caching
	t.Setenv("APP_ENV", "production")

	mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	asset := mgr.Get("/static/app.js")

	t.Run("serves asset with content", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/static/app.js", nil)
		rec := httptest.NewRecorder()

		mgr.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}

		body, _ := io.ReadAll(rec.Body)
		if string(body) != "console.log('hello')" {
			t.Errorf("unexpected body: %s", body)
		}
	})

	t.Run("sets immutable cache for versioned requests", func(t *testing.T) {
		req := httptest.NewRequest("GET", asset.VersionedPath, nil)
		rec := httptest.NewRecorder()

		mgr.ServeHTTP(rec, req)

		cacheControl := rec.Header().Get("Cache-Control")
		if !strings.Contains(cacheControl, "immutable") {
			t.Errorf("expected immutable cache control, got %s", cacheControl)
		}
		if !strings.Contains(cacheControl, "max-age=31536000") {
			t.Errorf("expected max-age=31536000, got %s", cacheControl)
		}
	})

	t.Run("sets no-cache for non-versioned requests", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/static/app.js", nil)
		rec := httptest.NewRecorder()

		mgr.ServeHTTP(rec, req)

		cacheControl := rec.Header().Get("Cache-Control")
		if cacheControl != "no-cache" {
			t.Errorf("expected no-cache, got %s", cacheControl)
		}

		etag := rec.Header().Get("ETag")
		if etag == "" {
			t.Error("expected ETag header")
		}
	})

	t.Run("returns 404 for unknown asset", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/static/unknown.js", nil)
		rec := httptest.NewRecorder()

		mgr.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("sets correct content type", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/static/app.js", nil)
		rec := httptest.NewRecorder()

		mgr.ServeHTTP(rec, req)

		contentType := rec.Header().Get("Content-Type")
		if !strings.Contains(contentType, "javascript") {
			t.Errorf("expected javascript content type, got %s", contentType)
		}
	})
}

func TestDevMode(t *testing.T) {
	fs := fstest.MapFS{
		"app.js": &fstest.MapFile{Data: []byte("console.log('hello')")},
	}

	mgr, err := assetmgr.New(
		assetmgr.WithFS("/static", fs),
		assetmgr.WithDevMode(true),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("no cache headers in dev mode", func(t *testing.T) {
		asset := mgr.Get("/static/app.js")
		req := httptest.NewRequest("GET", asset.VersionedPath, nil)
		rec := httptest.NewRecorder()

		mgr.ServeHTTP(rec, req)

		cacheControl := rec.Header().Get("Cache-Control")
		if cacheControl != "" {
			t.Errorf("expected no cache control in dev mode, got %s", cacheControl)
		}
	})
}

func TestMultipleFilesystems(t *testing.T) {
	appFS := fstest.MapFS{
		"js/app.js": &fstest.MapFile{Data: []byte("app")},
	}
	vendorFS := fstest.MapFS{
		"htmx.js": &fstest.MapFile{Data: []byte("htmx")},
	}

	mgr, err := assetmgr.New(
		assetmgr.WithFS("/static", appFS),
		assetmgr.WithFS("/vendor", vendorFS),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	appAsset := mgr.Get("/static/js/app.js")
	if appAsset == nil {
		t.Error("expected app asset")
	}

	vendorAsset := mgr.Get("/vendor/htmx.js")
	if vendorAsset == nil {
		t.Error("expected vendor asset")
	}

	// All() should return all assets
	all := mgr.All()
	if len(all) != 2 {
		t.Errorf("expected 2 assets, got %d", len(all))
	}
}

func TestMustGet(t *testing.T) {
	fs := fstest.MapFS{
		"app.js": &fstest.MapFile{Data: []byte("console.log('hello')")},
	}

	mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("returns asset when exists", func(t *testing.T) {
		asset := mgr.MustGet("/static/app.js")
		if asset == nil {
			t.Error("expected asset")
		}
	})

	t.Run("panics when asset not found", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		mgr.MustGet("/static/unknown.js")
	})
}

func TestHiddenFiles(t *testing.T) {
	fs := fstest.MapFS{
		".gitignore": &fstest.MapFile{Data: []byte("node_modules")},
		"app.js":     &fstest.MapFile{Data: []byte("app")},
	}

	mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Hidden files should be skipped
	hidden := mgr.Get("/static/.gitignore")
	if hidden != nil {
		t.Error("expected hidden file to be skipped")
	}

	// Regular files should be included
	app := mgr.Get("/static/app.js")
	if app == nil {
		t.Error("expected app.js to be included")
	}
}

func TestNestedDirectories(t *testing.T) {
	fs := fstest.MapFS{
		"js/components/button.js":  &fstest.MapFile{Data: []byte("button")},
		"js/components/modal.js":   &fstest.MapFile{Data: []byte("modal")},
		"css/components/button.css": &fstest.MapFile{Data: []byte("button")},
	}

	mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	button := mgr.Get("/static/js/components/button.js")
	if button == nil {
		t.Error("expected nested asset")
	}

	// ByPrefix should work with nested paths
	components := mgr.ByPrefix("/static/js/components")
	if len(components) != 2 {
		t.Errorf("expected 2 components, got %d", len(components))
	}
}
