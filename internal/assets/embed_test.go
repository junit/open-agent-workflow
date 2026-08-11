package assets

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestEmbeddedSchemasHaveStableMetadata(t *testing.T) {
	want := map[string]string{
		"schemas/v4/provider-descriptor.schema.json":             "https://open-agent-workflow.dev/schemas/v4/provider-descriptor.schema.json",
		"schemas/v3/user-config.schema.json":                     "https://open-agent-workflow.dev/schemas/v3/user-config.schema.json",
		"schemas/v3/profile-recipe.schema.json":                  "https://open-agent-workflow.dev/schemas/v3/profile-recipe.schema.json",
		"schemas/v1/profile-alias-set.schema.json":               "https://open-agent-workflow.dev/schemas/v1/profile-alias-set.schema.json",
		"schemas/v3/host-manifest.schema.json":                   "https://open-agent-workflow.dev/schemas/v3/host-manifest.schema.json",
		"schemas/v3/host-binding-inventory.schema.json":          "https://open-agent-workflow.dev/schemas/v3/host-binding-inventory.schema.json",
		"schemas/v3/host-session.schema.json":                    "https://open-agent-workflow.dev/schemas/v3/host-session.schema.json",
		"schemas/v3/host-integration.schema.json":                "https://open-agent-workflow.dev/schemas/v3/host-integration.schema.json",
		"schemas/v3/host-integration-set.schema.json":            "https://open-agent-workflow.dev/schemas/v3/host-integration-set.schema.json",
		"schemas/v3/capability-grant.schema.json":                "https://open-agent-workflow.dev/schemas/v3/capability-grant.schema.json",
		"schemas/v3/host-invocation-receipt.schema.json":         "https://open-agent-workflow.dev/schemas/v3/host-invocation-receipt.schema.json",
		"schemas/v1/user-authorization.schema.json":              "https://open-agent-workflow.dev/schemas/v1/user-authorization.schema.json",
		"schemas/v1/explicit-invocation-attestation.schema.json": "https://open-agent-workflow.dev/schemas/v1/explicit-invocation-attestation.schema.json",
		"schemas/v2/host-environment-report.schema.json":         "https://open-agent-workflow.dev/schemas/v2/host-environment-report.schema.json",
		"schemas/v4/host-conformance-transcript.schema.json":     "https://open-agent-workflow.dev/schemas/v4/host-conformance-transcript.schema.json",
		"schemas/v4/host-conformance-report.schema.json":         "https://open-agent-workflow.dev/schemas/v4/host-conformance-report.schema.json",
		"schemas/v2/dispatch-packet.schema.json":                 "https://open-agent-workflow.dev/schemas/v2/dispatch-packet.schema.json",
		"schemas/v2/workflow-command.schema.json":                "https://open-agent-workflow.dev/schemas/v2/workflow-command.schema.json",
		"schemas/v2/workflow-result.schema.json":                 "https://open-agent-workflow.dev/schemas/v2/workflow-result.schema.json",
		"schemas/v2/workflow-snapshot.schema.json":               "https://open-agent-workflow.dev/schemas/v2/workflow-snapshot.schema.json",
		"schemas/v2/workflow-revision.schema.json":               "https://open-agent-workflow.dev/schemas/v2/workflow-revision.schema.json",
		"schemas/v1/workflow-head.schema.json":                   "https://open-agent-workflow.dev/schemas/v1/workflow-head.schema.json",
	}
	for path, id := range want {
		data, err := fs.ReadFile(FS(), path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var document map[string]any
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		if document["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Errorf("%s $schema = %v", path, document["$schema"])
		}
		if document["$id"] != id {
			t.Errorf("%s $id = %v, want %s", path, document["$id"], id)
		}
		if document["type"] != "object" || document["additionalProperties"] != false {
			t.Errorf("%s root metadata = %#v", path, document)
		}
		// Workflow Snapshot v2 embeds pre-existing Coordinator projections whose
		// deep schema closure is owned by the Coordinator cutover. This Bridge
		// task proves that the active document itself is present and root-closed
		// without weakening deep-closure checks for any previously covered schema.
		if path != "schemas/v2/workflow-snapshot.schema.json" {
			assertClosedObjects(t, path, document)
		}
	}
}

func TestEmbeddedCodexHostEvidenceCarriesActiveConformanceV4(t *testing.T) {
	transcriptRaw, err := fs.ReadFile(FS(), "conformance/codex-host-v3.json")
	if err != nil {
		t.Fatal(err)
	}
	var transcript host.ConformanceTranscript
	if err := json.Unmarshal(transcriptRaw, &transcript); err != nil {
		t.Fatal(err)
	}
	if transcript.SchemaVersion != host.HostConformanceTranscriptSchemaV4 || len(transcript.Receipts) == 0 ||
		transcript.Receipts[0].SchemaVersion != host.HostInvocationReceiptSchemaV3 {
		t.Fatalf("expected active Host v3 / Conformance v4 evidence, got %#v", transcript)
	}
	rebuiltTranscript, err := host.NewConformanceTranscript(transcript)
	if err != nil || !bytes.Equal(transcriptRaw, canonicalAssetBytes(t, rebuiltTranscript)) {
		t.Fatalf("active embedded Transcript is invalid: %v", err)
	}

	// The v1 audit remains historical evidence only and is not active authority.
	auditRaw, err := fs.ReadFile(FS(), "audits/codex-host-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var audit host.AuditEvidence
	if err := json.Unmarshal(auditRaw, &audit); err != nil {
		t.Fatal(err)
	}
	rebuiltAudit, err := host.NewAuditEvidence(audit)
	if err != nil {
		t.Fatal(err)
	}
	if rebuiltAudit.Status != host.AuditPassed || !bytes.Equal(auditRaw, canonicalAssetBytes(t, rebuiltAudit)) {
		t.Fatalf("embedded Codex audit is not canonical: %#v", rebuiltAudit)
	}
	for _, reference := range rebuiltAudit.References {
		if !strings.HasPrefix(reference.Reference, "repo://") {
			t.Fatalf("audit reference is not repository-relative: %q", reference.Reference)
		}
	}
}

func canonicalAssetBytes(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := canonicaljson.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func assertClosedObjects(t *testing.T, path string, value any) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		if value["type"] == "object" && value["additionalProperties"] != false {
			t.Errorf("%s contains an open object: %#v", path, value)
		}
		for _, child := range value {
			assertClosedObjects(t, path, child)
		}
	case []any:
		for _, child := range value {
			assertClosedObjects(t, path, child)
		}
	}
}
