package coordinator

import "strings"

func validStableID(prefix, value string) bool {
	if len(value) != len(prefix)+32 || !strings.HasPrefix(value, prefix) {
		return false
	}
	return validHex(value[len(prefix):])
}

func validHex(value string) bool {
	if len(value) == 0 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
