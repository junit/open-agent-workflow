package oaw

import (
	"os"
	"strings"
	"testing"
)

func TestVersionMatchesRepositoryRelease(t *testing.T) {
	raw, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatalf("ReadFile(VERSION): %v", err)
	}
	want := strings.TrimSpace(string(raw))
	if want == "" {
		t.Fatal("VERSION is empty")
	}
	if got := Version(); got != want {
		t.Fatalf("Version() = %q, want %q", got, want)
	}
}
