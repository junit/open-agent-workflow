package host_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestHostV3PreservesControlFeaturesAndDeclaresLiveSurfaces(t *testing.T) {
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion:       host.HostManifestSchemaV3,
		ManifestVersion:     "3.0.0",
		HostID:              "codex",
		ControlSurface:      host.SurfaceHostNative,
		Protocols:           []string{host.WorkflowProtocolV1},
		BindingKinds:        allBindingKindsV3(),
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features:            allControlFeaturesV3(),
		DelegationFeatures:  allDelegationFeaturesV3(),
		HostActions:         allHostActionsV3(),
	})
	if err != nil {
		t.Fatalf("NewManifest() error = %v", err)
	}
	if manifest.SchemaVersion != host.HostManifestSchemaV3 || manifest.Digest == "" || manifest.ContentDigest() != manifest.Digest {
		t.Fatalf("Manifest = %#v", manifest)
	}
	if !slices.Equal(manifest.BindingKinds, allBindingKindsV3()) || !slices.Equal(manifest.Features, allControlFeaturesV3()) ||
		!slices.Equal(manifest.DelegationFeatures, allDelegationFeaturesV3()) || len(manifest.HostActions) != 3 {
		t.Fatalf("normalized Manifest = %#v", manifest)
	}

	hook := host.CloneManifest(manifest)
	hook.Digest = ""
	hook.BindingKinds = append(hook.BindingKinds, catalog.BindingKind("hook"))
	if _, err := host.NewManifest(hook); host.ErrorCode(err) != "HOST_MANIFEST_INVALID" {
		t.Fatalf("NewManifest(hook) error = %v", err)
	}

	policy, err := host.NewManifest(host.Manifest{
		SchemaVersion:       host.HostManifestSchemaV3,
		ManifestVersion:     "3.0.0",
		HostID:              "codex",
		ControlSurface:      host.SurfacePolicy,
		Protocols:           []string{},
		BindingKinds:        []catalog.BindingKind{},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features:            []host.Feature{},
		DelegationFeatures:  []host.FeatureID{},
		HostActions:         []host.HostActionContract{},
	})
	if err != nil || policy.Digest == "" {
		t.Fatalf("NewManifest(policy) = %#v, %v", policy, err)
	}
}

func TestFeatureObservationV3RequiresLiveEvidenceForAvailable(t *testing.T) {
	for _, feature := range allDelegationFeaturesV3() {
		observation, err := host.NewFeatureObservation(host.FeatureObservation{
			Feature: feature, State: host.AvailabilityAvailable, Source: host.SourceNativeAPI,
			EvidenceReference: "evidence://codex/delegation/" + string(feature),
		})
		if err != nil || observation.Digest == "" {
			t.Fatalf("NewFeatureObservation(%s) = %#v, %v", feature, observation, err)
		}
	}

	if _, err := host.NewFeatureObservation(host.FeatureObservation{
		Feature: host.FeatureChildDelegation, State: host.AvailabilityAvailable, Source: host.SourceStaticConfig,
		EvidenceReference: "evidence://codex/config/delegation",
	}); host.ErrorCode(err) != "HOST_FEATURE_OBSERVATION_INVALID" {
		t.Fatalf("static available error = %v", err)
	}
	if _, err := host.NewFeatureObservation(host.FeatureObservation{
		Feature: host.FeatureChildDelegation, State: host.AvailabilityConfigured, Source: host.SourceStaticConfig,
		EvidenceReference: "evidence://codex/config/delegation",
	}); err != nil {
		t.Fatalf("static configured error = %v", err)
	}
	if _, err := host.NewFeatureObservation(host.FeatureObservation{
		Feature: host.FeatureChildDelegation, State: host.AvailabilityAvailable, Source: host.SourceNativeAPI,
		EvidenceReference: "/private/host/config",
	}); host.ErrorCode(err) != "HOST_FEATURE_OBSERVATION_INVALID" {
		t.Fatalf("absolute evidence error = %v", err)
	}
}

func TestHostActionV3RejectsStaticAvailabilityAndContractDrift(t *testing.T) {
	for _, action := range allHostActionsV3() {
		observation, err := host.NewHostActionObservation(host.HostActionObservation{
			Action: action, State: host.AvailabilityAvailable, Source: host.SourceNativeAPI,
			EvidenceReference: "evidence://codex/actions/" + action.ID,
		})
		if err != nil || observation.Digest == "" {
			t.Fatalf("NewHostActionObservation(%s) = %#v, %v", action.ID, observation, err)
		}
	}

	static := host.HostActionObservation{
		Action: allHostActionsV3()[0], State: host.AvailabilityAvailable, Source: host.SourceStaticConfig,
		EvidenceReference: "evidence://codex/config/workspace",
	}
	if _, err := host.NewHostActionObservation(static); host.ErrorCode(err) != "HOST_ACTION_OBSERVATION_INVALID" {
		t.Fatalf("static available error = %v", err)
	}
	static.State = host.AvailabilityUnknown
	if _, err := host.NewHostActionObservation(static); err != nil {
		t.Fatalf("static unknown error = %v", err)
	}

	drifted := allHostActionsV3()[0]
	drifted.MaximumEffects[0] = "delete-project"
	if _, err := host.NewHostActionObservation(host.HostActionObservation{
		Action: drifted, State: host.AvailabilityUnavailable, Source: host.SourceNativeAPI,
		EvidenceReference: "evidence://codex/actions/workspace",
	}); host.ErrorCode(err) != "HOST_ACTION_OBSERVATION_INVALID" {
		t.Fatalf("contract drift error = %v", err)
	}
}

func TestHostV3SessionRejectsDuplicateAndUndeclaredObservations(t *testing.T) {
	manifest := mustHostManifestV3(t)
	feature, err := host.NewFeatureObservation(host.FeatureObservation{
		Feature: host.FeatureChildDelegation, State: host.AvailabilityAvailable, Source: host.SourceNativeAPI,
		EvidenceReference: "evidence://codex/delegation/child",
	})
	if err != nil {
		t.Fatal(err)
	}
	action, err := host.NewHostActionObservation(host.HostActionObservation{
		Action: allHostActionsV3()[0], State: host.AvailabilityAvailable, Source: host.SourceNativeAPI,
		EvidenceReference: "evidence://codex/actions/workspace",
	})
	if err != nil {
		t.Fatal(err)
	}
	base := host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: "oaw/codex-host",
		IntegrationVersion: "3.0.0", SessionID: "session-host-v3", ManifestDigest: manifest.Digest,
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, ProviderInventoryDigest: strings.Repeat("a", 64),
		FeatureObservations: []host.FeatureObservation{feature}, HostActionObservations: []host.HostActionObservation{action},
		EnvironmentReportDigest: strings.Repeat("b", 64),
	}
	first, err := host.NewSessionSnapshot(manifest, base)
	if err != nil || first.FeatureDigest == "" || first.HostActionDigest == "" || first.Digest == "" {
		t.Fatalf("NewSessionSnapshot() = %#v, %v", first, err)
	}

	changed := base
	changed.FeatureObservations = []host.FeatureObservation{feature}
	changed.FeatureObservations[0].Digest = ""
	changed.FeatureObservations[0].EvidenceReference = "evidence://codex/delegation/child-v2"
	second, err := host.NewSessionSnapshot(manifest, changed)
	if err != nil || second.FeatureDigest == first.FeatureDigest || second.Digest == first.Digest {
		t.Fatalf("feature digest did not bind Session: %#v / %#v / %v", first, second, err)
	}

	duplicate := base
	duplicate.FeatureObservations = []host.FeatureObservation{feature, feature}
	if _, err := host.NewSessionSnapshot(manifest, duplicate); host.ErrorCode(err) != "HOST_SESSION_INVALID" {
		t.Fatalf("duplicate feature error = %v", err)
	}
	duplicate = base
	duplicate.HostActionObservations = []host.HostActionObservation{action, action}
	if _, err := host.NewSessionSnapshot(manifest, duplicate); host.ErrorCode(err) != "HOST_SESSION_INVALID" {
		t.Fatalf("duplicate action error = %v", err)
	}

	undeclaredManifest := host.CloneManifest(manifest)
	undeclaredManifest.Digest = ""
	undeclaredManifest.DelegationFeatures = []host.FeatureID{}
	undeclaredManifest, err = host.NewManifest(undeclaredManifest)
	if err != nil {
		t.Fatal(err)
	}
	undeclared := base
	undeclared.ManifestDigest = undeclaredManifest.Digest
	if _, err := host.NewSessionSnapshot(undeclaredManifest, undeclared); host.ErrorCode(err) != "HOST_SESSION_INVALID" {
		t.Fatalf("undeclared feature error = %v", err)
	}
}

func TestHostV3DefensiveCopiesNestedActionAndObservationSlices(t *testing.T) {
	manifest := mustHostManifestV3(t)
	clone := host.CloneManifest(manifest)
	clone.HostActions[0].MaximumEffects[0] = "changed"
	clone.HostActions[0].Resources[0] = "changed"
	clone.DelegationFeatures[0] = host.FeatureID("changed")
	if manifest.HostActions[0].MaximumEffects[0] == "changed" || manifest.HostActions[0].Resources[0] == "changed" || manifest.DelegationFeatures[0] == "changed" {
		t.Fatal("CloneManifest() shares nested storage")
	}

	action, err := host.NewHostActionObservation(host.HostActionObservation{
		Action: allHostActionsV3()[0], State: host.AvailabilityUnknown, Source: host.SourceStaticConfig,
		EvidenceReference: "evidence://codex/config/workspace",
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: "oaw/codex-host",
		IntegrationVersion: "3.0.0", SessionID: "session-host-v3", ManifestDigest: manifest.Digest,
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, ProviderInventoryDigest: strings.Repeat("a", 64),
		FeatureObservations: []host.FeatureObservation{}, HostActionObservations: []host.HostActionObservation{action},
		EnvironmentReportDigest: strings.Repeat("b", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	sessionClone := host.CloneSessionSnapshot(session)
	sessionClone.HostActionObservations[0].Action.MaximumEffects[0] = "changed"
	if session.HostActionObservations[0].Action.MaximumEffects[0] == "changed" {
		t.Fatal("CloneSessionSnapshot() shares nested action storage")
	}
}

func TestConformanceV3KeepsEnvironmentAndReceiptV2Bridge(t *testing.T) {
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: "codex", ControlSurface: host.SurfaceHostNative,
		Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []catalog.BindingKind{catalog.BindingSkill},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features:            []host.Feature{host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
		DelegationFeatures:  []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := host.NewBindingObservation(host.BindingObservation{
		HostID: "codex", ProviderID: "oaw/provider", InstallationKey: "installation-provider", DistributionID: "distribution",
		BindingID: "binding-skill", Surface: "codex", Kind: catalog.BindingSkill, Reference: "provider:skill",
		Invocation: catalog.InvocationModel, BindingTreeDigest: "sha256:" + strings.Repeat("a", 64),
		Topologies: []execution.Topology{execution.TopologyCurrent}, Source: host.SourceNativeAPI,
		EvidenceReference: "evidence://codex/bindings/skill",
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := host.BuildBindingInventoryV3("codex", []host.BindingObservation{binding})
	if err != nil {
		t.Fatal(err)
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-host-v3", Topology: execution.TopologyCurrent,
		Observations: []execution.EnvironmentObservation{},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: "oaw/codex-host", IntegrationVersion: "3.0.0",
		SessionID: "session-host-v3", ManifestDigest: manifest.Digest, SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: inventory.Digest, FeatureObservations: []host.FeatureObservation{},
		HostActionObservations: []host.HostActionObservation{}, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptCompleted, WorkflowID: "workflow-host-v3",
		BundleGeneration: 1, BundleDigest: strings.Repeat("b", 64), NodeID: "verification", Topology: execution.TopologyCurrent,
		HostSessionDigest: session.Digest, DispatchDigest: strings.Repeat("c", 64), ContextFreshness: host.ContextShared,
		EnvironmentReportDigest: environment.Digest, Outcome: "succeeded",
		Evidence: []host.EvidenceReference{{Kind: "report", Reference: "evidence://codex/conformance/success", Digest: strings.Repeat("d", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	transcript, err := host.NewConformanceTranscript(host.ConformanceTranscript{
		SchemaVersion: host.HostConformanceTranscriptSchemaV3, Session: session, Inventory: inventory,
		EnvironmentReports: []host.EnvironmentReport{environment}, Receipts: []host.InvocationReceipt{receipt}, Invocations: []host.InvocationRecord{},
	})
	if err != nil {
		t.Fatalf("NewConformanceTranscript() error = %v", err)
	}
	report, err := host.ValidateConformanceTranscript(manifest, transcript)
	if err != nil || report.SchemaVersion != host.HostConformanceReportSchemaV3 || len(report.Diagnostics) != 0 ||
		!slices.Equal(report.VerifiedFeatures, manifest.Features) || len(report.VerifiedDelegationFeatures) != 0 || len(report.VerifiedHostActionIDs) != 0 {
		t.Fatalf("ValidateConformanceTranscript() = %#v, %v", report, err)
	}
}

func TestIntegrationV3DefensiveCopiesOptionalConformance(t *testing.T) {
	manifest := mustHostManifestV3(t)
	audit, err := host.NewAuditEvidence(host.AuditEvidence{
		Status:     host.AuditPassed,
		References: []host.AuditEvidenceReference{{Reference: "evidence://codex/audit/host-v3", Digest: strings.Repeat("a", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := host.NewConformanceReport(host.ConformanceReport{
		SchemaVersion: host.HostConformanceReportSchemaV3, ManifestDigest: manifest.Digest, TranscriptDigest: strings.Repeat("b", 64),
		VerifiedFeatures: allControlFeaturesV3(), VerifiedDelegationFeatures: allDelegationFeaturesV3(),
		VerifiedHostActionIDs: []string{"closeout.execute", "verification.execute", "workspace.prepare-or-confirm"}, Diagnostics: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	integration, err := host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV3, IntegrationVersion: "3.0.0", ID: "oaw/codex-host",
		Manifest: manifest, ManifestDigest: manifest.Digest, Audit: audit, Conformance: &report,
	})
	if err != nil {
		t.Fatalf("NewIntegration() error = %v", err)
	}
	clone := host.CloneIntegration(integration)
	clone.Manifest.HostActions[0].MaximumEffects[0] = "changed"
	clone.Conformance.VerifiedDelegationFeatures[0] = "changed"
	clone.Conformance.VerifiedHostActionIDs[0] = "changed"
	if integration.Manifest.HostActions[0].MaximumEffects[0] == "changed" || integration.Conformance.VerifiedDelegationFeatures[0] == "changed" || integration.Conformance.VerifiedHostActionIDs[0] == "changed" {
		t.Fatal("CloneIntegration() shares nested Conformance or Manifest storage")
	}
}

func allBindingKindsV3() []catalog.BindingKind {
	return []catalog.BindingKind{
		catalog.BindingAgent,
		catalog.BindingInstruction,
		catalog.BindingRole,
		catalog.BindingSkill,
		catalog.BindingTool,
	}
}

func allControlFeaturesV3() []host.Feature {
	return []host.Feature{
		host.FeatureCancellation,
		host.FeatureEnvironmentReporting,
		host.FeatureInvocationDedup,
		host.FeatureNormalizedReceipts,
		host.FeaturePause,
		host.FeatureProviderBindingInventory,
	}
}

func allDelegationFeaturesV3() []host.FeatureID {
	return []host.FeatureID{
		host.FeatureChildDelegation,
		host.FeatureNestedChildDelegation,
		host.FeatureNestedParallelDelegation,
		host.FeatureParallelChildDelegation,
	}
}

func allHostActionsV3() []host.HostActionContract {
	return []host.HostActionContract{
		{
			ID: "closeout.execute", InputSchema: "oaw.host-action.closeout-input/v1", OutcomeSchema: "oaw.host-action.closeout-outcome/v1",
			MaximumEffects: []string{"git-local", "network-mutation", "read-project", "run-process"}, Resources: []string{"git-repository", "network", "project-worktree"},
		},
		{
			ID: "verification.execute", InputSchema: "oaw.host-action.verification-input/v1", OutcomeSchema: "oaw.host-action.verification-outcome/v1",
			MaximumEffects: []string{"read-project", "run-process"}, Resources: []string{"project"},
		},
		{
			ID: "workspace.prepare-or-confirm", InputSchema: "oaw.host-action.workspace-input/v1", OutcomeSchema: "oaw.host-action.workspace-outcome/v1",
			MaximumEffects: []string{"git-local", "read-project", "run-process", "write-project"}, Resources: []string{"git-repository", "project-worktree"},
		},
	}
}

func mustHostManifestV3(t *testing.T) host.Manifest {
	t.Helper()
	value, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: "codex", ControlSurface: host.SurfaceHostNative,
		Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: allBindingKindsV3(), SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features: allControlFeaturesV3(), DelegationFeatures: allDelegationFeaturesV3(), HostActions: allHostActionsV3(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}
