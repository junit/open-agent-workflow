package policyroute_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
	"github.com/wifibaby4u/open-agent-workflow/internal/policyroute"
)

var (
	mattSkills = []string{
		"grill-with-docs",
		"grilling",
		"domain-modeling",
		"to-spec",
		"to-tickets",
		"implement",
		"tdd",
		"diagnosing-bugs",
		"code-review",
	}
	superpowersSkills = []string{
		"brainstorming",
		"writing-plans",
		"using-git-worktrees",
		"executing-plans",
		"test-driven-development",
		"systematic-debugging",
		"requesting-code-review",
		"receiving-code-review",
		"verification-before-completion",
		"finishing-a-development-branch",
	}
	eccSkills = []string{
		"intent-driven-development",
		"product-capability",
		"blueprint",
		"git-workflow",
		"tdd-workflow",
		"verification-loop",
	}
)

func TestInspectMakesEveryBuiltInProfileRoutableWithoutBridge(t *testing.T) {
	home := completeCodexHome(t)
	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}

	module := policyflow.New()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	for _, profileID := range []policyflow.ProfileID{
		policyflow.ProfileSPFull,
		policyflow.ProfileMattFull,
		policyflow.ProfileECCFull,
		policyflow.ProfileMattSPHybrid,
	} {
		profile := requireProfile(t, offer, profileID)
		if !profile.PolicySelectable || !profile.HostRoutable || len(profile.Missing) != 0 {
			t.Errorf("profile %s = %#v", profileID, profile)
			continue
		}
		if _, err := module.Start(inventory, policyflow.Selection{OfferRef: offer.Ref, Profile: profileID}); err != nil {
			t.Errorf("start %s: %v", profileID, err)
		}
	}
}

func TestInspectRecognizesMattSkillsWithoutLockfileOrDigestMetadata(t *testing.T) {
	home := newHome(t)
	writeSkills(t, filepath.Join(home, ".agents", "skills"), mattSkills...)
	writeFile(t, filepath.Join(home, ".agents", ".skill-lock.json"), "not valid metadata")

	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	module := policyflow.New()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	profile := requireProfile(t, offer, policyflow.ProfileMattFull)
	if !profile.HostRoutable {
		t.Fatalf("MATT-FULL missing routes = %#v", profile.Missing)
	}
}

func TestInspectPreservesMattExplicitInvocation(t *testing.T) {
	home := newHome(t)
	writeSkills(t, filepath.Join(home, ".agents", "skills"), mattSkills...)

	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	routes := inventoryByName(inventory)
	for _, name := range []string{"grill-with-docs", "to-spec", "to-tickets", "implement", "tdd", "diagnosing-bugs", "code-review"} {
		if got := routes[name].Mode; got != policyflow.UserExplicit {
			t.Errorf("route %s mode = %q, want %q", name, got, policyflow.UserExplicit)
		}
	}
	for _, name := range []string{"grilling", "domain-modeling"} {
		if got := routes[name].Mode; got != policyflow.UserExplicit {
			t.Errorf("route %s mode = %q, want %q", name, got, policyflow.UserExplicit)
		}
	}
}

func TestInspectOutputContainsOnlyCooperativeRouteFacts(t *testing.T) {
	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: completeCodexHome(t)})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(inventory)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(raw))
	for _, reserved := range []string{"provider", "binding", "attestation", "bundle", "grant", "lease", "receipt", "revision", "digest", "lockfile"} {
		if strings.Contains(lower, reserved) {
			t.Errorf("route inventory contains reserved machine-authority term %q: %s", reserved, raw)
		}
	}
}

func TestInspectReadsEnabledCodexPluginCaches(t *testing.T) {
	home := newHome(t)
	writeCodexConfig(t, home, true, true)
	writeSkills(t, filepath.Join(home, ".codex", "plugins", "cache", "openai-api-curated", "superpowers", "v1", "skills"), superpowersSkills...)
	writeSkills(t, filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc", "v1", "skills"), eccSkills...)
	writeSkills(t, filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc", "v1", ".codex-plugin", "migrated-command-skills"), "source-command-review-pr")

	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	routes := inventoryByName(inventory)
	for _, name := range superpowersSkills {
		assertRouteMode(t, routes, "superpowers:"+name, policyflow.UserExplicit)
	}
	for _, name := range eccSkills {
		assertRouteMode(t, routes, "ecc:"+name, policyflow.UserExplicit)
	}
	if _, found := routes["ecc:source-command-review-pr"]; found {
		t.Fatal("PR-only ECC Skill entered the generic Policy route inventory")
	}
}

func TestInspectSupportsECCPluginDirectoryVariants(t *testing.T) {
	tests := []struct {
		name string
		root func(string) string
	}{
		{
			name: "current cache",
			root: func(home string) string {
				return filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc", "v1")
			},
		},
		{
			name: "legacy cache",
			root: func(home string) string {
				return filepath.Join(home, ".codex", "plugins", "cache", "everything-claude-code", "ecc", "v1")
			},
		},
		{
			name: "direct",
			root: func(home string) string {
				return filepath.Join(home, ".codex", "plugins", "ecc")
			},
		},
		{
			name: "legacy direct",
			root: func(home string) string {
				return filepath.Join(home, ".codex", "plugins", "everything-claude-code", "ecc")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := newHome(t)
			writeCodexConfig(t, home, false, true)
			root := test.root(home)
			writeSkills(t, filepath.Join(root, "skills"), eccSkills...)

			inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
			if err != nil {
				t.Fatal(err)
			}
			profile := requireProfileFromInventory(t, inventory, policyflow.ProfileECCFull)
			if !profile.HostRoutable {
				t.Fatalf("ECC-FULL missing routes = %#v", profile.Missing)
			}
		})
	}
}

func TestInspectIgnoresDisabledPluginCache(t *testing.T) {
	home := newHome(t)
	writeCodexConfig(t, home, false, false)
	writeSkills(t, filepath.Join(home, ".codex", "plugins", "cache", "openai-api-curated", "superpowers", "v1", "skills"), superpowersSkills...)
	writeSkills(t, filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc", "v1", "skills"), eccSkills...)

	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range inventory {
		if strings.HasPrefix(route.Name, "superpowers:") || strings.HasPrefix(route.Name, "ecc:") {
			t.Errorf("disabled plugin route reported: %#v", route)
		}
	}
}

func TestInspectMissingSkillOnlyRemovesItsRoute(t *testing.T) {
	home := completeCodexHome(t)
	missingPath := filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc", "v1", "skills", "blueprint", "SKILL.md")
	if err := os.Remove(missingPath); err != nil {
		t.Fatal(err)
	}

	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if _, found := inventoryByName(inventory)["ecc:blueprint"]; found {
		t.Fatal("missing ECC Skill remained in route inventory")
	}
	module := policyflow.New()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	for _, profileID := range []policyflow.ProfileID{policyflow.ProfileSPFull, policyflow.ProfileMattFull, policyflow.ProfileMattSPHybrid} {
		if profile := requireProfile(t, offer, profileID); !profile.HostRoutable {
			t.Errorf("unrelated profile %s missing routes = %#v", profileID, profile.Missing)
		}
	}
	ecc := requireProfile(t, offer, policyflow.ProfileECCFull)
	if ecc.HostRoutable || len(ecc.Missing) != 1 || ecc.Missing[0].Name != "ecc:blueprint" {
		t.Fatalf("ECC-FULL = %#v", ecc)
	}
}

func TestInspectReturnsStableSortedDeduplicatedInventory(t *testing.T) {
	home := completeCodexHome(t)
	writeSkills(t, filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc", "v2", "skills"), eccSkills...)
	writeSkills(t, filepath.Join(home, ".codex", "plugins", "cache", "openai-api-curated", "superpowers", "v2", "skills"), superpowersSkills...)

	first, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	second, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("inventory changed between inspections:\nfirst=%#v\nsecond=%#v", first, second)
	}
	names := make([]string, len(first))
	seen := map[string]bool{}
	for index, route := range first {
		if seen[route.Name] {
			t.Errorf("duplicate route %q", route.Name)
		}
		seen[route.Name] = true
		names[index] = route.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("routes are not sorted: %v", names)
	}
}

func TestInspectDoesNotMergeSkillsAcrossPluginVersions(t *testing.T) {
	home := newHome(t)
	writeCodexConfig(t, home, false, true)
	oldRoot := filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc", "v1", "skills")
	newRoot := filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc", "v2", "skills")
	writeSkills(t, oldRoot, "intent-driven-development", "product-capability", "blueprint")
	writeSkills(t, newRoot, "git-workflow", "tdd-workflow", "verification-loop")

	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	profile := requireProfileFromInventory(t, inventory, policyflow.ProfileECCFull)
	if profile.HostRoutable {
		t.Fatalf("split plugin versions formed a synthetic ECC-FULL: %#v", profile)
	}
	for _, missing := range []string{"ecc:intent-driven-development", "ecc:product-capability", "ecc:blueprint"} {
		if _, found := inventoryByName(inventory)[missing]; found {
			t.Errorf("route from unselected old version was merged: %s", missing)
		}
	}
}

func TestInspectOrdersSemanticPluginVersionsAndKeepsSelectionAtomic(t *testing.T) {
	home := newHome(t)
	writeCodexConfig(t, home, false, true)
	parent := filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc")
	writeSkills(t, filepath.Join(parent, "2.0.0", "skills"),
		"intent-driven-development", "product-capability", "blueprint")
	writeSkills(t, filepath.Join(parent, "10.0.0", "skills"),
		"git-workflow", "tdd-workflow", "verification-loop")

	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	routes := inventoryByName(inventory)
	for _, name := range []string{"ecc:git-workflow", "ecc:tdd-workflow", "ecc:verification-loop"} {
		if _, found := routes[name]; !found {
			t.Errorf("route from selected 10.0.0 installation is missing: %s", name)
		}
	}
	for _, name := range []string{"ecc:intent-driven-development", "ecc:product-capability", "ecc:blueprint"} {
		if _, found := routes[name]; found {
			t.Errorf("route from unselected 2.0.0 installation was merged: %s", name)
		}
	}
}

func TestInspectTreatsNonSemanticPluginVersionsAsOpaque(t *testing.T) {
	home := newHome(t)
	writeCodexConfig(t, home, false, true)
	parent := filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc")
	writeSkills(t, filepath.Join(parent, "v10", "skills"), "intent-driven-development")
	writeSkills(t, filepath.Join(parent, "v2", "skills"), "git-workflow")

	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	routes := inventoryByName(inventory)
	if _, found := routes["ecc:git-workflow"]; !found {
		t.Fatal("route from Host-selected opaque v2 installation is missing")
	}
	if _, found := routes["ecc:intent-driven-development"]; found {
		t.Fatal("route from unselected opaque v10 installation was merged")
	}
}

func TestInspectPrefersLocalPluginVersion(t *testing.T) {
	home := newHome(t)
	writeCodexConfig(t, home, false, true)
	parent := filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc")
	writeSkills(t, filepath.Join(parent, "99.0.0", "skills"), "intent-driven-development")
	writeSkills(t, filepath.Join(parent, "local", "skills"), "git-workflow")

	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	routes := inventoryByName(inventory)
	if _, found := routes["ecc:git-workflow"]; !found {
		t.Fatal("route from Host-selected local installation is missing")
	}
	if _, found := routes["ecc:intent-driven-development"]; found {
		t.Fatal("route from unselected semantic version was merged")
	}
}

func TestInspectOrdersSemanticBuildMetadataLikeHost(t *testing.T) {
	home := newHome(t)
	writeCodexConfig(t, home, false, true)
	parent := filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc")
	writeSkills(t, filepath.Join(parent, "1.0.0+2", "skills"), "intent-driven-development")
	writeSkills(t, filepath.Join(parent, "1.0.0+10", "skills"), "git-workflow")

	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	routes := inventoryByName(inventory)
	if _, found := routes["ecc:git-workflow"]; !found {
		t.Fatal("route from Host-selected build metadata version is missing")
	}
	if _, found := routes["ecc:intent-driven-development"]; found {
		t.Fatal("route from unselected build metadata version was merged")
	}
}

func TestInspectPrefersCurrentCacheParentAtomically(t *testing.T) {
	home := newHome(t)
	writeCodexConfig(t, home, false, true)
	writeSkills(t, filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc", "v1", "skills"),
		"intent-driven-development")
	writeSkills(t, filepath.Join(home, ".codex", "plugins", "cache", "everything-claude-code", "ecc", "v99", "skills"),
		"git-workflow")

	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	routes := inventoryByName(inventory)
	if _, found := routes["ecc:intent-driven-development"]; !found {
		t.Fatal("route from preferred current cache installation is missing")
	}
	if _, found := routes["ecc:git-workflow"]; found {
		t.Fatal("route from unselected legacy cache installation was merged")
	}
}

func TestInspectPrefersDirectInstallationOverCacheAtomically(t *testing.T) {
	home := newHome(t)
	writeCodexConfig(t, home, false, true)
	writeSkills(t, filepath.Join(home, ".codex", "plugins", "ecc", "skills"),
		"intent-driven-development")
	writeSkills(t, filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc", "v99", "skills"),
		"git-workflow")

	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	routes := inventoryByName(inventory)
	if _, found := routes["ecc:intent-driven-development"]; !found {
		t.Fatal("route from preferred direct installation is missing")
	}
	if _, found := routes["ecc:git-workflow"]; found {
		t.Fatal("route from unselected cache installation was merged")
	}
}

func TestInspectFilesystemRoutesNeverClaimCurrentSessionVisibility(t *testing.T) {
	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: completeCodexHome(t)})
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range inventory {
		if route.Mode == policyflow.HostVisible {
			t.Errorf("filesystem inspection claimed current-session visibility: %#v", route)
		}
	}
}

func completeCodexHome(t *testing.T) string {
	t.Helper()
	home := newHome(t)
	writeCodexConfig(t, home, true, true)
	writeSkills(t, filepath.Join(home, ".agents", "skills"), mattSkills...)
	writeSkills(t, filepath.Join(home, ".codex", "plugins", "cache", "openai-api-curated", "superpowers", "v1", "skills"), superpowersSkills...)
	writeSkills(t, filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc", "v1", "skills"), eccSkills...)
	return home
}

func newHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	return home
}

func writeCodexConfig(t *testing.T, home string, superpowers, ecc bool) {
	t.Helper()
	document := "[plugins.\"superpowers@openai-api-curated\"]\nenabled = " + boolText(superpowers) + "\n\n" +
		"[plugins.\"ecc@ecc\"]\nenabled = " + boolText(ecc) + "\n"
	writeFile(t, filepath.Join(home, ".codex", "config.toml"), document)
}

func writeSkills(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, name := range names {
		writeFile(t, filepath.Join(root, name, "SKILL.md"), "---\nname: "+name+"\n---\n")
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func inventoryByName(inventory policyflow.RouteInventory) map[string]policyflow.Route {
	result := make(map[string]policyflow.Route, len(inventory))
	for _, route := range inventory {
		result[route.Name] = route
	}
	return result
}

func assertRouteMode(t *testing.T, routes map[string]policyflow.Route, name string, mode policyflow.InvocationMode) {
	t.Helper()
	route, found := routes[name]
	if !found {
		t.Errorf("route %q is missing", name)
		return
	}
	if route.Mode != mode {
		t.Errorf("route %q mode = %q, want %q", name, route.Mode, mode)
	}
}

func requireProfileFromInventory(t *testing.T, inventory policyflow.RouteInventory, id policyflow.ProfileID) policyflow.ProfileOffer {
	t.Helper()
	module := policyflow.New()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	return requireProfile(t, offer, id)
}

func requireProfile(t *testing.T, offer policyflow.Offer, id policyflow.ProfileID) policyflow.ProfileOffer {
	t.Helper()
	for _, profile := range offer.Profiles {
		if profile.ID == id {
			return profile
		}
	}
	t.Fatalf("profile %s is missing", id)
	return policyflow.ProfileOffer{}
}
