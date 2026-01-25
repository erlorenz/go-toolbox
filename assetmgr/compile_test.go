package assetmgr_test

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/erlorenz/go-toolbox/assetmgr"
)

func TestCSSCompilation(t *testing.T) {
	t.Run("rewrites url() with double quotes", func(t *testing.T) {
		fs := fstest.MapFS{
			"css/style.css":   &fstest.MapFile{Data: []byte(`body { background: url("../images/bg.png"); }`)},
			"images/bg.png":   &fstest.MapFile{Data: []byte("png data")},
		}

		t.Setenv("APP_ENV", "production")
		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Serve the CSS and check the compiled content
		req := httptest.NewRequest("GET", "/static/css/style.css", nil)
		rec := httptest.NewRecorder()
		mgr.ServeHTTP(rec, req)

		body, _ := io.ReadAll(rec.Body)
		content := string(body)

		// Should contain versioned path
		if !strings.Contains(content, "?v=") {
			t.Errorf("expected versioned path in CSS, got: %s", content)
		}
		if !strings.Contains(content, "/static/images/bg.png?v=") {
			t.Errorf("expected rewritten url(), got: %s", content)
		}
	})

	t.Run("rewrites url() with single quotes", func(t *testing.T) {
		fs := fstest.MapFS{
			"css/style.css":   &fstest.MapFile{Data: []byte(`body { background: url('../images/bg.png'); }`)},
			"images/bg.png":   &fstest.MapFile{Data: []byte("png data")},
		}

		t.Setenv("APP_ENV", "production")
		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		req := httptest.NewRequest("GET", "/static/css/style.css", nil)
		rec := httptest.NewRecorder()
		mgr.ServeHTTP(rec, req)

		body, _ := io.ReadAll(rec.Body)
		content := string(body)

		if !strings.Contains(content, "/static/images/bg.png?v=") {
			t.Errorf("expected rewritten url(), got: %s", content)
		}
	})

	t.Run("rewrites @import", func(t *testing.T) {
		fs := fstest.MapFS{
			"css/main.css":  &fstest.MapFile{Data: []byte(`@import "./reset.css";`)},
			"css/reset.css": &fstest.MapFile{Data: []byte(`* { margin: 0; }`)},
		}

		t.Setenv("APP_ENV", "production")
		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		req := httptest.NewRequest("GET", "/static/css/main.css", nil)
		rec := httptest.NewRecorder()
		mgr.ServeHTTP(rec, req)

		body, _ := io.ReadAll(rec.Body)
		content := string(body)

		if !strings.Contains(content, "/static/css/reset.css?v=") {
			t.Errorf("expected rewritten @import, got: %s", content)
		}
	})

	t.Run("skips remote URLs", func(t *testing.T) {
		fs := fstest.MapFS{
			"css/style.css": &fstest.MapFile{Data: []byte(`@import "https://example.com/style.css";`)},
		}

		t.Setenv("APP_ENV", "production")
		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		req := httptest.NewRequest("GET", "/static/css/style.css", nil)
		rec := httptest.NewRecorder()
		mgr.ServeHTTP(rec, req)

		body, _ := io.ReadAll(rec.Body)
		content := string(body)

		// Should be unchanged
		if !strings.Contains(content, "https://example.com/style.css") {
			t.Errorf("expected remote URL to be preserved, got: %s", content)
		}
	})

	t.Run("skips data URIs", func(t *testing.T) {
		fs := fstest.MapFS{
			"css/style.css": &fstest.MapFile{Data: []byte(`body { background: url("data:image/png;base64,abc"); }`)},
		}

		t.Setenv("APP_ENV", "production")
		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		req := httptest.NewRequest("GET", "/static/css/style.css", nil)
		rec := httptest.NewRecorder()
		mgr.ServeHTTP(rec, req)

		body, _ := io.ReadAll(rec.Body)
		content := string(body)

		// Should be unchanged
		if !strings.Contains(content, "data:image/png;base64,abc") {
			t.Errorf("expected data URI to be preserved, got: %s", content)
		}
	})

	t.Run("leaves unknown paths unchanged", func(t *testing.T) {
		fs := fstest.MapFS{
			"css/style.css": &fstest.MapFile{Data: []byte(`body { background: url("../images/unknown.png"); }`)},
		}

		t.Setenv("APP_ENV", "production")
		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		req := httptest.NewRequest("GET", "/static/css/style.css", nil)
		rec := httptest.NewRecorder()
		mgr.ServeHTTP(rec, req)

		body, _ := io.ReadAll(rec.Body)
		content := string(body)

		// Should be unchanged since the image doesn't exist
		if !strings.Contains(content, "../images/unknown.png") {
			t.Errorf("expected unknown path to be preserved, got: %s", content)
		}
	})
}

func TestJSCompilation(t *testing.T) {
	t.Run("rewrites relative imports", func(t *testing.T) {
		fs := fstest.MapFS{
			"js/app.js":   &fstest.MapFile{Data: []byte(`import { foo } from "./utils.js";`)},
			"js/utils.js": &fstest.MapFile{Data: []byte(`export const foo = 1;`)},
		}

		t.Setenv("APP_ENV", "production")
		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		req := httptest.NewRequest("GET", "/static/js/app.js", nil)
		rec := httptest.NewRecorder()
		mgr.ServeHTTP(rec, req)

		body, _ := io.ReadAll(rec.Body)
		content := string(body)

		if !strings.Contains(content, "/static/js/utils.js?v=") {
			t.Errorf("expected rewritten import, got: %s", content)
		}
	})

	t.Run("rewrites export from", func(t *testing.T) {
		fs := fstest.MapFS{
			"js/index.js":  &fstest.MapFile{Data: []byte(`export * from "./utils.js";`)},
			"js/utils.js":  &fstest.MapFile{Data: []byte(`export const foo = 1;`)},
		}

		t.Setenv("APP_ENV", "production")
		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		req := httptest.NewRequest("GET", "/static/js/index.js", nil)
		rec := httptest.NewRecorder()
		mgr.ServeHTTP(rec, req)

		body, _ := io.ReadAll(rec.Body)
		content := string(body)

		if !strings.Contains(content, "/static/js/utils.js?v=") {
			t.Errorf("expected rewritten export, got: %s", content)
		}
	})

	t.Run("rewrites dynamic imports", func(t *testing.T) {
		fs := fstest.MapFS{
			"js/app.js":  &fstest.MapFile{Data: []byte(`const mod = await import("./lazy.js");`)},
			"js/lazy.js": &fstest.MapFile{Data: []byte(`export default {};`)},
		}

		t.Setenv("APP_ENV", "production")
		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		req := httptest.NewRequest("GET", "/static/js/app.js", nil)
		rec := httptest.NewRecorder()
		mgr.ServeHTTP(rec, req)

		body, _ := io.ReadAll(rec.Body)
		content := string(body)

		if !strings.Contains(content, "/static/js/lazy.js?v=") {
			t.Errorf("expected rewritten dynamic import, got: %s", content)
		}
	})

	t.Run("skips bare specifiers", func(t *testing.T) {
		fs := fstest.MapFS{
			"js/app.js": &fstest.MapFile{Data: []byte(`import { foo } from "lodash";`)},
		}

		t.Setenv("APP_ENV", "production")
		mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		req := httptest.NewRequest("GET", "/static/js/app.js", nil)
		rec := httptest.NewRecorder()
		mgr.ServeHTTP(rec, req)

		body, _ := io.ReadAll(rec.Body)
		content := string(body)

		// Should be unchanged - bare specifiers are handled by import map
		if !strings.Contains(content, `from "lodash"`) {
			t.Errorf("expected bare specifier to be preserved, got: %s", content)
		}
	})
}

func TestDevModeSkipsCompilation(t *testing.T) {
	fs := fstest.MapFS{
		"css/style.css": &fstest.MapFile{Data: []byte(`@import "./reset.css";`)},
		"css/reset.css": &fstest.MapFile{Data: []byte(`* { margin: 0; }`)},
	}

	mgr, err := assetmgr.New(
		assetmgr.WithFS("/static", fs),
		assetmgr.WithDevMode(true),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest("GET", "/static/css/style.css", nil)
	rec := httptest.NewRecorder()
	mgr.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	content := string(body)

	// In dev mode, should NOT be rewritten
	if strings.Contains(content, "?v=") {
		t.Errorf("expected no versioning in dev mode, got: %s", content)
	}
	if !strings.Contains(content, `@import "./reset.css"`) {
		t.Errorf("expected original content in dev mode, got: %s", content)
	}
}

func TestMultipleImportMaps(t *testing.T) {
	fs := fstest.MapFS{
		"base.json": &fstest.MapFile{Data: []byte(`{
			"imports": {
				"lodash": "https://cdn.example.com/lodash.js",
				"utils": "/static/js/utils.js"
			}
		}`)},
		"app.json": &fstest.MapFile{Data: []byte(`{
			"imports": {
				"app": "/static/js/app.js",
				"lodash": "https://cdn.example.com/lodash-v2.js"
			}
		}`)},
		"js/utils.js": &fstest.MapFile{Data: []byte(`export const foo = 1;`)},
		"js/app.js":   &fstest.MapFile{Data: []byte(`export default {};`)},
	}

	t.Setenv("APP_ENV", "production")
	mgr, err := assetmgr.New(
		assetmgr.WithFS("/static", fs),
		assetmgr.WithImportMap("/static/base.json"),
		assetmgr.WithImportMap("/static/app.json"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tag := mgr.ImportMapTag()

	// Should have merged imports
	if !strings.Contains(tag, `"app"`) {
		t.Errorf("expected app import, got: %s", tag)
	}
	if !strings.Contains(tag, `"utils"`) {
		t.Errorf("expected utils import, got: %s", tag)
	}

	// Later import map should win for lodash
	if !strings.Contains(tag, "lodash-v2.js") {
		t.Errorf("expected later lodash to win, got: %s", tag)
	}

	// Local paths should be versioned
	if !strings.Contains(tag, "/static/js/app.js?v=") {
		t.Errorf("expected versioned app path, got: %s", tag)
	}
	if !strings.Contains(tag, "/static/js/utils.js?v=") {
		t.Errorf("expected versioned utils path, got: %s", tag)
	}
}

func TestExplicitDevModeWins(t *testing.T) {
	fs := fstest.MapFS{
		"app.js": &fstest.MapFile{Data: []byte(`console.log("hi")`)},
	}

	// Set env to production but explicitly set dev mode
	t.Setenv("APP_ENV", "production")

	mgr, err := assetmgr.New(
		assetmgr.WithFS("/static", fs),
		assetmgr.WithDevMode(true), // Should win over env var
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest("GET", "/static/app.js", nil)
	rec := httptest.NewRecorder()
	mgr.ServeHTTP(rec, req)

	// Should have no cache headers (dev mode)
	cacheControl := rec.Header().Get("Cache-Control")
	if cacheControl != "" {
		t.Errorf("expected no cache control in dev mode, got: %s", cacheControl)
	}
}

func TestCompiledContentHash(t *testing.T) {
	// When content is compiled, the hash should be based on compiled content
	fs := fstest.MapFS{
		"css/style.css": &fstest.MapFile{Data: []byte(`@import "./reset.css";`)},
		"css/reset.css": &fstest.MapFile{Data: []byte(`* { margin: 0; }`)},
	}

	t.Setenv("APP_ENV", "production")
	mgr, err := assetmgr.New(assetmgr.WithFS("/static", fs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	asset := mgr.Get("/static/css/style.css")

	// The versioned path should use the compiled content hash
	if !strings.Contains(asset.VersionedPath, "?v=") {
		t.Errorf("expected versioned path, got: %s", asset.VersionedPath)
	}

	// LinkTag should use the versioned path
	if !strings.Contains(asset.LinkTag, asset.VersionedPath) {
		t.Errorf("expected LinkTag to use versioned path, got: %s", asset.LinkTag)
	}
}
