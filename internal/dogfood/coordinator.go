package dogfood

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func readOnlyAuthority() admission.AuthorityCeiling {
	return admission.AuthorityCeiling{
		Effects: []string{"read-project"}, Resources: []string{"project"},
		ResourceLeases: false, AllowDelegation: false,
	}
}

func openPilotEngine(pilot pilotRecord) (*coordinator.Engine, error) {
	if _, err := verifyRepository(pilot.Repository); err != nil {
		return nil, err
	}
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: pilot.ConfigRoot})
	if err != nil {
		return nil, fmt.Errorf("load pilot configuration: %w", err)
	}
	if snapshot.Digest() != pilot.ConfigurationDigest {
		return nil, errors.New("pilot configuration digest changed")
	}
	resolved, inventory, err := resolveProvider(snapshot, pilot.Repository)
	if err != nil {
		return nil, err
	}
	if inventory.Digest != pilot.InventoryDigest || resolved.Report.Digest() != pilot.ResolutionDigest || resolved.Registry.Digest() != pilot.RegistryDigest {
		return nil, errors.New("pilot Provider resolution digest changed")
	}
	integration, found := snapshot.HostIntegration(integrationID)
	if !found {
		return nil, errors.New("pilot Host integration is missing")
	}
	var session host.SessionSnapshot
	if _, err := readCanonical(filepath.Join(pilot.EvidenceRoot, "session.json"), &session); err != nil {
		return nil, fmt.Errorf("pilot session: %w", err)
	}
	normalizedSession, err := host.NewSessionSnapshot(integration.Manifest, session)
	if err != nil {
		return nil, fmt.Errorf("pilot session is not canonical: %w", err)
	}
	if !reflect.DeepEqual(normalizedSession, session) {
		return nil, errors.New("pilot session is not canonical")
	}
	if session.Digest != pilot.HostSessionDigest || session.ProviderInventoryDigest != inventory.Digest {
		return nil, errors.New("pilot Host session digest changed")
	}
	var environment host.EnvironmentReport
	if _, err := readCanonical(filepath.Join(pilot.EvidenceRoot, "environment.json"), &environment); err != nil {
		return nil, fmt.Errorf("pilot environment: %w", err)
	}
	normalizedEnvironment, err := host.NewEnvironmentReport(environment)
	if err != nil {
		return nil, fmt.Errorf("pilot environment is not canonical: %w", err)
	}
	if !reflect.DeepEqual(normalizedEnvironment, environment) {
		return nil, errors.New("pilot environment is not canonical")
	}
	if environment.Digest != pilot.EnvironmentReportDigest || host.ValidateEnvironmentReport(session, environment) != nil {
		return nil, errors.New("pilot environment digest changed")
	}
	var storedInventory host.BindingInventory
	if _, err := readCanonical(filepath.Join(pilot.EvidenceRoot, "inventory.json"), &storedInventory); err != nil {
		return nil, fmt.Errorf("pilot inventory: %w", err)
	}
	normalizedInventory, err := host.NewBindingInventory(storedInventory.HostID, storedInventory.Observations)
	if err != nil {
		return nil, fmt.Errorf("pilot inventory is not pinned: %w", err)
	}
	if !reflect.DeepEqual(normalizedInventory, storedInventory) || !reflect.DeepEqual(normalizedInventory, inventory) {
		return nil, errors.New("pilot inventory is not pinned")
	}
	engine, err := coordinator.NewEngine(coordinator.Options{
		StateRoot: pilot.StateRoot, PhysicalProjectRoot: pilot.Repository.Root,
		Configuration: snapshot, Resolutions: resolved.Report, Registry: resolved.Registry,
		Authority: readOnlyAuthority(),
	})
	if err != nil {
		return nil, err
	}
	return engine, nil
}

func inspectWorkflow(engine *coordinator.Engine, workflowID string) (coordinator.Result, error) {
	if engine == nil || !validText(workflowID, 512) {
		return coordinator.Result{}, errors.New("Workflow inspection requires a valid Engine and Workflow")
	}
	return engine.Exchange(coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandInspect, WorkflowID: workflowID,
	})
}

func persistAndPrintDispatch(pilot pilotRecord, packet coordinator.DispatchPacket) error {
	if packet.WorkflowID != pilot.WorkflowID || packet.NodeID == "" || !validDigest(packet.Digest) || packet.Topology != execution.TopologyCurrent || packet.HostSessionDigest != pilot.HostSessionDigest || packet.BundleDigest != pilot.BundleDigest {
		return errors.New("Dispatch Packet is not pinned to the pilot")
	}
	path := filepath.Join(pilot.EvidenceRoot, "dispatch-"+packet.NodeID+".json")
	if err := writeCanonical(path, packet); err != nil {
		return err
	}
	return printCanonical(packet)
}

func validateReceiptPins(packet coordinator.DispatchPacket, receipt host.InvocationReceipt) error {
	if receipt.WorkflowID != packet.WorkflowID || receipt.BundleGeneration != packet.BundleGeneration || receipt.BundleDigest != packet.BundleDigest || receipt.NodeID != packet.NodeID || receipt.Topology != packet.Topology || receipt.HostSessionDigest != packet.HostSessionDigest || receipt.DispatchDigest != packet.Digest || receipt.EnvironmentReportDigest != packet.EnvironmentReportDigest {
		return errors.New("Receipt identity does not match Dispatch Packet")
	}
	return nil
}

func validateEvidence(root, node string, evidence []host.EvidenceReference) error {
	if len(evidence) != 1 || evidence[0].Kind != "report" {
		return errors.New("completed Receipt must contain exactly one report evidence reference")
	}
	expectedPath := filepath.Join(root, "evidence", node+".md")
	expected, err := containedRegularFile(expectedPath, root)
	if err != nil {
		return fmt.Errorf("evidence report: %w", err)
	}
	if evidence[0].Reference != "file://"+expected || !validDigest(evidence[0].Digest) {
		return errors.New("Receipt evidence is not pinned to the local report")
	}
	report, err := readLimited(expected)
	if err != nil {
		return err
	}
	if canonicaljson.DigestBytes(report) != evidence[0].Digest {
		return errors.New("evidence report digest changed")
	}
	return nil
}
