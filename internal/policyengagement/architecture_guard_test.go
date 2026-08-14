package policyengagement_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPolicyEngagementDependencyGraphExcludesMachineAuthorityPackages(t *testing.T) {
	t.Parallel()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate policyengagement package")
	}
	command := exec.Command("go", "list", "-deps", ".")
	command.Dir = filepath.Dir(filename)
	raw, err := command.Output()
	if err != nil {
		t.Fatalf("list policyengagement dependencies: %v", err)
	}
	forbidden := []string{
		"/internal/builtin",
		"/internal/catalog",
		"/internal/codexbridge",
		"/internal/coordinator",
		"/internal/core",
		"/internal/discovery",
		"/internal/integrity",
		"/internal/provideraudit",
		"/internal/registry",
	}
	for _, dependency := range strings.Fields(string(raw)) {
		for _, fragment := range forbidden {
			if strings.Contains(dependency, fragment) {
				t.Errorf("Policy Engagement transitively depends on machine-authority package %q", dependency)
			}
		}
	}
}
