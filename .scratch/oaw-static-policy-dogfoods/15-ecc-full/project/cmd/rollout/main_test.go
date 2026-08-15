package main

import (
	"bytes"
	"testing"
)

func TestRunPrintsSelectedKeysInInputOrder(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run([]string{"50", "a", "b", "hello"}, &stdout, &stderr); status != 0 {
		t.Fatalf("run() status = %d, stderr = %q", status, stderr.String())
	}
	if got, want := stdout.String(), "a\nhello\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunRequiresPercentageAndKey(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run([]string{"50"}, &stdout, &stderr); status != 2 {
		t.Fatalf("run() status = %d, want 2", status)
	}
	if stdout.Len() != 0 || stderr.String() != "usage: rollout <percentage> <key> [key...]\n" {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestRunRejectsNonNumericPercentage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run([]string{"many", "a"}, &stdout, &stderr); status != 2 {
		t.Fatalf("run() status = %d, want 2", status)
	}
	if stdout.Len() != 0 || stderr.String() != "invalid percentage \"many\"\n" {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestRunRejectsInvalidSelectionWithoutPartialOutput(t *testing.T) {
	for _, args := range [][]string{{"101", "a"}, {"50", "a", ""}} {
		var stdout, stderr bytes.Buffer
		if status := run(args, &stdout, &stderr); status != 2 {
			t.Fatalf("run(%q) status = %d, want 2", args, status)
		}
		if stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("run(%q) stdout = %q, stderr = %q", args, stdout.String(), stderr.String())
		}
	}
}
