package policyflow

import "testing"

func TestBuiltInProgramsUseOnlyPolicyRoutes(t *testing.T) {
	programs, err := loadBuiltInPrograms()
	if err != nil {
		t.Fatal(err)
	}
	if len(programs) != 4 {
		t.Fatalf("programs = %d", len(programs))
	}
	for _, program := range programs {
		if len(program.steps) == 0 {
			t.Fatalf("%s has no steps", program.id)
		}
		for _, step := range program.steps {
			if step.name == "" || step.slot == "" {
				t.Fatalf("%s has invalid step %#v", program.id, step)
			}
		}
	}
}

func TestECCGuidancePrecedesHostActionAndGate(t *testing.T) {
	programs, err := loadBuiltInPrograms()
	if err != nil {
		t.Fatal(err)
	}
	for _, program := range programs {
		if program.id != ProfileECCFull {
			continue
		}
		for index := 0; index+2 < len(program.steps); index++ {
			if program.steps[index].name == "ecc:git-workflow" &&
				program.steps[index+1].name == "workspace.prepare-or-confirm" &&
				program.steps[index+2].name == "workspace-ready" {
				return
			}
		}
		t.Fatalf("ECC workspace guidance/action/gate order is missing: %#v", program.steps)
	}
	t.Fatal("ECC-FULL program is missing")
}
