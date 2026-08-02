package check

import (
	"os"
	"path/filepath"
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
	status, block := managedBlock(path)
	if status != "present" {
		t.Fatalf("status = %q", status)
	}
	want := beginMarker + "\nbody\n" + endMarker + "\n"
	if string(block) != want {
		t.Fatalf("block = %q, want %q", block, want)
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
