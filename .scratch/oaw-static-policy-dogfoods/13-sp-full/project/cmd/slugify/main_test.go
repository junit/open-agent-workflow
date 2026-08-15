package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunWritesSlugForOneArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run([]string{"Fresh: Verification"}, &stdout, &stderr); status != 0 {
		t.Fatalf("run status = %d, stderr = %q", status, stderr.String())
	}
	if got := stdout.String(); got != "fresh-verification\n" {
		t.Fatalf("stdout = %q, want %q", got, "fresh-verification\n")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunRejectsArgumentCount(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run(nil, &stdout, &stderr); status != 2 {
		t.Fatalf("run status = %d, want 2", status)
	}
	if !strings.Contains(stderr.String(), "usage: slugify <text>") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}
