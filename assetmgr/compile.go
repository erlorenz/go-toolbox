package assetmgr

import (
	"path"
	"regexp"
	"strings"
)

// CSS patterns for url() and @import
// Note: Go regexp doesn't support backreferences, so we match both quote styles separately
var (
	// Matches: url("path"), url('path'), url(path)
	cssURLDoubleQuote = regexp.MustCompile(`url\(\s*"([^"]+)"\s*\)`)
	cssURLSingleQuote = regexp.MustCompile(`url\(\s*'([^']+)'\s*\)`)
	cssURLNoQuote     = regexp.MustCompile(`url\(\s*([^"')\s][^)\s]*)\s*\)`)

	// Matches: @import "path", @import 'path'
	cssImportDoubleQuote = regexp.MustCompile(`@import\s+"([^"]+)"`)
	cssImportSingleQuote = regexp.MustCompile(`@import\s+'([^']+)'`)
)

// JS patterns for import/export
var (
	// Matches static imports with double quotes
	jsImportDoubleQuote = regexp.MustCompile(`(\bimport\s+(?:[^"']*\s+from\s+)?)"([^"]+)"`)
	// Matches static imports with single quotes
	jsImportSingleQuote = regexp.MustCompile(`(\bimport\s+(?:[^"']*\s+from\s+)?)'([^']+)'`)

	// Matches exports with double quotes
	jsExportDoubleQuote = regexp.MustCompile(`(\bexport\s+[^"']*\s+from\s+)"([^"]+)"`)
	// Matches exports with single quotes
	jsExportSingleQuote = regexp.MustCompile(`(\bexport\s+[^"']*\s+from\s+)'([^']+)'`)

	// Matches dynamic imports with double quotes
	jsDynamicImportDoubleQuote = regexp.MustCompile(`(\bimport\s*\(\s*)"([^"]+)"(\s*\))`)
	// Matches dynamic imports with single quotes
	jsDynamicImportSingleQuote = regexp.MustCompile(`(\bimport\s*\(\s*)'([^']+)'(\s*\))`)
)

// compileCSS rewrites url() and @import references in CSS content.
// assetPath is the logical path of the CSS file (e.g., "/static/css/style.css").
// resolve takes a logical path and returns the versioned path, or empty string if not found.
func compileCSS(content []byte, assetPath string, resolve func(string) string) []byte {
	result := content

	// Helper to rewrite a URL path
	rewriteURL := func(urlPath string, quote string) string {
		if shouldSkipPath(urlPath) {
			return ""
		}
		resolved := resolvePath(assetPath, urlPath, resolve)
		if resolved == "" {
			return ""
		}
		return "url(" + quote + resolved + quote + ")"
	}

	// Rewrite url("...") with double quotes
	result = cssURLDoubleQuote.ReplaceAllFunc(result, func(match []byte) []byte {
		submatch := cssURLDoubleQuote.FindSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		if rewritten := rewriteURL(string(submatch[1]), `"`); rewritten != "" {
			return []byte(rewritten)
		}
		return match
	})

	// Rewrite url('...') with single quotes
	result = cssURLSingleQuote.ReplaceAllFunc(result, func(match []byte) []byte {
		submatch := cssURLSingleQuote.FindSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		if rewritten := rewriteURL(string(submatch[1]), "'"); rewritten != "" {
			return []byte(rewritten)
		}
		return match
	})

	// Rewrite url(...) without quotes
	result = cssURLNoQuote.ReplaceAllFunc(result, func(match []byte) []byte {
		submatch := cssURLNoQuote.FindSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		if rewritten := rewriteURL(string(submatch[1]), ""); rewritten != "" {
			return []byte(rewritten)
		}
		return match
	})

	// Helper to rewrite an @import path
	rewriteImport := func(importPath string, quote string) string {
		if shouldSkipPath(importPath) {
			return ""
		}
		resolved := resolvePath(assetPath, importPath, resolve)
		if resolved == "" {
			return ""
		}
		return "@import " + quote + resolved + quote
	}

	// Rewrite @import "..."
	result = cssImportDoubleQuote.ReplaceAllFunc(result, func(match []byte) []byte {
		submatch := cssImportDoubleQuote.FindSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		if rewritten := rewriteImport(string(submatch[1]), `"`); rewritten != "" {
			return []byte(rewritten)
		}
		return match
	})

	// Rewrite @import '...'
	result = cssImportSingleQuote.ReplaceAllFunc(result, func(match []byte) []byte {
		submatch := cssImportSingleQuote.FindSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		if rewritten := rewriteImport(string(submatch[1]), "'"); rewritten != "" {
			return []byte(rewritten)
		}
		return match
	})

	return result
}

// compileJS rewrites import/export references in JavaScript content.
// assetPath is the logical path of the JS file (e.g., "/static/js/app.js").
// resolve takes a logical path and returns the versioned path, or empty string if not found.
func compileJS(content []byte, assetPath string, resolve func(string) string) []byte {
	result := content

	// Helper to check and rewrite a JS import path
	rewriteJSPath := func(importPath string) string {
		if shouldSkipJSPath(importPath) {
			return ""
		}
		return resolvePath(assetPath, importPath, resolve)
	}

	// Rewrite static imports with double quotes
	result = jsImportDoubleQuote.ReplaceAllFunc(result, func(match []byte) []byte {
		submatch := jsImportDoubleQuote.FindSubmatch(match)
		if len(submatch) < 3 {
			return match
		}
		prefix := string(submatch[1])
		importPath := string(submatch[2])
		if resolved := rewriteJSPath(importPath); resolved != "" {
			return []byte(prefix + `"` + resolved + `"`)
		}
		return match
	})

	// Rewrite static imports with single quotes
	result = jsImportSingleQuote.ReplaceAllFunc(result, func(match []byte) []byte {
		submatch := jsImportSingleQuote.FindSubmatch(match)
		if len(submatch) < 3 {
			return match
		}
		prefix := string(submatch[1])
		importPath := string(submatch[2])
		if resolved := rewriteJSPath(importPath); resolved != "" {
			return []byte(prefix + "'" + resolved + "'")
		}
		return match
	})

	// Rewrite exports with double quotes
	result = jsExportDoubleQuote.ReplaceAllFunc(result, func(match []byte) []byte {
		submatch := jsExportDoubleQuote.FindSubmatch(match)
		if len(submatch) < 3 {
			return match
		}
		prefix := string(submatch[1])
		exportPath := string(submatch[2])
		if resolved := rewriteJSPath(exportPath); resolved != "" {
			return []byte(prefix + `"` + resolved + `"`)
		}
		return match
	})

	// Rewrite exports with single quotes
	result = jsExportSingleQuote.ReplaceAllFunc(result, func(match []byte) []byte {
		submatch := jsExportSingleQuote.FindSubmatch(match)
		if len(submatch) < 3 {
			return match
		}
		prefix := string(submatch[1])
		exportPath := string(submatch[2])
		if resolved := rewriteJSPath(exportPath); resolved != "" {
			return []byte(prefix + "'" + resolved + "'")
		}
		return match
	})

	// Rewrite dynamic imports with double quotes
	result = jsDynamicImportDoubleQuote.ReplaceAllFunc(result, func(match []byte) []byte {
		submatch := jsDynamicImportDoubleQuote.FindSubmatch(match)
		if len(submatch) < 4 {
			return match
		}
		prefix := string(submatch[1])
		importPath := string(submatch[2])
		suffix := string(submatch[3])
		if resolved := rewriteJSPath(importPath); resolved != "" {
			return []byte(prefix + `"` + resolved + `"` + suffix)
		}
		return match
	})

	// Rewrite dynamic imports with single quotes
	result = jsDynamicImportSingleQuote.ReplaceAllFunc(result, func(match []byte) []byte {
		submatch := jsDynamicImportSingleQuote.FindSubmatch(match)
		if len(submatch) < 4 {
			return match
		}
		prefix := string(submatch[1])
		importPath := string(submatch[2])
		suffix := string(submatch[3])
		if resolved := rewriteJSPath(importPath); resolved != "" {
			return []byte(prefix + "'" + resolved + "'" + suffix)
		}
		return match
	})

	return result
}

// shouldSkipPath returns true for paths that shouldn't be rewritten.
func shouldSkipPath(p string) bool {
	// Data URIs
	if strings.HasPrefix(p, "data:") {
		return true
	}
	// Remote URLs
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "//") {
		return true
	}
	// Fragment-only references
	if strings.HasPrefix(p, "#") {
		return true
	}
	return false
}

// shouldSkipJSPath returns true for JS import paths that shouldn't be rewritten.
func shouldSkipJSPath(p string) bool {
	// Skip remote URLs
	if shouldSkipPath(p) {
		return true
	}
	// Skip bare specifiers (no ./ or ../ or /) - these are handled by import map
	if !strings.HasPrefix(p, "./") && !strings.HasPrefix(p, "../") && !strings.HasPrefix(p, "/") {
		return true
	}
	return false
}

// resolvePath resolves a relative path from assetPath and looks up the versioned path.
// Returns empty string if the resolved path is not found in assets.
func resolvePath(assetPath, relativePath string, resolve func(string) string) string {
	var logicalPath string

	if strings.HasPrefix(relativePath, "/") {
		// Absolute path
		logicalPath = relativePath
	} else {
		// Relative path - resolve from asset's directory
		dir := path.Dir(assetPath)
		logicalPath = path.Clean(path.Join(dir, relativePath))
	}

	return resolve(logicalPath)
}
