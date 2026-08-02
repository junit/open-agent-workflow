package management

import (
	"path/filepath"
	"testing"
)

func TestInstallationHealthFailsClosedForUnknownInMemoryOwnership(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".claude", "CLAUDE.md")
	health := installationHealth{
		stateStatus: "valid",
		policyClean: true,
		coords: coordinates{
			currentScope: "user",
			environment:  Environment{Home: root, ConfigHome: root},
		},
		state: installationState{
			scope:   "user",
			targets: []targetRecord{{id: "claude", path: path, mode: "unknown"}},
		},
	}
	status, err := health.targetStatus("claude")
	if err != nil || status != "invalid-state" {
		t.Fatalf("targetStatus() = %q, %v", status, err)
	}
}
