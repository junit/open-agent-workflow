package builtin

import (
	"errors"
	"io/fs"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/provideraudit"
)

func TestBuiltInProviderDescriptorsV4(t *testing.T) {
	value := loadCatalog(t)
	providers := value.Providers()
	if got, want := len(providers), 3; got != want {
		t.Fatalf("provider count = %d, want %d", got, want)
	}
	for _, provider := range providers {
		if provider.SchemaVersion != catalog.ProviderDescriptorSchemaV4 || provider.DescriptorVersion != "4.0.0" {
			t.Errorf("%s version = %q/%q", provider.ID, provider.SchemaVersion, provider.DescriptorVersion)
		}
		if len(provider.Distributions) != 1 {
			t.Errorf("%s Distribution count = %d, want 1", provider.ID, len(provider.Distributions))
		}
	}
}

func TestBuiltInProviderPinsMatchSourceAudit(t *testing.T) {
	value := loadCatalog(t)
	audit := loadAudit(t)
	for _, source := range audit.Providers {
		provider := requireProvider(t, value, source.ProviderID)
		if got := provider.Distributions[0]; got.ID != source.DistributionID || got.SourceURI != source.SourceURI || got.Revision != source.Revision || got.TreeDigest != source.DistributionTreeDigest {
			t.Errorf("%s Distribution = %#v, want source audit pin", source.ProviderID, got)
		}
		if len(provider.Bindings) != len(source.Bindings) {
			t.Fatalf("%s Binding count = %d, want %d", source.ProviderID, len(provider.Bindings), len(source.Bindings))
		}
		byID := bindingIndex(provider)
		for _, audited := range source.Bindings {
			binding, found := byID[audited.ID]
			if !found {
				t.Errorf("%s missing audited Binding %s", source.ProviderID, audited.ID)
				continue
			}
			if binding.DistributionID != source.DistributionID || binding.ContentRoot != audited.ContentRoot || binding.InstallRoot != audited.InstallRoot || binding.TreeDigest != audited.TreeDigest || string(binding.Kind) != audited.Kind || !slices.Contains(audited.References, binding.Reference) {
				t.Errorf("%s/%s does not match source audit: %#v", source.ProviderID, audited.ID, binding)
			}
		}
	}
}

func TestBuiltInHostQualifiedBindingSets(t *testing.T) {
	value := loadCatalog(t)
	audit := loadAudit(t)
	for _, source := range audit.Providers {
		provider := requireProvider(t, value, source.ProviderID)
		for _, binding := range provider.Bindings {
			host, found := strings.CutSuffix(binding.ID, "-"+strings.TrimPrefix(binding.ID, binding.Host+"-"))
			if !found || host != binding.Host || (binding.Host != "codex" && binding.Host != "claude") {
				t.Errorf("%s/%s is not Host-qualified for %q", source.ProviderID, binding.ID, binding.Host)
			}
			if binding.Surface != expectedSurface(source.ProviderID, binding.Host) {
				t.Errorf("%s/%s surface = %q", source.ProviderID, binding.ID, binding.Surface)
			}
			if _, found := audit.Binding(source.ProviderID, binding.ID); !found {
				t.Errorf("%s/%s is not audited", source.ProviderID, binding.ID)
			}
		}
	}
}

func TestMattHasOnlyAuditedBindings(t *testing.T) {
	value := loadCatalog(t)
	audit := loadAudit(t)
	provider := requireProvider(t, value, "oaw/matt")
	for _, binding := range provider.Bindings {
		audited, found := audit.Binding(provider.ID, binding.ID)
		if !found || binding.ContentRoot != audited.ContentRoot || binding.InstallRoot != audited.InstallRoot {
			t.Errorf("Matt Binding %s is not the exact audited mapping", binding.ID)
		}
		if strings.HasPrefix(binding.InstallRoot, "skills/") {
			t.Errorf("Matt Binding %s was not flattened: %q", binding.ID, binding.InstallRoot)
		}
	}
}

func TestMattRejectsFictionalRequirementsVerificationAndCompletion(t *testing.T) {
	provider := requireProvider(t, loadCatalog(t), "oaw/matt")
	for _, binding := range provider.Bindings {
		identity := binding.ID + "\x00" + binding.Reference
		for _, forbidden := range []string{"requirements", "verification-loop", "completion"} {
			if strings.Contains(identity, forbidden) {
				t.Errorf("Matt contains fictional Binding %q in %s", forbidden, identity)
			}
		}
		for _, responsibility := range binding.Responsibilities {
			if responsibility.SlotID == catalog.SlotWorkspacePreparation || responsibility.SlotID == catalog.SlotFreshVerification || responsibility.SlotID == catalog.SlotCloseout {
				t.Errorf("Matt Binding %s claims forbidden slot %s", binding.ID, responsibility.SlotID)
			}
		}
	}
}

func TestSuperpowersHasEveryAuditedReference(t *testing.T) {
	value := loadCatalog(t)
	audit := loadAudit(t)
	provider := requireProvider(t, value, "oaw/superpowers")
	for _, binding := range provider.Bindings {
		audited, found := audit.Binding(provider.ID, binding.ID)
		if !found || !reflect.DeepEqual(audited.References, []string{binding.Reference}) {
			t.Errorf("Superpowers Binding %s reference = %q, audit = %#v", binding.ID, binding.Reference, audited.References)
		}
		if !strings.HasPrefix(binding.Reference, "superpowers:") {
			t.Errorf("Superpowers Binding %s has unqualified reference %q", binding.ID, binding.Reference)
		}
	}
}

func TestSuperpowersMacroModesAreExact(t *testing.T) {
	provider := requireProvider(t, loadCatalog(t), "oaw/superpowers")
	for _, host := range []string{"codex", "claude"} {
		brainstorming := requireBinding(t, provider, host+"-brainstorming")
		assertInternalCalls(t, brainstorming, []catalog.InternalCall{{BindingID: host + "-writing-plans", Required: true, Mode: catalog.InternalDispatchAfter, StageSpan: []catalog.SlotID{catalog.SlotDeliveryPlanning}}})

		for _, executorID := range []string{host + "-subagent-driven-development", host + "-executing-plans"} {
			executor := requireBinding(t, provider, executorID)
			assertInternalCalls(t, executor, []catalog.InternalCall{
				{BindingID: host + "-using-git-worktrees", Required: true, Mode: catalog.InternalDispatchBefore, StageSpan: []catalog.SlotID{catalog.SlotWorkspacePreparation}},
				{BindingID: host + "-finishing-a-development-branch", Required: true, Mode: catalog.InternalDispatchAfter, StageSpan: []catalog.SlotID{catalog.SlotCloseout}},
			})
		}
		for _, fictional := range []string{"test-driven-development", "requesting-code-review", "verification-before-completion"} {
			for _, call := range requireBinding(t, provider, host+"-subagent-driven-development").InternalCalls {
				if strings.Contains(call.BindingID, fictional) {
					t.Errorf("SDD has fictional InternalCall %s", call.BindingID)
				}
			}
		}
	}
}

func TestECCSeparatesSkillAgentRoleInstructionAndHookEvidence(t *testing.T) {
	provider := requireProvider(t, loadCatalog(t), "oaw/ecc")
	var roles, instructions []string
	for _, binding := range provider.Bindings {
		switch binding.Kind {
		case catalog.BindingAgent:
			if binding.Host != "claude" || !strings.HasPrefix(binding.ContentRoot, "agents/") {
				t.Errorf("invalid ECC Agent Binding %#v", binding)
			}
		case catalog.BindingRole:
			if binding.Host != "codex" {
				t.Errorf("ECC Role %s has Host %q", binding.ID, binding.Host)
			}
			roles = append(roles, binding.ID)
		case catalog.BindingInstruction:
			instructions = append(instructions, binding.ContentRoot)
		}
		if strings.Contains(binding.ID, "delivery-gate") || strings.HasPrefix(binding.ContentRoot, "hooks/") {
			t.Errorf("Hook evidence became a Binding: %#v", binding)
		}
	}
	sort.Strings(roles)
	sort.Strings(instructions)
	if !reflect.DeepEqual(roles, []string{"codex-docs-researcher", "codex-explorer", "codex-reviewer"}) {
		t.Errorf("ECC Codex roles = %v", roles)
	}
	if !reflect.DeepEqual(instructions, []string{"commands/feature-dev.md", "commands/plan.md"}) {
		t.Errorf("ECC instructions = %v", instructions)
	}
}

func TestECCDoesNotMapE2EToVerificationOrReviewToCloseout(t *testing.T) {
	provider := requireProvider(t, loadCatalog(t), "oaw/ecc")
	for _, binding := range provider.Bindings {
		for _, responsibility := range binding.Responsibilities {
			if strings.Contains(binding.ID, "e2e") && responsibility.SlotID == catalog.SlotFreshVerification && responsibility.OutcomeOwner {
				t.Errorf("ECC E2E Binding %s owns broad verification", binding.ID)
			}
			if (strings.Contains(binding.ID, "reviewer") || strings.Contains(binding.ID, "code-reviewer")) && responsibility.SlotID == catalog.SlotCloseout && responsibility.OutcomeOwner {
				t.Errorf("ECC reviewer Binding %s owns closeout", binding.ID)
			}
		}
	}
}

func TestBuiltInRecipeMatrixV3(t *testing.T) {
	value := loadCatalog(t)
	recipes := value.Recipes()
	if got, want := len(recipes), 4; got != want {
		t.Fatalf("recipe count = %d, want %d", got, want)
	}
	canonical := catalog.CanonicalSlots()
	for _, recipe := range recipes {
		if recipe.SchemaVersion != catalog.ProfileRecipeSchemaV3 || recipe.TaxonomyVersion != catalog.TaxonomyVersionV1 || recipe.RecipeVersion != "3.0.0" {
			t.Errorf("%s version = %q/%q/%q", recipe.ID, recipe.SchemaVersion, recipe.TaxonomyVersion, recipe.RecipeVersion)
		}
		if len(recipe.Slots) != len(canonical) {
			t.Fatalf("%s slots = %d", recipe.ID, len(recipe.Slots))
		}
		for index, slot := range recipe.Slots {
			if slot.SlotID != canonical[index].ID {
				t.Errorf("%s slot[%d] = %s", recipe.ID, index, slot.SlotID)
			}
			if slot.Applicability == catalog.SlotMandatory && slot.OutcomeOwner.Kind == catalog.OwnerNone {
				t.Errorf("%s mandatory slot %s has no owner", recipe.ID, slot.SlotID)
			}
		}
	}
}

func TestHybridDefaultProvenanceAndPausedOwners(t *testing.T) {
	recipe := requireRecipe(t, loadCatalog(t), "oaw/reliable-feature")
	if recipe.Family != "matt-superpowers" || recipe.Template != "default" {
		t.Fatalf("Hybrid provenance = %q/%q", recipe.Family, recipe.Template)
	}
	if len(recipe.Overlays) != 1 || recipe.Overlays[0].ID != "default-inline" || recipe.Overlays[0].SelectedAlternative != "" {
		t.Fatalf("Hybrid overlays = %#v", recipe.Overlays)
	}
	paused := make([]string, 0, len(recipe.Overlays[0].PausedBindings))
	for _, selector := range recipe.Overlays[0].PausedBindings {
		paused = append(paused, selector.ProviderID+"/"+selector.BindingID)
	}
	sort.Strings(paused)
	want := []string{
		"oaw/matt/claude-code-review", "oaw/matt/codex-code-review",
		"oaw/superpowers/claude-subagent-driven-development", "oaw/superpowers/claude-test-driven-development",
		"oaw/superpowers/codex-subagent-driven-development", "oaw/superpowers/codex-test-driven-development",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(paused, want) {
		t.Errorf("Hybrid paused Bindings = %v, want %v", paused, want)
	}
}

func TestAliasesRemainExactlyFour(t *testing.T) {
	aliases := loadCatalog(t).Aliases()
	got := make(map[string]string, len(aliases))
	for _, alias := range aliases {
		got[alias.Alias] = alias.RecipeID
	}
	want := map[string]string{
		"SP-FULL": "oaw/delivery", "MATT-FULL": "oaw/domain-engineering",
		"ECC-FULL": "oaw/ecc-engineering", "MATT-SP-HYBRID": "oaw/reliable-feature",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("aliases = %v, want %v", got, want)
	}
}

func TestHardeningAndRetiredAuthorityAreAbsent(t *testing.T) {
	if _, err := fs.Stat(assets.FS(), "recipes/oaw-hardening.json"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("retired hardening asset still exists: %v", err)
	}
	value := loadCatalog(t)
	for _, recipe := range value.Recipes() {
		if recipe.ID == "oaw/hardening" {
			t.Fatal("retired oaw/hardening Recipe is active")
		}
	}
}

func TestLoadRejectsMalformedBuiltInAsset(t *testing.T) {
	files := embeddedMap(t)
	files["providers/oaw-ecc.json"] = &fstest.MapFile{Data: []byte(`{"schema_version":"oaw.provider-descriptor/v4"}`)}
	if _, err := loadFromFS(files); err == nil || !strings.Contains(err.Error(), "BUILTIN_PROVIDER_INVALID") {
		t.Fatalf("loadFromFS(malformed) error = %v", err)
	}
}

func TestLoadRejectsDescriptorOrRecipeRetiredSchema(t *testing.T) {
	files := embeddedMap(t)
	files["providers/oaw-matt.json"] = &fstest.MapFile{Data: []byte(`{"schema_version":"oaw.provider-descriptor/v3"}`)}
	if _, err := loadFromFS(files); err == nil || !strings.Contains(err.Error(), "BUILTIN_PROVIDER_INVALID") {
		t.Fatalf("loadFromFS(retired provider) error = %v", err)
	}
	files = embeddedMap(t)
	files["recipes/oaw-delivery.json"] = &fstest.MapFile{Data: []byte(`{"schema_version":"oaw.profile-recipe/v2"}`)}
	if _, err := loadFromFS(files); err == nil || !strings.Contains(err.Error(), "BUILTIN_RECIPE_INVALID") {
		t.Fatalf("loadFromFS(retired recipe) error = %v", err)
	}
}

func TestLoadRejectsSourceAuditDrift(t *testing.T) {
	files := embeddedMap(t)
	raw := string(files["audits/provider-sources-v4.json"].Data)
	raw = strings.Replace(raw, "84fdeffd12f2ee307994d1eb6feb48173b6e0502", strings.Repeat("0", 40), 1)
	files["audits/provider-sources-v4.json"] = &fstest.MapFile{Data: []byte(raw)}
	if _, err := loadFromFS(files); err == nil || !strings.Contains(err.Error(), "BUILTIN_SOURCE_AUDIT_INVALID") {
		t.Fatalf("loadFromFS(source drift) error = %v", err)
	}
}

func TestBuiltInAssetLoadIsDeterministic(t *testing.T) {
	first := loadCatalog(t)
	second := loadCatalog(t)
	if first.Digest() != second.Digest() {
		t.Fatalf("Catalog digests differ: %s != %s", first.Digest(), second.Digest())
	}
	firstAudit := loadAudit(t)
	secondAudit := loadAudit(t)
	if firstAudit.Digest != secondAudit.Digest {
		t.Fatalf("audit digests differ: %s != %s", firstAudit.Digest, secondAudit.Digest)
	}
}

func loadCatalog(t *testing.T) catalog.Catalog {
	t.Helper()
	value, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return value
}

func loadAudit(t *testing.T) provideraudit.Manifest {
	t.Helper()
	value, err := LoadSourceAudit()
	if err != nil {
		t.Fatalf("LoadSourceAudit() error = %v", err)
	}
	return value
}

func requireProvider(t *testing.T, value catalog.Catalog, id string) catalog.ProviderDescriptorRecord {
	t.Helper()
	for _, provider := range value.Providers() {
		if provider.ID == id {
			return provider
		}
	}
	t.Fatalf("Provider %s not found", id)
	return catalog.ProviderDescriptorRecord{}
}

func requireRecipe(t *testing.T, value catalog.Catalog, id string) catalog.ProfileRecipeRecord {
	t.Helper()
	for _, recipe := range value.Recipes() {
		if recipe.ID == id {
			return recipe
		}
	}
	t.Fatalf("Recipe %s not found", id)
	return catalog.ProfileRecipeRecord{}
}

func bindingIndex(provider catalog.ProviderDescriptorRecord) map[string]catalog.BindingRecord {
	result := make(map[string]catalog.BindingRecord, len(provider.Bindings))
	for _, binding := range provider.Bindings {
		result[binding.ID] = binding
	}
	return result
}

func requireBinding(t *testing.T, provider catalog.ProviderDescriptorRecord, id string) catalog.BindingRecord {
	t.Helper()
	if binding, found := bindingIndex(provider)[id]; found {
		return binding
	}
	t.Fatalf("Binding %s/%s not found", provider.ID, id)
	return catalog.BindingRecord{}
}

func assertInternalCalls(t *testing.T, binding catalog.BindingRecord, want []catalog.InternalCall) {
	t.Helper()
	if !reflect.DeepEqual(binding.InternalCalls, want) {
		t.Errorf("%s InternalCalls = %#v, want %#v", binding.ID, binding.InternalCalls, want)
	}
}

func expectedSurface(providerID, host string) string {
	if providerID == "oaw/matt" && host == "codex" {
		return "codex-user-skills"
	}
	return host + "-plugin"
}

func embeddedMap(t *testing.T) fstest.MapFS {
	t.Helper()
	files := fstest.MapFS{}
	err := fs.WalkDir(assets.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		data, readErr := fs.ReadFile(assets.FS(), path)
		if readErr != nil {
			return readErr
		}
		files[path] = &fstest.MapFile{Data: data}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
