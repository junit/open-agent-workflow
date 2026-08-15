package manifest

import (
	"fmt"
	"strings"
)

// Validate checks the default release identity fields.
func Validate(data []byte) (string, error) {
	return validate(data, []string{"version", "commit"})
}

// ValidateRequired checks the manifest against caller-selected required fields.
func ValidateRequired(data []byte, required []string) (string, error) {
	return validate(data, required)
}

func validate(data []byte, required []string) (string, error) {
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return "", fmt.Errorf("manifest is empty")
	}
	fields := make(map[string]string)
	for lineNumber, line := range strings.Split(text, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return "", fmt.Errorf("malformed line %d: expected key=value", lineNumber+1)
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if _, exists := fields[key]; exists {
			return "", fmt.Errorf("duplicate field %q", key)
		}
		fields[key] = value
	}
	for _, key := range required {
		if _, exists := fields[key]; !exists {
			return "", fmt.Errorf("missing required field %q", key)
		}
	}
	return fmt.Sprintf("valid release manifest: %s", fields["version"]), nil
}
