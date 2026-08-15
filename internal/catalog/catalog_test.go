package catalog

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeProviderV5AcceptsOnlyIdentityFields(t *testing.T) {
	raw, err := json.Marshal(testProviderRecord())
	if err != nil {
		t.Fatal(err)
	}
	value, err := DecodeProvider(raw)
	if err != nil {
		t.Fatal(err)
	}
	if value.SchemaVersion != ProviderDescriptorSchemaV5 || value.ID != "acme/suite" || len(value.Bindings) != 1 {
		t.Fatalf("Provider = %#v", value)
	}
}

func TestDecodeProviderRejectsRemovedWorkflowFields(t *testing.T) {
	raw, err := json.Marshal(testProviderRecord())
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		`"capabilities":[]`,
		`"profile_recipes":[]`,
		`"request_modes":["WORKFLOW"]`,
		`"responsibilities":[]`,
		`"supported_topologies":["CURRENT"]`,
	} {
		t.Run(field, func(t *testing.T) {
			mutated := strings.Replace(string(raw), "{", "{"+field+",", 1)
			if _, err := DecodeProvider([]byte(mutated)); err == nil {
				t.Fatalf("DecodeProvider accepted removed workflow field %s", field)
			}
		})
	}
}

func TestDecodeProviderRejectsPreCutoverSchema(t *testing.T) {
	raw, err := json.Marshal(testProviderRecord())
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(raw), ProviderDescriptorSchemaV5, "oaw.provider-descriptor/v4", 1)
	if _, err := DecodeProvider([]byte(mutated)); err == nil || !strings.Contains(err.Error(), "UNSUPPORTED_PROVIDER_SCHEMA") {
		t.Fatalf("DecodeProvider v4 error = %v", err)
	}
}

func TestCatalogIsDeterministicAndReturnsImmutableSnapshots(t *testing.T) {
	first := testProviderRecord()
	second := testProviderRecord()
	second.ID = "acme/another"
	second.DisplayName = "Acme Another"

	left, err := New([]ProviderDescriptorRecord{first, second})
	if err != nil {
		t.Fatal(err)
	}
	right, err := New([]ProviderDescriptorRecord{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if left.Digest() == "" || left.Digest() != right.Digest() || !reflect.DeepEqual(left.Providers(), right.Providers()) {
		t.Fatalf("Catalog order is not deterministic: %q != %q", left.Digest(), right.Digest())
	}

	snapshot := left.Providers()
	snapshot[0].Bindings[0].Reference = "changed"
	if left.Providers()[0].Bindings[0].Reference == "changed" {
		t.Fatal("Catalog exposed mutable Provider storage")
	}
}

func testProviderRecord() ProviderDescriptorRecord {
	return ProviderDescriptorRecord{
		SchemaVersion:     ProviderDescriptorSchemaV5,
		DescriptorVersion: "5.0.0",
		ID:                "acme/suite",
		DisplayName:       "Acme Suite",
		Distributions: []DistributionRecord{{
			ID: "acme", SourceURI: "https://example.test/acme/suite",
			Revision: strings.Repeat("a", 40), TreeDigest: "sha256:" + strings.Repeat("b", 64),
		}},
		Discovery: []DiscoveryProbe{{
			ID: "codex", Hosts: []string{"codex"}, Surface: "codex-plugin", DistributionID: "acme",
			Kind: "path-exists", Root: "user-home", CandidatePath: ".codex/plugins/acme", EvidencePath: "marker.txt",
		}},
		Bindings: []BindingRecord{{
			ID: "codex-review", DistributionID: "acme", ContentRoot: "skills/review", InstallRoot: "skills/review",
			TreeDigest: "sha256:" + strings.Repeat("c", 64), Host: "codex", Surface: "codex-plugin",
			Kind: BindingSkill, Reference: "acme:review", Invocation: InvocationModel,
		}},
	}
}
