package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestGenerateCodexHostV3IsIdempotentAndPreservesPolicy(t *testing.T) {
	root := t.TempDir()
	copyGeneratorFixture(t, root, "internal/assets/host-integrations.json")
	integrationsPath := filepath.Join(root, "internal", "assets", "host-integrations.json")
	before := decodeIntegrationSetFile(t, integrationsPath)
	policyBefore := generatorIntegrationByID(t, before.Integrations, "oaw/codex-policy")

	if err := generateActiveAssets(root); err != nil {
		t.Fatal(err)
	}
	firstSet, err := os.ReadFile(integrationsPath)
	if err != nil {
		t.Fatal(err)
	}
	transcriptPath := filepath.Join(root, "internal", "assets", "conformance", "codex-host-v3.json")
	firstTranscript, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := generateActiveAssets(root); err != nil {
		t.Fatal(err)
	}
	secondSet, err := os.ReadFile(integrationsPath)
	if err != nil {
		t.Fatal(err)
	}
	secondTranscript, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstSet, secondSet) || !bytes.Equal(firstTranscript, secondTranscript) {
		t.Fatal("Codex Host v3 generation is not idempotent")
	}
	after := decodeIntegrationSetFile(t, integrationsPath)
	policyAfter := generatorIntegrationByID(t, after.Integrations, "oaw/codex-policy")
	if policyBefore.ID != policyAfter.ID || policyBefore.Manifest.HostID != policyAfter.Manifest.HostID || policyAfter.Manifest.ControlSurface != host.SurfacePolicy {
		t.Fatalf("Codex policy identity changed: before = %#v, after = %#v", policyBefore, policyAfter)
	}
	if !reflect.DeepEqual(policyAfter, generatorIntegrationByID(t, after.Integrations, "oaw/codex-policy")) {
		t.Fatal("generated policy lookup is nondeterministic")
	}
	native := generatorIntegrationByID(t, after.Integrations, codexHostIntegrationID)
	if native.Manifest.ControlSurface != host.SurfaceHostNative || native.Audit.Status != host.AuditPassed || native.Conformance == nil ||
		native.IntegrationVersion != "2.0.0" || native.Manifest.SchemaVersion != host.HostManifestSchemaV3 ||
		native.Conformance.SchemaVersion != host.HostConformanceReportSchemaV4 ||
		!reflect.DeepEqual(native.Manifest.DelegationFeatures, []host.FeatureID{host.FeatureChildDelegation}) ||
		!reflect.DeepEqual(native.Conformance.VerifiedDelegationFeatures, []host.FeatureID{host.FeatureChildDelegation}) || len(native.Manifest.HostActions) != 0 {
		t.Fatalf("generated native Integration = %#v", native)
	}
	var transcript host.ConformanceTranscript
	if err := json.Unmarshal(secondTranscript, &transcript); err != nil {
		t.Fatal(err)
	}
	if transcript.SchemaVersion != host.HostConformanceTranscriptSchemaV4 || len(transcript.Receipts) == 0 || transcript.Receipts[0].SchemaVersion != host.HostInvocationReceiptSchemaV3 {
		t.Fatalf("generated Transcript = %#v", transcript)
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
