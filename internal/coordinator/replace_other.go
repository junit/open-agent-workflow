//go:build !windows

package coordinator

import "os"

func replaceStateFile(source, target string) error {
	return os.Rename(source, target)
}
