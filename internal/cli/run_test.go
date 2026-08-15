package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultCommandSurfaceContainsOnlyManagementAndProfileInspection(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"home", "config", "state"} {
		path := filepath.Join(root, directory)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(map[string]string{
			"home": "HOME", "config": "XDG_CONFIG_HOME", "state": "XDG_STATE_HOME",
		}[directory], path)
	}

	var stdout, stderr bytes.Buffer
	if status := Run([]string{"--help"}, &stdout, &stderr); status != 0 || stderr.Len() != 0 {
		t.Fatalf("help status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	for _, expected := range []string{
		"check", "install", "update", "uninstall",
		"oaw profile list", "oaw profile show", "oaw profile check",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("help omits %q: %s", expected, stdout.String())
		}
	}
	for _, forbidden := range []string{
		"oaw profiles", "oaw use", "oaw status", "oaw complete", "oaw review",
		"oaw approve", "oaw satisfy", "oaw incident", "oaw switch", "oaw stop",
		"oaw uncertain", "oaw workflow", "oaw providers", "oaw policy",
		"oaw catalog", "oaw bridge", "oaw runtime", "oaw run",
	} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Errorf("help retains removed command %q: %s", forbidden, stdout.String())
		}
	}

	for _, command := range []string{
		"profiles", "use", "status", "complete", "review", "approve", "satisfy",
		"incident", "switch", "stop", "uncertain", "workflow", "providers",
		"policy", "catalog", "bridge", "runtime", "run",
	} {
		stdout.Reset()
		stderr.Reset()
		if status := Run([]string{command}, &stdout, &stderr); status != 64 {
			t.Errorf("Run(%s) status=%d stdout=%q stderr=%q", command, status, stdout.String(), stderr.String())
			continue
		}
		want := fmt.Sprintf("oaw: error: unknown command '%s'\n", command)
		if stdout.Len() != 0 || stderr.String() != want {
			t.Errorf("Run(%s) stdout=%q stderr=%q want stderr=%q", command, stdout.String(), stderr.String(), want)
		}
	}
}
