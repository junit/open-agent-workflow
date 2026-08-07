package assets

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestEmbeddedSchemasHaveStableMetadata(t *testing.T) {
	want := map[string]string{
		"schemas/v3/provider-descriptor.schema.json":         "https://open-agent-workflow.dev/schemas/v3/provider-descriptor.schema.json",
		"schemas/v3/user-config.schema.json":                 "https://open-agent-workflow.dev/schemas/v3/user-config.schema.json",
		"schemas/v2/profile-recipe.schema.json":              "https://open-agent-workflow.dev/schemas/v2/profile-recipe.schema.json",
		"schemas/v1/profile-alias-set.schema.json":           "https://open-agent-workflow.dev/schemas/v1/profile-alias-set.schema.json",
		"schemas/v2/host-manifest.schema.json":               "https://open-agent-workflow.dev/schemas/v2/host-manifest.schema.json",
		"schemas/v2/host-integration.schema.json":            "https://open-agent-workflow.dev/schemas/v2/host-integration.schema.json",
		"schemas/v2/host-integration-set.schema.json":        "https://open-agent-workflow.dev/schemas/v2/host-integration-set.schema.json",
		"schemas/v2/host-invocation-receipt.schema.json":     "https://open-agent-workflow.dev/schemas/v2/host-invocation-receipt.schema.json",
		"schemas/v2/host-conformance-transcript.schema.json": "https://open-agent-workflow.dev/schemas/v2/host-conformance-transcript.schema.json",
		"schemas/v2/host-conformance-report.schema.json":     "https://open-agent-workflow.dev/schemas/v2/host-conformance-report.schema.json",
		"schemas/v1/workflow-command.schema.json":            "https://open-agent-workflow.dev/schemas/v1/workflow-command.schema.json",
		"schemas/v1/workflow-result.schema.json":             "https://open-agent-workflow.dev/schemas/v1/workflow-result.schema.json",
		"schemas/v1/workflow-snapshot.schema.json":           "https://open-agent-workflow.dev/schemas/v1/workflow-snapshot.schema.json",
		"schemas/v1/workflow-revision.schema.json":           "https://open-agent-workflow.dev/schemas/v1/workflow-revision.schema.json",
		"schemas/v1/workflow-head.schema.json":               "https://open-agent-workflow.dev/schemas/v1/workflow-head.schema.json",
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
		assertClosedObjects(t, path, document)
	}
}

func TestCodexHostEvidenceAssetsAreEmbeddedAndPinned(t *testing.T) {
	transcriptRaw, err := fs.ReadFile(FS(), "conformance/codex-host-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var transcript host.ConformanceTranscript
	if err := json.Unmarshal(transcriptRaw, &transcript); err != nil {
		t.Fatal(err)
	}
	rebuiltTranscript, err := host.NewConformanceTranscript(transcript)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(transcriptRaw, canonicalAssetBytes(t, rebuiltTranscript)) {
		t.Fatal("embedded Codex conformance transcript is not canonical")
	}

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
		path := filepath.Join("..", "..", filepath.FromSlash(strings.TrimPrefix(reference.Reference, "repo://")))
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if digest := canonicaljson.DigestBytes(content); digest != reference.Digest {
			t.Fatalf("audit reference %q digest = %s, want %s", reference.Reference, reference.Digest, digest)
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
