package catalog

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

const testTreeDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestDecodeProviderV4AcceptsCompleteClosedRecord(t *testing.T) {
	raw, err := json.Marshal(validProviderV4Record())
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeProvider(raw)
	if err != nil {
		t.Fatalf("DecodeProvider() error = %v", err)
	}
	if got.SchemaVersion != ProviderDescriptorSchemaV4 || got.Bindings[0].InstallRoot != "skill" {
		t.Fatalf("DecodeProvider() = %#v", got)
	}
}

func TestDecodeRecipeV3AcceptsCompleteClosedRecord(t *testing.T) {
	raw, err := json.Marshal(validRecipeV3Record())
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeRecipe(raw)
	if err != nil {
		t.Fatalf("DecodeRecipe() error = %v", err)
	}
	if got.SchemaVersion != ProfileRecipeSchemaV3 || len(got.Slots) != len(CanonicalSlots()) {
		t.Fatalf("DecodeRecipe() = %#v", got)
	}
}

func TestDecodeAliasSetV1AcceptsCompleteClosedRecord(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-alias-set/v1","aliases":[{"alias":"SP-FULL","recipe_id":"oaw/delivery"}]}`)
	got, err := DecodeAliasSet(raw)
	if err != nil {
		t.Fatalf("DecodeAliasSet() error = %v", err)
	}
	got.Aliases[0].RecipeID = "changed/value"
	again, err := DecodeAliasSet(raw)
	if err != nil {
		t.Fatalf("DecodeAliasSet() second error = %v", err)
	}
	if again.Aliases[0].RecipeID != "oaw/delivery" {
		t.Fatal("DecodeAliasSet reused caller-visible storage")
	}
}

func TestDecodeAuthorityRejectsRetiredSchemas(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		run  func([]byte) error
		code string
	}{
		{"provider v3", []byte(`{"schema_version":"oaw.provider-descriptor/v3"}`), func(raw []byte) error { _, err := DecodeProvider(raw); return err }, "UNSUPPORTED_PROVIDER_SCHEMA"},
		{"recipe v2", []byte(`{"schema_version":"oaw.profile-recipe/v2"}`), func(raw []byte) error { _, err := DecodeRecipe(raw); return err }, "UNSUPPORTED_RECIPE_SCHEMA"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(test.raw); err == nil || !strings.Contains(err.Error(), test.code) {
				t.Fatalf("error = %v, want %s", err, test.code)
			}
		})
	}
}

func TestDecodeProviderV4RejectsUnknownField(t *testing.T) {
	value := validProviderV4Record()
	raw, _ := json.Marshal(value)
	raw = append(raw[:len(raw)-1], []byte(`,"unknown":true}`)...)
	if _, err := DecodeProvider(raw); err == nil || !strings.Contains(err.Error(), "INVALID_PROVIDER_DESCRIPTOR") {
		t.Fatalf("unknown-field error = %v", err)
	}
}

func TestDecodeRecipeV3RejectsUnknownFieldAndTrailingValue(t *testing.T) {
	raw, _ := json.Marshal(validRecipeV3Record())
	raw = append(raw[:len(raw)-1], []byte(`,"provider_id":"forbidden"}`)...)
	if _, err := DecodeRecipe(raw); err == nil || !strings.Contains(err.Error(), "INVALID_PROFILE_RECIPE") {
		t.Fatalf("unknown-field error = %v", err)
	}
	raw, _ = json.Marshal(validRecipeV3Record())
	raw = append(raw, []byte(` {}`)...)
	if _, err := DecodeRecipe(raw); err == nil || !strings.Contains(err.Error(), "INVALID_PROFILE_RECIPE") {
		t.Fatalf("trailing-value error = %v", err)
	}

	raw, _ = json.Marshal(validRecipeV3Record())
	raw = []byte(strings.Replace(string(raw), `"authority":"host"`, `"authority":"host","provider_id":"test/provider"`, 1))
	if _, err := DecodeRecipe(raw); err == nil || !strings.Contains(err.Error(), "INVALID_PROFILE_RECIPE") {
		t.Fatalf("gate-selector error = %v", err)
	}
}

func TestDecodeAliasSetV1RejectsUnknownFieldAndInvalidIdentity(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		code string
	}{
		{"unknown field", `{"schema_version":"oaw.profile-alias-set/v1","aliases":[{"alias":"SP-FULL","recipe_id":"oaw/delivery"}],"extra":true}`, "INVALID_PROFILE_ALIAS_SET"},
		{"invalid alias", `{"schema_version":"oaw.profile-alias-set/v1","aliases":[{"alias":"sp-full","recipe_id":"oaw/delivery"}]}`, "INVALID_PROFILE_ALIAS_SET"},
		{"invalid recipe id", `{"schema_version":"oaw.profile-alias-set/v1","aliases":[{"alias":"SP-FULL","recipe_id":"delivery"}]}`, "INVALID_PROFILE_ALIAS_SET"},
		{"duplicate alias", `{"schema_version":"oaw.profile-alias-set/v1","aliases":[{"alias":"SP-FULL","recipe_id":"oaw/delivery"},{"alias":"SP-FULL","recipe_id":"oaw/other"}]}`, "DUPLICATE_PROFILE_ALIAS"},
		{"nil aliases", `{"schema_version":"oaw.profile-alias-set/v1","aliases":null}`, "INVALID_PROFILE_ALIAS_SET"},
		{"duplicate field", `{"schema_version":"oaw.profile-alias-set/v1","schema_version":"oaw.profile-alias-set/v1","aliases":[{"alias":"SP-FULL","recipe_id":"oaw/delivery"}]}`, "INVALID_PROFILE_ALIAS_SET"},
		{"trailing value", `{"schema_version":"oaw.profile-alias-set/v1","aliases":[{"alias":"SP-FULL","recipe_id":"oaw/delivery"}]} {}`, "INVALID_PROFILE_ALIAS_SET"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeAliasSet([]byte(test.raw)); err == nil || !strings.Contains(err.Error(), test.code) {
				t.Fatalf("DecodeAliasSet() error = %v, want %s", err, test.code)
			}
		})
	}
}

func TestDecodeProviderV4RejectsInvalidDistributionBindingAndMacroShapes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ProviderDescriptorRecord)
		code   string
	}{
		{"branch revision", func(value *ProviderDescriptorRecord) { value.Distributions[0].Revision = "main" }, "PROVIDER_DISTRIBUTION_REVISION_INVALID"},
		{"distribution digest", func(value *ProviderDescriptorRecord) { value.Distributions[0].TreeDigest = strings.Repeat("a", 64) }, "PROVIDER_DISTRIBUTION_DIGEST_INVALID"},
		{"binding digest", func(value *ProviderDescriptorRecord) { value.Bindings[0].TreeDigest = strings.Repeat("a", 64) }, "PROVIDER_BINDING_DIGEST_INVALID"},
		{"absolute content root", func(value *ProviderDescriptorRecord) { value.Bindings[0].ContentRoot = "/absolute" }, "PROVIDER_BINDING_PATH_INVALID"},
		{"escaping install root", func(value *ProviderDescriptorRecord) { value.Bindings[0].InstallRoot = "../escape" }, "PROVIDER_BINDING_PATH_INVALID"},
		{"empty install root", func(value *ProviderDescriptorRecord) { value.Bindings[0].InstallRoot = "" }, "PROVIDER_BINDING_PATH_INVALID"},
		{"invalid internal mode", func(value *ProviderDescriptorRecord) {
			addHelperBinding(value, "helper", InvocationModel, false)
			value.Bindings[0].InternalCalls = []InternalCall{{BindingID: "helper", Required: true, Mode: "invalid", StageSpan: []SlotID{SlotImplementation}}}
		}, "INTERNAL_CALL_MODE_INVALID"},
		{"empty internal span", func(value *ProviderDescriptorRecord) {
			addHelperBinding(value, "helper", InvocationModel, false)
			value.Bindings[0].InternalCalls = []InternalCall{{BindingID: "helper", Required: true, Mode: InternalCreditOnly, StageSpan: []SlotID{}}}
		}, "STAGE_SPAN_INVALID"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := validProviderV4Record()
			test.mutate(&value)
			raw, _ := json.Marshal(value)
			if _, err := DecodeProvider(raw); err == nil || !strings.Contains(err.Error(), test.code) {
				t.Fatalf("DecodeProvider() error = %v, want %s", err, test.code)
			}
		})
	}
}

func TestDecodeProviderV4AcceptsExplicitNetworkWriteEffect(t *testing.T) {
	value := validProviderV4Record()
	value.Bindings[0].MaximumEffects = append(value.Bindings[0].MaximumEffects, "network-write")
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeProvider(raw); err != nil {
		t.Fatalf("DecodeProvider() rejected explicit network-write effect: %v", err)
	}
}

func TestDecodeRecipeV3RejectsInvalidOwnerGateIncidentAndOverlayShapes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ProfileRecipeRecord)
		code   string
	}{
		{"invalid owner kind", func(value *ProfileRecipeRecord) { value.Slots[0].OutcomeOwner.Kind = "invalid" }, "OUTCOME_OWNER_MISSING"},
		{"mandatory none", func(value *ProfileRecipeRecord) { value.Slots[0].OutcomeOwner = OutcomeOwner{Kind: OwnerNone} }, "OUTCOME_OWNER_MISSING"},
		{"invalid host owner", func(value *ProfileRecipeRecord) {
			value.Slots[0].Pipeline = []PipelineStep{}
			value.Slots[0].OutcomeOwner = OutcomeOwner{Kind: OwnerHostAction, HostAction: "workspace.prepare-or-confirm"}
			value.Slots[0].HostAction = &HostActionRef{ID: "workspace.prepare-or-confirm", InputArtifact: "artifact", OutputArtifact: "artifact"}
		}, "OUTCOME_OWNER_MISSING"},
		{"invalid gate authority", func(value *ProfileRecipeRecord) {
			value.Slots[0].Gates = []GateRecord{{ID: "gate", Authority: "provider", Predicate: "ready", EvidenceRequirements: []EvidenceRequirementRecord{}}}
		}, "INVALID_PROFILE_RECIPE"},
		{"invalid incident fallback", func(value *ProfileRecipeRecord) {
			value.IncidentRoutes = []IncidentRoute{{IncidentType: "build-failure", Handler: BindingSelector{ProviderID: "test/provider", BindingID: "binding"}, ReturnTo: SlotImplementation, IfUnavailable: "continue"}}
		}, "INVALID_PROFILE_RECIPE"},
		{"empty overlay precedence", func(value *ProfileRecipeRecord) {
			value.Overlays = []OverlayRecord{{ID: "overlay", Precedence: nil, PausedBindings: []BindingSelector{}, SelectedAlternative: "binding", Rationale: "test"}}
		}, "OVERLAY_INVALID"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := validRecipeV3Record()
			test.mutate(&value)
			raw, _ := json.Marshal(value)
			if _, err := DecodeRecipe(raw); err == nil || !strings.Contains(err.Error(), test.code) {
				t.Fatalf("DecodeRecipe() error = %v, want %s", err, test.code)
			}
		})
	}
}

func TestDecodedV4RecordsOwnAllNestedStorage(t *testing.T) {
	providerRaw, _ := json.Marshal(validProviderV4Record())
	firstProvider, err := DecodeProvider(providerRaw)
	if err != nil {
		t.Fatal(err)
	}
	firstProvider.Bindings[0].StageSpan[0] = SlotCloseout
	firstProvider.Bindings[0].Responsibilities[0].Name = "changed"
	secondProvider, _ := DecodeProvider(providerRaw)
	if secondProvider.Bindings[0].StageSpan[0] != SlotProblemFraming || secondProvider.Bindings[0].Responsibilities[0].Name == "changed" {
		t.Fatal("DecodeProvider reused nested storage")
	}

	recipeRaw, _ := json.Marshal(validRecipeV3Record())
	firstRecipe, err := DecodeRecipe(recipeRaw)
	if err != nil {
		t.Fatal(err)
	}
	firstRecipe.Slots[0].Pipeline[0].StageSpan[0] = SlotCloseout
	firstRecipe.Slots[3].HostAction.ID = "changed"
	secondRecipe, _ := DecodeRecipe(recipeRaw)
	if secondRecipe.Slots[0].Pipeline[0].StageSpan[0] != SlotProblemFraming || secondRecipe.Slots[3].HostAction.ID == "changed" {
		t.Fatal("DecodeRecipe reused nested storage")
	}
}

func validProviderV4Record() ProviderDescriptorRecord {
	claims := []ResponsibilityClaim{
		{OwnershipStage, "problem-framing", SlotProblemFraming, true},
		{OwnershipStage, "solution-specification", SlotSolutionSpecification, true},
		{OwnershipStage, "delivery-planning", SlotDeliveryPlanning, true},
		{OwnershipStage, "implementation", SlotImplementation, true},
		{OwnershipProcedure, "implementation-tdd", SlotImplementationTDD, true},
		{OwnershipAssurance, "review-remediation", SlotReviewRemediation, true},
		{OwnershipProcedure, "fresh-verification", SlotFreshVerification, true},
	}
	return ProviderDescriptorRecord{
		SchemaVersion: ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0", ID: "test/provider", DisplayName: "Test Provider",
		Distributions: []DistributionRecord{{ID: "distribution", SourceURI: "https://example.test/provider", Revision: strings.Repeat("a", 40), TreeDigest: testTreeDigest}},
		Discovery:     []DiscoveryProbe{{ID: "probe", Hosts: []string{"codex"}, Surface: "codex-skills", DistributionID: "distribution", Kind: "path-exists", Root: "user-home", CandidatePath: ".agents/skills", EvidencePath: "skill/SKILL.md"}},
		Bindings: []BindingRecord{{
			ID: "binding", DistributionID: "distribution", ContentRoot: "skills/skill", InstallRoot: "skill", TreeDigest: testTreeDigest,
			Host: "codex", Surface: "codex-skills", Kind: BindingSkill, Reference: "skill", Invocation: InvocationModel,
			Responsibilities: claims, InputArtifact: "artifact", OutputArtifact: "artifact", MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
			SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Delegation: DelegationRequirements{},
			StageSpan: canonicalSlotIDs(), InternalCalls: []InternalCall{}, Alternatives: []string{}, Conflicts: []string{},
		}},
		Capabilities: []CapabilityRecord{{ID: "workflow", InputSchema: "artifact", OutcomeSchema: "artifact", RequestModes: []RequestMode{RequestModeWorkflow}, BindingRefs: []string{"binding"}}},
	}
}

func validRecipeV3Record() ProfileRecipeRecord {
	slots := CanonicalSlots()
	result := ProfileRecipeRecord{
		SchemaVersion: ProfileRecipeSchemaV3, TaxonomyVersion: TaxonomyVersionV1, RecipeVersion: "3.0.0", ID: "test/recipe", DisplayName: "Test Recipe", Family: "test", Template: "",
		Slots: make([]SlotRecipe, len(slots)), AddOns: []AddOnRecord{}, IncidentRoutes: []IncidentRoute{}, Overlays: []OverlayRecord{}, StableBoundaries: []string{"between-slots"}, EnvironmentRequirements: []execution.EnvironmentRequirement{},
	}
	for index, definition := range slots {
		transitions := []RecipeTransition{}
		if index+1 < len(slots) {
			transitions = []RecipeTransition{{Signal: "succeeded", Target: slots[index+1].ID}}
		}
		slot := SlotRecipe{SlotID: definition.ID, Applicability: SlotMandatory, Pipeline: []PipelineStep{}, Gates: []GateRecord{}, Transitions: transitions}
		switch definition.ID {
		case SlotWorkspacePreparation:
			slot.HostAction = &HostActionRef{ID: "workspace.prepare-or-confirm", InputArtifact: "artifact", OutputArtifact: "artifact"}
			slot.OutcomeOwner = OutcomeOwner{Kind: OwnerHostAction, HostAction: slot.HostAction.ID}
			slot.Gates = []GateRecord{{ID: "workspace-ready", Authority: GateHost, Predicate: "workspace-ready", EvidenceRequirements: []EvidenceRequirementRecord{}}}
		case SlotIncidentRecovery:
			slot.Applicability = SlotConditional
			slot.OutcomeOwner = OutcomeOwner{Kind: OwnerNone}
		case SlotCloseout:
			slot.HostAction = &HostActionRef{ID: "closeout.execute", InputArtifact: "artifact", OutputArtifact: "artifact"}
			slot.OutcomeOwner = OutcomeOwner{Kind: OwnerHostAction, HostAction: slot.HostAction.ID}
			slot.Gates = []GateRecord{{ID: "user-closeout", Authority: GateUser, Predicate: "user-authorized", EvidenceRequirements: []EvidenceRequirementRecord{}}}
		default:
			step := PipelineStep{ID: "main", Selector: BindingSelector{ProviderID: "test/provider", BindingID: "binding"}, StageSpan: []SlotID{definition.ID}, RequiredInputArtifact: "artifact", ProducedOutputArtifact: "artifact"}
			slot.Pipeline = []PipelineStep{step}
			slot.OutcomeOwner = OutcomeOwner{Kind: OwnerProviderBinding, StepID: step.ID}
		}
		result.Slots[index] = slot
	}
	return result
}

func addHelperBinding(provider *ProviderDescriptorRecord, id string, invocation InvocationDisposition, outcomeOwner bool) {
	binding := provider.Bindings[0]
	binding.ID = id
	binding.Reference = id
	binding.Invocation = invocation
	binding.Responsibilities = []ResponsibilityClaim{{Namespace: OwnershipStage, Name: "helper", SlotID: SlotImplementation, OutcomeOwner: outcomeOwner}}
	binding.StageSpan = []SlotID{SlotImplementation}
	binding.InternalCalls = []InternalCall{}
	binding.Alternatives = []string{}
	binding.Conflicts = []string{}
	provider.Bindings = append(provider.Bindings, binding)
	provider.Capabilities[0].BindingRefs = append(provider.Capabilities[0].BindingRefs, id)
}

func canonicalSlotIDs() []SlotID {
	definitions := CanonicalSlots()
	result := make([]SlotID, len(definitions))
	for index := range definitions {
		result[index] = definitions[index].ID
	}
	return result
}
