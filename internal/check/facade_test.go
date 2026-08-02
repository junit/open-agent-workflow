package check_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/check"
	"github.com/wifibaby4u/open-agent-workflow/internal/management"
)

func TestFacadeCompatibility(t *testing.T) {
	tests := []struct {
		name    string
		request check.Request
		setup   func(*testing.T, check.Environment)
	}{
		{name: "success", request: check.Request{Targets: "claude,codex"}},
		{name: "invalid target", request: check.Request{Targets: "vscode"}},
		{
			name:    "invalid state",
			request: check.Request{Targets: "claude"},
			setup: func(t *testing.T, environment check.Environment) {
				writeFixtureFile(t, filepath.Join(environment.StateHome, "open-agent-workflow", "installations", "user.state"), "format\t2\n")
			},
		},
		{
			name:    "partial output symlink",
			request: check.Request{Targets: "claude"},
			setup: func(t *testing.T, environment check.Environment) {
				outside := filepath.Join(filepath.Dir(environment.Home), "outside")
				if err := os.MkdirAll(outside, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(environment.Home, ".claude")); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			environment := testEnvironment(t, t.TempDir())
			if tt.setup != nil {
				tt.setup(t, environment)
			}

			catalog := testCatalog(t)
			checkResult, checkCallErr := check.Execute(catalog, environment, tt.request)
			managementResult, managementCallErr := management.Check(catalog, management.Environment(environment), management.CheckRequest(tt.request))

			if !reflect.DeepEqual(checkResult, managementResult) {
				t.Fatalf("results differ:\ncheck:      %#v\nmanagement: %#v", checkResult, managementResult)
			}
			assertFacadeErrorsEqual(t, checkCallErr, managementCallErr)

			var checkOutput, managementOutput bytes.Buffer
			if err := check.Write(checkResult, &checkOutput); err != nil {
				t.Fatalf("check.Write() error = %v", err)
			}
			if err := management.WriteResult(managementResult, &managementOutput); err != nil {
				t.Fatalf("management.WriteResult() error = %v", err)
			}
			if !bytes.Equal(checkOutput.Bytes(), managementOutput.Bytes()) {
				t.Fatalf("written bytes differ:\ncheck:      %q\nmanagement: %q", checkOutput.Bytes(), managementOutput.Bytes())
			}
		})
	}
}

func assertFacadeErrorsEqual(t *testing.T, checkErr, managementErr error) {
	t.Helper()
	if (checkErr == nil) != (managementErr == nil) {
		t.Fatalf("errors differ: check=%v management=%v", checkErr, managementErr)
	}
	if checkErr == nil {
		return
	}
	var checkTyped *check.Error
	var managementTyped *management.Error
	if !errors.As(checkErr, &checkTyped) || !errors.As(managementErr, &managementTyped) {
		t.Fatalf("typed errors differ: check=%T management=%T", checkErr, managementErr)
	}
	if !reflect.DeepEqual(checkTyped, managementTyped) {
		t.Fatalf("typed errors differ: check=%#v management=%#v", checkTyped, managementTyped)
	}
}
