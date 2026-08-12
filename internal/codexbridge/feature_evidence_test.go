package codexbridge

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestSessionFeatureEvidenceRequiresExactCurrentSessionAndCWD(t *testing.T) {
	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	root := filepath.Join(t.TempDir(), "open-agent-workflow", "codex-bridge", "features")
	store, err := NewSessionFeatureEvidenceStore(SessionFeatureEvidenceOptions{
		Root: root, TTL: 5 * time.Minute, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	context := HookContext{SessionID: "session-private-a", CWD: t.TempDir()}
	if err := store.RecordChildDelegation(context); err != nil {
		t.Fatal(err)
	}

	result := store.ObserveFeatures(context)
	if len(result.Diagnostics) != 0 || len(result.Observations) != 1 {
		t.Fatalf("result = %#v", result)
	}
	observation := result.Observations[0]
	if observation.Feature != host.FeatureChildDelegation || observation.State != host.AvailabilityAvailable ||
		observation.Source != host.SourceNativeAPI || !strings.HasPrefix(observation.EvidenceReference, "evidence://codex/cooperative-subagent-start/") || observation.Digest == "" {
		t.Fatalf("observation = %#v", observation)
	}
	for _, foreign := range []HookContext{
		{SessionID: "session-private-b", CWD: context.CWD},
		{SessionID: context.SessionID, CWD: t.TempDir()},
	} {
		if result := store.ObserveFeatures(foreign); len(result.Observations) != 0 {
			t.Fatalf("foreign context received evidence: %#v", result)
		}
	}

	path, err := store.evidencePath(context)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(content, []byte(context.SessionID)) || bytes.Contains(content, []byte(context.CWD)) ||
		bytes.Contains(content, []byte("agent-private")) || bytes.Contains(content, []byte("transcript")) {
		t.Fatalf("private Hook data was persisted: %s", content)
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o600 {
		t.Fatalf("evidence mode = %v", info.Mode())
	}
}

func TestSessionFeatureEvidenceExpiresAndFailsClosedOnUnsafeRecords(t *testing.T) {
	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	root := filepath.Join(t.TempDir(), "open-agent-workflow", "codex-bridge", "features")
	store, err := NewSessionFeatureEvidenceStore(SessionFeatureEvidenceOptions{
		Root: root, TTL: time.Minute, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	context := HookContext{SessionID: "session-a", CWD: t.TempDir()}
	if err := store.RecordChildDelegation(context); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute + time.Second)
	if result := store.ObserveFeatures(context); len(result.Observations) != 0 {
		t.Fatalf("expired evidence remained available: %#v", result)
	}

	path, err := store.evidencePath(context)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		write func() error
	}{
		{name: "tampered", write: func() error {
			if err := store.RecordChildDelegation(context); err != nil {
				return err
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content = bytes.Replace(content, []byte(`"child-delegation"`), []byte(`"parallel-child-delegation"`), 1)
			return os.WriteFile(path, content, 0o600)
		}},
		{name: "malformed", write: func() error { return os.WriteFile(path, []byte(`{"schema_version":"wrong"}`), 0o600) }},
		{name: "permissive-mode", write: func() error {
			if err := store.RecordChildDelegation(context); err != nil {
				return err
			}
			return os.Chmod(path, 0o644)
		}},
		{name: "symlink", write: func() error {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			target := filepath.Join(t.TempDir(), "foreign.json")
			if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
				return err
			}
			return os.Symlink(target, path)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.write(); err != nil {
				t.Fatal(err)
			}
			result := store.ObserveFeatures(context)
			if len(result.Observations) != 0 || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "HOST_OBSERVATION_PARTIAL" {
				t.Fatalf("unsafe evidence did not fail closed: %#v", result)
			}
		})
	}
}

func TestSessionFeatureEvidenceRejectsSymlinkedRoot(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "open-agent-workflow", "codex-bridge", "features")
	if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, root); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := NewSessionFeatureEvidenceStore(SessionFeatureEvidenceOptions{Root: root, TTL: time.Minute}); err == nil {
		t.Fatal("symlinked evidence root was accepted")
	}
}

func TestSessionFeatureEvidenceAllowsReadOnlySharedStateParent(t *testing.T) {
	base := t.TempDir()
	shared := filepath.Join(base, "open-agent-workflow")
	if err := os.Mkdir(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(shared, "codex-bridge", "features")
	store, err := NewSessionFeatureEvidenceStore(SessionFeatureEvidenceOptions{Root: root, TTL: time.Minute})
	if err != nil {
		t.Fatalf("read-only shared state parent was rejected: %v", err)
	}
	context := HookContext{SessionID: "session-a", CWD: t.TempDir()}
	if err := store.RecordChildDelegation(context); err != nil {
		t.Fatal(err)
	}
	if result := store.ObserveFeatures(context); len(result.Observations) != 1 {
		t.Fatalf("live evidence unavailable: %#v", result)
	}
	for _, directory := range []string{filepath.Dir(root), root} {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("private directory mode = %v for %s", info.Mode(), directory)
		}
	}
}

func TestSessionFeatureEvidenceAllowsReadOnlyManagedStateParent(t *testing.T) {
	base := t.TempDir()
	managed := filepath.Join(base, "open-agent-workflow", "codex-bridge")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(managed, "features")
	store, err := NewSessionFeatureEvidenceStore(SessionFeatureEvidenceOptions{Root: root, TTL: time.Minute})
	if err != nil {
		t.Fatalf("read-only managed state parent was rejected: %v", err)
	}
	context := HookContext{SessionID: "session-a", CWD: t.TempDir()}
	if err := store.RecordChildDelegation(context); err != nil {
		t.Fatal(err)
	}
	if result := store.ObserveFeatures(context); len(result.Observations) != 1 {
		t.Fatalf("live evidence unavailable: %#v", result)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("feature directory mode = %v", info.Mode())
	}
}
