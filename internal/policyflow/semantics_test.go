package policyflow

import "testing"

func TestProgramDigestIncludesRequirementRouteAndSlot(t *testing.T) {
	programs, err := loadBuiltInPrograms()
	if err != nil {
		t.Fatal(err)
	}
	want := programDigest(programs)

	routeChanged := cloneProgramsForTest(programs)
	routeChanged[1].requires[0].route = "different-route"
	if got := programDigest(routeChanged); got == want {
		t.Fatal("requirement route did not change the Policy semantics digest")
	}

	slotChanged := cloneProgramsForTest(programs)
	slotChanged[1].requires[0].slot = SlotSolutionSpecification
	if got := programDigest(slotChanged); got == want {
		t.Fatal("requirement slot did not change the Policy semantics digest")
	}
}

func TestRequirementSemanticsChangeMakesOfferStale(t *testing.T) {
	programs, err := loadBuiltInPrograms()
	if err != nil {
		t.Fatal(err)
	}
	inventory := inventoryForPrograms(programs)
	original := moduleForPrograms(programs)
	offer, err := original.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}

	changedPrograms := cloneProgramsForTest(programs)
	changedPrograms[1].requires[0].slot = SlotSolutionSpecification
	changed := moduleForPrograms(changedPrograms)
	_, err = changed.Start(inventory, Selection{OfferRef: offer.Ref, Profile: ProfileMattFull})
	requireInternalFailureCode(t, err, FailureOfferStale)
}

func TestRequirementSemanticsChangeRejectsRestore(t *testing.T) {
	programs, err := loadBuiltInPrograms()
	if err != nil {
		t.Fatal(err)
	}
	inventory := inventoryForPrograms(programs)
	original := moduleForPrograms(programs)
	offer, err := original.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := original.Start(inventory, Selection{OfferRef: offer.Ref, Profile: ProfileMattFull})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := original.Export(progress)
	if err != nil {
		t.Fatal(err)
	}

	changedPrograms := cloneProgramsForTest(programs)
	changedPrograms[1].requires[0].route = "different-route"
	changed := moduleForPrograms(changedPrograms)
	_, err = changed.Restore(inventory, saved)
	requireInternalFailureCode(t, err, FailureSemanticsInvalid)
}

func moduleForPrograms(programs []profileProgram) *Module {
	return &Module{
		programs:  programs,
		semantics: programDigest(programs),
		runs:      map[string]*runState{},
	}
}

func cloneProgramsForTest(programs []profileProgram) []profileProgram {
	result := append([]profileProgram(nil), programs...)
	for index := range result {
		result[index].steps = append([]programStep(nil), programs[index].steps...)
		result[index].requires = append([]programRequirement(nil), programs[index].requires...)
		result[index].incidents = append([]programIncidentRoute(nil), programs[index].incidents...)
		result[index].stableBoundaries = append([]string(nil), programs[index].stableBoundaries...)
	}
	return result
}

func inventoryForPrograms(programs []profileProgram) RouteInventory {
	seen := map[string]InvocationMode{}
	for _, program := range programs {
		for _, step := range program.steps {
			switch step.kind {
			case stepSkill:
				seen[step.name] = HostVisible
			case stepHostAction:
				seen[step.name] = HostControlled
			}
		}
		for _, requirement := range program.requires {
			seen[requirement.route] = HostVisible
		}
	}
	result := make(RouteInventory, 0, len(seen))
	for name, mode := range seen {
		result = append(result, Route{Name: name, Mode: mode})
	}
	return result
}

func requireInternalFailureCode(t *testing.T, err error, code FailureCode) {
	t.Helper()
	failure, ok := err.(*Failure)
	if !ok || failure.Code != code {
		t.Fatalf("error = %v, want failure code %s", err, code)
	}
}
