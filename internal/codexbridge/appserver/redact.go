package appserver

import (
	"unicode"
	"unicode/utf8"
)

const maximumMetadataEntries = 4096

func normalizeMetadataErrors(values []metadataErrorWire) ([]MetadataError, error) {
	if len(values) > maximumMetadataEntries {
		return nil, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "metadata error list is oversized", nil)
	}
	result := make([]MetadataError, 0, len(values))
	for _, value := range values {
		if value.Message == nil || value.Path == nil {
			return nil, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "metadata error projection is incomplete", nil)
		}
		result = append(result, MetadataError{Message: "metadata source reported an error"})
	}
	return result, nil
}

func validMetadataText(value string, maximum int) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum &&
		!hasControl(value)
}

func hasControl(value string) bool {
	return !utf8.ValidString(value) || stringsIndexControl(value)
}

func stringsIndexControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}
