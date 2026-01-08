package casing

import (
	"strings"
	"unicode"
)

func ToSnake(s string) string {

	r := []rune(s)

	var str strings.Builder

	for i, char := range r {

		// Replace the dot
		if char == '.' {
			str.WriteRune('_')
			continue
		}

		// Start is always lower
		isStart := i == 0 || r[i-1] == '.'
		if isStart {
			str.WriteRune(unicode.ToLower(char))
			continue
		}

		// End always lower
		isEnd := i == len(r)-1 || r[i+1] == '.'

		if isEnd {
			str.WriteRune(unicode.ToLower(char))
			continue
		}

		// Write _ if beginning of word or end of acronym
		isUpper := unicode.IsUpper(char)
		prevIsUpper := unicode.IsUpper(r[i-1])
		nextIsUpper := !isEnd && unicode.IsUpper(r[i+1])

		isBeginningOfWord := isUpper && !prevIsUpper
		isAfterAcronym := isUpper && prevIsUpper && !nextIsUpper && !isEnd

		if isBeginningOfWord || isAfterAcronym {
			str.WriteRune('_')
		}

		str.WriteRune(unicode.ToLower(char))
	}

	return str.String()
}

func ToScreamingSnake(s string) string {
	return strings.ToUpper(ToSnake(s))
}

func ToKebab(s string) string {
	return strings.ReplaceAll(ToSnake(s), "_", "-")
}

// ToPascal converts snake_case, kebab-case, or mixed input to PascalCase
func ToPascal(s string) string {
	if s == "" {
		return ""
	}

	var str strings.Builder
	capitalizeNext := true

	for _, char := range s {
		// Treat separators as word boundaries
		if char == '_' || char == '-' || char == '.' || char == ' ' {
			capitalizeNext = true
			continue
		}

		if capitalizeNext {
			str.WriteRune(unicode.ToUpper(char))
			capitalizeNext = false
		} else {
			str.WriteRune(unicode.ToLower(char))
		}
	}

	return str.String()
}

// ToCamel converts snake_case, kebab-case, or mixed input to camelCase
func ToCamel(s string) string {
	if s == "" {
		return ""
	}

	pascal := ToPascal(s)

	// Convert first character to lowercase
	r := []rune(pascal)
	if len(r) > 0 {
		r[0] = unicode.ToLower(r[0])
	}

	return string(r)
}
