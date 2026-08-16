package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	oaw "github.com/wifibaby4u/open-agent-workflow"
)

func TestParseManagementAcceptsBashOptions(t *testing.T) {
	project := t.TempDir()
	for _, operation := range []string{"install", "update", "uninstall"} {
		t.Run(operation, func(t *testing.T) {
			tests := []struct {
				name string
				args []string
				want managementCommand
			}{
				{name: "empty", args: []string{operation}, want: managementCommand{operation: operation}},
				{name: "separate values", args: []string{operation, "--target", "codex,claude", "--project", project, "--dry-run", "--force"}, want: managementCommand{operation: operation, targets: "codex,claude", project: project, dryRun: true, force: true}},
				{name: "equals values", args: []string{operation, "--target=claude", "--project=" + project}, want: managementCommand{operation: operation, targets: "claude", project: project}},
				{name: "short help", args: []string{operation, "-h"}, want: managementCommand{operation: operation, help: true}},
				{name: "long help", args: []string{operation, "--help"}, want: managementCommand{operation: operation, help: true}},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := parseManagement(tt.args)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, tt.want) {
						t.Fatalf("parseManagement() = %#v, want %#v", got, tt.want)
					}
				})
			}
		})
	}
}

func TestRunManagementMatchesBashArgumentErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "wrong command", args: []string{"check"}, message: "unknown command 'check'"},
		{name: "missing target", args: []string{"install", "--target"}, message: "--target requires a value"},
		{name: "empty target", args: []string{"install", "--target="}, message: "--target requires a value"},
		{name: "dash target", args: []string{"install", "--target", "--force"}, message: "--target requires a value"},
		{name: "duplicate target", args: []string{"install", "--target", "claude", "--target=codex"}, message: "--target may be specified only once"},
		{name: "missing project", args: []string{"install", "--project"}, message: "--project requires a value"},
		{name: "empty project", args: []string{"install", "--project="}, message: "--project requires a value"},
		{name: "duplicate project", args: []string{"install", "--project", "/one", "--project=/two"}, message: "--project may be specified only once"},
		{name: "duplicate dry run", args: []string{"install", "--dry-run", "--dry-run"}, message: "--dry-run may be specified only once"},
		{name: "duplicate force", args: []string{"install", "--force", "--force"}, message: "--force may be specified only once"},
		{name: "duplicate help", args: []string{"install", "--help", "-h"}, message: "--help may be specified only once"},
		{name: "unknown option", args: []string{"install", "--bogus"}, message: "unknown option '--bogus'"},
		{name: "operand", args: []string{"install", "operand"}, message: "unexpected argument 'operand'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if status := runManagement(tt.args, &stdout, &stderr); status != 64 {
				t.Fatalf("runManagement() = %d, stdout=%q stderr=%q", status, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 || stderr.String() != "oaw: error: "+tt.message+"\n" {
				t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunManagementHelpAndLifecycle(t *testing.T) {
	var stdout, stderr bytes.Buffer
	for _, operation := range []string{"install", "update", "uninstall"} {
		stdout.Reset()
		stderr.Reset()
		if status := runManagement([]string{operation, "--help"}, &stdout, &stderr); status != 0 {
			t.Fatalf("%s help status=%d stderr=%q", operation, status, stderr.String())
		}
		if stderr.Len() != 0 || stdout.String() != installerUsage() {
			t.Fatalf("%s help stdout=%q stderr=%q", operation, stdout.String(), stderr.String())
		}
	}

	root := t.TempDir()
	home := filepath.Join(root, "home")
	config := filepath.Join(root, "config")
	state := filepath.Join(root, "state")
	for _, directory := range []string{home, config, state} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_STATE_HOME", state)
	stdout.Reset()
	stderr.Reset()
	if status := runManagement([]string{"install", "--target", "claude", "--dry-run"}, &stdout, &stderr); status != 0 {
		t.Fatalf("dry-run status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 || !strings.Contains(stdout.String(), "oaw: would-create: "+filepath.Join(home, ".claude", "CLAUDE.md")+"\n") {
		t.Fatalf("dry-run stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	for _, directory := range []string{home, config, state} {
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != 0 {
			t.Fatalf("dry-run changed %s: entries=%v error=%v", directory, entries, err)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if status := runManagement([]string{"update", "--target", "claude"}, &stdout, &stderr); status != 66 {
		t.Fatalf("missing-state update status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 || stderr.String() != "oaw: error: no installation state; run install first\n" {
		t.Fatalf("missing-state update stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if status := runManagement([]string{"install", "--target", "claude"}, &stdout, &stderr); status != 0 {
		t.Fatalf("install status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if status := runManagement([]string{"update", "--target", "claude"}, &stdout, &stderr); status != 0 {
		t.Fatalf("update status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "oaw: unchanged: claude\n") || stderr.Len() != 0 {
		t.Fatalf("update stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if status := runManagement([]string{"uninstall", "--target", "claude"}, &stdout, &stderr); status != 0 {
		t.Fatalf("uninstall status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "oaw: remove: ") || stderr.Len() != 0 {
		t.Fatalf("uninstall stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestProjectInstallWritesSelfContainedPolicySet(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	config := filepath.Join(root, "config")
	state := filepath.Join(root, "state")
	project := filepath.Join(root, "project")
	customProfile := filepath.Join(project, ".oaw", "profiles", "team-delivery.md")
	for _, directory := range []string{home, config, state, filepath.Dir(customProfile)} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	const customContent = "---\nid: team-delivery\nname: Team Delivery\n---\n"
	if err := os.WriteFile(customProfile, []byte(customContent), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_STATE_HOME", state)

	var stdout, stderr bytes.Buffer
	args := []string{"install", "--project", project, "--target", "codex"}
	if status := Run(args, &stdout, &stderr); status != 0 {
		t.Fatalf("Run(%v)=%d stdout=%q stderr=%q", args, status, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("Run(%v) stderr=%q", args, stderr.String())
	}

	for _, file := range oaw.CanonicalPolicySet() {
		installed := filepath.Join(project, ".oaw", "policy", filepath.FromSlash(file.Path))
		got, err := os.ReadFile(installed)
		if err != nil {
			t.Fatalf("read installed Policy Set file %q: %v", file.Path, err)
		}
		if !bytes.Equal(got, file.Content) {
			t.Errorf("installed Policy Set file %q differs from canonical content", file.Path)
		}
	}

	router, err := os.ReadFile(filepath.Join(project, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(router, []byte("read `.oaw/policy/POLICY.md`")) {
		t.Fatalf("project Activation Router is not project-relative: %q", router)
	}
	if bytes.Contains(router, []byte(project)) {
		t.Fatalf("project Activation Router contains physical project path: %q", router)
	}
	if _, err := os.Stat(filepath.Join(config, "open-agent-workflow", "ENGINEERING.md")); !os.IsNotExist(err) {
		t.Fatalf("project install wrote legacy global policy: %v", err)
	}
	gotCustom, err := os.ReadFile(customProfile)
	if err != nil || string(gotCustom) != customContent {
		t.Fatalf("project install changed Custom Profile: content=%q error=%v", gotCustom, err)
	}
}

func TestUserInstallWritesCompletePolicySetWithProjectPrecedence(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	config := filepath.Join(root, "config")
	state := filepath.Join(root, "state")
	userPolicyRoot := filepath.Join(config, "open-agent-workflow")
	customProfile := filepath.Join(userPolicyRoot, "profiles", "team-delivery.md")
	for _, directory := range []string{home, state, filepath.Dir(customProfile)} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	const customContent = "---\nid: team-delivery\nname: Team Delivery\n---\n"
	if err := os.WriteFile(customProfile, []byte(customContent), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_STATE_HOME", state)

	var stdout, stderr bytes.Buffer
	args := []string{"install", "--target", "codex"}
	if status := Run(args, &stdout, &stderr); status != 0 {
		t.Fatalf("Run(%v)=%d stdout=%q stderr=%q", args, status, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("Run(%v) stderr=%q", args, stderr.String())
	}

	for _, file := range oaw.CanonicalPolicySet() {
		relative := file.Path
		if strings.HasPrefix(relative, "profiles/") {
			relative = filepath.ToSlash(filepath.Join("profiles", "builtin", strings.TrimPrefix(relative, "profiles/")))
		}
		installed := filepath.Join(userPolicyRoot, filepath.FromSlash(relative))
		content, err := os.ReadFile(installed)
		if err != nil {
			t.Fatalf("read installed user Policy Set file %q: %v", relative, err)
		}
		if len(bytes.TrimSpace(content)) == 0 {
			t.Errorf("installed user Policy Set file %q is empty", relative)
		}
	}

	policy, err := os.ReadFile(filepath.Join(userPolicyRoot, "POLICY.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(policy, []byte("profiles/builtin/SP-FULL.md")) {
		t.Fatalf("user Policy links do not match the builtin Profile layout: %q", policy)
	}
	adapterDetails := map[string][]string{
		"claude":   {".claude/skills", "claude plugin list --json", "Claude Plan Mode"},
		"codex":    {".codex/skills", ".codex/plugins/cache", "Codex /plan"},
		"gemini":   {"gemini skills list --all", "gemini extensions list", "Gemini Plan mode"},
		"opencode": {"opencode debug skill", "skills.paths", ".opencode/command(s)"},
		"cursor":   {".cursor/skills", ".cursor/rules/open-agent-workflow.mdc", "Team Rule policy"},
		"windsurf": {".windsurf/skills", ".devin/rules/open-agent-workflow.md", "Quick Review"},
		"cline":    {".cline/skills", ".clinerules/skills", "Plan and Act modes"},
		"roo":      {".roo/skills", "skills-<mode>", "Roo's mode permissions"},
		"copilot":  {".github/skills", "copilot plugin list", "/fleet"},
	}
	for host, details := range adapterDetails {
		adapter, err := os.ReadFile(filepath.Join(userPolicyRoot, "adapters", host+"-policy.md"))
		if err != nil {
			t.Fatalf("read %s Adapter: %v", host, err)
		}
		for _, path := range []string{".oaw/profiles/", "open-agent-workflow/profiles/"} {
			if !bytes.Contains(adapter, []byte(path)) {
				t.Errorf("%s Adapter omits source-qualified Custom Profile path %q", host, path)
			}
		}
		for _, detail := range details {
			if !bytes.Contains(adapter, []byte(detail)) {
				t.Errorf("%s Adapter omits Host-specific detail %q", host, detail)
			}
		}
	}

	router, err := os.ReadFile(filepath.Join(home, ".codex", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		".oaw/policy/POLICY.md",
		filepath.Join(userPolicyRoot, "POLICY.md"),
		"do not read or merge the User Policy Set",
	} {
		if !bytes.Contains(router, []byte(required)) {
			t.Fatalf("user Activation Router omits %q: %q", required, router)
		}
	}

	gotCustom, err := os.ReadFile(customProfile)
	if err != nil || string(gotCustom) != customContent {
		t.Fatalf("user install changed Custom Profile: content=%q error=%v", gotCustom, err)
	}
	if _, err := os.Stat(filepath.Join(userPolicyRoot, "ENGINEERING.md")); !os.IsNotExist(err) {
		t.Fatalf("user install retained legacy ENGINEERING.md: %v", err)
	}
	stateContent, err := os.ReadFile(filepath.Join(state, "open-agent-workflow", "installations", "user.state"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := bytes.Count(stateContent, []byte("policy-file\t")), len(oaw.CanonicalPolicySet()); got != want {
		t.Fatalf("user state Policy Set records=%d want=%d\n%s", got, want, stateContent)
	}
}

func TestInstalledProjectPolicySetWorksWithoutOAWExecutable(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	config := filepath.Join(root, "config")
	state := filepath.Join(root, "state")
	project := filepath.Join(root, "project")
	emptyPath := filepath.Join(root, "empty-bin")
	for _, directory := range []string{home, config, state, project, emptyPath} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_STATE_HOME", state)

	var stdout, stderr bytes.Buffer
	if status := Run([]string{"install", "--project", project, "--target", "codex"}, &stdout, &stderr); status != 0 {
		t.Fatalf("install status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	t.Setenv("PATH", emptyPath)
	if path, err := exec.LookPath("oaw"); err == nil {
		t.Fatalf("oaw executable unexpectedly remains available at %q", path)
	}

	installed := make([]oaw.PolicyFile, 0, len(oaw.CanonicalPolicySet()))
	for _, canonical := range oaw.CanonicalPolicySet() {
		content, err := os.ReadFile(filepath.Join(project, ".oaw", "policy", filepath.FromSlash(canonical.Path)))
		if err != nil {
			t.Fatalf("read installed Policy Set file %q: %v", canonical.Path, err)
		}
		installed = append(installed, oaw.PolicyFile{Path: canonical.Path, Content: content})
	}
	if err := oaw.ValidatePolicySet(installed); err != nil {
		t.Fatalf("installed Policy Set is not self-contained: %v", err)
	}
	profileContent, err := os.ReadFile(filepath.Join(project, ".oaw", "policy", "profiles", "MATT-FULL.md"))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := oaw.ParsePolicyProfile("profiles/MATT-FULL.md", profileContent)
	if err != nil {
		t.Fatalf("parse installed MATT-FULL Profile: %v", err)
	}
	if len(profile.Responsibilities) != len(oaw.PolicyResponsibilities()) {
		t.Fatalf("installed Profile Responsibilities=%d want=%d", len(profile.Responsibilities), len(oaw.PolicyResponsibilities()))
	}
}

func TestPublicRunRoutesManagementAndPreservesPublicCommands(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	config := filepath.Join(root, "config")
	state := filepath.Join(root, "state")
	for _, directory := range []string{home, config, state} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_STATE_HOME", state)

	for _, args := range [][]string{nil, {"help"}, {"-h"}, {"--help"}} {
		var stdout, stderr bytes.Buffer
		if status := Run(args, &stdout, &stderr); status != 0 {
			t.Fatalf("Run(%v)=%d stdout=%q stderr=%q", args, status, stdout.String(), stderr.String())
		}
		if stdout.String() != rootUsage() || stderr.Len() != 0 {
			t.Fatalf("Run(%v) stdout=%q stderr=%q", args, stdout.String(), stderr.String())
		}
	}

	var stdout, stderr bytes.Buffer
	if status := Run([]string{"profile", "list"}, &stdout, &stderr); status != 0 {
		t.Fatalf("profile list status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "built-in:SP-FULL") || stderr.Len() != 0 {
		t.Fatalf("profile list stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if status := Run([]string{"check", "--target", "claude"}, &stdout, &stderr); status != 0 {
		t.Fatalf("check status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "installed claude: not-installed\n") || stderr.Len() != 0 {
		t.Fatalf("check stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if status := Run([]string{"unknown"}, &stdout, &stderr); status != 64 {
		t.Fatalf("unknown status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 || stderr.String() != "oaw: error: unknown command 'unknown'\n" {
		t.Fatalf("unknown stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	for _, operation := range []string{"install", "update", "uninstall"} {
		stdout.Reset()
		stderr.Reset()
		if status := Run([]string{operation, "--target", "claude"}, &stdout, &stderr); status != 0 {
			t.Fatalf("Run(%s)=%d stdout=%q stderr=%q", operation, status, stdout.String(), stderr.String())
		}
		if stdout.Len() == 0 || stderr.Len() != 0 {
			t.Fatalf("%s stdout=%q stderr=%q", operation, stdout.String(), stderr.String())
		}
	}
}

func TestPublicManagementDoesNotImportPolicyOnlyOrInstallStateIntoRuntimeState(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	config := filepath.Join(root, "config")
	state := filepath.Join(root, "state")
	project := filepath.Join(root, "policy-only-project")
	policyOnly := filepath.Join(project, ".scratch", "existing-task", "workflow.md")
	for _, directory := range []string{home, config, state, filepath.Dir(policyOnly)} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	const policyOnlyContent = "profile: ECC-FULL\nstage: implementation\n"
	if err := os.WriteFile(policyOnly, []byte(policyOnlyContent), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_STATE_HOME", state)

	runPublicManagement := func(operation string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		args := []string{operation, "--project", project, "--target", "cursor"}
		if status := Run(args, &stdout, &stderr); status != 0 {
			t.Fatalf("Run(%v)=%d stdout=%q stderr=%q", args, status, stdout.String(), stderr.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("Run(%v) stderr=%q", args, stderr.String())
		}
	}

	runPublicManagement("install")
	runtimeRoot := filepath.Join(state, "open-agent-workflow", "runtime")
	if _, err := os.Stat(runtimeRoot); !os.IsNotExist(err) {
		t.Fatalf("install created Runtime State: %v", err)
	}
	installStates, err := filepath.Glob(filepath.Join(state, "open-agent-workflow", "installations", "projects", "*.state"))
	if err != nil || len(installStates) != 1 {
		t.Fatalf("Install State paths=%v error=%v", installStates, err)
	}

	if err := os.MkdirAll(runtimeRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	runtimeSentinel := filepath.Join(runtimeRoot, "preexisting-runtime-sentinel")
	const runtimeContent = "unrelated Runtime State\n"
	if err := os.WriteFile(runtimeSentinel, []byte(runtimeContent), 0o600); err != nil {
		t.Fatal(err)
	}

	runPublicManagement("update")
	runPublicManagement("uninstall")

	policyBytes, err := os.ReadFile(policyOnly)
	if err != nil || string(policyBytes) != policyOnlyContent {
		t.Fatalf("Policy-only task changed: bytes=%q error=%v", policyBytes, err)
	}
	runtimeBytes, err := os.ReadFile(runtimeSentinel)
	if err != nil || string(runtimeBytes) != runtimeContent {
		t.Fatalf("Runtime sentinel changed: bytes=%q error=%v", runtimeBytes, err)
	}
	runtimeEntries, err := os.ReadDir(runtimeRoot)
	if err != nil || len(runtimeEntries) != 1 || runtimeEntries[0].Name() != filepath.Base(runtimeSentinel) {
		t.Fatalf("Runtime State tree changed: entries=%v error=%v", runtimeEntries, err)
	}
	if _, err := os.Stat(installStates[0]); !os.IsNotExist(err) {
		t.Fatalf("uninstall did not consume Install State: %v", err)
	}
}
