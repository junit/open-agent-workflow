package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	oawruntime "github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func TestRunWithInputRuntimeExchangeEmitsCanonicalJSONOnly(t *testing.T) {
	projectRoot := t.TempDir()
	frame := oawruntime.RunFrame{
		SchemaVersion:  oawruntime.RuntimeSchemaV1,
		Kind:           oawruntime.FrameStart,
		MessageID:      "cli-runtime-start",
		IdempotencyKey: "cli-runtime-start",
		Start: &oawruntime.StartInput{
			RequestID: "cli-runtime-request",
			Project:   oawruntime.ProjectIdentity{Root: projectRoot, ConfigurationDigest: strings.Repeat("a", 64)},
			Proposal:  cliDirectProposal(),
		},
	}
	input, err := canonicaljson.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	status := RunWithInput(
		[]string{"runtime", "exchange", "--state-root", filepath.Join(t.TempDir(), "runtime")},
		bytes.NewReader(input), &stdout, &stderr,
	)
	if status != 0 || stderr.Len() != 0 {
		t.Fatalf("runtime exchange status=%d stderr=%q", status, stderr.String())
	}
	var reply oawruntime.RunReply
	if err := json.Unmarshal(stdout.Bytes(), &reply); err != nil {
		t.Fatalf("stdout = %q: %v", stdout.String(), err)
	}
	canonical, err := oawruntime.EncodeReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), canonical) || reply.Kind != oawruntime.ReplyModeDecided {
		t.Fatalf("runtime exchange stdout = %q", stdout.String())
	}
}

func TestRunWithInputRuntimeExchangeReturnsMachineDenialAndStderrDiagnostic(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := RunWithInput(
		[]string{"runtime", "exchange", "--state-root", filepath.Join(t.TempDir(), "runtime")},
		strings.NewReader(`{"schema_version":"oaw.runtime/v1","kind":"INSPECT","unknown":true}`),
		&stdout, &stderr,
	)
	if status == 0 || !json.Valid(stdout.Bytes()) {
		t.Fatalf("invalid frame status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	var denial struct {
		SchemaVersion string `json:"schema_version"`
		Kind          string `json:"kind"`
		Reason        string `json:"reason"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &denial); err != nil {
		t.Fatal(err)
	}
	if denial.SchemaVersion != oawruntime.RuntimeSchemaV1 || denial.Kind != "DENIED" || denial.Reason != "RUNTIME_FRAME_DECODE_INVALID" {
		t.Fatalf("denial = %#v", denial)
	}
	if !strings.Contains(stderr.String(), "RUNTIME_FRAME_DECODE_INVALID") || strings.Contains(stdout.String(), "unknown field") {
		t.Fatalf("stream separation stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestParseRunCommandAcceptsBoundedProjectRootOption(t *testing.T) {
	projectRoot := filepath.Join(string(filepath.Separator), "workspace", "project")
	parsed, err := parseRunCommand([]string{"--host", "codex", "--state-root", filepath.Join(string(filepath.Separator), "state"), "--project-root=" + projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.projectRoot != projectRoot {
		t.Fatalf("project root = %q, want %q", parsed.projectRoot, projectRoot)
	}
}

func TestParseRunCommandRejectsUnsafeProjectRootOption(t *testing.T) {
	if _, err := parseRunCommand([]string{"--host", "codex", "--project-root", "relative/project"}); err == nil {
		t.Fatal("relative project root was accepted")
	}
}

func TestNewCLIEngineRejectsStartProjectRootMismatch(t *testing.T) {
	frame := oawruntime.RunFrame{Start: &oawruntime.StartInput{Project: oawruntime.ProjectIdentity{Root: filepath.Join(string(filepath.Separator), "frame")}, Proposal: cliDirectProposal()}}
	if _, err := newCLIEngine(filepath.Join(string(filepath.Separator), "state"), filepath.Join(string(filepath.Separator), "option"), frame, "codex"); err == nil || !strings.Contains(err.Error(), "RUNTIME_PROJECT_ROOT_MISMATCH") {
		t.Fatalf("START project root mismatch error = %v", err)
	}
}

func TestRunCodexFixtureDispatchIsDeduplicatedAcrossCLIReplay(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("POSIX fixture executable")
	}
	home := t.TempDir()
	configBase := t.TempDir()
	projectRoot := t.TempDir()
	stateRoot := filepath.Join(t.TempDir(), "runtime")
	binRoot := t.TempDir()
	counterPath := filepath.Join(t.TempDir(), "codex-count")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configBase)
	t.Setenv("OAW_CODEX_FIXTURE_COUNT", counterPath)
	t.Setenv("PATH", binRoot+string(os.PathListSeparator)+os.Getenv("PATH"))

	configRoot := filepath.Join(configBase, "open-agent-workflow")
	if err := os.MkdirAll(configRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configRoot, "config.toml"), []byte("schema_version = \"oaw.user-config/v2\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	eccIndicator := filepath.Join(home, ".agents", "skills", "everything-claude-code", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(eccIndicator), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eccIndicator, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	eccAgent := filepath.Join(home, ".agents", "skills", "everything-claude-code", "agents", "code-reviewer.toml")
	if err := os.MkdirAll(filepath.Dir(eccAgent), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eccAgent, []byte("name = \"code-reviewer\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("[agents.code-reviewer]\nconfig_file = \""+eccAgent+"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixture := "#!/bin/sh\nprintf x >> \"$OAW_CODEX_FIXTURE_COUNT\"\nprintf '%s\\n' '{\"type\":\"turn.completed\",\"id\":\"fixture-turn\"}'\n"
	if err := os.WriteFile(filepath.Join(binRoot, "codex"), []byte(fixture), 0o700); err != nil {
		t.Fatal(err)
	}
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: configRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	proposal := cliDirectProposal()
	for index := range proposal.Traits {
		if proposal.Traits[index].Trait == classification.TraitBoundedCapabilityRequest {
			proposal.Traits[index].Value = classification.TraitTrue
		}
	}
	proposal.CapabilitySelector = &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}
	start := oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameStart,
		MessageID: "fixture-start", IdempotencyKey: "fixture-start",
		Start: &oawruntime.StartInput{
			RequestID: "fixture-request", Project: oawruntime.ProjectIdentity{Root: projectRoot, ConfigurationDigest: snapshot.Digest()}, Proposal: proposal,
			Bounded: &oawruntime.BoundedInput{DeliverableID: "fixture-review", InputDigest: strings.Repeat("e", 64), RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project"}, TerminationCondition: "one review", ExecutorID: "oaw-codex-write"},
		},
	}
	started := runCodexFrame(t, []string{"--host", "codex", "--state-root", stateRoot}, start)
	if len(started) != 1 || started[0].Snapshot.Status != oawruntime.RunReady {
		t.Fatalf("START replies = %#v", started)
	}
	dispatch := oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: "fixture-dispatch", IdempotencyKey: "fixture-dispatch",
		RunID: started[0].RunID, ExpectedRevision: started[0].Revision,
		Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalRequestDispatch},
	}
	args := []string{"--host", "codex", "--state-root", stateRoot, "--project-root", projectRoot}
	first := runCodexFrame(t, args, dispatch)
	if len(first) != 3 || first[len(first)-1].Kind != oawruntime.ReplyFinished {
		t.Fatalf("first dispatch replies = %#v", first)
	}
	second := runCodexFrame(t, args, dispatch)
	if len(second) < 2 || second[len(second)-1].Snapshot.Status != oawruntime.RunFinished {
		t.Fatalf("replayed dispatch replies = %#v", second)
	}
	count, err := os.ReadFile(counterPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(count) != "x" {
		t.Fatalf("Codex fixture invocation count = %q, want one call", count)
	}
}

func TestRunCodexReportsAmbiguousProviderWithoutDispatch(t *testing.T) {
	userHome := t.TempDir()
	configBase := t.TempDir()
	projectRoot := t.TempDir()
	stateRoot := filepath.Join(t.TempDir(), "runtime")
	counterPath := filepath.Join(t.TempDir(), "codex-count")
	t.Setenv("HOME", userHome)
	t.Setenv("XDG_CONFIG_HOME", configBase)
	t.Setenv("OAW_CODEX_FIXTURE_COUNT", counterPath)
	t.Setenv("PATH", t.TempDir())

	for _, version := range []string{"6.0.3", "6.1.1"} {
		indicator := filepath.Join(userHome, ".codex", "plugins", "cache", "openai-api-curated", "superpowers", version, "skills", "using-superpowers", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(indicator), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(indicator, []byte(version), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	configRoot := filepath.Join(configBase, "open-agent-workflow")
	if err := os.MkdirAll(configRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: configRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	proposal := cliDirectProposal()
	for index := range proposal.Traits {
		if proposal.Traits[index].Trait == classification.TraitBoundedCapabilityRequest {
			proposal.Traits[index].Value = classification.TraitTrue
		}
	}
	proposal.CapabilitySelector = &classification.CapabilitySelector{ProviderID: "oaw/superpowers", CapabilityID: "review", Source: classification.SelectorUserIntent}
	frame := oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameStart,
		MessageID: "ambiguous-start", IdempotencyKey: "ambiguous-start",
		Start: &oawruntime.StartInput{
			RequestID: "ambiguous-request", Project: oawruntime.ProjectIdentity{Root: projectRoot, ConfigurationDigest: snapshot.Digest()}, Proposal: proposal,
			Bounded: &oawruntime.BoundedInput{DeliverableID: "ambiguous-review", InputDigest: strings.Repeat("f", 64), RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project"}, TerminationCondition: "one review", ExecutorID: "oaw-codex-write"},
		},
	}
	raw, err := canonicaljson.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	status := RunWithInput([]string{"run", "--host", "codex", "--state-root", stateRoot}, bytes.NewReader(raw), &stdout, &stderr)
	if status != 0 || stderr.Len() != 0 {
		t.Fatalf("run status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	var reply oawruntime.RunReply
	if err := json.Unmarshal(stdout.Bytes(), &reply); err != nil {
		t.Fatalf("stdout = %q: %v", stdout.String(), err)
	}
	if reply.Kind != oawruntime.ReplyCapabilitySelectionRequired || reply.Snapshot.Status != oawruntime.RunAwaitingCapability || len(reply.Diagnostics) != 1 || reply.Diagnostics[0].Code != "PROVIDER_CANDIDATE_AMBIGUOUS" {
		t.Fatalf("reply = %#v", reply)
	}
	encoded, err := oawruntime.EncodeReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(stdout.Bytes()), encoded) {
		t.Fatalf("stdout is not canonical Runtime JSON: %q", stdout.String())
	}
	if _, err := os.Stat(counterPath); !os.IsNotExist(err) {
		t.Fatalf("Codex dispatch occurred: %v", err)
	}
}

func runCodexFrame(t *testing.T, args []string, frame oawruntime.RunFrame) []oawruntime.RunReply {
	t.Helper()
	input, err := canonicaljson.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := runCodex(args, bytes.NewReader(input), &stdout, &stderr); status != 0 {
		t.Fatalf("runCodex status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	lines := bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte{'\n'})
	replies := make([]oawruntime.RunReply, len(lines))
	for index, line := range lines {
		if err := json.Unmarshal(line, &replies[index]); err != nil {
			t.Fatalf("reply %q: %v", line, err)
		}
	}
	return replies
}

func cliDirectProposal() *classification.ClassificationProposal {
	trueTraits := map[classification.Trait]bool{
		classification.TraitScopeClear:               true,
		classification.TraitChangePointKnown:         true,
		classification.TraitRecoverable:              true,
		classification.TraitFocusedVerificationKnown: true,
	}
	traits := make([]classification.TraitObservation, 0, 19)
	for _, trait := range []classification.Trait{
		classification.TraitScopeClear,
		classification.TraitChangePointKnown,
		classification.TraitRecoverable,
		classification.TraitFocusedVerificationKnown,
		classification.TraitBoundedCapabilityRequest,
		classification.TraitArchitectureDecision,
		classification.TraitPublicContractChange,
		classification.TraitSchemaChange,
		classification.TraitDependencyChange,
		classification.TraitSecuritySensitive,
		classification.TraitDataSensitive,
		classification.TraitDeploymentChange,
		classification.TraitDomainUncertainty,
		classification.TraitRootCauseUncertain,
		classification.TraitMultipleResponsibilities,
		classification.TraitMultipleTickets,
		classification.TraitLongLivedDelegation,
		classification.TraitDestructiveMutation,
		classification.TraitCriticalRelease,
	} {
		value := classification.TraitFalse
		if trueTraits[trait] {
			value = classification.TraitTrue
		}
		traits = append(traits, classification.TraitObservation{Trait: trait, Value: value})
	}
	return &classification.ClassificationProposal{
		SchemaVersion: classification.ProposalSchemaV1,
		Traits:        traits,
		Resources:     []classification.Resource{classification.ResourceProject},
		Evidence: []classification.ProposalEvidence{
			{Kind: classification.EvidenceScope, Reference: "test:scope", Digest: strings.Repeat("b", 64)},
			{Kind: classification.EvidenceChangePoint, Reference: "test:change", Digest: strings.Repeat("c", 64)},
			{Kind: classification.EvidenceVerification, Reference: "test:verification", Digest: strings.Repeat("d", 64)},
		},
	}
}
