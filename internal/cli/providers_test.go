package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestParseProvidersCommand(t *testing.T) {
	projectRoot := filepath.Join(string(filepath.Separator), "workspace", "project")
	accepted := []struct {
		args []string
		want providerCommand
	}{
		{[]string{"inspect", "--host", "codex"}, providerCommand{hostID: "codex", format: "text"}},
		{[]string{"inspect", "--host", "codex", "--strict"}, providerCommand{hostID: "codex", format: "text", strict: true}},
		{[]string{"inspect", "--host=codex", "--format=text"}, providerCommand{hostID: "codex", format: "text"}},
		{[]string{"inspect", "--host", "codex", "--project-root", projectRoot, "--format", "json"}, providerCommand{hostID: "codex", projectRoot: projectRoot, format: "json"}},
	}
	for _, test := range accepted {
		got, err := parseProvidersCommand(test.args)
		if err != nil || got != test.want {
			t.Fatalf("parseProvidersCommand(%v) = %#v, %v; want %#v", test.args, got, err, test.want)
		}
	}
	for _, args := range [][]string{
		{"inspect"},
		{"inspect", "--host", "codex", "--host", "codex"},
		{"inspect", "--host=codex", "--project-root", "relative/project"},
		{"inspect", "--host=codex", "--format", "json", "--format=text"},
		{"inspect", "--host=codex", "--format=yaml"},
		{"unknown", "--host=codex"},
		{"inspect", "--host=codex", "operand"},
	} {
		if _, err := parseProvidersCommand(args); err == nil {
			t.Fatalf("parseProvidersCommand(%v) accepted invalid arguments", args)
		}
	}
}

func TestStrictProviderInspectionFailsClosedWithoutBridgeInventory(t *testing.T) {
	newProviderInspectionHostsFixture(t, []string{"current"}, false)
	var stdout, stderr bytes.Buffer
	status := RunWithInput(
		[]string{"providers", "inspect", "--host", "codex", "--strict"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if status != 69 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "HOST_BRIDGE_UNAVAILABLE") || !strings.Contains(stderr.String(), "invoke core.inspect in the active Codex session") {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
}

func TestProvidersInspectV3SeparatesCurrentDiscoveryFromForeignDiagnostics(t *testing.T) {
	newProviderInspectionHostsFixture(t, []string{"current"}, true)
	var stdout, stderr bytes.Buffer
	if status := RunWithInput([]string{"providers", "inspect", "--host=codex", "--format=json"}, strings.NewReader(""), &stdout, &stderr); status != 0 || stderr.Len() != 0 {
		t.Fatalf("inspect status=%d stderr=%q", status, stderr.String())
	}
	var output providerInspectionV3Document
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("JSON output = %q: %v", stdout.String(), err)
	}
	if output.SchemaVersion != "oaw.provider-inspection/v3" || output.CurrentHost.HostID != "codex" || output.CurrentHost.DiscoveryDigest == "" || output.CurrentHost.ResolutionDigest == "" || output.CurrentHost.RegistryDigest == "" {
		t.Fatalf("current Host = %#v", output)
	}
	current := inspectionProviderByID(t, output.CurrentHost.Providers, "oaw/superpowers")
	if current.State != registry.ProviderCandidate || current.Reason != "HOST_BINDING_EVIDENCE_REQUIRED" || current.Instance != nil || len(current.Candidates) != 1 {
		t.Fatalf("current Provider = %#v", current)
	}
	if candidate := current.Candidates[0]; candidate.HostID != "codex" || candidate.SurfaceID == "" || candidate.DistributionKey == "" || candidate.InstallationKey == "" || strings.Contains(candidate.Location, ".claude") || candidate.ProviderPin != nil {
		t.Fatalf("current Candidate = %#v", candidate)
	}
	if len(output.ForeignHosts) != 1 || output.ForeignHosts[0].HostID != "claude" || output.ForeignHosts[0].DiscoveryDigest == "" {
		t.Fatalf("foreign Hosts = %#v", output.ForeignHosts)
	}
	foreign := inspectionForeignProviderByID(t, output.ForeignHosts[0].Providers, "oaw/superpowers")
	if foreign.DiagnosticReason != "PROVIDER_FOREIGN_HOST_ONLY" || len(foreign.Candidates) != 1 || foreign.Candidates[0].HostID != "claude" || !strings.Contains(foreign.Candidates[0].Location, ".claude") || foreign.Candidates[0].ProviderPin != nil {
		t.Fatalf("foreign Provider = %#v", foreign)
	}
}

func TestProvidersInspectForeignOnlyLeavesCurrentResolutionNotFound(t *testing.T) {
	newProviderInspectionHostsFixture(t, nil, true)
	var stdout, stderr bytes.Buffer
	if status := RunWithInput([]string{"providers", "inspect", "--host=codex", "--format=json"}, strings.NewReader(""), &stdout, &stderr); status != 0 || stderr.Len() != 0 {
		t.Fatalf("inspect status=%d stderr=%q", status, stderr.String())
	}
	var output providerInspectionV3Document
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("JSON output = %q: %v", stdout.String(), err)
	}
	current := inspectionProviderByID(t, output.CurrentHost.Providers, "oaw/superpowers")
	if current.State != registry.ProviderNotFound || current.Reason != "PROVIDER_NOT_FOUND" || len(current.Candidates) != 0 {
		t.Fatalf("current Provider = %#v", current)
	}
	if len(output.ForeignHosts) != 1 {
		t.Fatalf("foreign Hosts = %#v", output.ForeignHosts)
	}
	foreign := inspectionForeignProviderByID(t, output.ForeignHosts[0].Providers, "oaw/superpowers")
	if foreign.DiagnosticReason != "PROVIDER_FOREIGN_HOST_ONLY" || len(foreign.Candidates) != 1 || foreign.Candidates[0].ProviderPin != nil {
		t.Fatalf("foreign Provider = %#v", foreign)
	}
}

func TestProvidersInspectSupportsPolicyOnlyHost(t *testing.T) {
	newProviderInspectionHostsFixture(t, nil, true)
	var stdout, stderr bytes.Buffer
	if status := RunWithInput([]string{"providers", "inspect", "--host=claude", "--format=json"}, strings.NewReader(""), &stdout, &stderr); status != 0 || stderr.Len() != 0 {
		t.Fatalf("inspect status=%d stderr=%q", status, stderr.String())
	}
	var output providerInspectionV3Document
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("JSON output = %q: %v", stdout.String(), err)
	}
	if output.CurrentHost.HostID != "claude" || output.CurrentHost.RegistryDigest == "" {
		t.Fatalf("policy-only current Host = %#v", output.CurrentHost)
	}
	provider := inspectionProviderByID(t, output.CurrentHost.Providers, "oaw/superpowers")
	if provider.State != registry.ProviderCandidate || provider.Reason != "HOST_BINDING_EVIDENCE_REQUIRED" || provider.Instance != nil || len(provider.Candidates) != 1 || provider.Candidates[0].ProviderPin != nil {
		t.Fatalf("policy-only Provider = %#v", provider)
	}
}

func TestProvidersInspectTextAndJSONAreDeterministic(t *testing.T) {
	newProviderInspectionFixture(t, false)
	args := []string{"providers", "inspect", "--host", "codex", "--format", "text"}
	var firstText, firstErr bytes.Buffer
	if status := RunWithInput(args, strings.NewReader(""), &firstText, &firstErr); status != 0 || firstErr.Len() != 0 {
		t.Fatalf("text inspect status=%d stderr=%q", status, firstErr.String())
	}
	var secondText bytes.Buffer
	if status := RunWithInput(args, strings.NewReader(""), &secondText, new(bytes.Buffer)); status != 0 {
		t.Fatal("second text inspection failed")
	}
	if firstText.String() != secondText.String() {
		t.Fatal("text inspection is not deterministic")
	}
	if !strings.Contains(firstText.String(), "provider oaw/superpowers state=ambiguous reason=PROVIDER_CANDIDATE_AMBIGUOUS") || !strings.Contains(firstText.String(), "schema_version = \"oaw.user-config/v3\"") {
		t.Fatalf("text output = %q", firstText.String())
	}
	for _, field := range []string{"provider_id =", "host_id =", "installation_key =", "evidence_digest ="} {
		if !strings.Contains(firstText.String(), field) {
			t.Fatalf("text output omits exact pin field %q: %q", field, firstText.String())
		}
	}
	if strings.Contains(firstText.String(), "indicator-secret") || strings.Contains(firstText.String(), "SKILL.md") {
		t.Fatalf("text output leaked evidence content/path: %q", firstText.String())
	}
	documents := pinDocumentsFromText(firstText.String())
	if len(documents) != 3 {
		t.Fatalf("pin documents = %#v", documents)
	}
	for _, documentText := range documents {
		var document config.UserConfigRecord
		if _, err := toml.Decode(documentText, &document); err != nil {
			t.Fatalf("pin TOML = %q: %v", documentText, err)
		}
		if len(document.ProviderPins) != 1 {
			t.Fatalf("ProviderPins = %#v", document.ProviderPins)
		}
	}

	jsonArgs := []string{"providers", "inspect", "--host=codex", "--format=json"}
	var firstJSON, secondJSON bytes.Buffer
	if status := RunWithInput(jsonArgs, strings.NewReader(""), &firstJSON, new(bytes.Buffer)); status != 0 {
		t.Fatal("JSON inspection failed")
	}
	if status := RunWithInput(jsonArgs, strings.NewReader(""), &secondJSON, new(bytes.Buffer)); status != 0 {
		t.Fatal("second JSON inspection failed")
	}
	if firstJSON.String() != secondJSON.String() {
		t.Fatal("JSON inspection is not deterministic")
	}
	var output providerInspectionOutput
	if err := json.Unmarshal(firstJSON.Bytes(), &output); err != nil {
		t.Fatalf("JSON output = %q: %v", firstJSON.String(), err)
	}
	if output.SchemaVersion != providerInspectionSchemaV3 || output.CurrentHost.HostID != "codex" || output.ConfigurationDigest == "" || output.CatalogDigest == "" || output.CurrentHost.DiscoveryDigest == "" || output.CurrentHost.ResolutionDigest == "" || output.CurrentHost.RegistryDigest == "" {
		t.Fatalf("output = %#v", output)
	}
	found := false
	for _, provider := range output.CurrentHost.Providers {
		if provider.ProviderID != "oaw/superpowers" {
			continue
		}
		found = true
		if provider.State != registry.ProviderAmbiguous || len(provider.Candidates) != 3 {
			t.Fatalf("superpowers = %#v", provider)
		}
		for _, candidate := range provider.Candidates {
			if candidate.ProviderPin == nil || candidate.ProviderPin.ProviderID != provider.ProviderID || candidate.ProviderPin.HostID != "codex" || candidate.ProviderPin.InstallationKey == "" || candidate.ProviderPin.EvidenceDigest != candidate.EvidenceDigest || candidate.ProviderPin.Location != candidate.Location || candidate.ProviderPin.Version != candidate.Version {
				t.Fatalf("candidate pin = %#v", candidate)
			}
		}
	}
	if !found {
		t.Fatalf("providers = %#v", output.CurrentHost.Providers)
	}
}

func TestProvidersInspectDoesNotMutateUserConfig(t *testing.T) {
	fixture := newProviderInspectionFixture(t, true)
	path := fixture.configPath
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	mode, modTime := info.Mode(), info.ModTime()
	for _, format := range []string{"text", "json"} {
		var stdout, stderr bytes.Buffer
		if status := RunWithInput([]string{"providers", "inspect", "--host=codex", "--format=" + format}, strings.NewReader(""), &stdout, &stderr); status != 0 || stderr.Len() != 0 {
			t.Fatalf("format=%s status=%d stderr=%q", format, status, stderr.String())
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) || updated.Mode() != mode || !updated.ModTime().Equal(modTime) {
		t.Fatalf("user config changed: bytes=%v mode=%v modtime=%v", !bytes.Equal(got, original), updated.Mode(), updated.ModTime())
	}

	missing := filepath.Join(t.TempDir(), "open-agent-workflow", "config.toml")
	var stdout, stderr bytes.Buffer
	if status := RunWithInput([]string{"providers", "inspect", "--host=codex"}, strings.NewReader(""), &stdout, &stderr); status != 0 || stderr.Len() != 0 {
		t.Fatalf("absent config status=%d stderr=%q", status, stderr.String())
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("absent config was created: %v", err)
	}
}

type providerInspectionFixture struct {
	configPath string
}

type providerInspectionV3Document struct {
	SchemaVersion string `json:"schema_version"`
	CurrentHost   struct {
		HostID           string                         `json:"host_id"`
		DiscoveryDigest  string                         `json:"discovery_digest"`
		ResolutionDigest string                         `json:"resolution_digest"`
		RegistryDigest   string                         `json:"registry_digest"`
		Providers        []providerInspectionV2Provider `json:"providers"`
	} `json:"current_host"`
	ForeignHosts []struct {
		HostID          string                                `json:"host_id"`
		DiscoveryDigest string                                `json:"discovery_digest"`
		Providers       []providerInspectionV2ForeignProvider `json:"providers"`
	} `json:"foreign_hosts"`
}

type providerInspectionV2Provider struct {
	ProviderID string                          `json:"provider_id"`
	State      registry.ProviderState          `json:"state"`
	Reason     string                          `json:"reason"`
	Instance   *providerInspectionInstance     `json:"instance"`
	Candidates []providerInspectionV2Candidate `json:"candidates"`
}

type providerInspectionV2ForeignProvider struct {
	ProviderID       string                          `json:"provider_id"`
	DiagnosticReason string                          `json:"diagnostic_reason"`
	Candidates       []providerInspectionV2Candidate `json:"candidates"`
}

type providerInspectionV2Candidate struct {
	HostID          string              `json:"host_id"`
	SurfaceID       string              `json:"surface_id"`
	DistributionKey string              `json:"distribution_key"`
	InstallationKey string              `json:"installation_key"`
	Location        string              `json:"location"`
	Version         string              `json:"version"`
	EvidenceDigest  string              `json:"evidence_digest"`
	ProviderPin     *config.ProviderPin `json:"provider_pin"`
}

func inspectionProviderByID(t *testing.T, values []providerInspectionV2Provider, providerID string) providerInspectionV2Provider {
	t.Helper()
	for _, value := range values {
		if value.ProviderID == providerID {
			return value
		}
	}
	t.Fatalf("Provider %s missing from %#v", providerID, values)
	return providerInspectionV2Provider{}
}

func inspectionForeignProviderByID(t *testing.T, values []providerInspectionV2ForeignProvider, providerID string) providerInspectionV2ForeignProvider {
	t.Helper()
	for _, value := range values {
		if value.ProviderID == providerID {
			return value
		}
	}
	t.Fatalf("foreign Provider %s missing from %#v", providerID, values)
	return providerInspectionV2ForeignProvider{}
}

func newProviderInspectionFixture(t *testing.T, existingConfig bool) providerInspectionFixture {
	t.Helper()
	fixture := newProviderInspectionHostsFixture(t, []string{"6.0.3", "6.1.1", `11c74d6b"quoted`}, false)
	if existingConfig {
		if err := os.WriteFile(fixture.configPath, []byte("schema_version = \"oaw.user-config/v3\"\n"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	return fixture
}

func newProviderInspectionHostsFixture(t *testing.T, codexVersions []string, includeClaude bool) providerInspectionFixture {
	t.Helper()
	userHome := t.TempDir()
	configBase := t.TempDir()
	t.Setenv("HOME", userHome)
	t.Setenv("XDG_CONFIG_HOME", configBase)
	configRoot := filepath.Join(configBase, "open-agent-workflow")
	if err := os.MkdirAll(configRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configRoot, "config.toml")
	for _, version := range codexVersions {
		indicator := filepath.Join(userHome, ".codex", "plugins", "cache", "openai-api-curated", "superpowers", version, "skills", "using-superpowers", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(indicator), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(indicator, []byte("indicator-secret"), 0o600); err != nil {
			t.Fatal(err)
		}
		binding := filepath.Join(filepath.Dir(filepath.Dir(indicator)), "writing-plans", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(binding), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(binding, []byte("---\nname: writing-plans\n---\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if includeClaude {
		indicator := filepath.Join(userHome, ".claude", "plugins", "superpowers", "skills", "using-superpowers", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(indicator), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(indicator, []byte("indicator-secret"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return providerInspectionFixture{configPath: configPath}
}

func pinDocumentsFromText(value string) []string {
	lines := strings.Split(value, "\n")
	documents := make([]string, 0)
	current := make([]string, 0)
	for _, line := range lines {
		if line == "schema_version = \"oaw.user-config/v3\"" || line == "[[provider_pins]]" || strings.HasPrefix(line, "  id = ") || strings.HasPrefix(line, "  location = ") || strings.HasPrefix(line, "  version = ") {
			if line == "[[provider_pins]]" && len(current) != 0 && strings.Contains(strings.Join(current, "\n"), "[[provider_pins]]") {
				documents = append(documents, strings.Join(current, "\n"))
				current = make([]string, 0)
			}
			current = append(current, line)
		}
	}
	if len(current) != 0 && strings.Contains(strings.Join(current, "\n"), "[[provider_pins]]") {
		documents = append(documents, strings.Join(current, "\n"))
	}
	return documents
}
