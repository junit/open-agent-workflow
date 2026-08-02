package check

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

type providerCompatibilityRule struct {
	name       string
	providerID string
	probeIDs   []string
	requireAll bool
}

var providerCompatibilityRules = [...]providerCompatibilityRule{
	{
		name:       "superpowers",
		providerID: "oaw/superpowers",
		probeIDs: []string{
			"claude-direct", "codex-direct", "claude-marketplace-checkout",
			"claude-official-cache", "claude-marketplace-cache", "codex-curated-cache",
		},
	},
	{
		name:       "matt",
		providerID: "oaw/matt",
		probeIDs:   []string{"matt-to-spec", "matt-to-tickets", "matt-tdd", "matt-diagnosing-bugs"},
		requireAll: true,
	},
	{name: "ecc", providerID: "oaw/ecc", probeIDs: []string{"ecc-global-skill"}},
}

func providerLines(value catalog.Catalog, home string) ([]string, error) {
	lines := make([]string, 0, len(providerCompatibilityRules))
	for _, rule := range providerCompatibilityRules {
		detected, err := providerDetected(value, home, rule)
		if err != nil {
			return nil, err
		}
		status := "missing"
		if detected {
			status = "detected"
		}
		lines = append(lines, fmt.Sprintf("provider %s: %s", rule.name, status))
	}
	return lines, nil
}

func providerDetected(value catalog.Catalog, home string, rule providerCompatibilityRule) (bool, error) {
	var descriptor catalog.ProviderDescriptorRecord
	found := false
	for _, candidate := range value.Providers() {
		if candidate.ID == rule.providerID {
			descriptor = candidate
			found = true
			break
		}
	}
	if !found {
		return false, compatibilityError(fmt.Sprintf("provider descriptor is missing: %s", rule.providerID))
	}
	probes := make(map[string]catalog.DiscoveryProbe, len(descriptor.Discovery))
	for _, probe := range descriptor.Discovery {
		probes[probe.ID] = probe
	}
	matched := 0
	for _, id := range rule.probeIDs {
		probe, exists := probes[id]
		if !exists {
			return false, compatibilityError(fmt.Sprintf("provider probe is missing: %s/%s", rule.providerID, id))
		}
		if compatibilityProbeExists(home, probe) {
			matched++
		}
	}
	if rule.requireAll {
		return matched == len(rule.probeIDs), nil
	}
	return matched != 0, nil
}

func compatibilityProbeExists(home string, probe catalog.DiscoveryProbe) bool {
	if probe.Root != "user-home" {
		return false
	}
	switch probe.Kind {
	case "path-exists":
		return regularFile(compatibilityRootedPath(home, probe.Path))
	case "one-level-version-path-exists":
		prefix := compatibilityRootedPath(home, probe.Prefix)
		info, err := os.Stat(prefix)
		if err != nil || !info.IsDir() {
			return false
		}
		directory, err := os.Open(prefix)
		if err != nil {
			return false
		}
		defer directory.Close()
		for {
			entries, readErr := directory.ReadDir(1)
			if readErr != nil && len(entries) == 0 {
				return false
			}
			entry := entries[0]
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			candidate := filepath.Join(prefix, entry.Name(), filepath.FromSlash(probe.Suffix))
			if regularFile(candidate) {
				return true
			}
		}
	}
	return false
}

func compatibilityRootedPath(root, suffix string) string {
	return root + string(filepath.Separator) + filepath.FromSlash(suffix)
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func compatibilityError(message string) error {
	return &Error{Status: 65, Message: message}
}
