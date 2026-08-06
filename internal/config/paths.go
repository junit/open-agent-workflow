package config

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
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

func physicalRoot(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("CONFIG_PATH_INVALID: empty root")
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("CONFIG_PATH_INVALID: %w", err)
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("CONFIG_PATH_INVALID: %w", err)
	}
	info, err := os.Stat(physical)
	if err != nil {
		return "", fmt.Errorf("CONFIG_PATH_INVALID: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("CONFIG_PATH_INVALID: root is not a directory")
	}
	return filepath.Clean(physical), nil
}

func readContained(root, relative string, maximum int64) ([]byte, string, error) {
	if err := validateReferencePath(relative); err != nil {
		return nil, "", err
	}
	physical, err := physicalRoot(root)
	if err != nil {
		return nil, "", err
	}
	candidate := filepath.Join(physical, filepath.FromSlash(relative))
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return nil, "", fmt.Errorf("CONFIG_FILE_READ_FAILED: %s: %w", relative, err)
	}
	relation, err := filepath.Rel(physical, resolved)
	if err != nil || relation == ".." || strings.HasPrefix(relation, ".."+string(filepath.Separator)) || filepath.IsAbs(relation) {
		return nil, "", fmt.Errorf("CONFIG_PATH_ESCAPE: %s", relative)
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil, "", fmt.Errorf("CONFIG_FILE_READ_FAILED: %s: %w", relative, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("CONFIG_FILE_READ_FAILED: %s: %w", relative, err)
	}
	if !info.Mode().IsRegular() {
		return nil, "", fmt.Errorf("CONFIG_FILE_NOT_REGULAR: %s", relative)
	}
	if maximum <= 0 {
		maximum = maximumConfigBytes
	}
	if info.Size() > maximum {
		return nil, "", fmt.Errorf("CONFIG_FILE_TOO_LARGE: %s", relative)
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, "", fmt.Errorf("CONFIG_FILE_READ_FAILED: %s: %w", relative, err)
	}
	if int64(len(data)) > maximum {
		return nil, "", fmt.Errorf("CONFIG_FILE_TOO_LARGE: %s", relative)
	}
	return data, filepath.Clean(resolved), nil
}
