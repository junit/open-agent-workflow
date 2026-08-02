package check

import (
	"fmt"
	"os"
	"path/filepath"
)

func readinessLines(environment Environment, targets []string) []string {
	lines := make([]string, 0, len(targets))
	for _, id := range targets {
		candidate, _ := findTarget(id)
		if !candidate.User {
			lines = append(lines, fmt.Sprintf("target %s: adapter-only (project)", id))
			continue
		}
		status := "missing"
		if coreTargetDetected(environment, id) {
			status = "detected"
		}
		lines = append(lines, fmt.Sprintf("target %s: %s (user, project)", id, status))
	}
	return lines
}

func coreTargetDetected(environment Environment, id string) bool {
	if executableOnPath(environment.Path, id) {
		return true
	}
	var root string
	switch id {
	case "claude", "codex", "gemini":
		root = filepath.Join(environment.Home, "."+id)
	case "opencode":
		root = filepath.Join(environment.ConfigHome, "opencode")
	}
	info, err := os.Stat(root)
	return err == nil && info.IsDir()
}

func executableOnPath(pathValue, name string) bool {
	for _, directory := range filepath.SplitList(pathValue) {
		if directory == "" {
			directory = "."
		}
		info, err := os.Stat(filepath.Join(directory, name))
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return true
		}
	}
	return false
}
