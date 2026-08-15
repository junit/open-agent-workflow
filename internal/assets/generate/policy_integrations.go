package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const policyIntegrationVersion = "2.0.0"

var policyHostIDs = []string{"claude", "cline", "codex", "copilot", "cursor", "gemini", "opencode", "roo", "windsurf"}

func generatePolicyIntegrations(root string) error {
	integrations := make([]host.IntegrationRecord, 0, len(policyHostIDs))
	for _, hostID := range policyHostIDs {
		integration, err := buildPolicyIntegration(hostID)
		if err != nil {
			return err
		}
		integrations = append(integrations, integration)
	}
	sort.Slice(integrations, func(left, right int) bool { return integrations[left].ID < integrations[right].ID })
	set := host.IntegrationSetRecord{SchemaVersion: host.HostIntegrationSetSchemaV3, Integrations: integrations}
	encoded, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Policy Integration set: %w", err)
	}
	path := filepath.Join(root, "internal", "assets", "host-integrations.json")
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		return fmt.Errorf("write Policy Integration set: %w", err)
	}
	return nil
}

func buildPolicyIntegration(hostID string) (host.IntegrationRecord, error) {
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: policyIntegrationVersion, HostID: hostID,
		ControlSurface: host.SurfacePolicy, Protocols: []string{}, BindingKinds: []catalog.BindingKind{},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Features: []host.Feature{},
		DelegationFeatures: []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		return host.IntegrationRecord{}, fmt.Errorf("build %s Policy Manifest: %w", hostID, err)
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{Status: host.AuditPending, References: []host.AuditEvidenceReference{}})
	if err != nil {
		return host.IntegrationRecord{}, fmt.Errorf("build %s Policy audit: %w", hostID, err)
	}
	return host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV3, IntegrationVersion: policyIntegrationVersion,
		ID: "oaw/" + hostID + "-policy", Manifest: manifest, ManifestDigest: manifest.Digest, Audit: audit,
	})
}
