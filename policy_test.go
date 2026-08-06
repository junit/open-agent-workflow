package oaw

import (
	"bytes"
	"os"
	"testing"
)

func TestCanonicalPolicyMatchesRepositoryAndReturnsCopies(t *testing.T) {
	want, err := os.ReadFile("policy/ENGINEERING.md")
	if err != nil {
		t.Fatalf("ReadFile(policy/ENGINEERING.md): %v", err)
	}
	if len(want) == 0 {
		t.Fatal("policy/ENGINEERING.md is empty")
	}

	first := CanonicalPolicy()
	if !bytes.Equal(first, want) {
		t.Fatal("CanonicalPolicy() differs from policy/ENGINEERING.md")
	}
	first[0] ^= 0xff
	second := CanonicalPolicy()
	if !bytes.Equal(second, want) {
		t.Fatal("CanonicalPolicy() returned mutable embedded storage")
	}
	if len(first) != 0 && &first[0] == &second[0] {
		t.Fatal("CanonicalPolicy() reused its result buffer")
	}
}

func TestCanonicalPolicyDefinesCoreCoordinatorHostBoundary(t *testing.T) {
	policy := CanonicalPolicy()
	required := []string{
		"OAW Core",
		"Workflow Coordinator",
		"Agent Host",
		"CURRENT",
		"SUBAGENT",
		"Only Workflow Mode runs the Startup Gate",
		"DIRECT and BOUNDED do not create Workflow State",
		"OAW never starts a model process",
		"The Agent Host owns physical execution authority",
	}

	for _, literal := range required {
		if !bytes.Contains(policy, []byte(literal)) {
			t.Errorf("CanonicalPolicy() is missing required literal %q", literal)
		}
	}
}

func TestCanonicalPolicyRejectsRemovedExecutionContracts(t *testing.T) {
	policy := CanonicalPolicy()
	forbidden := []string{
		"Runtime Plane",
		"Runtime-managed",
		"oaw runtime exchange",
		"oaw run --host codex",
		"oaw/codex-runner",
		"runner-managed",
		"native-managed",
		"INLINE",
		"NATIVE_SUBAGENT",
		"main-agent-allowed",
		"isolated-required",
		"private HOME",
	}

	for _, literal := range forbidden {
		if bytes.Contains(policy, []byte(literal)) {
			t.Errorf("CanonicalPolicy() contains forbidden literal %q", literal)
		}
	}
}
