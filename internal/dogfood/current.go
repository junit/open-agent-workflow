package dogfood

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

// Run coordinates a read-only CURRENT pilot. It never invokes a Provider
// binding or an Agent process; the active Host supplies completed Receipts.
func Run(args []string) error {
	if len(args) == 0 {
		return errors.New("missing helper command")
	}
	switch args[0] {
	case "start":
		if len(args) != 5 {
			return errors.New("invalid start arguments")
		}
		return startPilot(args[1], args[2], args[3], args[4])
	case "prepare":
		if len(args) != 2 {
			return errors.New("invalid prepare arguments")
		}
		return preparePilot(args[1])
	case "inspect":
		if len(args) != 2 {
			return errors.New("invalid inspect arguments")
		}
		return inspectPilot(args[1])
	case "receipt":
		if len(args) != 3 {
			return errors.New("invalid receipt arguments")
		}
		return receivePilot(args[1], args[2])
	default:
		return fmt.Errorf("unknown helper command %q", args[0])
	}
}

func startPilot(repositoryPath, evidencePath, approvedPath, sessionID string) (resultErr error) {
	repository, err := canonicalExistingDirectory(repositoryPath)
	if err != nil {
		return fmt.Errorf("repository: %w", err)
	}
	approved, err := canonicalExistingDirectory(approvedPath)
	if err != nil {
		return fmt.Errorf("approved repository: %w", err)
	}
	if repository != approved {
		return errors.New("approved repository does not match requested repository")
	}
	if !validText(sessionID, 512) {
		return errors.New("OAW_HOST_SESSION_ID is invalid")
	}
	if _, err := os.Lstat(filepath.Join(repository, ".oaw-production")); err == nil {
		return errors.New("production-marked repositories are not eligible for dogfood")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect production marker: %w", err)
	}
	fingerprint, err := inspectRepository(repository)
	if err != nil {
		return err
	}
	evidence, err := createEvidenceRoot(evidencePath)
	if err != nil {
		return err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = os.RemoveAll(evidence)
		}
	}()

	configRoot := filepath.Join(evidence, "runtime", "config")
	stateRoot := filepath.Join(evidence, "runtime", "state", "workflows")
	if err := os.MkdirAll(configRoot, 0o700); err != nil {
		return err
	}
	snapshot, integration, err := writePilotConfiguration(configRoot, fingerprint, sessionID)
	if err != nil {
		return err
	}
	resolved, inventory, err := resolveProvider(snapshot, fingerprint)
	if err != nil {
		return err
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2,
		SessionID:     sessionID,
		Topology:      execution.TopologyCurrent,
		Observations: []execution.EnvironmentObservation{{
			Surface: "skills", Disposition: execution.DispositionInherited,
			Source: "codex-current-session", Digest: fingerprint.SkillDigest,
		}},
	})
	if err != nil {
		return err
	}
	session, err := host.NewSessionSnapshot(integration.Manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV2, HostID: hostID,
		IntegrationID: integration.ID, IntegrationVersion: integration.IntegrationVersion,
		SessionID: sessionID, SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: inventory.Digest, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		return err
	}
	engine, err := coordinator.NewEngine(coordinator.Options{
		StateRoot: stateRoot, PhysicalProjectRoot: repository,
		Configuration: snapshot, Resolutions: resolved.Report, Registry: resolved.Registry,
		Authority: readOnlyAuthority(),
	})
	if err != nil {
		return err
	}
	if _, err := verifyRepository(fingerprint); err != nil {
		return err
	}
	command := coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandStart,
		MessageID: "dogfood-start", IdempotencyKey: "dogfood-current-start",
		Start: &coordinator.StartInput{
			RequestID: "request-ocr-readonly", DeliverableID: "deliverable-ocr-readonly",
			InputDigest: canonicaljson.DigestBytes([]byte(fingerprint.Commit + "\n" + fingerprint.SkillDigest)),
			Proposal:    dogfoodWorkflowProposal(fingerprint),
			Selection: core.Selection{
				Profile: profileID, ProfileSource: core.SelectionUser,
				Topology: execution.TopologyCurrent, TopologySource: core.SelectionHostOnlyOption,
				AddOns: []string{}, Bindings: []profile.ProfileBinding{},
			},
			HostSession: session, Environment: environment,
		},
	}
	result, err := engine.Exchange(command)
	if err != nil {
		return err
	}
	if result.Kind != coordinator.ResultState || result.Snapshot == nil || result.Snapshot.Status != coordinator.StatusReady || result.Snapshot.ActiveNodeID != "review-scope" {
		return fmt.Errorf("unexpected START Result: %#v", result)
	}
	bundle := result.Snapshot.Bundles[0]
	pilot := pilotRecord{
		SchemaVersion: pilotSchema, EvidenceRoot: evidence, ConfigRoot: configRoot, StateRoot: stateRoot,
		WorkflowID: result.WorkflowID, Profile: profileID, Repository: fingerprint,
		HostSessionDigest: session.Digest, EnvironmentReportDigest: environment.Digest, InventoryDigest: inventory.Digest,
		ConfigurationDigest: snapshot.Digest(), ResolutionDigest: resolved.Report.Digest(), RegistryDigest: resolved.Registry.Digest(),
		BundleDigest: bundle.Digest,
	}
	if err := sealPilot(&pilot); err != nil {
		return err
	}
	for path, value := range map[string]any{
		filepath.Join(evidence, "pilot.json"):        pilot,
		filepath.Join(evidence, "session.json"):      session,
		filepath.Join(evidence, "environment.json"):  environment,
		filepath.Join(evidence, "inventory.json"):    inventory,
		filepath.Join(evidence, "start-result.json"): result,
	} {
		if err := writeCanonical(path, value); err != nil {
			return err
		}
	}
	succeeded = true
	return printCanonical(result)
}

func dogfoodWorkflowProposal(fingerprint repositoryFingerprint) classification.ClassificationProposal {
	trueTraits := map[classification.Trait]bool{
		classification.TraitScopeClear: true, classification.TraitChangePointKnown: true,
		classification.TraitRecoverable: true, classification.TraitFocusedVerificationKnown: true,
		classification.TraitMultipleResponsibilities: true,
	}
	traits := []classification.Trait{
		classification.TraitScopeClear, classification.TraitChangePointKnown, classification.TraitRecoverable,
		classification.TraitFocusedVerificationKnown, classification.TraitBoundedCapabilityRequest,
		classification.TraitArchitectureDecision, classification.TraitPublicContractChange, classification.TraitSchemaChange,
		classification.TraitDependencyChange, classification.TraitSecuritySensitive, classification.TraitDataSensitive,
		classification.TraitDeploymentChange, classification.TraitDomainUncertainty, classification.TraitRootCauseUncertain,
		classification.TraitMultipleResponsibilities, classification.TraitMultipleTickets,
		classification.TraitLongLivedDelegation, classification.TraitDestructiveMutation, classification.TraitCriticalRelease,
	}
	observations := make([]classification.TraitObservation, 0, len(traits))
	for _, trait := range traits {
		value := classification.TraitFalse
		if trueTraits[trait] {
			value = classification.TraitTrue
		}
		observations = append(observations, classification.TraitObservation{Trait: trait, Value: value})
	}
	return classification.ClassificationProposal{
		SchemaVersion: classification.ProposalSchemaV1, Traits: observations,
		Resources: []classification.Resource{classification.ResourceProject},
		Evidence: []classification.ProposalEvidence{
			{Kind: classification.EvidenceScope, Reference: "git+file://" + fingerprint.Root + "#" + fingerprint.Commit, Digest: canonicaljson.DigestBytes([]byte(fingerprint.Commit))},
			{Kind: classification.EvidenceChangePoint, Reference: "file://" + fingerprint.SkillPath, Digest: fingerprint.SkillDigest},
			{Kind: classification.EvidenceVerification, Reference: "dogfood:read-only-current", Digest: canonicaljson.DigestBytes([]byte("read-only CURRENT verification\n"))},
		},
	}
}

func preparePilot(evidencePath string) error {
	pilot, err := loadPilot(evidencePath)
	if err != nil {
		return err
	}
	if _, err := verifyRepository(pilot.Repository); err != nil {
		return err
	}
	engine, err := openPilotEngine(pilot)
	if err != nil {
		return err
	}
	current, err := inspectWorkflow(engine, pilot.WorkflowID)
	if err != nil {
		return err
	}
	if current.Snapshot == nil {
		return errors.New("Workflow inspection has no snapshot")
	}
	if current.Snapshot.Status == coordinator.StatusPrepared && current.Dispatch != nil {
		return persistAndPrintDispatch(pilot, *current.Dispatch)
	}
	if current.Snapshot.Status != coordinator.StatusReady {
		return fmt.Errorf("Workflow status %s cannot be prepared", current.Snapshot.Status)
	}
	node := current.Snapshot.ActiveNodeID
	result, err := engine.Exchange(coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandPrepare,
		MessageID: "dogfood-prepare-" + node, IdempotencyKey: fmt.Sprintf("dogfood-prepare-%d-%s", current.Snapshot.ActiveGeneration, node),
		WorkflowID: pilot.WorkflowID, ExpectedRevision: current.Revision,
		Prepare: &coordinator.PrepareInput{
			RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project"},
			TerminationCondition: "return read-only " + node + " evidence",
			InputReferences: []coordinator.ArtifactReference{
				{Kind: "git-commit", Reference: "git+file://" + pilot.Repository.Root + "#" + pilot.Repository.Commit, Digest: canonicaljson.DigestBytes([]byte(pilot.Repository.Commit))},
				{Kind: "skill", Reference: "file://" + pilot.Repository.SkillPath, Digest: pilot.Repository.SkillDigest},
			},
			EvidenceRequirements: []coordinator.EvidenceRequirement{{Kind: "report", Minimum: 1, Description: "digest-pinned " + node + " report"}},
		},
	})
	if err != nil {
		return err
	}
	if result.Kind != coordinator.ResultDispatch || result.Dispatch == nil || result.Snapshot == nil || result.Snapshot.Status != coordinator.StatusPrepared {
		return fmt.Errorf("unexpected PREPARE Result: %#v", result)
	}
	return persistAndPrintDispatch(pilot, *result.Dispatch)
}

func inspectPilot(evidencePath string) error {
	pilot, err := loadPilot(evidencePath)
	if err != nil {
		return err
	}
	if _, err := verifyRepository(pilot.Repository); err != nil {
		return err
	}
	engine, err := openPilotEngine(pilot)
	if err != nil {
		return err
	}
	result, err := inspectWorkflow(engine, pilot.WorkflowID)
	if err != nil {
		return err
	}
	if result.Snapshot == nil {
		return errors.New("Workflow inspection has no snapshot")
	}
	for _, receipt := range result.Snapshot.Receipts {
		if receipt.Kind == host.ReceiptCompleted {
			if err := validateEvidence(pilot.EvidenceRoot, receipt.NodeID, receipt.Evidence); err != nil {
				return err
			}
		}
	}
	if err := writeCanonical(filepath.Join(pilot.EvidenceRoot, "inspect.json"), result); err != nil {
		return err
	}
	return printCanonical(result)
}

func receivePilot(evidencePath, receiptPath string) error {
	pilot, err := loadPilot(evidencePath)
	if err != nil {
		return err
	}
	if _, err := verifyRepository(pilot.Repository); err != nil {
		return err
	}
	receiptFile, err := containedRegularFile(receiptPath, pilot.EvidenceRoot)
	if err != nil {
		return fmt.Errorf("receipt file: %w", err)
	}
	raw, err := readLimited(receiptFile)
	if err != nil {
		return err
	}
	engine, err := openPilotEngine(pilot)
	if err != nil {
		return err
	}
	current, err := inspectWorkflow(engine, pilot.WorkflowID)
	if err != nil {
		return err
	}
	if current.Snapshot == nil || current.Snapshot.Status != coordinator.StatusPrepared || current.Dispatch == nil {
		return errors.New("Workflow has no prepared Dispatch Packet")
	}
	node := current.Snapshot.ActiveNodeID
	signal, boundary := receiptTransition(node)
	decoded, err := decodeReceiptCommand(raw, pilot.WorkflowID, current.Revision, node, signal, boundary)
	if err != nil {
		return err
	}
	receipt := decoded.Receipt.Receipt
	if receipt.Kind != host.ReceiptCompleted {
		return errors.New("dogfood receipt must be COMPLETED")
	}
	if err := validateReceiptPins(*current.Dispatch, receipt); err != nil {
		return err
	}
	if err := validateEvidence(pilot.EvidenceRoot, node, receipt.Evidence); err != nil {
		return err
	}
	canonicalReceipt, err := canonicaljson.Marshal(receipt)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, canonicalReceipt) {
		return errors.New("receipt JSON is not the canonical normalized Host record")
	}
	return commitReceipt(engine, pilot, current, decoded, receipt)
}

func commitReceipt(engine *coordinator.Engine, pilot pilotRecord, current coordinator.Result, completed coordinator.Command, receipt host.InvocationReceipt) error {
	node := current.Snapshot.ActiveNodeID
	startedReceipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptStarted,
		WorkflowID: receipt.WorkflowID, BundleGeneration: receipt.BundleGeneration, BundleDigest: receipt.BundleDigest,
		NodeID: receipt.NodeID, Topology: receipt.Topology, HostSessionDigest: receipt.HostSessionDigest,
		DispatchDigest: receipt.DispatchDigest, ContextFreshness: host.ContextShared,
		EnvironmentReportDigest: receipt.EnvironmentReportDigest,
	})
	if err != nil {
		return err
	}
	started, err := engine.Exchange(coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandReceipt,
		MessageID:      "dogfood-started-" + node,
		IdempotencyKey: fmt.Sprintf("dogfood-started-%d-%s", receipt.BundleGeneration, node),
		WorkflowID:     pilot.WorkflowID, ExpectedRevision: current.Revision,
		Receipt: &coordinator.ReceiptInput{Receipt: startedReceipt},
	})
	if err != nil {
		return err
	}
	completed.ExpectedRevision = started.Revision
	result, err := engine.Exchange(completed)
	if err != nil {
		return err
	}
	if result.Snapshot == nil || result.Snapshot.Status != coordinator.StatusReady && result.Snapshot.Status != coordinator.StatusFinished {
		return fmt.Errorf("unexpected COMPLETED Result: %#v", result)
	}
	receiptsRoot := filepath.Join(pilot.EvidenceRoot, "receipts")
	if err := os.MkdirAll(receiptsRoot, 0o700); err != nil {
		return err
	}
	if err := writeCanonical(filepath.Join(receiptsRoot, node+".json"), receipt); err != nil {
		return err
	}
	if err := writeCanonical(filepath.Join(pilot.EvidenceRoot, "receipt-result-"+node+".json"), result); err != nil {
		return err
	}
	return printCanonical(result)
}

func decodeReceiptCommand(raw []byte, workflowID string, revision uint64, node, signal, boundary string) (coordinator.Command, error) {
	wrapper := rawReceiptCommand{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandReceipt,
		MessageID:      "dogfood-completed-" + node,
		IdempotencyKey: fmt.Sprintf("dogfood-completed-%s-%d", node, revision),
		WorkflowID:     workflowID, ExpectedRevision: revision,
		Receipt: rawReceiptPayload{Receipt: json.RawMessage(raw), Signal: signal, StableBoundary: boundary},
	}
	encoded, err := json.Marshal(wrapper)
	if err != nil {
		return coordinator.Command{}, err
	}
	command, err := coordinator.DecodeCommand(encoded)
	if err != nil {
		return coordinator.Command{}, fmt.Errorf("invalid normalized Receipt: %w", err)
	}
	return command, nil
}

func receiptTransition(node string) (string, string) {
	switch node {
	case "review-scope":
		return "succeeded", "scope-complete"
	case "code-review":
		return "succeeded", "review-complete"
	case "verification":
		return "", "verification-complete"
	default:
		return "", ""
	}
}
