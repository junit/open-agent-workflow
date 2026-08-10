package builtin

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/provideraudit"
)

var updateProfileMatrix = flag.Bool("update", false, "update the committed Profile Matrix projection")

func TestProfileMatrixProjectionHasCanonicalTenSlots(t *testing.T) {
	matrix := buildMatrix(t)
	canonical := catalog.CanonicalSlots()
	if len(matrix.Profiles) != 4 {
		t.Fatalf("profile count = %d, want 4", len(matrix.Profiles))
	}
	for _, profile := range matrix.Profiles {
		if len(profile.Slots) != len(canonical) {
			t.Fatalf("%s slots = %d", profile.Alias, len(profile.Slots))
		}
		for index, slot := range profile.Slots {
			if slot.SlotID != canonical[index].ID {
				t.Errorf("%s slot[%d] = %s", profile.Alias, index, slot.SlotID)
			}
		}
	}
}

func TestProfileMatrixProjectionIncludesFourAliases(t *testing.T) {
	matrix := buildMatrix(t)
	got := map[string]string{}
	for _, profile := range matrix.Profiles {
		got[profile.Alias] = profile.RecipeID
	}
	want := map[string]string{
		"SP-FULL": "oaw/delivery", "MATT-FULL": "oaw/domain-engineering",
		"ECC-FULL": "oaw/ecc-engineering", "MATT-SP-HYBRID": "oaw/reliable-feature",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matrix aliases = %v, want %v", got, want)
	}
}

func TestProfileMatrixProjectionIncludesHostSurfaceAndSourcePins(t *testing.T) {
	value := loadCatalog(t)
	audit := loadAudit(t)
	matrix := buildMatrix(t)
	seenHosts := map[string]bool{}
	for _, profile := range matrix.Profiles {
		for _, slot := range profile.Slots {
			for _, binding := range slot.Pipeline {
				seenHosts[binding.Host] = true
				provider := requireProvider(t, value, binding.ProviderID)
				declared := requireBinding(t, provider, binding.BindingID)
				audited, found := audit.Binding(binding.ProviderID, binding.BindingID)
				if !found || binding.Surface != declared.Surface || binding.Kind != declared.Kind || binding.Reference != declared.Reference || binding.DistributionRevision != provider.Distributions[0].Revision || binding.BindingTreeDigest != audited.TreeDigest {
					t.Errorf("matrix Binding provenance mismatch: %#v", binding)
				}
			}
		}
	}
	if !seenHosts["codex"] || !seenHosts["claude"] {
		t.Fatalf("matrix Host coverage = %v", seenHosts)
	}
}

func TestProfileMatrixProjectionPreservesPipelineAndMacroOrder(t *testing.T) {
	matrix := buildMatrix(t)
	matt := matrixProfile(t, matrix, "MATT-FULL")
	framing := matrixSlot(t, matt, catalog.SlotProblemFraming)
	got := matrixBindingIDs(framing.Pipeline)
	wantPrefix := []string{"codex-grill-with-docs", "codex-grilling", "codex-domain-modeling"}
	if len(got) < len(wantPrefix) || !reflect.DeepEqual(got[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("Matt framing macro order = %v", got)
	}
	if framing.Pipeline[1].MacroMode != catalog.InternalCreditOnly || framing.Pipeline[2].MacroMode != catalog.InternalCreditOnly {
		t.Fatalf("Matt framing macro modes = %#v", framing.Pipeline)
	}

	sp := matrixProfile(t, matrix, "SP-FULL")
	workspace := matrixSlot(t, sp, catalog.SlotWorkspacePreparation)
	if got := matrixBindingIDs(workspace.Pipeline); len(got) < 2 || got[0] != "codex-using-git-worktrees" || got[1] != "claude-using-git-worktrees" {
		t.Fatalf("SP workspace macro order = %v", got)
	}
	implementation := matrixSlot(t, sp, catalog.SlotImplementation)
	if got := matrixBindingIDs(implementation.Pipeline); len(got) < 2 || got[0] != "codex-executing-plans" || got[1] != "claude-executing-plans" {
		t.Fatalf("SP implementation macro order = %v", got)
	}
	closeout := matrixSlot(t, sp, catalog.SlotCloseout)
	got = matrixBindingIDs(closeout.Pipeline)
	if len(got) < 2 || got[0] != "codex-finishing-a-development-branch" || got[1] != "claude-finishing-a-development-branch" {
		t.Fatalf("SP closeout macro order = %v", got)
	}
}

func TestProfileMatrixProjectionMarksHybridPausedBindings(t *testing.T) {
	hybrid := matrixProfile(t, buildMatrix(t), "MATT-SP-HYBRID")
	paused := []string{}
	for _, slot := range hybrid.Slots {
		for _, binding := range slot.Pipeline {
			if binding.Paused {
				paused = append(paused, binding.ProviderID+"/"+binding.BindingID)
			}
		}
	}
	sort.Strings(paused)
	want := []string{
		"oaw/matt/claude-code-review", "oaw/matt/codex-code-review",
		"oaw/superpowers/claude-subagent-driven-development", "oaw/superpowers/claude-test-driven-development",
		"oaw/superpowers/codex-subagent-driven-development", "oaw/superpowers/codex-test-driven-development",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(uniqueStrings(paused), want) {
		t.Fatalf("Hybrid paused Bindings = %v, want %v", uniqueStrings(paused), want)
	}
}

func TestProfileMatrixProjectionRejectsFictionalOrHookBinding(t *testing.T) {
	value := loadCatalog(t)
	audit := loadAudit(t)
	matrix := buildMatrix(t)
	matrix.Profiles[0].Slots[0].Pipeline[0].Reference = "requirements"
	matrix.Digest = matrix.ContentDigest()
	if err := ValidateProfileMatrix(value, audit, matrix); err == nil || !strings.Contains(err.Error(), "BUILTIN_PROFILE_MATRIX_INVALID") {
		t.Fatalf("ValidateProfileMatrix(fictional) error = %v", err)
	}
	matrix = buildMatrix(t)
	matrix.Profiles[0].Slots[0].Pipeline[0].Reference = "delivery-gate"
	matrix.Digest = matrix.ContentDigest()
	if err := ValidateProfileMatrix(value, audit, matrix); err == nil {
		t.Fatal("ValidateProfileMatrix accepted Hook Binding")
	}
}

func TestCommittedProfileMatrixMatchesProjectionByteForByte(t *testing.T) {
	matrix := buildMatrix(t)
	want := canonicalMatrixBytes(t, matrix)
	got, err := os.ReadFile(filepath.Join("..", "assets", "profile-matrix.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("committed Profile Matrix differs from generated projection")
	}
}

func TestCommittedProfileMatrixRejectsDigestDrift(t *testing.T) {
	raw := canonicalMatrixBytes(t, buildMatrix(t))
	raw = bytes.Replace(raw, []byte(provideraudit.CanonicalMatrixDigest), []byte(strings.Repeat("a", 64)), 1)
	if _, err := DecodeProfileMatrix(raw); err == nil || !strings.Contains(err.Error(), "BUILTIN_PROFILE_MATRIX_INVALID") {
		t.Fatalf("DecodeProfileMatrix(drift) error = %v", err)
	}
}

func TestLoadRejectsProfileMatrixDrift(t *testing.T) {
	files := embeddedMap(t)
	raw := canonicalMatrixBytes(t, buildMatrix(t))
	raw = bytes.Replace(raw, []byte(provideraudit.CanonicalMatrixDigest), []byte(strings.Repeat("a", 64)), 1)
	files["profile-matrix.json"] = &fstest.MapFile{Data: raw}
	if _, err := loadFromFS(files); err == nil || !strings.Contains(err.Error(), "BUILTIN_PROFILE_MATRIX_INVALID") {
		t.Fatalf("loadFromFS(matrix drift) error = %v", err)
	}
}

func TestLoadEmbedsProfileMatrix(t *testing.T) {
	matrix, err := LoadProfileMatrix()
	if err != nil {
		t.Fatal(err)
	}
	if matrix.SchemaVersion != ProfileMatrixSchemaV1 || len(matrix.Profiles) != 4 {
		t.Fatalf("LoadProfileMatrix() = %#v", matrix)
	}
}

func TestWriteProfileMatrix(t *testing.T) {
	if !*updateProfileMatrix {
		t.Skip("pass -update to rewrite the committed Profile Matrix")
	}
	path := filepath.Join("..", "assets", "profile-matrix.json")
	if err := os.WriteFile(path, canonicalMatrixBytes(t, buildMatrix(t)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func buildMatrix(t *testing.T) ProfileMatrixRecord {
	t.Helper()
	value := loadCatalog(t)
	audit := loadAudit(t)
	matrix, err := BuildProfileMatrix(value, audit)
	if err != nil {
		t.Fatal(err)
	}
	if matrix.SchemaVersion != ProfileMatrixSchemaV1 || matrix.CanonicalMatrixDigest != provideraudit.CanonicalMatrixDigest || matrix.SourceAuditDigest != audit.Digest || matrix.Digest == "" {
		t.Fatalf("BuildProfileMatrix() header = %#v", matrix)
	}
	return matrix
}

func matrixProfile(t *testing.T, matrix ProfileMatrixRecord, alias string) MatrixProfile {
	t.Helper()
	for _, profile := range matrix.Profiles {
		if profile.Alias == alias {
			return profile
		}
	}
	t.Fatalf("matrix profile %s not found", alias)
	return MatrixProfile{}
}

func matrixSlot(t *testing.T, profile MatrixProfile, slotID catalog.SlotID) MatrixSlot {
	t.Helper()
	for _, slot := range profile.Slots {
		if slot.SlotID == slotID {
			return slot
		}
	}
	t.Fatalf("matrix slot %s/%s not found", profile.Alias, slotID)
	return MatrixSlot{}
}

func matrixBindingIDs(values []MatrixBinding) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.BindingID
	}
	return result
}

func canonicalMatrixBytes(t *testing.T, matrix ProfileMatrixRecord) []byte {
	t.Helper()
	raw, err := canonicaljson.Marshal(matrix)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func uniqueStrings(values []string) []string {
	result := []string{}
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}
