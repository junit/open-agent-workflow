package policyflow_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
)

func TestOfferMakesAllBuiltInProfilesRoutableFromHostRoutes(t *testing.T) {
	module := policyflow.New()
	offer, err := module.Offer(completeInventory())
	if err != nil {
		t.Fatal(err)
	}

	want := []policyflow.ProfileID{
		policyflow.ProfileSPFull,
		policyflow.ProfileMattFull,
		policyflow.ProfileECCFull,
		policyflow.ProfileMattSPHybrid,
	}
	if len(offer.Profiles) != len(want) {
		t.Fatalf("profiles = %#v", offer.Profiles)
	}
	for index, profileID := range want {
		profile := offer.Profiles[index]
		if profile.ID != profileID || !profile.PolicySelectable || !profile.HostRoutable || len(profile.Missing) != 0 {
			t.Errorf("profile[%d] = %#v", index, profile)
		}
	}

	matt := requireProfile(t, offer, policyflow.ProfileMattFull)
	grill := requireRoute(t, matt, "grill-with-docs")
	if grill.Mode != policyflow.UserExplicit {
		t.Fatalf("grill-with-docs mode = %q", grill.Mode)
	}
	implement := requireRoute(t, matt, "implement")
	if implement.Mode != policyflow.UserExplicit {
		t.Fatalf("implement mode = %q", implement.Mode)
	}
}

func TestMattFullUsesExplicitCompositeSkillsOnce(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := module.Start(inventory, policyflow.Selection{
		OfferRef: offer.Ref,
		Profile:  policyflow.ProfileMattFull,
	})
	if err != nil {
		t.Fatal(err)
	}
	if progress.Plan.Topology != policyflow.TopologyCurrent || progress.Plan.AddOn != policyflow.AddOnNone {
		t.Fatalf("plan = %#v", progress.Plan)
	}

	routes, hostActions, gates := traverse(t, module, inventory, progress)
	if routes["grill-with-docs"] != 1 || routes["implement"] != 1 {
		t.Fatalf("composite route counts = %v", routes)
	}
	for _, credited := range []string{"grilling", "domain-modeling", "tdd", "code-review"} {
		if routes[credited] != 0 {
			t.Errorf("credited internal route %q invoked %d times", credited, routes[credited])
		}
	}
	if hostActions["workspace.prepare-or-confirm"] != 1 || hostActions["verification.execute"] != 1 || hostActions["closeout.execute"] != 1 {
		t.Fatalf("host actions = %v", hostActions)
	}
	if gates["shared-understanding"] != 1 || gates["specification-approved"] != 1 || gates["delivery-plan-approved"] != 1 || gates["user-closeout"] != 1 {
		t.Fatalf("gates = %v", gates)
	}
}

func TestMattCompositeRequiresItsCreditedSkillContracts(t *testing.T) {
	module := policyflow.New()
	for _, missing := range []string{"grilling", "domain-modeling", "tdd", "code-review"} {
		inventory := removeRoute(completeInventory(), missing)
		offer, err := module.Offer(inventory)
		if err != nil {
			t.Fatal(err)
		}
		profile := requireProfile(t, offer, policyflow.ProfileMattFull)
		if profile.HostRoutable || !hasMissing(profile, missing) {
			t.Errorf("MATT-FULL without %s = %#v", missing, profile)
		}
	}
}

func TestMattCompositeRequirementsExposeTheirCreditedSlots(t *testing.T) {
	offer, err := policyflow.New().Offer(completeInventory())
	if err != nil {
		t.Fatal(err)
	}
	matt := requireProfile(t, offer, policyflow.ProfileMattFull)
	want := map[string][]policyflow.LifecycleSlot{
		"grilling":        {policyflow.SlotProblemFraming},
		"domain-modeling": {policyflow.SlotProblemFraming},
		"tdd":             {policyflow.SlotImplementationTDD},
		"code-review":     {policyflow.SlotReviewRemediation},
	}
	for route, slots := range want {
		status := requireRoute(t, matt, route)
		if got := status.Covers; !reflect.DeepEqual(got, slots) {
			t.Errorf("Matt requirement %s covers = %v, want %v", route, got, slots)
		}
		if !status.Credited {
			t.Errorf("Matt requirement %s is not marked as credited", route)
		}
	}
}

func TestRepeatedRouteExposesEveryCreditedSlot(t *testing.T) {
	offer, err := policyflow.New().Offer(completeInventory())
	if err != nil {
		t.Fatal(err)
	}
	ecc := requireProfile(t, offer, policyflow.ProfileECCFull)
	got := requireRoute(t, ecc, "ecc:git-workflow").Covers
	want := []policyflow.LifecycleSlot{
		policyflow.SlotWorkspacePreparation,
		policyflow.SlotCloseout,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ECC git-workflow covers = %v, want %v", got, want)
	}
}

func TestMissingRepeatedRouteRetainsEveryCreditedSlot(t *testing.T) {
	inventory := removeRoute(completeInventory(), "ecc:git-workflow")
	offer, err := policyflow.New().Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	ecc := requireProfile(t, offer, policyflow.ProfileECCFull)
	var got []policyflow.LifecycleSlot
	for _, route := range ecc.Missing {
		if route.Name == "ecc:git-workflow" {
			got = route.Covers
			break
		}
	}
	want := []policyflow.LifecycleSlot{
		policyflow.SlotWorkspacePreparation,
		policyflow.SlotCloseout,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("missing ECC git-workflow covers = %v, want %v", got, want)
	}
}

func TestECCUsesOnlyPublicCodexSkillRoutesAndCreditsTDDSpan(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	profile := requireProfile(t, offer, policyflow.ProfileECCFull)
	for _, route := range profile.Routes {
		if route.Kind == policyflow.RouteSkill && len(route.Name) >= 4 && route.Name[:4] != "ecc:" {
			t.Errorf("ECC skill route is not a public Codex ECC skill: %#v", route)
		}
	}
	for _, forbidden := range []string{"reviewer", "/plan", "planner", "architect"} {
		if hasRoute(profile, forbidden) {
			t.Errorf("ECC offer contains non-Skill Host surface %q", forbidden)
		}
	}

	progress, err := module.Start(inventory, policyflow.Selection{OfferRef: offer.Ref, Profile: policyflow.ProfileECCFull})
	if err != nil {
		t.Fatal(err)
	}
	routes, _, _ := traverse(t, module, inventory, progress)
	if routes["ecc:tdd-workflow"] != 1 {
		t.Fatalf("ECC tdd-workflow count = %d; routes = %v", routes["ecc:tdd-workflow"], routes)
	}
	if routes["ecc:source-command-review-pr"] != 0 {
		t.Fatalf("PR-only ECC review route was invoked; routes = %v", routes)
	}
}

func TestSPBrainstormingCoversFramingAndSpecificationOnce(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := module.Start(inventory, policyflow.Selection{OfferRef: offer.Ref, Profile: policyflow.ProfileSPFull})
	if err != nil {
		t.Fatal(err)
	}

	first, ok := progress.Next.(policyflow.InvokeSkill)
	if !ok {
		t.Fatalf("first work = %T", progress.Next)
	}
	if first.Skill != "superpowers:brainstorming" || !reflect.DeepEqual(first.Covers, []policyflow.LifecycleSlot{
		policyflow.SlotProblemFraming,
		policyflow.SlotSolutionSpecification,
	}) {
		t.Fatalf("first work = %#v", first)
	}
	routes, _, _ := traverse(t, module, inventory, progress)
	if routes["superpowers:brainstorming"] != 1 {
		t.Fatalf("brainstorming count = %d", routes["superpowers:brainstorming"])
	}
}

func TestCompositeSkillsExposeTheirCompleteCreditedCoverage(t *testing.T) {
	tests := []struct {
		profile policyflow.ProfileID
		skill   string
		want    []policyflow.LifecycleSlot
	}{
		{
			profile: policyflow.ProfileMattFull,
			skill:   "implement",
			want: []policyflow.LifecycleSlot{
				policyflow.SlotImplementation,
				policyflow.SlotImplementationTDD,
				policyflow.SlotReviewRemediation,
			},
		},
		{
			profile: policyflow.ProfileECCFull,
			skill:   "ecc:tdd-workflow",
			want: []policyflow.LifecycleSlot{
				policyflow.SlotImplementation,
				policyflow.SlotImplementationTDD,
			},
		},
	}
	for _, test := range tests {
		t.Run(string(test.profile), func(t *testing.T) {
			module := policyflow.New()
			inventory := completeInventory()
			offer, err := module.Offer(inventory)
			if err != nil {
				t.Fatal(err)
			}
			progress, err := module.Start(inventory, policyflow.Selection{OfferRef: offer.Ref, Profile: test.profile})
			if err != nil {
				t.Fatal(err)
			}
			got := findSkillWork(t, module, inventory, progress, test.skill)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("%s covers = %v, want %v", test.skill, got, test.want)
			}
		})
	}
}

func TestHybridInvokesMattTDDOnce(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := module.Start(inventory, policyflow.Selection{OfferRef: offer.Ref, Profile: policyflow.ProfileMattSPHybrid})
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for {
		switch next := progress.Next.(type) {
		case policyflow.InvokeSkill:
			if next.Skill == "tdd" {
				count++
			}
			progress, err = module.Apply(inventory, successfulSkillEvent(next.WorkRef, next.Review))
		case policyflow.AwaitUserSkill:
			if next.Skill == "tdd" {
				count++
			}
			progress, err = module.Apply(inventory, successfulSkillEvent(next.WorkRef, next.Review))
		case policyflow.HostAction:
			progress, err = module.Apply(inventory, successfulHostActionEvent(next.WorkRef, next.Review))
		case policyflow.UserGate:
			progress, err = module.Apply(inventory, policyflow.UserGateApproved{GateRef: next.GateRef})
		case policyflow.HostGate:
			progress, err = module.Apply(inventory, policyflow.HostGateSatisfied{GateRef: next.GateRef})
		case policyflow.Done:
			if count != 1 {
				t.Fatalf("hybrid Matt tdd count = %d, want 1", count)
			}
			return
		default:
			t.Fatalf("next = %T", progress.Next)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestHybridUsesDeclaredMattAndSuperpowersOwnership(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := module.Start(inventory, policyflow.Selection{OfferRef: offer.Ref, Profile: policyflow.ProfileMattSPHybrid})
	if err != nil {
		t.Fatal(err)
	}

	routes, _, _ := traverse(t, module, inventory, progress)
	for _, route := range []string{"grill-with-docs", "to-spec", "to-tickets", "tdd"} {
		if routes[route] != 1 {
			t.Errorf("hybrid Matt route %q count = %d", route, routes[route])
		}
	}
	for _, route := range []string{
		"superpowers:writing-plans",
		"superpowers:using-git-worktrees",
		"superpowers:executing-plans",
		"superpowers:requesting-code-review",
		"superpowers:receiving-code-review",
		"superpowers:verification-before-completion",
		"superpowers:finishing-a-development-branch",
	} {
		if routes[route] != 1 {
			t.Errorf("hybrid Superpowers route %q count = %d", route, routes[route])
		}
	}
	for _, paused := range []string{"implement", "superpowers:test-driven-development", "code-review"} {
		if routes[paused] != 0 {
			t.Errorf("hybrid paused route %q invoked %d times", paused, routes[paused])
		}
	}
}

func TestApplyRejectsWrongEventKindAndConsumedReference(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := module.Start(inventory, policyflow.Selection{OfferRef: offer.Ref, Profile: policyflow.ProfileSPFull})
	if err != nil {
		t.Fatal(err)
	}
	first := progress.Next.(policyflow.InvokeSkill)

	_, err = module.Apply(inventory, policyflow.HostActionCompleted{WorkRef: first.WorkRef})
	requireFailureCode(t, err, policyflow.FailureEventOutOfOrder)

	progress, err = module.Apply(inventory, policyflow.SkillCompleted{WorkRef: first.WorkRef})
	if err != nil {
		t.Fatal(err)
	}
	_, err = module.Apply(inventory, policyflow.SkillCompleted{WorkRef: first.WorkRef})
	requireFailureCode(t, err, policyflow.FailureEventOutOfOrder)
	if _, ok := progress.Next.(policyflow.UserGate); !ok {
		t.Fatalf("next after brainstorming = %T", progress.Next)
	}
}

func TestStartRejectsMissingRouteAndStaleOffer(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}

	withoutImplement := removeRoute(inventory, "implement")
	_, err = module.Start(withoutImplement, policyflow.Selection{OfferRef: offer.Ref, Profile: policyflow.ProfileMattFull})
	requireFailureCode(t, err, policyflow.FailureOfferStale)

	missingOffer, err := module.Offer(withoutImplement)
	if err != nil {
		t.Fatal(err)
	}
	profile := requireProfile(t, missingOffer, policyflow.ProfileMattFull)
	if profile.HostRoutable || !hasMissing(profile, "implement") {
		t.Fatalf("MATT-FULL = %#v", profile)
	}
	_, err = module.Start(withoutImplement, policyflow.Selection{OfferRef: missingOffer.Ref, Profile: policyflow.ProfileMattFull})
	requireFailureCode(t, err, policyflow.FailureProfileIncomplete)
}

func TestPolicyOfferDependsOnlyOnHostRoutes(t *testing.T) {
	module := policyflow.New()
	want, err := module.Offer(completeInventory())
	if err != nil {
		t.Fatal(err)
	}

	// Reordering an otherwise identical observation simulates a different Host
	// inventory source. Bridge state, Provider provenance, lockfiles, paths,
	// revisions, and tree hashes cannot be supplied to this Interface.
	reordered := append(policyflow.RouteInventory(nil), completeInventory()...)
	for left, right := 0, len(reordered)-1; left < right; left, right = left+1, right-1 {
		reordered[left], reordered[right] = reordered[right], reordered[left]
	}
	got, err := module.Offer(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if got.Ref != want.Ref || !reflect.DeepEqual(got.Profiles, want.Profiles) {
		t.Fatalf("same Host routes produced different offers: want=%#v got=%#v", want, got)
	}
}

func TestOfferReferenceBindsProfileSemanticsAndRouteInventory(t *testing.T) {
	offer, err := policyflow.New().Offer(completeInventory())
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(offer.Ref.String(), "-")
	if len(parts) != 4 || parts[0] != "policy" || parts[1] != "offer" || len(parts[2]) != 24 || len(parts[3]) != 24 {
		t.Fatalf("offer ref does not contain semantics and inventory fingerprints: %q", offer.Ref)
	}
}

func TestPolicyProjectionContainsNoMachineAuthorityTerms(t *testing.T) {
	module := policyflow.New()
	offer, err := module.Offer(completeInventory())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(offer)
	if err != nil {
		t.Fatal(err)
	}
	projection := strings.ToLower(string(raw))
	for _, reserved := range []string{
		"provider_id", "binding_id", "bundle", "grant", "lease", "receipt", "machine_revision",
	} {
		if strings.Contains(projection, reserved) {
			t.Errorf("Policy projection contains reserved machine term %q: %s", reserved, raw)
		}
	}
}

func traverse(t *testing.T, module *policyflow.Module, inventory policyflow.RouteInventory, progress policyflow.Progress) (map[string]int, map[string]int, map[string]int) {
	t.Helper()
	routes := map[string]int{}
	hostActions := map[string]int{}
	gates := map[string]int{}
	for steps := 0; steps < 64; steps++ {
		var err error
		switch next := progress.Next.(type) {
		case policyflow.InvokeSkill:
			routes[next.Skill]++
			progress, err = module.Apply(inventory, successfulSkillEvent(next.WorkRef, next.Review))
		case policyflow.AwaitUserSkill:
			routes[next.Skill]++
			progress, err = module.Apply(inventory, successfulSkillEvent(next.WorkRef, next.Review))
		case policyflow.HostAction:
			hostActions[next.Action]++
			progress, err = module.Apply(inventory, successfulHostActionEvent(next.WorkRef, next.Review))
		case policyflow.UserGate:
			gates[next.Gate]++
			progress, err = module.Apply(inventory, policyflow.UserGateApproved{GateRef: next.GateRef})
		case policyflow.HostGate:
			gates[next.Gate]++
			progress, err = module.Apply(inventory, policyflow.HostGateSatisfied{GateRef: next.GateRef})
		case policyflow.Done:
			return routes, hostActions, gates
		default:
			t.Fatalf("next = %T", progress.Next)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("workflow did not terminate")
	return nil, nil, nil
}

func findSkillWork(t *testing.T, module *policyflow.Module, inventory policyflow.RouteInventory, progress policyflow.Progress, skill string) []policyflow.LifecycleSlot {
	t.Helper()
	for steps := 0; steps < 64; steps++ {
		var err error
		switch next := progress.Next.(type) {
		case policyflow.InvokeSkill:
			if next.Skill == skill {
				return next.Covers
			}
			progress, err = module.Apply(inventory, successfulSkillEvent(next.WorkRef, next.Review))
		case policyflow.AwaitUserSkill:
			if next.Skill == skill {
				return next.Covers
			}
			progress, err = module.Apply(inventory, successfulSkillEvent(next.WorkRef, next.Review))
		case policyflow.HostAction:
			progress, err = module.Apply(inventory, successfulHostActionEvent(next.WorkRef, next.Review))
		case policyflow.UserGate:
			progress, err = module.Apply(inventory, policyflow.UserGateApproved{GateRef: next.GateRef})
		case policyflow.HostGate:
			progress, err = module.Apply(inventory, policyflow.HostGateSatisfied{GateRef: next.GateRef})
		case policyflow.Done:
			t.Fatalf("skill %q not found", skill)
		default:
			t.Fatalf("next = %T", progress.Next)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatalf("skill %q not found before traversal limit", skill)
	return nil
}

func completeInventory() policyflow.RouteInventory {
	hostVisible := []string{
		"superpowers:brainstorming",
		"superpowers:writing-plans",
		"superpowers:using-git-worktrees",
		"superpowers:executing-plans",
		"superpowers:test-driven-development",
		"superpowers:requesting-code-review",
		"superpowers:receiving-code-review",
		"superpowers:verification-before-completion",
		"superpowers:finishing-a-development-branch",
		"ecc:intent-driven-development",
		"ecc:product-capability",
		"ecc:blueprint",
		"ecc:git-workflow",
		"ecc:tdd-workflow",
		"ecc:verification-loop",
	}
	userExplicit := []string{
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
	hostControlled := []string{"workspace.prepare-or-confirm", "review.execute", "verification.execute", "closeout.execute"}

	result := make(policyflow.RouteInventory, 0, len(hostVisible)+len(userExplicit)+len(hostControlled))
	for _, name := range hostVisible {
		result = append(result, policyflow.Route{Name: name, Mode: policyflow.HostVisible})
	}
	for _, name := range userExplicit {
		result = append(result, policyflow.Route{Name: name, Mode: policyflow.UserExplicit})
	}
	for _, name := range hostControlled {
		result = append(result, policyflow.Route{Name: name, Mode: policyflow.HostControlled})
	}
	return result
}

func successfulSkillEvent(ref policyflow.WorkRef, review bool) policyflow.Event {
	if review {
		return policyflow.ReviewCompleted{WorkRef: ref, Outcome: policyflow.ReviewClean}
	}
	return policyflow.SkillCompleted{WorkRef: ref}
}

func successfulHostActionEvent(ref policyflow.WorkRef, review bool) policyflow.Event {
	if review {
		return policyflow.ReviewCompleted{WorkRef: ref, Outcome: policyflow.ReviewClean}
	}
	return policyflow.HostActionCompleted{WorkRef: ref}
}

func removeRoute(inventory policyflow.RouteInventory, name string) policyflow.RouteInventory {
	result := make(policyflow.RouteInventory, 0, len(inventory))
	for _, route := range inventory {
		if route.Name != name {
			result = append(result, route)
		}
	}
	return result
}

func requireProfile(t *testing.T, offer policyflow.Offer, profile policyflow.ProfileID) policyflow.ProfileOffer {
	t.Helper()
	for _, candidate := range offer.Profiles {
		if candidate.ID == profile {
			return candidate
		}
	}
	t.Fatalf("profile %q missing", profile)
	return policyflow.ProfileOffer{}
}

func requireRoute(t *testing.T, profile policyflow.ProfileOffer, name string) policyflow.RouteStatus {
	t.Helper()
	for _, route := range profile.Routes {
		if route.Name == name {
			return route
		}
	}
	t.Fatalf("route %q missing from %s", name, profile.ID)
	return policyflow.RouteStatus{}
}

func hasRoute(profile policyflow.ProfileOffer, name string) bool {
	for _, route := range profile.Routes {
		if route.Name == name {
			return true
		}
	}
	return false
}

func hasMissing(profile policyflow.ProfileOffer, name string) bool {
	for _, route := range profile.Missing {
		if route.Name == name {
			return true
		}
	}
	return false
}

func requireFailureCode(t *testing.T, err error, code policyflow.FailureCode) {
	t.Helper()
	var failure *policyflow.Failure
	if !errors.As(err, &failure) || failure.Code != code {
		t.Fatalf("error = %v, want failure code %s", err, code)
	}
}
