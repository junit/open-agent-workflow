package config

import (
	"fmt"
	"path"
	"strings"
	"unicode"
)

func validateReferencePath(value string) error {
	if value == "" || path.IsAbs(value) || path.Clean(value) != value || strings.ContainsAny(value, `\*?[]{}()`) {
		return fmt.Errorf("CONFIG_PATH_INVALID: %q", value)
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("CONFIG_PATH_INVALID: %q", value)
		}
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("CONFIG_PATH_INVALID: %q", value)
		}
	}
	return nil
}
