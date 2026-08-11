package coordinator

import (
	"fmt"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func validateDispatchBundleClosure(packet DispatchPacket, bundle core.LifecycleBundle) error {
	if packet.BundleID != bundle.ID || packet.BundleGeneration != bundle.Generation || packet.BundleDigest != bundle.Digest ||
		packet.Topology != bundle.Topology || packet.HostSessionDigest != bundle.HostSessionDigest ||
		packet.EnvironmentReportDigest != bundle.EnvironmentReportDigest ||
		!sameCanonicalValue(packet.EnvironmentRequirements, bundle.EnvironmentRequirements) {
		return fmt.Errorf("Dispatch Packet Bundle identity or Host environment does not match")
	}
	if bundle.Graph.HostID != bundle.HostID || bundle.Graph.HostEvidenceDigest != bundle.HostEvidenceDigest ||
		bundle.Graph.RegistryDigest != bundle.RegistryDigest || bundle.Graph.RecipeID != bundle.Recipe.ID ||
		bundle.Graph.RecipeDigest != bundle.RecipeDigest || bundle.Graph.Topology != bundle.Topology ||
		!sameCanonicalValue(bundle.Graph.EnvironmentRequirements, bundle.EnvironmentRequirements) {
		return fmt.Errorf("Lifecycle Bundle outer contract does not match its execution Graph")
	}
	if err := validateGrantBundleClosure(packet.Grant, bundle); err != nil {
		return err
	}
	return validateDispatchAuthorityClosure(packet, bundle)
}

func validateGrantBundleClosure(grant admission.CapabilityGrant, bundle core.LifecycleBundle) error {
	if grant.BundleID != bundle.ID || grant.BundleGeneration != bundle.Generation || grant.BundleDigest != bundle.Digest ||
		grant.Topology != bundle.Topology || grant.HostSessionDigest != bundle.HostSessionDigest {
		return fmt.Errorf("Grant Bundle identity, topology, or Host session does not match")
	}
	unit, err := profile.UnitAtCursor(bundle.Graph, grant.Cursor)
	if err != nil {
		return fmt.Errorf("Grant cursor is not an active Graph unit: %w", err)
	}
	switch grant.Target.TargetKind {
	case admission.GrantProviderBinding:
		if unit.ProviderBinding == nil || grant.Target.ProviderBinding == nil ||
			!providerBindingAuthorityMatchesUnit(*grant.Target.ProviderBinding, *unit.ProviderBinding) ||
			!topologyAllowed(unit.ProviderBinding.SupportedTopologies, grant.Topology) ||
			!stringSetSubset(grant.Effects, unit.ProviderBinding.MaximumEffects) ||
			!stringSetSubset(grant.Resources, unit.ProviderBinding.Resources) {
			return fmt.Errorf("Grant Provider Binding target does not match its Graph unit")
		}
	case admission.GrantHostAction:
		if unit.HostAction == nil || grant.Target.HostAction == nil {
			return fmt.Errorf("Grant Host action target does not match its Graph unit")
		}
		expected, authorityErr := admission.NewHostActionAuthority(*unit.HostAction)
		if authorityErr != nil {
			return fmt.Errorf("Grant Host action target is invalid: %w", authorityErr)
		}
		if !sameCanonicalValue(expected, *grant.Target.HostAction) ||
			!stringSetSubset(grant.Effects, unit.HostAction.MaximumEffects) ||
			!stringSetSubset(grant.Resources, unit.HostAction.Resources) {
			return fmt.Errorf("Grant Host action target does not match its Graph unit")
		}
	default:
		return fmt.Errorf("Grant target kind is not dispatchable")
	}
	return nil
}

func validateDispatchAuthorityClosure(packet DispatchPacket, bundle core.LifecycleBundle) error {
	if packet.Authorization != nil {
		authorization := packet.Authorization
		if authorization.Decision != admission.AuthorizationAllowed || authorization.IssuerHostID != bundle.HostID ||
			authorization.HostSessionDigest != packet.Grant.HostSessionDigest || authorization.WorkflowID != packet.Grant.WorkflowID ||
			authorization.BundleID != packet.Grant.BundleID || authorization.BundleGeneration != packet.Grant.BundleGeneration ||
			authorization.BundleDigest != packet.Grant.BundleDigest || authorization.Cursor != packet.Grant.Cursor ||
			!sameCanonicalValue(authorization.Target, packet.Grant.Target) || !sameCanonicalValue(authorization.Effects, packet.Grant.Effects) ||
			!sameCanonicalValue(authorization.Resources, packet.Grant.Resources) {
			return fmt.Errorf("Dispatch Authorization does not exactly match its Grant and Bundle")
		}
	}
	if packet.InvocationAttestation != nil {
		attestation := packet.InvocationAttestation
		if packet.Grant.Target.ProviderBinding == nil || attestation.IssuerHostID != bundle.HostID ||
			attestation.HostSessionDigest != packet.Grant.HostSessionDigest || attestation.WorkflowID != packet.Grant.WorkflowID ||
			attestation.BundleID != packet.Grant.BundleID || attestation.BundleGeneration != packet.Grant.BundleGeneration ||
			attestation.BundleDigest != packet.Grant.BundleDigest || attestation.Cursor != packet.Grant.Cursor ||
			!sameCanonicalValue(attestation.ProviderBinding, *packet.Grant.Target.ProviderBinding) {
			return fmt.Errorf("Dispatch Invocation Attestation does not exactly match its Grant and Bundle")
		}
	}
	return nil
}

func bundleForGeneration(snapshot Snapshot, generation uint64) (core.LifecycleBundle, error) {
	var matched core.LifecycleBundle
	count := 0
	for _, bundle := range snapshot.Bundles {
		if bundle.Generation == generation {
			matched = bundle
			count++
		}
	}
	if count != 1 {
		return core.LifecycleBundle{}, fmt.Errorf("Bundle generation %d does not identify exactly one Lifecycle Bundle", generation)
	}
	return matched, nil
}

func validateAuthorizationHistoryClosure(authorization admission.UserAuthorization, snapshot Snapshot) error {
	bundle, err := bundleForGeneration(snapshot, authorization.BundleGeneration)
	if err != nil {
		return err
	}
	if authorization.Decision != admission.AuthorizationAllowed || authorization.IssuerHostID != bundle.HostID ||
		authorization.BundleID != bundle.ID || authorization.BundleDigest != bundle.Digest ||
		authorization.HostSessionDigest != bundle.HostSessionDigest {
		return fmt.Errorf("User Authorization does not match its Lifecycle Bundle")
	}
	for _, grant := range snapshot.GrantHistory {
		if grant.AuthorizationDigest == authorization.Digest && grant.WorkflowID == authorization.WorkflowID &&
			grant.BundleID == authorization.BundleID && grant.BundleGeneration == authorization.BundleGeneration &&
			grant.BundleDigest == authorization.BundleDigest && grant.Cursor == authorization.Cursor &&
			grant.HostSessionDigest == authorization.HostSessionDigest && sameCanonicalValue(grant.Target, authorization.Target) &&
			sameCanonicalValue(grant.Effects, authorization.Effects) && sameCanonicalValue(grant.Resources, authorization.Resources) {
			return nil
		}
	}
	return fmt.Errorf("User Authorization is not pinned by an exact Grant history entry")
}

func validateInvocationHistoryClosure(attestation admission.ExplicitInvocationAttestation, snapshot Snapshot) error {
	bundle, err := bundleForGeneration(snapshot, attestation.BundleGeneration)
	if err != nil {
		return err
	}
	if attestation.IssuerHostID != bundle.HostID || attestation.BundleID != bundle.ID || attestation.BundleDigest != bundle.Digest ||
		attestation.HostSessionDigest != bundle.HostSessionDigest {
		return fmt.Errorf("Invocation Attestation does not match its Lifecycle Bundle")
	}
	for _, grant := range snapshot.GrantHistory {
		if grant.InvocationAttestationDigest == attestation.Digest && grant.Target.ProviderBinding != nil &&
			grant.WorkflowID == attestation.WorkflowID && grant.BundleID == attestation.BundleID &&
			grant.BundleGeneration == attestation.BundleGeneration && grant.BundleDigest == attestation.BundleDigest &&
			grant.Cursor == attestation.Cursor && grant.HostSessionDigest == attestation.HostSessionDigest &&
			sameCanonicalValue(*grant.Target.ProviderBinding, attestation.ProviderBinding) {
			return nil
		}
	}
	return fmt.Errorf("Invocation Attestation is not pinned by an exact Grant history entry")
}

func validateGateHistoryClosure(attestation GateAttestation, snapshot Snapshot) error {
	bundle, err := bundleForGeneration(snapshot, attestation.BundleGeneration)
	if err != nil {
		return err
	}
	if attestation.BundleID != bundle.ID || attestation.BundleDigest != bundle.Digest {
		return fmt.Errorf("Gate Attestation does not match its Lifecycle Bundle")
	}
	unit, err := profile.UnitAtCursor(bundle.Graph, attestation.Cursor)
	if err != nil {
		return fmt.Errorf("Gate Attestation cursor is invalid: %w", err)
	}
	if unit.Gate == nil || unit.Gate.ID != attestation.GateID || unit.Gate.Authority != attestation.Authority {
		return fmt.Errorf("Gate Attestation does not match its Graph gate")
	}
	return validateGateEvidenceClosure(unit.Gate.EvidenceRequirements, attestation.Evidence)
}

func validateReceiptHistoryClosure(receipt host.InvocationReceipt, snapshot Snapshot) error {
	bundle, err := bundleForGeneration(snapshot, receipt.BundleGeneration)
	if err != nil {
		return err
	}
	if receipt.BundleID != bundle.ID || receipt.BundleDigest != bundle.Digest || receipt.Topology != bundle.Topology ||
		receipt.HostSessionDigest != bundle.HostSessionDigest || receipt.EnvironmentReportDigest != bundle.EnvironmentReportDigest {
		return fmt.Errorf("Host Receipt does not match its Lifecycle Bundle")
	}
	unit, err := profile.UnitAtCursor(bundle.Graph, receipt.Cursor)
	if err != nil {
		return fmt.Errorf("Host Receipt cursor is invalid: %w", err)
	}
	if unit.ProviderBinding == nil && unit.HostAction == nil {
		return fmt.Errorf("Host Receipt cursor is not a dispatchable Graph unit")
	}
	for _, grant := range snapshot.GrantHistory {
		if grant.BundleID == receipt.BundleID && grant.BundleGeneration == receipt.BundleGeneration && grant.BundleDigest == receipt.BundleDigest &&
			grant.Cursor == receipt.Cursor && grant.Topology == receipt.Topology && grant.HostSessionDigest == receipt.HostSessionDigest {
			return nil
		}
	}
	return fmt.Errorf("Host Receipt has no matching Grant history entry")
}

func providerBindingAuthorityMatchesUnit(authority admission.ProviderBindingAuthority, unit profile.ResolvedBinding) bool {
	return authority.ProviderID == unit.ProviderID && authority.ProviderInstanceDigest == unit.ProviderInstanceDigest &&
		authority.DistributionID == unit.DistributionID && authority.DistributionRevision == unit.DistributionRevision &&
		authority.DistributionTreeDigest == unit.DistributionTreeDigest && authority.BindingID == unit.BindingID &&
		authority.Surface == unit.Surface && authority.Kind == unit.Kind && authority.Reference == unit.Reference &&
		authority.Invocation == unit.Invocation && authority.BindingTreeDigest == unit.BindingTreeDigest &&
		authority.BindingEvidenceDigest == unit.BindingEvidenceDigest && authority.InputArtifact == unit.InputArtifact &&
		authority.OutputArtifact == unit.OutputArtifact && authority.RequiresExplicitInvocation == unit.RequiresExplicitInvocation
}

func topologyAllowed(values []execution.Topology, target execution.Topology) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func stringSetSubset(values, ceiling []string) bool {
	for _, value := range values {
		if !containsString(ceiling, value) {
			return false
		}
	}
	return true
}
