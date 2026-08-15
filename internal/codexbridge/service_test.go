package codexbridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assurance"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

type bridgeTestObserver struct {
	metadata appserver.MetadataObservation
}

func TestNewServiceLoadsBuiltInProviderIdentityByDefault(t *testing.T) {
	service, err := NewService(ServiceOptions{
		Observer: &bridgeTestObserver{}, UserHome: t.TempDir(), ProfileConfigHome: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if providers := service.catalog.Providers(); len(providers) != 3 {
		t.Fatalf("built-in Providers = %d, want 3", len(providers))
	}
}

func TestServicePreservesAssuranceAndAppServerErrorCodes(t *testing.T) {
	assuranceErr := assuranceBridgeError("inspect", &assurance.Error{Code: "PROFILE_NOT_ASSURABLE"})
	if Code(assuranceErr) != "PROFILE_NOT_ASSURABLE" {
		t.Fatalf("assurance error = %v", assuranceErr)
	}
	appServerErr := bridgeErrorFromAppServer(appserver.NewError("HOST_BRIDGE_UNAVAILABLE", "offline", nil))
	if Code(appServerErr) != "HOST_BRIDGE_UNAVAILABLE" {
		t.Fatalf("App Server error = %v", appServerErr)
	}
	if got := bridgeErrorFromAppServer(errors.New("transport")); Code(got) != "HOST_OBSERVATION_FAILED" {
		t.Fatalf("generic App Server error = %v", got)
	}
	if bridgeErrorFromAppServer(nil) != nil {
		t.Fatal("nil App Server error did not remain nil")
	}
}

func (observer *bridgeTestObserver) Observe(_ context.Context, cwd string) (appserver.MetadataObservation, error) {
	value := observer.metadata
	value.Skills.CWD = cwd
	value.Skills.Skills = append([]appserver.SkillMetadata(nil), observer.metadata.Skills.Skills...)
	return value, nil
}

func TestObserveProfileIssuesOverlayFromExactCurrentBinding(t *testing.T) {
	service, profilePath := newBridgeTestService(t, true)
	before, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	output, err := service.ObserveProfile(
		context.Background(),
		ObserveProfileInput{Profile: "project:team-delivery"},
		bridgeTestContext(filepath.Dir(filepath.Dir(filepath.Dir(profilePath)))),
	)
	if err != nil {
		t.Fatal(err)
	}
	if output.Overlay.Issuer != BridgeIntegrationID || len(output.Overlay.Claims) != 1 {
		t.Fatalf("Overlay = %#v", output.Overlay)
	}
	claim := output.Overlay.Claims[0]
	if claim.ProviderID != "acme/suite" || claim.BindingReference != "acme:delivery" ||
		claim.BindingKind != "skill" || claim.HostID != "codex" || len(claim.Evidence) != 2 {
		t.Fatalf("Binding claim = %#v", claim)
	}
	profile, err := service.resolveProfile(filepath.Dir(filepath.Dir(filepath.Dir(profilePath))), "project:team-delivery")
	if err != nil {
		t.Fatal(err)
	}
	if err := assurance.Verify(profile, output.Overlay); err != nil {
		t.Fatalf("Assurance Verify() = %v", err)
	}
	after, err := os.ReadFile(profilePath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("Bridge modified the Policy Profile: %v", err)
	}
}

func TestMissingBridgeBindingRemovesOnlyMachineClaim(t *testing.T) {
	service, profilePath := newBridgeTestService(t, false)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(profilePath)))
	before, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ObserveProfile(
		context.Background(), ObserveProfileInput{Profile: "project:team-delivery"}, bridgeTestContext(projectRoot),
	)
	if Code(err) != "ASSURANCE_BINDING_UNAVAILABLE" {
		t.Fatalf("ObserveProfile() error = %v", err)
	}
	profile, resolveErr := service.resolveProfile(projectRoot, "project:team-delivery")
	if resolveErr != nil || profile.Metadata.ID != "team-delivery" {
		t.Fatalf("Policy Profile became unavailable: profile=%#v error=%v", profile, resolveErr)
	}
	after, readErr := os.ReadFile(profilePath)
	if readErr != nil || !bytes.Equal(before, after) {
		t.Fatalf("failed Bridge changed the Profile: %v", readErr)
	}
}

func TestObserveProfileRequiresSourceQualifiedSelector(t *testing.T) {
	service, profilePath := newBridgeTestService(t, true)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(profilePath)))
	_, err := service.ObserveProfile(
		context.Background(), ObserveProfileInput{Profile: "team-delivery"}, bridgeTestContext(projectRoot),
	)
	if Code(err) != "PROFILE_SELECTION_INVALID" {
		t.Fatalf("ObserveProfile() error = %v", err)
	}
}

func TestObserveProfilePreservesStableProfileResolutionReasons(t *testing.T) {
	service, profilePath := newBridgeTestService(t, true)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(profilePath)))
	_, err := service.ObserveProfile(
		context.Background(), ObserveProfileInput{Profile: "project:missing"}, bridgeTestContext(projectRoot),
	)
	if Code(err) != "PROFILE_NOT_FOUND" {
		t.Fatalf("missing Profile error = %v", err)
	}

	duplicatePath := filepath.Join(filepath.Dir(profilePath), "duplicate.md")
	writeBridgeTestFile(t, duplicatePath, "---\nid: team-delivery\nname: Duplicate Team Delivery\n---\n")
	_, err = service.ObserveProfile(
		context.Background(), ObserveProfileInput{Profile: "project:team-delivery"}, bridgeTestContext(projectRoot),
	)
	if Code(err) != "PROFILE_AMBIGUOUS" {
		t.Fatalf("ambiguous Profile error = %v", err)
	}
}

func TestObserveProfileInputIsClosed(t *testing.T) {
	for _, raw := range []string{
		`{"profile":"project:team-delivery","workflow":{"mode":"WORKFLOW"}}`,
		`{"profile":"project:team-delivery"}{}`,
	} {
		if _, err := DecodeObserveProfileInput([]byte(raw)); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
			t.Fatalf("DecodeObserveProfileInput(%s) error = %v", raw, err)
		}
	}
}

func newBridgeTestService(t *testing.T, exposeSkill bool) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	projectRoot := filepath.Join(root, "project")
	configHome := filepath.Join(root, "config")
	userHome := filepath.Join(root, "home")
	profilePath := filepath.Join(projectRoot, ".oaw", "profiles", "team-delivery.md")
	writeBridgeTestFile(t, profilePath, "---\nid: team-delivery\nname: Team Delivery\n---\n\n## Responsibilities\n\n| Responsibility | Skill or action |\n| --- | --- |\n| Implementation and TDD | `acme:delivery` |\n\n## Rules\n\nThe Markdown Profile remains normative.\n")

	providerRoot := filepath.Join(userHome, ".codex", "plugins", "acme")
	skillRoot := filepath.Join(providerRoot, "skills", "delivery")
	skillPath := filepath.Join(skillRoot, "SKILL.md")
	writeBridgeTestFile(t, skillPath, "---\nname: acme:delivery\n---\n")
	bindingTree, err := integrity.DigestTree(skillRoot)
	if err != nil {
		t.Fatal(err)
	}
	distributionTree, err := integrity.DigestTree(providerRoot)
	if err != nil {
		t.Fatal(err)
	}

	descriptor := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV5, DescriptorVersion: "5.0.0",
		ID: "acme/suite", DisplayName: "Acme Suite",
		Distributions: []catalog.DistributionRecord{{
			ID: "acme", SourceURI: "https://example.test/acme/suite",
			Revision: strings.Repeat("a", 40), TreeDigest: distributionTree.RootDigest,
		}},
		Discovery: []catalog.DiscoveryProbe{{
			ID: "codex", Hosts: []string{"codex"}, Surface: "codex-plugin", DistributionID: "acme",
			Kind: "path-exists", Root: "user-home", CandidatePath: ".codex/plugins/acme", EvidencePath: "skills/delivery/SKILL.md",
		}},
		Bindings: []catalog.BindingRecord{{
			ID: "codex-delivery", DistributionID: "acme", ContentRoot: "skills/delivery", InstallRoot: "skills/delivery",
			TreeDigest: bindingTree.RootDigest, Host: "codex", Surface: "codex-plugin", Kind: catalog.BindingSkill,
			Reference: "acme:delivery", Invocation: catalog.InvocationModel,
		}},
	}
	value, err := catalog.New([]catalog.ProviderDescriptorRecord{descriptor})
	if err != nil {
		t.Fatal(err)
	}

	metadata := appserver.MetadataObservation{
		Skills:       appserver.SkillsEntry{Errors: []appserver.MetadataError{}, Skills: []appserver.SkillMetadata{}},
		CodexVersion: "codex-cli/0.146.1",
	}
	if exposeSkill {
		metadata.Skills.Skills = []appserver.SkillMetadata{{Name: "acme:delivery", Enabled: true, Path: skillPath, Scope: "user"}}
	}
	service, err := NewService(ServiceOptions{
		Observer: &bridgeTestObserver{metadata: metadata}, Catalog: &value,
		ProfileConfigHome: configHome, UserHome: userHome,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, profilePath
}

func bridgeTestContext(projectRoot string) HookContext {
	return HookContext{
		SchemaVersion: HookContextSchemaV3, BridgeProtocolVersion: BridgeProtocolVersion,
		SessionID: "session-test", TurnID: "turn-test", ToolUseID: "tool-test", CWD: projectRoot,
		Model: "gpt-test", PermissionMode: "workspace-write",
	}
}

func writeBridgeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
