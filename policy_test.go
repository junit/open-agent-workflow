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
