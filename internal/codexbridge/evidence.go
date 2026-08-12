package codexbridge

import (
	"fmt"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

type Facts struct {
	Session                host.SessionSnapshot
	Inventory              host.BindingInventory
	Environment            host.EnvironmentReport
	Configuration          config.Snapshot
	Discovery              discovery.Report
	Resolutions            registry.ResolutionReport
	Registry               registry.Registry
	VersionEvidence        VersionEvidence
	ReporterIdentityDigest string
	FactDigests            FactDigests
}

func cloneFacts(value Facts) Facts {
	value.Session = host.CloneSessionSnapshot(value.Session)
	value.Inventory = host.CloneBindingInventory(value.Inventory)
	value.Environment = host.CloneEnvironmentReport(value.Environment)
	value.VersionEvidence = cloneVersionEvidence(value.VersionEvidence)
	return value
}

func validateFacts(value Facts) error {
	if len(value.ReporterIdentityDigest) != 64 || strings.Trim(value.ReporterIdentityDigest, "0123456789abcdef") != "" {
		return NewError("HOST_EVIDENCE_HANDLE_INVALID", "Host reporter identity is not canonical", nil)
	}
	manifest, err := bridgeManifest(value.Session)
	if err != nil {
		return err
	}
	normalizedSession, err := host.NewSessionSnapshot(manifest, value.Session)
	if err != nil || normalizedSession.Digest != value.Session.Digest {
		return NewError("HOST_EVIDENCE_HANDLE_INVALID", "Host session facts are not canonical", err)
	}
	normalizedInventory, err := host.ValidateBindingInventory(value.Inventory)
	if err != nil || normalizedInventory.Digest != value.Inventory.Digest {
		return NewError("HOST_EVIDENCE_HANDLE_INVALID", "Host binding facts are not canonical", err)
	}
	normalizedEnvironment, err := host.NewEnvironmentReport(value.Environment)
	if err != nil || normalizedEnvironment.Digest != value.Environment.Digest {
		return NewError("HOST_EVIDENCE_HANDLE_INVALID", "Host environment facts are not canonical", err)
	}
	if value.Session.ProviderInventoryDigest != value.Inventory.Digest ||
		value.Session.FeatureDigest != value.FactDigests.Features ||
		value.Session.HostActionDigest != value.FactDigests.Actions ||
		value.Session.EnvironmentReportDigest != value.Environment.Digest ||
		value.Environment.SessionID != value.Session.SessionID ||
		value.Environment.Topology != execution.TopologyCurrent {
		return NewError("HOST_EVIDENCE_HANDLE_INVALID", "Host facts are not pinned to the same session", nil)
	}
	if _, err := Negotiate(value.VersionEvidence); err != nil {
		return NewError("HOST_EVIDENCE_HANDLE_INVALID", "VersionEvidence is not canonical", err)
	}
	if err := validateFactDigests(value); err != nil {
		return err
	}
	return nil
}

func bridgeManifest(session host.SessionSnapshot) (host.Manifest, error) {
	if session.HostID != "codex" || session.IntegrationID != BridgeIntegrationID || session.IntegrationVersion != BridgeIntegrationVersion {
		return host.Manifest{}, NewError("HOST_EVIDENCE_HANDLE_INVALID", "Host session does not belong to the Codex Bridge", nil)
	}
	manifest, err := CodexHostManifest()
	if err != nil {
		return host.Manifest{}, NewError("HOST_EVIDENCE_HANDLE_INVALID", "Codex Host Manifest is invalid", err)
	}
	return manifest, nil
}

func validateFactDigests(value Facts) error {
	values := []struct {
		name     string
		declared string
		actual   string
	}{
		{"session", value.FactDigests.Session, value.Session.Digest},
		{"reporter identity", value.FactDigests.Reporter, value.ReporterIdentityDigest},
		{"inventory", value.FactDigests.Inventory, value.Inventory.Digest},
		{"environment", value.FactDigests.Environment, value.Environment.Digest},
		{"features", value.FactDigests.Features, value.Session.FeatureDigest},
		{"actions", value.FactDigests.Actions, value.Session.HostActionDigest},
		{"configuration", value.FactDigests.Configuration, value.Configuration.Digest()},
		{"discovery", value.FactDigests.Discovery, value.Discovery.Digest()},
		{"resolution", value.FactDigests.Resolution, value.Resolutions.Digest()},
		{"registry", value.FactDigests.Registry, value.Registry.Digest()},
		{"version evidence", value.FactDigests.Version, value.VersionEvidence.Digest},
	}
	for _, item := range values {
		if item.declared != item.actual || item.actual != "" && (len(item.actual) != 64 || strings.Trim(item.actual, "0123456789abcdef") != "") {
			return NewError("HOST_EVIDENCE_HANDLE_INVALID", fmt.Sprintf("invalid %s fact digest", item.name), nil)
		}
	}
	return nil
}
