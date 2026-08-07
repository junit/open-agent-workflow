package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestGenerateCodexHostIsIdempotentAndPreservesPolicy(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{
		"internal/assets/audits/codex-host-v1.json",
		"internal/assets/conformance/codex-host-v1.json",
		"internal/assets/host-integrations.json",
	} {
		copyGeneratorFixture(t, root, relative)
	}
	integrationsPath := filepath.Join(root, "internal", "assets", "host-integrations.json")
	before := decodeIntegrationSetFile(t, integrationsPath)
	policyBefore := generatorIntegrationByID(t, before.Integrations, "oaw/codex-policy")

	if err := generateCodexHost(root); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(integrationsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := generateCodexHost(root); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(integrationsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("Codex Host Integration generation is not idempotent")
	}
	after := decodeIntegrationSetFile(t, integrationsPath)
	policyAfter := generatorIntegrationByID(t, after.Integrations, "oaw/codex-policy")
	if !reflect.DeepEqual(policyBefore, policyAfter) {
		t.Fatalf("Codex policy changed: before = %#v, after = %#v", policyBefore, policyAfter)
	}
	native := generatorIntegrationByID(t, after.Integrations, codexbridge.BridgeIntegrationID)
	if native.Manifest.ControlSurface != host.SurfaceHostNative || native.Audit.Status != host.AuditPassed || native.Conformance == nil {
		t.Fatalf("generated native Integration = %#v", native)
	}
}

func copyGeneratorFixture(t *testing.T, root, relative string) {
	t.Helper()
	source := filepath.Join("..", "..", "..", filepath.FromSlash(relative))
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func decodeIntegrationSetFile(t *testing.T, path string) host.IntegrationSetRecord {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	set, err := host.DecodeIntegrationSetJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func generatorIntegrationByID(t *testing.T, values []host.IntegrationRecord, id string) host.IntegrationRecord {
	t.Helper()
	for _, value := range values {
		if value.ID == id {
			return value
		}
	}
	t.Fatalf("Integration %q not found", id)
	return host.IntegrationRecord{}
}
