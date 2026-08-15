// Package slug converts human-readable headings into stable URL slugs.
package slug

import (
	"strings"
	"unicode"
)

// Slugify lowercases letters and replaces runs of separators with one hyphen.
func Slugify(value string) string {
	var result strings.Builder
	pendingSeparator := false

	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			if pendingSeparator && result.Len() > 0 {
				result.WriteByte('-')
			}
			result.WriteRune(unicode.ToLower(character))
			pendingSeparator = false
			continue
		}
		pendingSeparator = result.Len() > 0
	}

	return result.String()
}
