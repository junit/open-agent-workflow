package policyflow_test

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
)

func TestSPIncidentUsesRecipeHandlerAndReturnsToImplementation(t *testing.T) {
	module := policyflow.New()
	inventory := append(completeInventory(), policyflow.Route{
		Name: "superpowers:systematic-debugging", Mode: policyflow.HostVisible,
	})
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	progress = advanceToSkill(t, module, inventory, progress, "superpowers:test-driven-development")
	failed := progress.Next.(policyflow.InvokeSkill)

	progress, err := module.Apply(inventory, policyflow.IncidentReported{
		WorkRef: failed.WorkRef, Incident: policyflow.IncidentBuildFailure,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler, ok := progress.Next.(policyflow.InvokeSkill)
	if !ok || handler.Skill != "superpowers:systematic-debugging" || !reflect.DeepEqual(handler.Covers, []policyflow.LifecycleSlot{policyflow.SlotIncidentRecovery}) {
		t.Fatalf("incident handler = %#v", progress.Next)
	}

	progress, err = module.Apply(inventory, policyflow.SkillCompleted{WorkRef: handler.WorkRef})
	if err != nil {
		t.Fatal(err)
	}
	returned, ok := progress.Next.(policyflow.InvokeSkill)
	if !ok || returned.Skill != "superpowers:executing-plans" {
		t.Fatalf("work after recovery = %#v", progress.Next)
	}
	if !containsString(progress.StableBoundaries, "debugging-cycle-complete") {
		t.Fatalf("stable boundaries after recovery = %v", progress.StableBoundaries)
	}
}

func TestMattIncidentReturnsToCompositeImplementation(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	progress := startProfile(t, module, inventory, policyflow.ProfileMattFull)
	progress = advanceToSkill(t, module, inventory, progress, "implement")
	failed := progress.Next.(policyflow.AwaitUserSkill)

	progress, err := module.Apply(inventory, policyflow.IncidentReported{
		WorkRef: failed.WorkRef, Incident: policyflow.IncidentFunctionalFailure,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler, ok := progress.Next.(policyflow.AwaitUserSkill)
	if !ok || handler.Skill != "diagnosing-bugs" {
		t.Fatalf("Matt incident handler = %#v", progress.Next)
	}
	progress, err = module.Apply(inventory, policyflow.SkillCompleted{WorkRef: handler.WorkRef})
	if err != nil {
		t.Fatal(err)
	}
	returned, ok := progress.Next.(policyflow.AwaitUserSkill)
	if !ok || returned.Skill != "implement" {
		t.Fatalf("work after Matt recovery = %#v", progress.Next)
	}
}

func TestUnavailableECCIncidentHandlerDoesNotMakeProfileIncomplete(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	profile := requireProfile(t, offer, policyflow.ProfileECCFull)
	if !profile.HostRoutable || !profile.PolicySelectable {
		t.Fatalf("ECC profile = %#v", profile)
	}
	for _, incident := range profile.IncidentRoutes {
		if incident.Incident == policyflow.IncidentBuildFailure {
			if incident.Available || incident.Skill != "" {
				t.Fatalf("ECC build incident route = %#v", incident)
			}
			goto found
		}
	}
	t.Fatal("ECC build incident metadata is missing")

found:
	progress, err := module.Start(inventory, policyflow.Selection{OfferRef: offer.Ref, Profile: policyflow.ProfileECCFull})
	if err != nil {
		t.Fatal(err)
	}
	progress = advanceToSkill(t, module, inventory, progress, "ecc:tdd-workflow")
	current := progress.Next.(policyflow.InvokeSkill)
	progress, err = module.Apply(inventory, policyflow.IncidentReported{
		WorkRef: current.WorkRef, Incident: policyflow.IncidentBuildFailure,
	})
	if err != nil {
		t.Fatal(err)
	}
	stopped, ok := progress.Next.(policyflow.Stopped)
	if !ok || stopped.Code != policyflow.StopIncidentHandlerUnavailable || stopped.Incident != policyflow.IncidentBuildFailure {
		t.Fatalf("unavailable incident result = %#v", progress.Next)
	}
}

func TestUnavailableHybridTechnicalIncidentsStopOnlyWhenReported(t *testing.T) {
	for _, incident := range []policyflow.IncidentType{
		policyflow.IncidentBuildFailure,
		policyflow.IncidentDependencyFailure,
		policyflow.IncidentTypeFailure,
	} {
		t.Run(string(incident), func(t *testing.T) {
			module := policyflow.New()
			inventory := completeInventory()
			offer, err := module.Offer(inventory)
			if err != nil {
				t.Fatal(err)
			}
			profile := requireProfile(t, offer, policyflow.ProfileMattSPHybrid)
			if !profile.PolicySelectable || !profile.HostRoutable || len(profile.Missing) != 0 {
				t.Fatalf("Hybrid profile = %#v", profile)
			}
			routeFound := false
			for _, route := range profile.IncidentRoutes {
				if route.Incident != incident {
					continue
				}
				routeFound = true
				if route.Available || route.Skill != "" {
					t.Fatalf("Hybrid %s incident route = %#v", incident, route)
				}
			}
			if !routeFound {
				t.Fatalf("Hybrid %s incident metadata is missing", incident)
			}

			progress, err := module.Start(inventory, policyflow.Selection{
				OfferRef: offer.Ref, Profile: policyflow.ProfileMattSPHybrid,
			})
			if err != nil {
				t.Fatal(err)
			}
			progress = advanceToSkill(t, module, inventory, progress, "superpowers:executing-plans")
			current := progress.Next.(policyflow.InvokeSkill)
			progress, err = module.Apply(inventory, policyflow.IncidentReported{
				WorkRef: current.WorkRef, Incident: incident,
			})
			if err != nil {
				t.Fatal(err)
			}
			stopped, ok := progress.Next.(policyflow.Stopped)
			if !ok || stopped.Code != policyflow.StopIncidentHandlerUnavailable || stopped.Incident != incident {
				t.Fatalf("Hybrid %s incident result = %#v", incident, progress.Next)
			}
		})
	}
}

func TestIncidentCannotBypassLifecyclePrerequisites(t *testing.T) {
	module := policyflow.New()
	inventory := append(completeInventory(), policyflow.Route{
		Name: "superpowers:systematic-debugging", Mode: policyflow.HostVisible,
	})
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	current := progress.Next.(policyflow.InvokeSkill)

	_, err := module.Apply(inventory, policyflow.IncidentReported{
		WorkRef: current.WorkRef, Incident: policyflow.IncidentBuildFailure,
	})
	requireFailureCode(t, err, policyflow.FailureEventOutOfOrder)

	// Rejection must not consume the original work.
	if _, err := module.Apply(inventory, policyflow.SkillCompleted{WorkRef: current.WorkRef}); err != nil {
		t.Fatal(err)
	}
}

func TestTerminalEventsClearActiveIncidentContext(t *testing.T) {
	for _, test := range []struct {
		name  string
		event func(policyflow.WorkRef) policyflow.Event
	}{
		{"stop", func(ref policyflow.WorkRef) policyflow.Event {
			return policyflow.StopRequested{Current: ref, Reason: "cancelled"}
		}},
		{"uncertain", func(ref policyflow.WorkRef) policyflow.Event {
			return policyflow.ExecutionUncertain{WorkRef: ref, Reason: "unknown result"}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			module := policyflow.New()
			inventory := completeInventory()
			progress := startProfile(t, module, inventory, policyflow.ProfileMattFull)
			progress = advanceToSkill(t, module, inventory, progress, "implement")
			current := progress.Next.(policyflow.AwaitUserSkill)
			progress = applyOK(t, module, inventory, policyflow.IncidentReported{
				WorkRef: current.WorkRef, Incident: policyflow.IncidentFunctionalFailure,
			})
			handler := progress.Next.(policyflow.AwaitUserSkill)
			progress = applyOK(t, module, inventory, test.event(handler.WorkRef))
			saved, err := module.Export(progress)
			if err != nil {
				t.Fatal(err)
			}
			if saved.InIncident || saved.ReturnIndex != -1 || saved.ActiveIncident != "" ||
				saved.IncidentHandler != "" || saved.IncidentReturnSlot != "" {
				t.Fatalf("terminal state retained incident context: %#v", saved)
			}
			if _, err := policyflow.New().Restore(inventory, saved); err != nil {
				t.Fatalf("restore terminal incident state: %v", err)
			}
		})
	}
}

func TestReviewFindingsReturnToImplementationAndRequireFreshReview(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	progress = advanceToSkill(t, module, inventory, progress, "superpowers:requesting-code-review")
	review := progress.Next.(policyflow.InvokeSkill)

	progress = applyOK(t, module, inventory, policyflow.ReviewCompleted{
		WorkRef: review.WorkRef, Outcome: policyflow.ReviewFindings,
	})
	remediation, ok := progress.Next.(policyflow.InvokeSkill)
	if !ok || remediation.Skill != "superpowers:executing-plans" {
		t.Fatalf("remediation work = %#v", progress.Next)
	}
	progress = applyOK(t, module, inventory, policyflow.SkillCompleted{WorkRef: remediation.WorkRef})
	reviewAgain, ok := progress.Next.(policyflow.InvokeSkill)
	if !ok || reviewAgain.Skill != "superpowers:requesting-code-review" {
		t.Fatalf("fresh review work = %#v", progress.Next)
	}
	if containsString(progress.StableBoundaries, "review-complete") ||
		containsSlotForTest(progress.CompletedSlots, policyflow.SlotReviewRemediation) {
		t.Fatalf("findings incorrectly completed review: %#v", progress)
	}
	progress = applyOK(t, module, inventory, policyflow.ReviewCompleted{
		WorkRef: reviewAgain.WorkRef, Outcome: policyflow.ReviewClean,
	})
	if next, ok := progress.Next.(policyflow.InvokeSkill); !ok || next.Skill != "superpowers:receiving-code-review" {
		t.Fatalf("clean review did not continue review pipeline: %#v", progress.Next)
	}
}

func TestECCGenericReviewFindingsReturnToImplementationAndRequireFreshReview(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	progress := startProfile(t, module, inventory, policyflow.ProfileECCFull)
	progress = advanceToHostAction(t, module, inventory, progress, "review.execute")
	review := progress.Next.(policyflow.HostAction)
	if !review.Review {
		t.Fatalf("ECC review Host action = %#v", review)
	}

	_, err := module.Apply(inventory, policyflow.HostActionCompleted{WorkRef: review.WorkRef})
	requireFailureCode(t, err, policyflow.FailureEventOutOfOrder)

	progress = applyOK(t, module, inventory, policyflow.ReviewCompleted{
		WorkRef: review.WorkRef, Outcome: policyflow.ReviewFindings,
	})
	remediation, ok := progress.Next.(policyflow.InvokeSkill)
	if !ok || remediation.Skill != "ecc:tdd-workflow" {
		t.Fatalf("ECC remediation work = %#v", progress.Next)
	}
	progress = applyOK(t, module, inventory, policyflow.SkillCompleted{WorkRef: remediation.WorkRef})
	reviewAgain, ok := progress.Next.(policyflow.HostAction)
	if !ok || reviewAgain.Action != "review.execute" || !reviewAgain.Review {
		t.Fatalf("fresh ECC review work = %#v", progress.Next)
	}
	progress = applyOK(t, module, inventory, policyflow.ReviewCompleted{
		WorkRef: reviewAgain.WorkRef, Outcome: policyflow.ReviewClean,
	})
	if next, ok := progress.Next.(policyflow.InvokeSkill); !ok || next.Skill != "ecc:verification-loop" {
		t.Fatalf("clean ECC review did not continue to verification: %#v", progress.Next)
	}
}

func TestReviewWorkRejectsUntypedSkillCompletion(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	progress = advanceToSkill(t, module, inventory, progress, "superpowers:requesting-code-review")
	review := progress.Next.(policyflow.InvokeSkill)

	_, err := module.Apply(inventory, policyflow.SkillCompleted{WorkRef: review.WorkRef})
	requireFailureCode(t, err, policyflow.FailureEventOutOfOrder)
}

func TestSwitchAtStableBoundaryPreservesCompletedWorkAndGates(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	progress = advancePastGate(t, module, inventory, progress, "specification-approved")
	if !containsString(progress.StableBoundaries, "specification-approved") {
		t.Fatalf("stable boundaries = %v", progress.StableBoundaries)
	}
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}

	progress, err = module.Apply(inventory, policyflow.ProfileSwitchRequested{
		Current: currentRef(t, progress.Next), OfferRef: offer.Ref, Profile: policyflow.ProfileMattFull,
	})
	if err != nil {
		t.Fatal(err)
	}
	if progress.Plan.Profile != policyflow.ProfileMattFull {
		t.Fatalf("plan after switch = %#v", progress.Plan)
	}
	next, ok := progress.Next.(policyflow.AwaitUserSkill)
	if !ok || next.Skill != "to-tickets" {
		t.Fatalf("next after switch = %#v", progress.Next)
	}
}

func TestSwitchOutsideStableBoundaryIsRejected(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}

	_, err = module.Apply(inventory, policyflow.ProfileSwitchRequested{
		Current: currentRef(t, progress.Next), OfferRef: offer.Ref, Profile: policyflow.ProfileMattFull,
	})
	requireFailureCode(t, err, policyflow.FailureSwitchNotStable)
}

func TestHistoricalStableBoundaryDoesNotAuthorizeLaterSwitch(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	progress = advancePastGate(t, module, inventory, progress, "specification-approved")
	if !containsString(progress.StableBoundaries, "specification-approved") {
		t.Fatalf("stable boundaries = %v", progress.StableBoundaries)
	}

	current := progress.Next.(policyflow.InvokeSkill)
	progress = applyOK(t, module, inventory, policyflow.SkillCompleted{WorkRef: current.WorkRef})
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	_, err = module.Apply(inventory, policyflow.ProfileSwitchRequested{
		Current: currentRef(t, progress.Next), OfferRef: offer.Ref, Profile: policyflow.ProfileMattFull,
	})
	requireFailureCode(t, err, policyflow.FailureSwitchNotStable)
}

func TestIncidentConsumesStableSwitchWindow(t *testing.T) {
	module := policyflow.New()
	inventory := append(completeInventory(), policyflow.Route{
		Name: "superpowers:systematic-debugging", Mode: policyflow.HostVisible,
	})
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	progress = advancePastGate(t, module, inventory, progress, "specification-approved")
	progress = advanceToSkill(t, module, inventory, progress, "superpowers:executing-plans")
	current := progress.Next.(policyflow.InvokeSkill)
	progress = applyOK(t, module, inventory, policyflow.IncidentReported{
		WorkRef: current.WorkRef, Incident: policyflow.IncidentBuildFailure,
	})
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}

	_, err = module.Apply(inventory, policyflow.ProfileSwitchRequested{
		Current: currentRef(t, progress.Next), OfferRef: offer.Ref, Profile: policyflow.ProfileMattFull,
	})
	requireFailureCode(t, err, policyflow.FailureSwitchNotStable)
}

func TestDeliveryPlanningDoesNotInventTicketCompletionBoundary(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	progress := startProfile(t, module, inventory, policyflow.ProfileMattFull)
	progress = advanceToSkill(t, module, inventory, progress, "to-tickets")
	current := progress.Next.(policyflow.AwaitUserSkill)
	progress = applyOK(t, module, inventory, policyflow.SkillCompleted{WorkRef: current.WorkRef})
	if containsString(progress.StableBoundaries, "ticket-complete") {
		t.Fatalf("delivery planning invented ticket completion: %v", progress.StableBoundaries)
	}
}

func TestSwitchRequiresCurrentRoutableOffer(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	progress = advancePastGate(t, module, inventory, progress, "specification-approved")
	current := currentRef(t, progress.Next)

	withoutMatt := removeRoute(inventory, "implement")
	offer, err := module.Offer(withoutMatt)
	if err != nil {
		t.Fatal(err)
	}
	_, err = module.Apply(withoutMatt, policyflow.ProfileSwitchRequested{
		Current: current, OfferRef: offer.Ref, Profile: policyflow.ProfileMattFull,
	})
	requireFailureCode(t, err, policyflow.FailureProfileIncomplete)
}

func TestApplyDistinguishesInventoryDriftFromStaleSelection(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	withUnrelatedRoute := append(inventory, policyflow.Route{Name: "host:new-route", Mode: policyflow.HostVisible})
	_, err = module.Start(withUnrelatedRoute, policyflow.Selection{OfferRef: offer.Ref, Profile: policyflow.ProfileSPFull})
	requireFailureCode(t, err, policyflow.FailureOfferStale)

	progress, err := module.Start(inventory, policyflow.Selection{OfferRef: offer.Ref, Profile: policyflow.ProfileSPFull})
	if err != nil {
		t.Fatal(err)
	}
	current := progress.Next.(policyflow.InvokeSkill)
	changed := removeRoute(inventory, current.Skill)
	_, err = module.Apply(changed, policyflow.SkillCompleted{WorkRef: current.WorkRef})
	requireFailureCode(t, err, policyflow.FailureInventoryDrift)

	// Drift rejection does not consume the current work reference.
	if _, err := module.Apply(inventory, policyflow.SkillCompleted{WorkRef: current.WorkRef}); err != nil {
		t.Fatal(err)
	}
}

func TestUnrelatedProfileRouteChangeDoesNotInterruptActiveProfile(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	current := progress.Next.(policyflow.InvokeSkill)
	withoutMatt := removeRoute(inventory, "implement")

	if _, err := module.Apply(withoutMatt, policyflow.SkillCompleted{WorkRef: current.WorkRef}); err != nil {
		t.Fatalf("unrelated Matt route interrupted SP-FULL: %v", err)
	}
}

func TestRouteInventoryDriftStillAllowsTerminalSafetyEvents(t *testing.T) {
	t.Run("explicit stop", func(t *testing.T) {
		module := policyflow.New()
		inventory := completeInventory()
		progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
		current := progress.Next.(policyflow.InvokeSkill)
		changed := removeRoute(inventory, current.Skill)

		progress, err := module.Apply(changed, policyflow.StopRequested{
			Current: current.WorkRef, Reason: "route disappeared",
		})
		if err != nil {
			t.Fatal(err)
		}
		stopped, ok := progress.Next.(policyflow.Stopped)
		if !ok || stopped.Code != policyflow.StopExplicit {
			t.Fatalf("stop result = %#v", progress.Next)
		}
		saved, err := module.Export(progress)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := policyflow.New().Restore(changed, saved); err != nil {
			t.Fatalf("restore stopped state: %v", err)
		}
	})

	t.Run("execution uncertain", func(t *testing.T) {
		module := policyflow.New()
		inventory := completeInventory()
		progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
		current := progress.Next.(policyflow.InvokeSkill)
		changed := removeRoute(inventory, current.Skill)

		progress, err := module.Apply(changed, policyflow.ExecutionUncertain{
			WorkRef: current.WorkRef, Reason: "Host result is unknown",
		})
		if err != nil {
			t.Fatal(err)
		}
		blocked, ok := progress.Next.(policyflow.Blocked)
		if !ok || blocked.Code != policyflow.BlockExecutionUncertain || blocked.RetryAllowed {
			t.Fatalf("uncertain result = %#v", progress.Next)
		}
		saved, err := module.Export(progress)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := policyflow.New().Restore(changed, saved); err != nil {
			t.Fatalf("restore uncertain state: %v", err)
		}
	})
}

func TestExplicitStopAndExecutionUncertaintyAreDistinctTerminalStates(t *testing.T) {
	t.Run("explicit stop at gate", func(t *testing.T) {
		module := policyflow.New()
		inventory := completeInventory()
		progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
		first := progress.Next.(policyflow.InvokeSkill)
		progress, err := module.Apply(inventory, policyflow.SkillCompleted{WorkRef: first.WorkRef})
		if err != nil {
			t.Fatal(err)
		}
		gate := progress.Next.(policyflow.UserGate)
		progress, err = module.Apply(inventory, policyflow.StopRequested{
			Current: gate.GateRef, Reason: "user cancelled",
		})
		if err != nil {
			t.Fatal(err)
		}
		stopped, ok := progress.Next.(policyflow.Stopped)
		if !ok || stopped.Code != policyflow.StopExplicit || stopped.Reason != "user cancelled" {
			t.Fatalf("stop result = %#v", progress.Next)
		}
	})

	t.Run("unknown external effect", func(t *testing.T) {
		module := policyflow.New()
		inventory := completeInventory()
		progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
		current := progress.Next.(policyflow.InvokeSkill)
		progress, err := module.Apply(inventory, policyflow.ExecutionUncertain{
			WorkRef: current.WorkRef, Reason: "Host result is unknown",
		})
		if err != nil {
			t.Fatal(err)
		}
		blocked, ok := progress.Next.(policyflow.Blocked)
		if !ok || blocked.Code != policyflow.BlockExecutionUncertain || blocked.RetryAllowed {
			t.Fatalf("uncertain result = %#v", progress.Next)
		}
		_, err = module.Apply(inventory, policyflow.SkillCompleted{WorkRef: current.WorkRef})
		requireFailureCode(t, err, policyflow.FailureEventOutOfOrder)
	})

}

func TestConcurrentApplyConsumesWorkReferenceExactlyOnce(t *testing.T) {
	module := policyflow.New()
	inventory := completeInventory()
	progress := startProfile(t, module, inventory, policyflow.ProfileSPFull)
	current := progress.Next.(policyflow.InvokeSkill)

	start := make(chan struct{})
	errorsSeen := make(chan error, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := module.Apply(inventory, policyflow.SkillCompleted{WorkRef: current.WorkRef})
			errorsSeen <- err
		}()
	}
	close(start)
	group.Wait()
	close(errorsSeen)

	succeeded := 0
	rejected := 0
	for err := range errorsSeen {
		if err == nil {
			succeeded++
			continue
		}
		var failure *policyflow.Failure
		if errors.As(err, &failure) && failure.Code == policyflow.FailureEventOutOfOrder {
			rejected++
			continue
		}
		t.Fatalf("unexpected Apply error: %v", err)
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("concurrent results: succeeded=%d rejected=%d", succeeded, rejected)
	}
}

func startProfile(t *testing.T, module *policyflow.Module, inventory policyflow.RouteInventory, profile policyflow.ProfileID) policyflow.Progress {
	t.Helper()
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := module.Start(inventory, policyflow.Selection{OfferRef: offer.Ref, Profile: profile})
	if err != nil {
		t.Fatal(err)
	}
	return progress
}

func advanceToSkill(t *testing.T, module *policyflow.Module, inventory policyflow.RouteInventory, progress policyflow.Progress, skill string) policyflow.Progress {
	t.Helper()
	for range 64 {
		switch next := progress.Next.(type) {
		case policyflow.InvokeSkill:
			if next.Skill == skill {
				return progress
			}
			progress = applyOK(t, module, inventory, successfulSkillEvent(next.WorkRef, next.Review))
		case policyflow.AwaitUserSkill:
			if next.Skill == skill {
				return progress
			}
			progress = applyOK(t, module, inventory, successfulSkillEvent(next.WorkRef, next.Review))
		case policyflow.HostAction:
			progress = applyOK(t, module, inventory, successfulHostActionEvent(next.WorkRef, next.Review))
		case policyflow.UserGate:
			progress = applyOK(t, module, inventory, policyflow.UserGateApproved{GateRef: next.GateRef})
		case policyflow.HostGate:
			progress = applyOK(t, module, inventory, policyflow.HostGateSatisfied{GateRef: next.GateRef})
		default:
			t.Fatalf("skill %q not reached; next = %#v", skill, progress.Next)
		}
	}
	t.Fatalf("skill %q not reached", skill)
	return policyflow.Progress{}
}

func advanceToHostAction(t *testing.T, module *policyflow.Module, inventory policyflow.RouteInventory, progress policyflow.Progress, action string) policyflow.Progress {
	t.Helper()
	for range 64 {
		switch next := progress.Next.(type) {
		case policyflow.InvokeSkill:
			progress = applyOK(t, module, inventory, successfulSkillEvent(next.WorkRef, next.Review))
		case policyflow.AwaitUserSkill:
			progress = applyOK(t, module, inventory, successfulSkillEvent(next.WorkRef, next.Review))
		case policyflow.HostAction:
			if next.Action == action {
				return progress
			}
			progress = applyOK(t, module, inventory, successfulHostActionEvent(next.WorkRef, next.Review))
		case policyflow.UserGate:
			progress = applyOK(t, module, inventory, policyflow.UserGateApproved{GateRef: next.GateRef})
		case policyflow.HostGate:
			progress = applyOK(t, module, inventory, policyflow.HostGateSatisfied{GateRef: next.GateRef})
		default:
			t.Fatalf("Host action %q not reached; next = %#v", action, progress.Next)
		}
	}
	t.Fatalf("Host action %q not reached", action)
	return policyflow.Progress{}
}

func advancePastGate(t *testing.T, module *policyflow.Module, inventory policyflow.RouteInventory, progress policyflow.Progress, gate string) policyflow.Progress {
	t.Helper()
	for range 64 {
		switch next := progress.Next.(type) {
		case policyflow.InvokeSkill:
			progress = applyOK(t, module, inventory, successfulSkillEvent(next.WorkRef, next.Review))
		case policyflow.AwaitUserSkill:
			progress = applyOK(t, module, inventory, successfulSkillEvent(next.WorkRef, next.Review))
		case policyflow.HostAction:
			progress = applyOK(t, module, inventory, successfulHostActionEvent(next.WorkRef, next.Review))
		case policyflow.UserGate:
			progress = applyOK(t, module, inventory, policyflow.UserGateApproved{GateRef: next.GateRef})
			if next.Gate == gate {
				return progress
			}
		case policyflow.HostGate:
			progress = applyOK(t, module, inventory, policyflow.HostGateSatisfied{GateRef: next.GateRef})
			if next.Gate == gate {
				return progress
			}
		default:
			t.Fatalf("gate %q not reached; next = %#v", gate, progress.Next)
		}
	}
	t.Fatalf("gate %q not reached", gate)
	return policyflow.Progress{}
}

func applyOK(t *testing.T, module *policyflow.Module, inventory policyflow.RouteInventory, event policyflow.Event) policyflow.Progress {
	t.Helper()
	progress, err := module.Apply(inventory, event)
	if err != nil {
		t.Fatal(err)
	}
	return progress
}

func currentRef(t *testing.T, next policyflow.NextWork) policyflow.CurrentRef {
	t.Helper()
	switch value := next.(type) {
	case policyflow.InvokeSkill:
		return value.WorkRef
	case policyflow.AwaitUserSkill:
		return value.WorkRef
	case policyflow.HostAction:
		return value.WorkRef
	case policyflow.UserGate:
		return value.GateRef
	case policyflow.HostGate:
		return value.GateRef
	default:
		t.Fatalf("next work has no current reference: %#v", next)
		return nil
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func containsSlotForTest(values []policyflow.LifecycleSlot, wanted policyflow.LifecycleSlot) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
