package main

import (
	"strings"
	"unicode"
)

func toPascalCase(s string) string {
	// Handle empty strings early
	if len(s) == 0 {
		return s
	}

	// Handle multi-word strings by splitting on spaces
	words := strings.Fields(s)
	if len(words) > 1 {
		var result strings.Builder
		for _, word := range words {
			result.WriteString(toPascalCase(word))
		}
		return result.String()
	}

	// For single words, we'll process character by character
	var result strings.Builder

	// We'll use runes to properly handle Unicode characters
	runes := []rune(s)

	// Always capitalize the first character
	result.WriteRune(unicode.ToUpper(runes[0]))

	// Process the remaining characters
	for i := 1; i < len(runes); i++ {
		current := runes[i]

		// If we're at an uppercase letter, we need to determine if it's part
		// of an uppercase sequence (like "DB" in "DBHost")
		if unicode.IsUpper(current) {
			// Check if we're in the middle of an uppercase sequence
			isPreviousUpper := i > 0 && unicode.IsUpper(runes[i-1])
			isNextUpper := i < len(runes)-1 && unicode.IsUpper(runes[i+1])

			if isPreviousUpper && (isNextUpper || i == len(runes)-1) {
				// We're in an uppercase sequence - preserve the case
				result.WriteRune(current)
			} else {
				// We're at a camelCase boundary - preserve the uppercase
				result.WriteRune(current)
			}
		} else {
			// For lowercase letters, we just copy them as-is
			result.WriteRune(current)
		}
	}

	return result.String()
}
