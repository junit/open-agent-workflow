package codex

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestRunnerExecutionProfileStagesVerifiedSkillAndIsolatesCodex(t *testing.T) {
	executionRoot := t.TempDir()
	projectRoot := t.TempDir()
	codexHome := t.TempDir()
	skillPath := writeProfileSkill(t, "review")
	verifiedEvidence, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(skillPath), "unverified-helper.sh"), []byte("unverified"), 0o600); err != nil {
		t.Fatal(err)
	}
	observation := profileObservation(skillPath, "review")
	request := profileDispatchRequest("review")

	runner := New(Options{
		Command: "codex", ExecutionRoot: executionRoot, ProjectRoot: projectRoot, CodexHome: codexHome,
		BindingResolver: func(got host.DispatchRequest) (host.BindingObservation, error) {
			if !host.EqualDispatchRequest(got, request) {
				t.Fatalf("resolver request = %#v", got)
			}
			return observation, nil
		},
	})
	runner.mcpInventory = func(context.Context, string, int64) ([]string, error) {
		t.Fatal("isolated execution profile consulted interactive MCP inventory")
		return nil, nil
	}
	runner.runProfile = func(_ context.Context, _ string, args []string, profile *executionProfile, _ int64) ([]byte, []byte, error) {
		if profile == nil || profile.projectRoot != projectRoot || profile.codexHome != codexHome {
			t.Fatalf("profile = %#v", profile)
		}
		if !pathInside(executionRoot, profile.home) || !pathInside(executionRoot, profile.workspace) {
			t.Fatalf("profile escaped execution root: %#v", profile)
		}
		for _, privatePath := range []string{filepath.Join(executionRoot, "codex-execution"), profile.root, profile.home, profile.workspace} {
			info, err := os.Stat(privatePath)
			if err != nil {
				t.Fatalf("stat private directory %q: %v", privatePath, err)
			}
			if info.Mode().Perm() != 0o700 {
				t.Fatalf("private directory %q mode = %04o", privatePath, info.Mode().Perm())
			}
		}
		info, err := os.Lstat(profile.stagedPath)
		if err != nil {
			t.Fatalf("stat staged skill: %v", err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			t.Fatalf("staged skill mode = %v", info.Mode())
		}
		stagedEvidence, err := os.ReadFile(profile.stagedPath)
		if err != nil || !bytes.Equal(stagedEvidence, verifiedEvidence) {
			t.Fatalf("staged skill changed: %q, error = %v", stagedEvidence, err)
		}
		entries, err := os.ReadDir(filepath.Dir(profile.stagedPath))
		if err != nil || len(entries) != 1 || entries[0].Name() != "SKILL.md" {
			t.Fatalf("staged skill directory = %#v, error = %v", entries, err)
		}
		joined := strings.Join(args, "\x00")
		for _, required := range []string{"--ignore-user-config", "--ignore-rules", "--disable\x00hooks", "--sandbox\x00read-only", "--add-dir\x00" + projectRoot, profile.stagedPath} {
			if !strings.Contains(joined, required) {
				t.Fatalf("Codex args = %#v, missing %q", args, required)
			}
		}
		return []byte(`{"type":"turn.completed","id":"profile-turn"}` + "\n"), nil, nil
	}

	if err := runner.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillPath, []byte("changed after Prepare"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := runner.Invoke(context.Background(), request)
	if err != nil || result.Outcome != host.DispatchSucceeded {
		t.Fatalf("Invoke() = %#v, %v", result, err)
	}
}

func TestRunnerExecutionProfileRejectsChangedBindingEvidence(t *testing.T) {
	executionRoot := t.TempDir()
	skillPath := writeProfileSkill(t, "review")
	observation := profileObservation(skillPath, "review")
	if err := os.WriteFile(skillPath, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := New(Options{
		ExecutionRoot: executionRoot, ProjectRoot: t.TempDir(), CodexHome: t.TempDir(),
		BindingResolver: func(host.DispatchRequest) (host.BindingObservation, error) { return observation, nil },
	})
	if err := runner.Prepare(context.Background(), profileDispatchRequest("review")); err == nil || !strings.Contains(err.Error(), "CODEX_BINDING_EVIDENCE_CHANGED") {
		t.Fatalf("Prepare() error = %v, want changed Binding evidence", err)
	}
}

func TestRunnerExecutionProfileRequiresBindingResolver(t *testing.T) {
	runner := New(Options{ExecutionRoot: t.TempDir(), ProjectRoot: t.TempDir(), CodexHome: t.TempDir()})
	if err := runner.Prepare(context.Background(), profileDispatchRequest("review")); err == nil || !strings.Contains(err.Error(), "CODEX_BINDING_EVIDENCE_REQUIRED") {
		t.Fatalf("Prepare() error = %v, want Binding resolver denial", err)
	}
}

func TestRunnerExecutionProfileRejectsUnmappedBindingKinds(t *testing.T) {
	skillPath := writeProfileSkill(t, "review")
	for _, kind := range []string{"agent", "tool"} {
		t.Run(kind, func(t *testing.T) {
			observation := profileObservation(skillPath, "review")
			observation.Binding.Kind = kind
			request := profileDispatchRequest("review")
			request.Binding.Kind = kind
			runner := New(Options{
				ExecutionRoot: t.TempDir(), ProjectRoot: t.TempDir(), CodexHome: t.TempDir(),
				BindingResolver: func(host.DispatchRequest) (host.BindingObservation, error) { return observation, nil },
			})
			if err := runner.Prepare(context.Background(), request); err == nil || !strings.Contains(err.Error(), "CODEX_BINDING_KIND_UNSUPPORTED") {
				t.Fatalf("Prepare() error = %v, want unsupported Binding denial", err)
			}
		})
	}
}

func TestRunCommandProfileUsesPrivateHomeAndOriginalCodexHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture executable")
	}
	root := t.TempDir()
	profile := &executionProfile{home: filepath.Join(root, "home"), workspace: filepath.Join(root, "workspace"), codexHome: filepath.Join(root, "codex")}
	for _, path := range []string{profile.home, profile.workspace, profile.codexHome} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	fixture := filepath.Join(root, "fixture")
	script := "#!/bin/sh\nprintf '%s\\n' \"$HOME\" >&2\nprintf '%s\\n' \"$CODEX_HOME\" >&2\nprintf '%s\\n' '{\"type\":\"turn.completed\",\"id\":\"fixture\"}'\n"
	if err := os.WriteFile(fixture, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := runCommandProfile(context.Background(), fixture, nil, profile, 4096)
	if err != nil || string(stderr) != profile.home+"\n"+profile.codexHome+"\n" {
		t.Fatalf("runCommandProfile() stderr = %q, err = %v", stderr, err)
	}
}

func TestProfileEnvironmentReplacesInheritedHomes(t *testing.T) {
	got := profileEnvironment([]string{"PATH=/bin", "HOME=/inherited", "codex_home=/other", "LANG=C"}, "/private/home", "/auth/codex")
	want := []string{"PATH=/bin", "LANG=C", "HOME=/private/home", "CODEX_HOME=/auth/codex"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("profileEnvironment() = %#v, want %#v", got, want)
	}
}

func TestRunnerExecutionProfileRejectsUnsafeProjectRoot(t *testing.T) {
	skillPath := writeProfileSkill(t, "review")
	observation := profileObservation(skillPath, "review")
	runner := New(Options{
		ExecutionRoot: t.TempDir(), ProjectRoot: "relative/project", CodexHome: t.TempDir(),
		BindingResolver: func(host.DispatchRequest) (host.BindingObservation, error) { return observation, nil },
	})
	if err := runner.Prepare(context.Background(), profileDispatchRequest("review")); err == nil || !strings.Contains(err.Error(), "project root must be a clean absolute path") {
		t.Fatalf("Prepare() error = %v, want unsafe project root denial", err)
	}
}

func TestRunnerExecutionProfileRejectsInvocationPathEscape(t *testing.T) {
	skillPath := writeProfileSkill(t, "review")
	observation := profileObservation(skillPath, "review")
	for _, invocationID := range []string{"../escaped", "nested/escaped"} {
		t.Run(invocationID, func(t *testing.T) {
			executionRoot := t.TempDir()
			request := profileDispatchRequest("review")
			request.InvocationID = invocationID
			runner := New(Options{
				ExecutionRoot: executionRoot, ProjectRoot: t.TempDir(), CodexHome: t.TempDir(),
				BindingResolver: func(host.DispatchRequest) (host.BindingObservation, error) { return observation, nil },
			})
			if err := runner.Prepare(context.Background(), request); err == nil || !strings.Contains(err.Error(), "Invocation ID is not a safe path component") {
				t.Fatalf("Prepare() error = %v, want path escape denial", err)
			}
		})
	}
}

func profileDispatchRequest(reference string) host.DispatchRequest {
	return host.DispatchRequest{
		GrantID: "grant-profile", InvocationID: "invocation-profile", ExecutorID: "executor-profile",
		BundleDigest: strings.Repeat("a", 64), ProviderID: "acme/provider", CapabilityID: "review", ProviderInstanceDigest: strings.Repeat("b", 64),
		Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: reference}, Effects: []string{"read-project"}, Resources: []string{"project"},
	}
}

func writeProfileSkill(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "skills", name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: "+name+"\ndescription: test\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func profileObservation(path, reference string) host.BindingObservation {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return host.BindingObservation{
		HostID: "codex", InstallationKey: "installation-acme", Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: reference},
		Source: "host-filesystem", EvidenceReference: path, Digest: canonicaljson.DigestBytes(data),
	}
}

func pathInside(root, path string) bool {
	relation, err := filepath.Rel(root, path)
	return err == nil && relation != ".." && !strings.HasPrefix(relation, ".."+string(filepath.Separator)) && !filepath.IsAbs(relation)
}
