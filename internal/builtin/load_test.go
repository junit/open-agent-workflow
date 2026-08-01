package builtin

import (
	"io/fs"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

func TestBuiltInProviderDescriptors(t *testing.T) {
	registry, err := schema.New(assets.FS())
	if err != nil {
		t.Fatalf("schema.New() error = %v", err)
	}
	paths, err := fs.Glob(assets.FS(), "providers/*.json")
	if err != nil {
		t.Fatalf("fs.Glob() error = %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("provider asset count = %d, want 3", len(paths))
	}

	expectedIDs := []string{"oaw/ecc", "oaw/matt", "oaw/superpowers"}
	expectedCapabilities := map[string][]string{
		"oaw/ecc":         {"architecture", "build-repair", "completion", "functional-debugging", "implementation", "planning", "remediation", "review", "security-review", "tdd", "verification"},
		"oaw/matt":        {"completion", "debugging", "implementation", "remediation", "review", "specification", "tdd", "tickets", "verification"},
		"oaw/superpowers": {"completion", "debugging", "discovery-design", "implementation", "implementation-planning", "remediation", "review", "tdd", "verification", "workspace"},
	}
	var gotIDs []string
	for _, path := range paths {
		raw, err := fs.ReadFile(assets.FS(), path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if err := registry.Validate(schema.ProviderDescriptorV1, raw); err != nil {
			t.Fatalf("schema validation for %s: %v", path, err)
		}
		record, err := catalog.DecodeProvider(raw)
		if err != nil {
			t.Fatalf("DecodeProvider(%s): %v", path, err)
		}
		gotIDs = append(gotIDs, record.ID)
		if record.DescriptorVersion != "1.0.0" {
			t.Errorf("%s descriptor version = %q", record.ID, record.DescriptorVersion)
		}
		var capabilityIDs []string
		for _, capability := range record.Capabilities {
			capabilityIDs = append(capabilityIDs, capability.ID)
			if len(capability.HostBindings) == 0 {
				t.Errorf("%s/%s has no Host Binding", record.ID, capability.ID)
			}
			for _, mode := range capability.RequestModes {
				if mode != catalog.RequestModeBounded && mode != catalog.RequestModeWorkflow {
					t.Errorf("%s/%s has invalid mode %q", record.ID, capability.ID, mode)
				}
			}
			if capability.ExecutorTopology == catalog.MainAgentAllowed {
				t.Errorf("%s/%s permits main-agent execution", record.ID, capability.ID)
			}
		}
		sort.Strings(capabilityIDs)
		if strings.Join(capabilityIDs, "\x00") != strings.Join(expectedCapabilities[record.ID], "\x00") {
			t.Errorf("%s capabilities = %v, want %v", record.ID, capabilityIDs, expectedCapabilities[record.ID])
		}
		for _, probe := range record.Discovery {
			for _, pathValue := range []string{probe.Path, probe.Prefix, probe.Suffix} {
				assertSafeDiscoveryPath(t, record.ID, probe.ID, pathValue)
			}
			for _, pathValue := range probe.Paths {
				assertSafeDiscoveryPath(t, record.ID, probe.ID, pathValue)
			}
		}
	}
	sort.Strings(gotIDs)
	if strings.Join(gotIDs, "\x00") != strings.Join(expectedIDs, "\x00") {
		t.Fatalf("provider IDs = %v, want %v", gotIDs, expectedIDs)
	}
}

func assertSafeDiscoveryPath(t *testing.T, providerID, probeID, value string) {
	t.Helper()
	if value == "" {
		return
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		t.Errorf("%s/%s path %q is not relative/portable", providerID, probeID, value)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			t.Errorf("%s/%s path %q contains control character", providerID, probeID, value)
		}
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." {
			t.Errorf("%s/%s path %q contains unsafe component", providerID, probeID, value)
		}
	}
}
