//go:build !windows

package policyrun

import "os"

func replaceRunFile(source, target string) error {
	return os.Rename(source, target)
}

func syncRunDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
