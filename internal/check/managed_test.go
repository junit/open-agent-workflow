package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedBlockStatusAndExtraction(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "AGENTS.md")
	if got, _ := managedBlock(path); got != "absent" {
		t.Fatalf("missing status = %q", got)
	}
	content := "before\n" + beginMarker + "\nbody\n" + endMarker + "\nafter"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	status, checksum := managedBlock(path)
	if status != "present" {
		t.Fatalf("status = %q", status)
	}
	want := beginMarker + "\nbody\n" + endMarker + "\n"
	if checksum != checksumBytes([]byte(want)) {
		t.Fatalf("checksum = %q, want checksum of %q", checksum, want)
	}
}

func TestManagedBlockStreamsLongLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	longLine := strings.Repeat("x", 128*1024)
	block := beginMarker + "\n" + longLine + "\n" + endMarker + "\n"
	if err := os.WriteFile(path, []byte("before\n"+block+"after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, checksum := managedBlock(path)
	if status != "present" || checksum != checksumBytes([]byte(block)) {
		t.Fatalf("managedBlock() = %q, %q", status, checksum)
	}
}

func TestManagedBlockMatchesAwkForUnterminatedEndMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	block := beginMarker + "\nbody\n" + endMarker
	if err := os.WriteFile(path, []byte(block), 0o644); err != nil {
		t.Fatal(err)
	}
	status, checksum := managedBlock(path)
	want := block + "\n"
	if status != "present" || checksum != checksumBytes([]byte(want)) {
		t.Fatalf("managedBlock() = %q, %q; want checksum of %q", status, checksum, want)
	}
}

func TestManagedBlockRejectsMalformedMarkers(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"duplicate": beginMarker + "\n" + beginMarker + "\n" + endMarker + "\n",
		"reversed":  endMarker + "\n" + beginMarker + "\n",
		"partial":   beginMarker + "\nbody\n",
		"directory": "",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, name)
			if name == "directory" {
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			if status, _ := managedBlock(path); status != "drift" {
				t.Fatalf("status = %q, want drift", status)
			}
		})
	}
}
