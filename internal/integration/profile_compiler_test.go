package integration_test

import (
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

var profileAliases = []string{"SP-FULL", "MATT-FULL", "ECC-FULL", "MATT-SP-HYBRID"}

func TestAllFourBuiltInAliasesCompileWithCompleteCodexEvidence(t *testing.T) {
	fixture := completeCodexEvidence(t)
	for _, alias := range profileAliases {
		t.Run(alias, func(t *testing.T) {
			graph, _ := profileCompile(t, fixture, alias)
			assertV4Graph(t, fixture, graph)
		})
	}
}

func TestAllFourBuiltInAliasesCompileWithCompleteClaudeEvidence(t *testing.T) {
	fixture := completeClaudeEvidence(t)
	for _, alias := range profileAliases {
		t.Run(alias, func(t *testing.T) {
			graph, _ := profileCompile(t, fixture, alias)
			assertV4Graph(t, fixture, graph)
			for _, slot := range graph.Slots {
				for _, unit := range slot.Pipeline {
					if !strings.HasPrefix(unit.BindingID, "claude-") {
						t.Fatalf("non-Claude Binding in Claude graph: %#v", unit)
					}
				}
			}
		})
	}
}

func TestBuiltInGraphsMatchDeclaredMatrix(t *testing.T) {
	fixture := completeCodexEvidence(t)
	for _, alias := range profileAliases {
		t.Run(alias, func(t *testing.T) {
			graph, _ := profileCompile(t, fixture, alias)
			declared := declaredMatrixProfile(t, fixture, alias)
			for slotIndex, compiled := range graph.Slots {
				projected := declared.Slots[slotIndex]
				if projected.SlotID != compiled.SlotID || projected.OutcomeOwner != compiledOwnerIdentity(compiled.OutcomeOwner) {
					t.Fatalf("slot owner mismatch: matrix %#v graph %#v", projected, compiled.OutcomeOwner)
				}
				if projected.HostActionID != compiledHostActionID(compiled) {
					t.Fatalf("Host action mismatch for %s: %q != %q", compiled.SlotID, projected.HostActionID, compiledHostActionID(compiled))
				}
				gates := make([]string, len(compiled.Gates))
				for index, gate := range compiled.Gates {
					gates[index] = gate.ID
				}
				sort.Strings(gates)
				if !slices.Equal(gates, projected.GateIDs) {
					t.Fatalf("gate mismatch for %s: %v != %v", compiled.SlotID, gates, projected.GateIDs)
				}
				assertCompiledPipelineInMatrix(t, compiled, projected)
			}
		})
	}
}

func TestMattGraphRequiresFreshExplicitInvocationAttestation(t *testing.T) {
	graph, _ := profileCompile(t, completeCodexEvidence(t), "MATT-FULL")
	want := map[string]bool{
		"codex-grill-with-docs": false, "codex-to-spec": false,
		"codex-to-tickets": false, "codex-implement": false,
	}
	for _, slot := range graph.Slots {
		for _, unit := range slot.Pipeline {
			if _, found := want[unit.BindingID]; found {
				want[unit.BindingID] = unit.RequiresExplicitInvocation
			}
		}
	}
	for bindingID, explicit := range want {
		if !explicit {
			t.Errorf("Matt Binding %s did not retain explicit-invocation requirement", bindingID)
		}
	}
}

func TestMattGraphUsesNeutralHostActionsForWorkspaceVerificationAndCloseout(t *testing.T) {
	graph, _ := profileCompile(t, completeCodexEvidence(t), "MATT-FULL")
	want := map[catalog.SlotID]string{
		catalog.SlotWorkspacePreparation: "workspace.prepare-or-confirm",
		catalog.SlotFreshVerification:    "verification.execute",
		catalog.SlotCloseout:             "closeout.execute",
	}
	for slotID, actionID := range want {
		slot := profileCompiledSlot(t, graph, slotID)
		if slot.OutcomeOwner.Kind != catalog.OwnerHostAction || slot.OutcomeOwner.HostActionID != actionID || slot.HostAction == nil || slot.HostAction.ID != actionID {
			t.Fatalf("Matt neutral action %s = %#v", slotID, slot)
		}
	}
}

func TestMattGraphHasNoFictionalRequirementsVerificationOrCompletionBinding(t *testing.T) {
	graph, _ := profileCompile(t, completeCodexEvidence(t), "MATT-FULL")
	for _, slot := range graph.Slots {
		for _, unit := range slot.Pipeline {
			identity := unit.BindingID + "\x00" + unit.Reference
			for _, forbidden := range []string{"requirements", "verification-loop", "completion"} {
				if strings.Contains(identity, forbidden) {
					t.Errorf("Matt graph contains fictional %q Binding: %#v", forbidden, unit)
				}
			}
		}
	}
}

func TestSuperpowersSDDRequiresChildAndNestedDelegation(t *testing.T) {
	current := completeCodexEvidence(t)
	recipe := sddRecipeClone(t, current, "oaw/delivery", "user/sp-sdd")
	compileCustomRecipe(t, current, recipe)

	tests := []struct {
		name     string
		topology execution.Topology
		feature  host.FeatureID
	}{
		{"child", execution.TopologyCurrent, host.FeatureChildDelegation},
		{"nested child", execution.TopologySubagent, host.FeatureNestedChildDelegation},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := completeEvidenceWithOptions(t, profileFixtureOptions{
				hostID: "codex", topology: test.topology, unavailableFeatures: map[host.FeatureID]bool{test.feature: true},
			})
			result := compileCustomRecipeResult(t, fixture, recipe)
			assertDiagnostic(t, result, "HOST_FEATURE_UNATTESTED", "codex-subagent-driven-development")
		})
	}
}

func TestSuperpowersBuiltInInlineKeepsReviewerChildRequirement(t *testing.T) {
	fixture := completeEvidenceWithOptions(t, profileFixtureOptions{
		hostID: "codex", topology: execution.TopologyCurrent,
		unavailableFeatures: map[host.FeatureID]bool{host.FeatureChildDelegation: true},
	})
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, profileCompileRequest(t, fixture, "SP-FULL", profileRecipeFor(t, fixture.catalog, "SP-FULL")))
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnostic(t, result, "HOST_FEATURE_UNATTESTED", "codex-requesting-code-review")
	for _, diagnostic := range result.Diagnostics() {
		if diagnostic.BindingID == "codex-executing-plans" {
			t.Fatalf("inline executor incorrectly required child delegation: %#v", diagnostic)
		}
	}
}

func TestSuperpowersMacroInternalsAreDispatchedExactlyOnce(t *testing.T) {
	fixture := completeCodexEvidence(t)
	graph := compileCustomRecipe(t, fixture, sddRecipeClone(t, fixture, "oaw/delivery", "user/sp-sdd-count"))
	want := map[string]int{
		"codex-using-git-worktrees": 1, "codex-finishing-a-development-branch": 1,
		"codex-test-driven-development": 1, "codex-verification-before-completion": 1,
	}
	counts := graphBindingCounts(graph)
	for bindingID, expected := range want {
		if counts[bindingID] != expected {
			t.Errorf("%s count = %d, want %d", bindingID, counts[bindingID], expected)
		}
	}
	if counts["codex-requesting-code-review"] != 0 {
		t.Fatalf("SDD clone created fictional requesting-code-review cursor: %v", counts)
	}
}

func TestECCCodexAndClaudeSurfaceChoicesRemainDistinct(t *testing.T) {
	codex, _ := profileCompile(t, completeCodexEvidence(t), "ECC-FULL")
	claude, _ := profileCompile(t, completeClaudeEvidence(t), "ECC-FULL")
	for _, slot := range codex.Slots {
		for _, unit := range slot.Pipeline {
			if !strings.HasPrefix(unit.BindingID, "codex-") || unit.Kind == catalog.BindingAgent {
				t.Errorf("Codex graph crossed into Claude Agent surface: %#v", unit)
			}
		}
	}
	for _, slot := range claude.Slots {
		for _, unit := range slot.Pipeline {
			if !strings.HasPrefix(unit.BindingID, "claude-") || unit.Kind == catalog.BindingRole {
				t.Errorf("Claude graph crossed into Codex Role surface: %#v", unit)
			}
		}
	}
}

func TestECCCurrentCodexV1ReportsExactMissingFacts(t *testing.T) {
	fixture := currentCodexV1Evidence(t)
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, profileCompileRequest(t, fixture, "ECC-FULL", profileRecipeFor(t, fixture.catalog, "ECC-FULL")))
	if err != nil {
		t.Fatal(err)
	}
	if _, found := result.Graph(); found {
		t.Fatal("current Codex v1 evidence unexpectedly compiled ECC-FULL")
	}
	codes := diagnosticCodes(result.Diagnostics())
	for _, code := range []string{"HOST_ACTION_UNATTESTED", "PROFILE_BINDING_UNAVAILABLE"} {
		if !slices.Contains(codes, code) {
			t.Errorf("missing diagnostic %s in %v", code, result.Diagnostics())
		}
	}
}

func TestECCNeverUsesE2EAsBroadVerificationOrReviewerAsCloseout(t *testing.T) {
	graph, _ := profileCompile(t, completeCodexEvidence(t), "ECC-FULL")
	verification := profileCompiledSlot(t, graph, catalog.SlotFreshVerification)
	if strings.Contains(verification.OutcomeOwner.BindingID, "e2e") {
		t.Fatalf("ECC E2E became broad verification owner: %#v", verification.OutcomeOwner)
	}
	closeout := profileCompiledSlot(t, graph, catalog.SlotCloseout)
	if strings.Contains(closeout.OutcomeOwner.BindingID, "review") || closeout.OutcomeOwner.Kind != catalog.OwnerHostAction {
		t.Fatalf("ECC reviewer became closeout owner: %#v", closeout.OutcomeOwner)
	}
}

func TestHybridDefaultUsesMattTDDAndSPInlineReview(t *testing.T) {
	graph, _ := profileCompile(t, completeCodexEvidence(t), "MATT-SP-HYBRID")
	counts := graphBindingCounts(graph)
	for bindingID, expected := range map[string]int{
		"codex-executing-plans": 1, "codex-tdd": 1, "codex-requesting-code-review": 1,
		"codex-receiving-code-review": 1, "codex-verification-before-completion": 1,
		"codex-finishing-a-development-branch": 1,
	} {
		if counts[bindingID] != expected {
			t.Errorf("Hybrid %s count = %d, want %d", bindingID, counts[bindingID], expected)
		}
	}
	if counts["codex-subagent-driven-development"] != 0 || counts["codex-test-driven-development"] != 0 {
		t.Fatalf("Hybrid retained paused SP owners: %v", counts)
	}
	for _, provider := range graph.ProviderInstances {
		if provider.ProviderID == "oaw/ecc" {
			t.Fatal("Hybrid no-Add-on graph retained ECC Provider")
		}
	}
}

func TestHybridSDDCloneRetainsSingleMattTDD(t *testing.T) {
	fixture := completeCodexEvidence(t)
	graph := compileCustomRecipe(t, fixture, sddRecipeClone(t, fixture, "oaw/reliable-feature", "user/hybrid-sdd"))
	counts := graphBindingCounts(graph)
	if counts["codex-subagent-driven-development"] != 1 || counts["codex-tdd"] != 1 || counts["codex-test-driven-development"] != 0 || counts["codex-requesting-code-review"] != 0 {
		t.Fatalf("Hybrid SDD ownership = %v", counts)
	}
}

func TestHybridNoAddOnStopsBuildTypeAndDependencyIncidents(t *testing.T) {
	graph, _ := profileCompile(t, completeCodexEvidence(t), "MATT-SP-HYBRID")
	wanted := map[string]bool{"build-failure": false, "dependency-failure": false, "type-failure": false}
	for _, route := range graph.IncidentRoutes {
		if _, found := wanted[route.IncidentType]; !found {
			continue
		}
		wanted[route.IncidentType] = route.IfUnavailable == catalog.IncidentStop && len(route.HandlerPipeline) == 0
	}
	for incident, stopped := range wanted {
		if !stopped {
			t.Errorf("Hybrid incident %s did not stop without Add-on: %#v", incident, graph.IncidentRoutes)
		}
	}
}

func TestUSERDEFINEDCloneComposesVerifiedBindingsWithoutMutatingTemplate(t *testing.T) {
	fixture := completeCodexEvidence(t)
	base := profileRecipeFor(t, fixture.catalog, "MATT-SP-HYBRID")
	_, originalDigest, err := catalog.NormalizeAndDigestRecipe(fixture.catalog.Providers(), base)
	if err != nil {
		t.Fatal(err)
	}
	clone, err := profile.CloneRecipe(fixture.catalog, base.ID, "user/composed-hybrid", "3.1.0")
	if err != nil {
		t.Fatal(err)
	}
	planning := clone.Slots[2]
	slices.Reverse(planning.Pipeline)
	planning.OutcomeOwner.StepID = "hybrid-tickets"
	clone, err = profile.EditRecipe(clone, []profile.RecipeEdit{{SlotID: planning.SlotID, Pipeline: planning.Pipeline, OutcomeOwner: planning.OutcomeOwner}})
	if err != nil {
		t.Fatal(err)
	}
	request := builderRequestFor(t, fixture, clone)
	projection, err := profile.BuildProjection(fixture.catalog, fixture.registry, fixture.evidence, clone, profile.BuilderBaseRecipe, base.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := profile.ConfirmRecipe(fixture.catalog, fixture.registry, fixture.evidence, clone, request, projection, projection.ConfirmationDigest)
	if err != nil || profile.ValidateConfirmedRecipe(confirmed) != nil {
		t.Fatalf("ConfirmRecipe() = %#v, %v", confirmed, err)
	}
	_, afterDigest, _ := catalog.NormalizeAndDigestRecipe(fixture.catalog.Providers(), profileRecipeFor(t, fixture.catalog, "MATT-SP-HYBRID"))
	if originalDigest != afterDigest || fixture.matrix.Digest != fixture.matrix.ContentDigest() || confirmed.Graph.Slots[2].Pipeline[0].ProviderID != "oaw/superpowers" {
		t.Fatalf("USER-DEFINED composition mutated template or order: %#v", confirmed.Graph.Slots[2])
	}
}

func TestUSERDEFINEDUntrustedSameNameBindingIsUnavailable(t *testing.T) {
	fixture := completeCodexEvidence(t)
	delete(fixture.registry.bindings, "oaw/matt\x00codex-to-spec")
	fixture.registry.bindings["foreign/provider\x00codex-to-spec"] = registry.VerifiedBinding{
		BindingID: "codex-to-spec", Reference: "to-spec", BindingEvidenceDigest: strings.Repeat("f", 64),
	}
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, profileCompileRequest(t, fixture, "MATT-FULL", profileRecipeFor(t, fixture.catalog, "MATT-FULL")))
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnostic(t, result, "PROFILE_BINDING_UNAVAILABLE", "codex-to-spec")
}

func TestBuiltInCompilationDiagnosticsAreStableAndSorted(t *testing.T) {
	fixture := currentCodexV1Evidence(t)
	request := profileCompileRequest(t, fixture, "ECC-FULL", profileRecipeFor(t, fixture.catalog, "ECC-FULL"))
	first, err := profile.CompileProfile(fixture.catalog, fixture.registry, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := profile.CompileProfile(fixture.catalog, fixture.registry, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Diagnostics(), second.Diagnostics()) {
		t.Fatalf("diagnostics differ: %#v != %#v", first.Diagnostics(), second.Diagnostics())
	}
	keys := diagnosticSortKeys(first.Diagnostics())
	if !sort.StringsAreSorted(keys) {
		t.Fatalf("diagnostics are not sorted: %v", first.Diagnostics())
	}
}

func TestEquivalentBuiltInInputsProduceIdenticalGraphDigest(t *testing.T) {
	fixture := completeCodexEvidence(t)
	firstRecipe := profileRecipeFor(t, fixture.catalog, "MATT-SP-HYBRID")
	secondRecipe := firstRecipe
	secondRecipe.StableBoundaries = append([]string{}, firstRecipe.StableBoundaries...)
	secondRecipe.IncidentRoutes = append([]catalog.IncidentRoute{}, firstRecipe.IncidentRoutes...)
	slices.Reverse(secondRecipe.StableBoundaries)
	slices.Reverse(secondRecipe.IncidentRoutes)
	first := compileCustomRecipe(t, fixture, firstRecipe)
	second := compileCustomRecipe(t, fixture, secondRecipe)
	if first.Digest != second.Digest || first.RecipeDigest != second.RecipeDigest {
		t.Fatalf("equivalent graph digests differ: %s/%s != %s/%s", first.Digest, first.RecipeDigest, second.Digest, second.RecipeDigest)
	}
}

func assertV4Graph(t testing.TB, fixture builtInProfileFixture, graph profile.ExecutionGraphRecord) {
	t.Helper()
	if graph.SchemaVersion != profile.ExecutionGraphSchemaV4 || graph.TaxonomyVersion != catalog.TaxonomyVersionV1 || len(graph.Slots) != len(catalog.CanonicalSlots()) ||
		graph.RegistryDigest != fixture.registry.Digest() || graph.HostEvidenceDigest != fixture.evidence.Digest() {
		t.Fatalf("invalid v4 graph header: %#v", graph)
	}
	if err := profile.ValidateExecutionGraphRecord(graph); err != nil {
		t.Fatal(err)
	}
}

func declaredMatrixProfile(t testing.TB, fixture builtInProfileFixture, alias string) builtinMatrixProfile {
	t.Helper()
	for _, candidate := range fixture.matrix.Profiles {
		if candidate.Alias == alias {
			return builtinMatrixProfile{Slots: candidate.Slots}
		}
	}
	t.Fatalf("matrix profile %s not found", alias)
	return builtinMatrixProfile{}
}

type builtinMatrixProfile struct{ Slots []builtin.MatrixSlot }

func compiledOwnerIdentity(owner profile.CompiledOwner) string {
	switch owner.Kind {
	case catalog.OwnerProviderBinding:
		return owner.ProviderID + "/" + owner.BindingID
	case catalog.OwnerHostAction:
		return "host-action:" + owner.HostActionID
	case catalog.OwnerNone:
		return "none"
	default:
		return ""
	}
}

func compiledHostActionID(slot profile.CompiledSlot) string {
	if slot.HostAction == nil {
		return ""
	}
	return slot.HostAction.ID
}

func assertCompiledPipelineInMatrix(t testing.TB, compiled profile.CompiledSlot, projected builtin.MatrixSlot) {
	t.Helper()
	position := 0
	seenIncidentBindings := map[string]struct{}{}
	for _, unit := range compiled.Pipeline {
		if compiled.SlotID == catalog.SlotIncidentRecovery {
			key := unit.ProviderID + "\x00" + unit.BindingID + "\x00" + string(unit.MacroMode)
			if _, duplicate := seenIncidentBindings[key]; duplicate {
				continue
			}
			seenIncidentBindings[key] = struct{}{}
		}
		found := false
		for position < len(projected.Pipeline) {
			row := projected.Pipeline[position]
			position++
			if row.Paused || row.ProviderID != unit.ProviderID || row.BindingID != unit.BindingID || row.MacroMode != unit.MacroMode {
				continue
			}
			if row.DistributionRevision != unit.DistributionRevision || row.BindingTreeDigest != unit.BindingTreeDigest || row.Kind != unit.Kind || row.Reference != unit.Reference {
				t.Fatalf("matrix provenance drift for %#v / %#v", row, unit)
			}
			found = true
			break
		}
		if !found {
			t.Fatalf("compiled Binding %s/%s missing or reordered in matrix slot %s", unit.ProviderID, unit.BindingID, compiled.SlotID)
		}
	}
}

func sddRecipeClone(t testing.TB, fixture builtInProfileFixture, baseID, newID string) catalog.ProfileRecipeRecord {
	t.Helper()
	clone, err := profile.CloneRecipe(fixture.catalog, baseID, newID, "3.1.0")
	if err != nil {
		t.Fatal(err)
	}
	provider := profileProvider(t, fixture.catalog, "oaw/superpowers")
	binding := profileDescriptorBinding(t, provider, "codex-subagent-driven-development")
	stepFor := func(id string) catalog.PipelineStep {
		return catalog.PipelineStep{
			ID: id, Selector: catalog.BindingSelector{ProviderID: provider.ID, BindingID: binding.ID},
			StageSpan: append([]catalog.SlotID{}, binding.StageSpan...), RequiredInputArtifact: binding.InputArtifact, ProducedOutputArtifact: binding.OutputArtifact,
		}
	}
	for index := range clone.Slots {
		slot := &clone.Slots[index]
		var stepID string
		switch slot.SlotID {
		case catalog.SlotWorkspacePreparation:
			stepID = "sdd-workspace"
		case catalog.SlotImplementation:
			stepID = "sdd-implementation"
		case catalog.SlotReviewRemediation:
			stepID = "sdd-review"
		case catalog.SlotCloseout:
			stepID = "sdd-closeout"
		default:
			continue
		}
		step := stepFor(stepID)
		slot.Pipeline = []catalog.PipelineStep{step}
		slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerProviderBinding, StepID: step.ID}
	}
	clone.Overlays = []catalog.OverlayRecord{}
	return clone
}

func profileProvider(t testing.TB, available catalog.Catalog, providerID string) catalog.ProviderDescriptorRecord {
	t.Helper()
	for _, provider := range available.Providers() {
		if provider.ID == providerID {
			return provider
		}
	}
	t.Fatalf("Provider %s not found", providerID)
	return catalog.ProviderDescriptorRecord{}
}

func compileCustomRecipe(t testing.TB, fixture builtInProfileFixture, recipe catalog.ProfileRecipeRecord) profile.ExecutionGraphRecord {
	t.Helper()
	result := compileCustomRecipeResult(t, fixture, recipe)
	graph, found := result.Graph()
	if !found {
		t.Fatalf("CompileRecipe(%s) diagnostics = %#v", recipe.ID, result.Diagnostics())
	}
	assertV4Graph(t, fixture, graph)
	return graph
}

func compileCustomRecipeResult(t testing.TB, fixture builtInProfileFixture, recipe catalog.ProfileRecipeRecord) profile.CompileResult {
	t.Helper()
	request := profileCompileRequest(t, fixture, recipe.ID, recipe)
	result, err := profile.CompileRecipe(fixture.catalog, fixture.registry, recipe, request)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func graphBindingCounts(graph profile.ExecutionGraphRecord) map[string]int {
	result := map[string]int{}
	for _, slot := range graph.Slots {
		for _, unit := range slot.Pipeline {
			if unit.Disposition != profile.OmittedBySelection {
				result[unit.BindingID]++
			}
		}
	}
	return result
}

func assertDiagnostic(t testing.TB, result profile.CompileResult, code, bindingID string) {
	t.Helper()
	if _, found := result.Graph(); found {
		t.Fatalf("compile unexpectedly produced a graph while expecting %s/%s", code, bindingID)
	}
	for _, diagnostic := range result.Diagnostics() {
		if diagnostic.Code == code && diagnostic.BindingID == bindingID {
			return
		}
	}
	t.Fatalf("missing diagnostic %s/%s in %#v", code, bindingID, result.Diagnostics())
}

func diagnosticCodes(values []profile.CompileDiagnostic) []string {
	result := []string{}
	for _, diagnostic := range values {
		if !slices.Contains(result, diagnostic.Code) {
			result = append(result, diagnostic.Code)
		}
	}
	sort.Strings(result)
	return result
}

func diagnosticSortKeys(values []profile.CompileDiagnostic) []string {
	result := make([]string, len(values))
	for index, diagnostic := range values {
		result[index] = diagnostic.Code + "\x00" + string(diagnostic.SlotID) + "\x00" + diagnostic.StepID + "\x00" + diagnostic.ProviderID + "\x00" + diagnostic.BindingID + "\x00" + diagnostic.AddOnID + "\x00" + diagnostic.AlternativeID + "\x00" + diagnostic.OverlayID + "\x00" + diagnostic.IncidentType + "\x00" + string(diagnostic.Topology) + "\x00" + diagnostic.Detail
	}
	return result
}

func builderRequestFor(t testing.TB, fixture builtInProfileFixture, recipe catalog.ProfileRecipeRecord) profile.BuilderSelectionRequest {
	t.Helper()
	request := profileCompileRequest(t, fixture, recipe.ID, recipe)
	return profile.BuilderSelectionRequest{
		Profile: recipe.ID, Topology: fixture.topology, AddOns: request.AddOns,
		Alternatives: request.Alternatives, Overlays: request.Overlays,
	}
}
