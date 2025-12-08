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
