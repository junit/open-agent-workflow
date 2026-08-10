package catalog

import (
	"reflect"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func TestCatalogV4PreservesSemanticOrderAndOwnsNestedStorage(t *testing.T) {
	provider := validProviderV4Record()
	recipe := validRecipeV3Record()
	recipe.AddOns = []AddOnRecord{
		{ID: "first", Kind: AddOnSpecialistCheck, Selector: BindingSelector{ProviderID: provider.ID, BindingID: "binding"}, SlotID: SlotReviewRemediation, IncidentTypes: []string{}, EvidenceRequirements: []EvidenceRequirementRecord{}},
		{ID: "second", Kind: AddOnSpecialistCheck, Selector: BindingSelector{ProviderID: provider.ID, BindingID: "binding"}, SlotID: SlotReviewRemediation, IncidentTypes: []string{}, EvidenceRequirements: []EvidenceRequirementRecord{}},
	}
	value, err := New([]ProviderDescriptorRecord{provider}, []ProfileRecipeRecord{recipe}, []ProfileAliasRecord{{Alias: "SP-FULL", RecipeID: recipe.ID}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	digest := value.Digest()
	providers := value.Providers()
	providers[0].Bindings[0].StageSpan[0] = SlotCloseout
	providers[0].Discovery[0].Hosts[0] = "changed"
	recipes := value.Recipes()
	recipes[0].AddOns[0].ID = "changed"
	recipes[0].Slots[3].HostAction.ID = "changed"
	if value.Providers()[0].Bindings[0].StageSpan[0] != SlotProblemFraming || value.Providers()[0].Discovery[0].Hosts[0] == "changed" || value.Recipes()[0].AddOns[0].ID != "first" || value.Recipes()[0].Slots[3].HostAction.ID == "changed" || value.Digest() != digest {
		t.Fatal("Catalog exposed mutable nested storage")
	}

	reversed := recipe
	reversed.AddOns = append([]AddOnRecord(nil), recipe.AddOns...)
	reversed.AddOns[0], reversed.AddOns[1] = reversed.AddOns[1], reversed.AddOns[0]
	reordered, err := New([]ProviderDescriptorRecord{provider}, []ProfileRecipeRecord{reversed}, []ProfileAliasRecord{{Alias: "SP-FULL", RecipeID: recipe.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if reordered.Digest() == digest {
		t.Fatal("semantic Add-on declaration order did not change Catalog digest")
	}
}

func TestCatalogV4RejectsCompleteInvariantMatrix(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ProviderDescriptorRecord, *ProfileRecipeRecord, *[]ProfileAliasRecord)
		code   string
	}{
		{"discovery distribution missing", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			provider.Discovery[0].DistributionID = "missing"
		}, "PROVIDER_DISTRIBUTION_NOT_FOUND"},
		{"binding distribution missing", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			provider.Bindings[0].DistributionID = "missing"
		}, "PROVIDER_DISTRIBUTION_NOT_FOUND"},
		{"distribution revision branch", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			provider.Distributions[0].Revision = "main"
		}, "PROVIDER_DISTRIBUTION_REVISION_INVALID"},
		{"distribution revision uppercase", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			provider.Distributions[0].Revision = strings.Repeat("A", 40)
		}, "PROVIDER_DISTRIBUTION_REVISION_INVALID"},
		{"distribution digest unprefixed", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			provider.Distributions[0].TreeDigest = strings.Repeat("a", 64)
		}, "PROVIDER_DISTRIBUTION_DIGEST_INVALID"},
		{"binding digest uppercase", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			provider.Bindings[0].TreeDigest = "sha256:" + strings.Repeat("A", 64)
		}, "PROVIDER_BINDING_DIGEST_INVALID"},
		{"content root empty", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			provider.Bindings[0].ContentRoot = ""
		}, "PROVIDER_BINDING_PATH_INVALID"},
		{"install root dot", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			provider.Bindings[0].InstallRoot = "skill/./child"
		}, "PROVIDER_BINDING_PATH_INVALID"},
		{"capability binding missing", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			provider.Capabilities[0].BindingRefs[0] = "missing"
		}, "PROVIDER_BINDING_NOT_FOUND"},
		{"capability binding repeated", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			provider.Capabilities[0].BindingRefs = append(provider.Capabilities[0].BindingRefs, "binding")
		}, "CAPABILITY_BINDING_AMBIGUOUS"},
		{"taxonomy unsupported", func(_ *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			recipe.TaxonomyVersion = "oaw.lifecycle-taxonomy/v2"
		}, "RECIPE_TAXONOMY_UNSUPPORTED"},
		{"slot omitted", func(_ *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			recipe.Slots = recipe.Slots[:len(recipe.Slots)-1]
		}, "RECIPE_SLOT_COVERAGE_INVALID"},
		{"slot duplicated", func(_ *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			recipe.Slots[1].SlotID = recipe.Slots[0].SlotID
		}, "RECIPE_SLOT_COVERAGE_INVALID"},
		{"mandatory owner none", func(_ *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			recipe.Slots[0].OutcomeOwner = OutcomeOwner{Kind: OwnerNone}
		}, "OUTCOME_OWNER_MISSING"},
		{"owner step missing", func(_ *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			recipe.Slots[0].OutcomeOwner.StepID = "missing"
		}, "OUTCOME_OWNER_MISSING"},
		{"owner ambiguous", func(provider *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			addHelperBinding(provider, "second-owner", InvocationModel, true)
			provider.Bindings[len(provider.Bindings)-1].StageSpan = []SlotID{SlotProblemFraming}
			recipe.Slots[0].Pipeline = append(recipe.Slots[0].Pipeline, PipelineStep{ID: "second", Selector: BindingSelector{ProviderID: provider.ID, BindingID: "second-owner"}, StageSpan: []SlotID{SlotProblemFraming}, RequiredInputArtifact: "artifact", ProducedOutputArtifact: "artifact"})
			provider.Bindings[len(provider.Bindings)-1].Responsibilities[0].SlotID = SlotProblemFraming
		}, "OUTCOME_OWNER_AMBIGUOUS"},
		{"pipeline artifact mismatch", func(provider *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			addHelperBinding(provider, "helper", InvocationModel, false)
			helper := &provider.Bindings[len(provider.Bindings)-1]
			helper.StageSpan = []SlotID{SlotProblemFraming}
			recipe.Slots[0].Pipeline = append(recipe.Slots[0].Pipeline, PipelineStep{ID: "second", Selector: BindingSelector{ProviderID: provider.ID, BindingID: "helper"}, StageSpan: []SlotID{SlotProblemFraming}, RequiredInputArtifact: "other", ProducedOutputArtifact: "artifact"})
		}, "PIPELINE_ARTIFACT_INCOMPATIBLE"},
		{"binding span empty", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			provider.Bindings[0].StageSpan = []SlotID{}
		}, "STAGE_SPAN_INVALID"},
		{"binding span reversed", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			provider.Bindings[0].StageSpan = []SlotID{SlotSolutionSpecification, SlotProblemFraming}
		}, "STAGE_SPAN_INVALID"},
		{"internal call span outside parent", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			addHelperBinding(provider, "helper", InvocationModel, false)
			provider.Bindings[0].StageSpan = []SlotID{SlotImplementation}
			provider.Bindings[0].InternalCalls = []InternalCall{{BindingID: "helper", Required: true, Mode: InternalCreditOnly, StageSpan: []SlotID{SlotCloseout}}}
		}, "STAGE_SPAN_INVALID"},
		{"internal call mode invalid", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			addHelperBinding(provider, "helper", InvocationModel, false)
			provider.Bindings[0].InternalCalls = []InternalCall{{BindingID: "helper", Required: true, Mode: "invalid", StageSpan: []SlotID{SlotImplementation}}}
		}, "INTERNAL_CALL_MODE_INVALID"},
		{"dispatch target internal", func(provider *ProviderDescriptorRecord, _ *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			addHelperBinding(provider, "helper", InvocationInternal, false)
			provider.Bindings[0].InternalCalls = []InternalCall{{BindingID: "helper", Required: true, Mode: InternalDispatchBefore, StageSpan: []SlotID{SlotImplementation}}}
		}, "INTERNAL_CALL_NOT_HOST_CALLABLE"},
		{"macro child peer conflict", func(provider *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			addHelperBinding(provider, "helper", InvocationModel, false)
			helper := &provider.Bindings[len(provider.Bindings)-1]
			helper.StageSpan = []SlotID{SlotImplementation}
			provider.Bindings[0].InternalCalls = []InternalCall{{BindingID: "helper", Required: true, Mode: InternalCreditOnly, StageSpan: []SlotID{SlotImplementation}}}
			recipe.Slots[4].Pipeline = append(recipe.Slots[4].Pipeline, PipelineStep{ID: "helper", Selector: BindingSelector{ProviderID: provider.ID, BindingID: "helper"}, StageSpan: []SlotID{SlotImplementation}, RequiredInputArtifact: "artifact", ProducedOutputArtifact: "artifact"})
		}, "MACRO_INTERNAL_CONFLICT"},
		{"host action used as selector", func(_ *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			recipe.Slots[0].Pipeline[0].Selector = BindingSelector{ProviderID: "host/actions", BindingID: "workspace.prepare-or-confirm"}
		}, "PROVIDER_BINDING_NOT_FOUND"},
		{"incident handler missing", func(_ *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			recipe.IncidentRoutes = []IncidentRoute{{IncidentType: "build-failure", Handler: BindingSelector{ProviderID: "test/provider", BindingID: "missing"}, ReturnTo: SlotImplementation, IfUnavailable: IncidentStop}}
		}, "INCIDENT_HANDLER_UNAVAILABLE"},
		{"overlay alternative missing", func(_ *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			recipe.Overlays = []OverlayRecord{{ID: "inline", Precedence: []string{"inline"}, PausedBindings: []BindingSelector{}, SelectedAlternative: "missing", Rationale: "test"}}
		}, "OVERLAY_INVALID"},
		{"overlay pauses mandatory call", func(provider *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			addHelperBinding(provider, "helper", InvocationModel, false)
			provider.Bindings[0].InternalCalls = []InternalCall{{BindingID: "helper", Required: true, Mode: InternalCreditOnly, StageSpan: []SlotID{SlotImplementation}}}
			provider.Bindings[0].Alternatives = []string{"helper"}
			recipe.Overlays = []OverlayRecord{{ID: "inline", Precedence: []string{"inline"}, PausedBindings: []BindingSelector{{ProviderID: provider.ID, BindingID: "helper"}}, SelectedAlternative: "helper", Rationale: "test"}}
		}, "OVERLAY_INVALID"},
		{"alias recipe missing", func(_ *ProviderDescriptorRecord, _ *ProfileRecipeRecord, aliases *[]ProfileAliasRecord) {
			*aliases = []ProfileAliasRecord{{Alias: "SP-FULL", RecipeID: "test/missing"}}
		}, "ALIAS_RECIPE_NOT_FOUND"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := validProviderV4Record()
			recipe := validRecipeV3Record()
			aliases := []ProfileAliasRecord{{Alias: "SP-FULL", RecipeID: recipe.ID}}
			test.mutate(&provider, &recipe, &aliases)
			if _, err := New([]ProviderDescriptorRecord{provider}, []ProfileRecipeRecord{recipe}, aliases); err == nil || !strings.Contains(err.Error(), test.code) {
				t.Fatalf("New() error = %v, want %s", err, test.code)
			}
		})
	}
}

func TestCatalogV4NormalizesSetLikeOrder(t *testing.T) {
	provider := validProviderV4Record()
	recipe := validRecipeV3Record()
	provider.Distributions = append(provider.Distributions, DistributionRecord{ID: "other", SourceURI: "https://example.test/other", Revision: strings.Repeat("b", 40), TreeDigest: "sha256:" + strings.Repeat("b", 64)})
	provider.Discovery[0].Hosts = []string{"codex", "claude"}
	provider.Bindings[0].MaximumEffects = []string{"read-project", "run-process"}
	provider.Bindings[0].Resources = []string{"project", "project-worktree"}
	provider.Bindings[0].SupportedTopologies = []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
	recipe.StableBoundaries = []string{"between-slots", "after-review"}
	recipe.EnvironmentRequirements = []execution.EnvironmentRequirement{
		{Surface: "skills", Required: true, AcceptedDispositions: []execution.EnvironmentDisposition{execution.DispositionInherited, execution.DispositionHostConfigured}},
		{Surface: "tools", Required: false, AcceptedDispositions: []execution.EnvironmentDisposition{}},
	}
	first, err := New([]ProviderDescriptorRecord{provider}, []ProfileRecipeRecord{recipe}, nil)
	if err != nil {
		t.Fatal(err)
	}
	reorderedProvider := cloneProvider(provider)
	reorderedRecipe := cloneRecipe(recipe)
	reverse(reorderedProvider.Distributions)
	reverse(reorderedProvider.Discovery[0].Hosts)
	reverse(reorderedProvider.Bindings[0].MaximumEffects)
	reverse(reorderedProvider.Bindings[0].Resources)
	reverse(reorderedProvider.Bindings[0].SupportedTopologies)
	reverse(reorderedRecipe.StableBoundaries)
	reverse(reorderedRecipe.EnvironmentRequirements)
	reverse(reorderedRecipe.EnvironmentRequirements[1].AcceptedDispositions)
	second, err := New([]ProviderDescriptorRecord{reorderedProvider}, []ProfileRecipeRecord{reorderedRecipe}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() != second.Digest() {
		t.Fatalf("set-like reorder changed digest: %s != %s", first.Digest(), second.Digest())
	}
	if reflect.DeepEqual(provider, first.Providers()[0]) {
		t.Fatal("fixture did not exercise canonical set ordering")
	}
}

func TestNormalizeAndDigestRecipeMatchesCatalogRecord(t *testing.T) {
	provider := validProviderV4Record()
	recipe := validRecipeV3Record()
	normalized, digest, err := NormalizeAndDigestRecipe([]ProviderDescriptorRecord{provider}, recipe)
	if err != nil {
		t.Fatalf("NormalizeAndDigestRecipe() error = %v", err)
	}
	value, err := New([]ProviderDescriptorRecord{provider}, []ProfileRecipeRecord{recipe}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fromCatalog := value.Recipes()[0]
	if normalized.ID != fromCatalog.ID || digest == "" {
		t.Fatalf("normalized = %#v, digest = %q", normalized, digest)
	}
	normalized.Slots[0].Pipeline[0].ID = "changed"
	if value.Recipes()[0].Slots[0].Pipeline[0].ID == "changed" {
		t.Fatal("normalized Recipe shares Catalog storage")
	}
}

func TestCatalogV4RejectsMissingBindingAliasAndOwner(t *testing.T) {
	provider := validProviderV4Record()
	recipe := validRecipeV3Record()
	tests := []struct {
		name    string
		mutate  func(*ProviderDescriptorRecord, *ProfileRecipeRecord, *[]ProfileAliasRecord)
		wantErr string
	}{
		{"missing binding", func(_ *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			recipe.Slots[0].Pipeline[0].Selector.BindingID = "missing"
		}, "PROVIDER_BINDING_NOT_FOUND"},
		{"missing owner", func(_ *ProviderDescriptorRecord, recipe *ProfileRecipeRecord, _ *[]ProfileAliasRecord) {
			recipe.Slots[0].OutcomeOwner.StepID = "missing"
		}, "OUTCOME_OWNER_MISSING"},
		{"missing alias recipe", func(_ *ProviderDescriptorRecord, _ *ProfileRecipeRecord, aliases *[]ProfileAliasRecord) {
			*aliases = []ProfileAliasRecord{{Alias: "SP-FULL", RecipeID: "test/missing"}}
		}, "ALIAS_RECIPE_NOT_FOUND"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			providerCopy := cloneProvider(provider)
			recipeCopy := cloneRecipe(recipe)
			aliases := []ProfileAliasRecord{{Alias: "SP-FULL", RecipeID: recipe.ID}}
			test.mutate(&providerCopy, &recipeCopy, &aliases)
			if _, err := New([]ProviderDescriptorRecord{providerCopy}, []ProfileRecipeRecord{recipeCopy}, aliases); err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("New() error = %v, want %s", err, test.wantErr)
			}
		})
	}
}

func TestCatalogV4DigestIncludesInstallRoot(t *testing.T) {
	provider := validProviderV4Record()
	recipe := validRecipeV3Record()
	first, err := New([]ProviderDescriptorRecord{provider}, []ProfileRecipeRecord{recipe}, nil)
	if err != nil {
		t.Fatal(err)
	}
	provider.Bindings[0].InstallRoot = "other-skill"
	second, err := New([]ProviderDescriptorRecord{provider}, []ProfileRecipeRecord{recipe}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() == second.Digest() {
		t.Fatal("InstallRoot did not affect Catalog digest")
	}
}

func reverse[T any](values []T) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
