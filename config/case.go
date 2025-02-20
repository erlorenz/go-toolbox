package config

import (
	"strings"
	"unicode"
)

func toScreamingSnakeCase(s string) string {
	// First replace dots

	r := []rune(s)

	var str strings.Builder

	for i, char := range r {

		// Always upper
		if i == 0 {
			str.WriteRune(char)
			continue
		}
		// Skip the dot
		if char == '.' {
			continue
		}

		// Write _ if beginning of word or end of acronym

		isUpper := unicode.IsUpper(char)
		isLast := i == len(r)-1
		prev := r[i-1]
		prevIsUpper := unicode.IsUpper(prev)
		nextIsUpper := !isLast && unicode.IsUpper(r[i+1])

		isBeginningOfWord := isUpper && !prevIsUpper
		isEndOfAcronym := isUpper && prevIsUpper && !nextIsUpper && !isLast

		if isBeginningOfWord || isEndOfAcronym {
			str.WriteRune('_')
		}

		str.WriteRune(unicode.ToUpper(char))
	}

	return str.String()
}

func toKebabCase(s string) string {
	return strings.ToLower(strings.ReplaceAll(toScreamingSnakeCase(s), "_", "-"))
}
