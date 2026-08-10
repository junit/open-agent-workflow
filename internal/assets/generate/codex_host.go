package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const (
	codexHostIntegrationID      = "oaw/codex-host"
	codexHostIntegrationVersion = "1.0.0"
	policyIntegrationVersion    = "2.0.0"
)

var policyHostIDs = []string{"claude", "cline", "codex", "copilot", "cursor", "gemini", "opencode", "roo", "windsurf"}

func generateCodexHost(root string) error {
	transcriptPath := filepath.Join(root, "internal", "assets", "conformance", "codex-host-v3.json")
	integrationsPath := filepath.Join(root, "internal", "assets", "host-integrations.json")

	transcript, native, err := buildCodexHostFixture()
	if err != nil {
		return err
	}
	transcriptRaw, err := canonicaljson.Marshal(transcript)
	if err != nil {
		return fmt.Errorf("encode Codex Host transcript: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(transcriptPath), 0o755); err != nil {
		return fmt.Errorf("create Codex Host transcript directory: %w", err)
	}
	if err := os.WriteFile(transcriptPath, append(transcriptRaw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write Codex Host transcript: %w", err)
	}

	integrations := make([]host.IntegrationRecord, 0, len(policyHostIDs)+1)
	for _, hostID := range policyHostIDs {
		policy, policyErr := buildPolicyIntegration(hostID)
		if policyErr != nil {
			return policyErr
		}
		integrations = append(integrations, policy)
	}
	integrations = append(integrations, native)
	sort.Slice(integrations, func(left, right int) bool { return integrations[left].ID < integrations[right].ID })
	set := host.IntegrationSetRecord{SchemaVersion: host.HostIntegrationSetSchemaV3, Integrations: integrations}
	encoded, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Host Integration set: %w", err)
	}
	if err := os.WriteFile(integrationsPath, append(encoded, '\n'), 0o644); err != nil {
		return fmt.Errorf("write Host Integration set: %w", err)
	}
	return nil
}

func buildCodexHostFixture() (host.ConformanceTranscript, host.IntegrationRecord, error) {
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: codexHostIntegrationVersion, HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []catalog.BindingKind{catalog.BindingSkill},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features:            []host.Feature{host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
		DelegationFeatures:  []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		return host.ConformanceTranscript{}, host.IntegrationRecord{}, fmt.Errorf("build Codex Host Manifest: %w", err)
	}
	binding, err := host.NewBindingObservation(host.BindingObservation{
		HostID: "codex", ProviderID: "oaw/conformance-fixture", InstallationKey: "installation-codex-conformance",
		DistributionID: "fixture", BindingID: "skill", Surface: "codex", Kind: catalog.BindingSkill, Reference: "fixture:skill",
		Invocation: catalog.InvocationModel, BindingTreeDigest: "sha256:" + strings.Repeat("a", 64),
		Topologies: []execution.Topology{execution.TopologyCurrent}, Source: host.SourceNativeAPI,
		EvidenceReference: "evidence://codex-host/conformance/skill-binding",
	})
	if err != nil {
		return host.ConformanceTranscript{}, host.IntegrationRecord{}, fmt.Errorf("build Codex Binding observation: %w", err)
	}
	inventory, err := host.BuildBindingInventoryV3("codex", []host.BindingObservation{binding})
	if err != nil {
		return host.ConformanceTranscript{}, host.IntegrationRecord{}, fmt.Errorf("build Codex Binding Inventory: %w", err)
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-codex-conformance", Topology: execution.TopologyCurrent,
		Observations: []execution.EnvironmentObservation{},
	})
	if err != nil {
		return host.ConformanceTranscript{}, host.IntegrationRecord{}, fmt.Errorf("build Codex Environment Report: %w", err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: codexHostIntegrationID,
		IntegrationVersion: codexHostIntegrationVersion, SessionID: "session-codex-conformance", ManifestDigest: manifest.Digest,
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, ProviderInventoryDigest: inventory.Digest,
		FeatureObservations: []host.FeatureObservation{}, HostActionObservations: []host.HostActionObservation{}, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		return host.ConformanceTranscript{}, host.IntegrationRecord{}, fmt.Errorf("build Codex Host Session: %w", err)
	}
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptCompleted,
		WorkflowID: "workflow-codex-host-conformance", BundleGeneration: 1, BundleDigest: strings.Repeat("b", 64), NodeID: "verification",
		Topology: execution.TopologyCurrent, HostSessionDigest: session.Digest, DispatchDigest: strings.Repeat("c", 64),
		ContextFreshness: host.ContextShared, EnvironmentReportDigest: environment.Digest, Outcome: "succeeded",
		Evidence: []host.EvidenceReference{{Kind: "report", Reference: "evidence://codex-host/conformance/current-completion", Digest: strings.Repeat("d", 64)}},
	})
	if err != nil {
		return host.ConformanceTranscript{}, host.IntegrationRecord{}, fmt.Errorf("build Codex Invocation Receipt: %w", err)
	}
	transcript, err := host.NewConformanceTranscript(host.ConformanceTranscript{
		SchemaVersion: host.HostConformanceTranscriptSchemaV3, Session: session, Inventory: inventory,
		EnvironmentReports: []host.EnvironmentReport{environment}, Receipts: []host.InvocationReceipt{receipt}, Invocations: []host.InvocationRecord{},
	})
	if err != nil {
		return host.ConformanceTranscript{}, host.IntegrationRecord{}, fmt.Errorf("build Codex Conformance Transcript: %w", err)
	}
	report, err := host.ValidateConformanceTranscript(manifest, transcript)
	if err != nil {
		return host.ConformanceTranscript{}, host.IntegrationRecord{}, fmt.Errorf("validate Codex Host conformance: %w", err)
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{
		Status: host.AuditPassed,
		References: []host.AuditEvidenceReference{
			{Reference: "evidence://codex-host/checks/host-v3-security", Digest: canonicaljson.DigestBytes([]byte("host-v3-security-boundary/v1"))},
			{Reference: "evidence://codex-host/conformance/codex-host-v3", Digest: transcript.Digest},
		},
	})
	if err != nil {
		return host.ConformanceTranscript{}, host.IntegrationRecord{}, fmt.Errorf("build Codex Host audit: %w", err)
	}
	integration, err := host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV3, IntegrationVersion: codexHostIntegrationVersion, ID: codexHostIntegrationID,
		Manifest: manifest, ManifestDigest: manifest.Digest, Audit: audit, Conformance: &report,
	})
	if err != nil {
		return host.ConformanceTranscript{}, host.IntegrationRecord{}, fmt.Errorf("build Codex Host Integration: %w", err)
	}
	return transcript, integration, nil
}

func buildPolicyIntegration(hostID string) (host.IntegrationRecord, error) {
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: policyIntegrationVersion, HostID: hostID, ControlSurface: host.SurfacePolicy,
		Protocols: []string{}, BindingKinds: []catalog.BindingKind{}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features: []host.Feature{}, DelegationFeatures: []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		return host.IntegrationRecord{}, fmt.Errorf("build %s policy Manifest: %w", hostID, err)
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{Status: host.AuditPending, References: []host.AuditEvidenceReference{}})
	if err != nil {
		return host.IntegrationRecord{}, fmt.Errorf("build %s policy audit: %w", hostID, err)
	}
	return host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV3, IntegrationVersion: policyIntegrationVersion, ID: "oaw/" + hostID + "-policy",
		Manifest: manifest, ManifestDigest: manifest.Digest, Audit: audit,
	})
}
