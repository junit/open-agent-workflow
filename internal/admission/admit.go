package admission

import (
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

var (
	workflowIDPattern            = regexp.MustCompile(`^workflow-[0-9a-f]{32}$`)
	bundleIDPattern              = regexp.MustCompile(`^bundle-[0-9a-f]{32}$`)
	grantIDPattern               = regexp.MustCompile(`^grant-[0-9a-f]{32}$`)
	authorizationIDPattern       = regexp.MustCompile(`^authorization-[0-9a-f]{32}$`)
	invocationAttestationPattern = regexp.MustCompile(`^invocation-attestation-[0-9a-f]{32}$`)
	revisionPattern              = regexp.MustCompile(`^(?:[0-9a-f]{40}|sha256:[0-9a-f]{64})$`)
	treeDigestPattern            = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

func NewProviderBindingAuthority(unit profile.ResolvedBinding, capability catalog.CapabilityRecord) (ProviderBindingAuthority, error) {
	if err := validateResolvedBindingSource(unit); err != nil {
		return ProviderBindingAuthority{}, err
	}
	if !validLocalID(capability.ID) || !validText(capability.InputSchema, 512) || !validText(capability.OutcomeSchema, 512) ||
		!slices.Contains(capability.RequestModes, catalog.RequestModeWorkflow) || !canonicalBindingRefs(capability.BindingRefs, unit.BindingID) {
		return ProviderBindingAuthority{}, admissionError("WORKFLOW_GRANT_INVALID", "Provider Binding Capability contract is missing or inconsistent", nil)
	}
	return ProviderBindingAuthority{
		ProviderID: unit.ProviderID, ProviderInstanceDigest: unit.ProviderInstanceDigest,
		DistributionID: unit.DistributionID, DistributionRevision: unit.DistributionRevision, DistributionTreeDigest: unit.DistributionTreeDigest,
		BindingID: unit.BindingID, Surface: unit.Surface, Kind: unit.Kind, Reference: unit.Reference, Invocation: unit.Invocation,
		BindingTreeDigest: unit.BindingTreeDigest, BindingEvidenceDigest: unit.BindingEvidenceDigest,
		InputArtifact: unit.InputArtifact, OutputArtifact: unit.OutputArtifact, InputSchema: capability.InputSchema, OutcomeSchema: capability.OutcomeSchema,
		RequiresExplicitInvocation: unit.RequiresExplicitInvocation,
	}, nil
}

func NewHostActionAuthority(action profile.CompiledHostAction) (HostActionAuthority, error) {
	if err := validateCompiledHostAction(action); err != nil {
		return HostActionAuthority{}, err
	}
	return HostActionAuthority{
		ID: action.ID, InputArtifact: action.InputArtifact, OutputArtifact: action.OutputArtifact,
		InputSchema: action.InputSchema, OutcomeSchema: action.OutcomeSchema,
		MaximumEffects: append([]string{}, action.MaximumEffects...), Resources: append([]string{}, action.Resources...),
		ObservationDigest: action.ObservationDigest,
	}, nil
}

func NewUserAuthorization(input UserAuthorization) (UserAuthorization, error) {
	if input.SchemaVersion != UserAuthorizationSchemaV1 {
		return UserAuthorization{}, admissionError("USER_AUTHORIZATION_SCHEMA_UNSUPPORTED", "unsupported User Authorization schema", nil)
	}
	providedID, providedDigest := input.ID, input.Digest
	input = CloneUserAuthorization(input)
	input.ID, input.Digest = "", ""
	var err error
	input.Effects, err = normalizeSet(input.Effects, knownEffect)
	if err != nil {
		return UserAuthorization{}, admissionError("USER_AUTHORIZATION_INVALID", "invalid authorized effects", err)
	}
	input.Resources, err = normalizeSet(input.Resources, knownResource)
	if err != nil {
		return UserAuthorization{}, admissionError("USER_AUTHORIZATION_INVALID", "invalid authorized resources", err)
	}
	input.Evidence, err = normalizeEvidence(input.Evidence)
	if err != nil {
		return UserAuthorization{}, admissionError("USER_AUTHORIZATION_INVALID", "invalid authorization evidence", err)
	}
	if !validHostAuthorityIdentity(input.IssuerHostID, input.HostSessionDigest, input.EvidenceHandleDigest, input.AuthorizationNonce,
		input.WorkflowID, input.BundleID, input.BundleGeneration, input.BundleDigest, input.Cursor) ||
		(input.Decision != AuthorizationAllowed && input.Decision != AuthorizationDenied) || len(input.Effects) == 0 || len(input.Resources) == 0 || len(input.Evidence) == 0 {
		return UserAuthorization{}, admissionError("USER_AUTHORIZATION_INVALID", "invalid User Authorization identity or decision", nil)
	}
	target, err := normalizeAuthorizationTarget(input.Target)
	if err != nil {
		return UserAuthorization{}, admissionError("USER_AUTHORIZATION_INVALID", "invalid authorization target", err)
	}
	input.Target = target
	if !targetMatchesCursor(input.Target, input.Cursor) {
		return UserAuthorization{}, admissionError("USER_AUTHORIZATION_INVALID", "authorization cursor does not match target", nil)
	}
	identityDigest, _, err := canonicaljson.Digest(input)
	if err != nil {
		return UserAuthorization{}, admissionError("USER_AUTHORIZATION_INVALID", "digest User Authorization identity", err)
	}
	input.ID = "authorization-" + identityDigest[:32]
	if providedID != "" && (providedID != input.ID || !authorizationIDPattern.MatchString(providedID)) {
		return UserAuthorization{}, admissionError("USER_AUTHORIZATION_INVALID", "User Authorization ID does not match content", nil)
	}
	input.Digest, _, err = canonicaljson.Digest(input)
	if err != nil {
		return UserAuthorization{}, admissionError("USER_AUTHORIZATION_INVALID", "digest User Authorization", err)
	}
	if providedDigest != "" && providedDigest != input.Digest {
		return UserAuthorization{}, admissionError("USER_AUTHORIZATION_INVALID", "User Authorization digest does not match content", nil)
	}
	return input, nil
}

func ValidateUserAuthorization(value UserAuthorization) error {
	normalized, err := NewUserAuthorization(value)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(normalized, value) {
		return admissionError("USER_AUTHORIZATION_INVALID", "User Authorization is not canonical", nil)
	}
	return nil
}

func NewExplicitInvocationAttestation(input ExplicitInvocationAttestation) (ExplicitInvocationAttestation, error) {
	if input.SchemaVersion != ExplicitInvocationAttestationSchemaV1 {
		return ExplicitInvocationAttestation{}, admissionError("EXPLICIT_INVOCATION_SCHEMA_UNSUPPORTED", "unsupported Explicit Invocation Attestation schema", nil)
	}
	providedID, providedDigest := input.ID, input.Digest
	input = CloneExplicitInvocationAttestation(input)
	input.ID, input.Digest = "", ""
	var err error
	input.Evidence, err = normalizeEvidence(input.Evidence)
	if err != nil {
		return ExplicitInvocationAttestation{}, admissionError("EXPLICIT_INVOCATION_INVALID", "invalid invocation evidence", err)
	}
	if !validHostAuthorityIdentity(input.IssuerHostID, input.HostSessionDigest, input.EvidenceHandleDigest, input.InvocationNonce,
		input.WorkflowID, input.BundleID, input.BundleGeneration, input.BundleDigest, input.Cursor) || len(input.Evidence) == 0 ||
		input.Cursor.Kind != execution.CursorBinding || !input.ProviderBinding.RequiresExplicitInvocation {
		return ExplicitInvocationAttestation{}, admissionError("EXPLICIT_INVOCATION_INVALID", "invalid Explicit Invocation Attestation identity", nil)
	}
	if err := validateProviderBindingAuthority(input.ProviderBinding); err != nil {
		return ExplicitInvocationAttestation{}, admissionError("EXPLICIT_INVOCATION_INVALID", "invalid Provider Binding target", err)
	}
	identityDigest, _, err := canonicaljson.Digest(input)
	if err != nil {
		return ExplicitInvocationAttestation{}, admissionError("EXPLICIT_INVOCATION_INVALID", "digest Explicit Invocation identity", err)
	}
	input.ID = "invocation-attestation-" + identityDigest[:32]
	if providedID != "" && (providedID != input.ID || !invocationAttestationPattern.MatchString(providedID)) {
		return ExplicitInvocationAttestation{}, admissionError("EXPLICIT_INVOCATION_INVALID", "Explicit Invocation ID does not match content", nil)
	}
	input.Digest, _, err = canonicaljson.Digest(input)
	if err != nil {
		return ExplicitInvocationAttestation{}, admissionError("EXPLICIT_INVOCATION_INVALID", "digest Explicit Invocation Attestation", err)
	}
	if providedDigest != "" && providedDigest != input.Digest {
		return ExplicitInvocationAttestation{}, admissionError("EXPLICIT_INVOCATION_INVALID", "Explicit Invocation digest does not match content", nil)
	}
	return input, nil
}

func ValidateExplicitInvocationAttestation(value ExplicitInvocationAttestation) error {
	normalized, err := NewExplicitInvocationAttestation(value)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(normalized, value) {
		return admissionError("EXPLICIT_INVOCATION_INVALID", "Explicit Invocation Attestation is not canonical", nil)
	}
	return nil
}

func IssueWorkflowGrant(request WorkflowGrantRequest) (CapabilityGrant, error) {
	normalized, target, maximumEffects, targetResources, err := normalizeWorkflowGrantRequest(request)
	if err != nil {
		return CapabilityGrant{}, err
	}
	if !subset(normalized.Effects, maximumEffects) || !subset(normalized.Resources, targetResources) ||
		!subset(normalized.Effects, normalized.Authority.Effects) || !subset(normalized.Resources, normalized.Authority.Resources) {
		return CapabilityGrant{}, admissionError("CAPABILITY_AUTHORITY_EXCEEDED", "requested authority exceeds target or Engine ceiling", nil)
	}
	if requiresResourceLease(normalized.Effects) && (!normalized.Authority.ResourceLeases || !contains(normalized.Resources, "project-worktree")) {
		return CapabilityGrant{}, admissionError("RESOURCE_LEASE_REQUIRED", "write-capable Grant requires a project-worktree Resource Lease", nil)
	}
	authorizationDigest, err := validateRequestAuthorization(normalized, target)
	if err != nil {
		return CapabilityGrant{}, err
	}
	invocationDigest, err := validateRequestInvocation(normalized, target)
	if err != nil {
		return CapabilityGrant{}, err
	}
	grant := CapabilityGrant{
		SchemaVersion: CapabilityGrantSchemaV3, WorkflowID: normalized.WorkflowID, RequestID: normalized.RequestID,
		BundleID: normalized.BundleID, BundleGeneration: normalized.BundleGeneration, BundleDigest: normalized.BundleDigest,
		Cursor: normalized.Cursor, Target: target, Topology: normalized.Topology, HostSessionDigest: normalized.HostSessionDigest,
		Effects: normalized.Effects, Resources: normalized.Resources, TerminationCondition: normalized.TerminationCondition,
		AuthorizationDigest: authorizationDigest, InvocationAttestationDigest: invocationDigest,
	}
	seed := CloneGrant(grant)
	identityDigest, _, err := canonicaljson.Digest(seed)
	if err != nil {
		return CapabilityGrant{}, admissionError("CAPABILITY_GRANT_INVALID", "digest Grant identity", err)
	}
	grant.ID = "grant-" + identityDigest[:32]
	grant.Digest, _, err = canonicaljson.Digest(grant)
	if err != nil {
		return CapabilityGrant{}, admissionError("CAPABILITY_GRANT_INVALID", "digest Grant", err)
	}
	return grant, nil
}

func ValidateGrant(value CapabilityGrant) error {
	if value.SchemaVersion != CapabilityGrantSchemaV3 {
		return admissionError("CAPABILITY_GRANT_SCHEMA_UNSUPPORTED", "unsupported Capability Grant schema", nil)
	}
	if !grantIDPattern.MatchString(value.ID) || !validWorkflowID(value.WorkflowID) || !validText(value.RequestID, 512) ||
		!bundleIDPattern.MatchString(value.BundleID) || value.BundleGeneration == 0 || !validDigest(value.BundleDigest) ||
		execution.ValidateGraphCursor(value.Cursor) != nil || (value.Topology != execution.TopologyCurrent && value.Topology != execution.TopologySubagent) ||
		!validDigest(value.HostSessionDigest) || !validText(value.TerminationCondition, 2048) ||
		!validSortedSet(value.Effects, knownEffect) || !validSortedSet(value.Resources, knownResource) || !optionalDigest(value.AuthorizationDigest) ||
		!optionalDigest(value.InvocationAttestationDigest) || !validDigest(value.Digest) {
		return admissionError("CAPABILITY_GRANT_INVALID", "invalid Grant identity or authority", nil)
	}
	target, err := normalizeAuthorizationTarget(value.Target)
	if err != nil || !reflect.DeepEqual(target, value.Target) {
		return admissionError("CAPABILITY_GRANT_INVALID", "invalid or non-canonical Grant target", err)
	}
	if value.Cursor.Kind == execution.CursorBinding && target.TargetKind != GrantProviderBinding ||
		value.Cursor.Kind == execution.CursorHostAction && target.TargetKind != GrantHostAction ||
		value.Cursor.Kind != execution.CursorBinding && value.Cursor.Kind != execution.CursorHostAction {
		return admissionError("CAPABILITY_GRANT_INVALID", "Grant cursor does not match target", nil)
	}
	if requiresNetworkAuthorization(value.Effects) && value.AuthorizationDigest == "" ||
		target.ProviderBinding != nil && target.ProviderBinding.RequiresExplicitInvocation && value.InvocationAttestationDigest == "" {
		return admissionError("CAPABILITY_GRANT_INVALID", "Grant omits required Host authority", nil)
	}
	seed := CloneGrant(value)
	seed.ID, seed.Digest = "", ""
	identityDigest, _, err := canonicaljson.Digest(seed)
	if err != nil || value.ID != "grant-"+identityDigest[:32] {
		return admissionError("CAPABILITY_GRANT_INVALID", "Grant ID does not match content", err)
	}
	unsigned := CloneGrant(value)
	unsigned.Digest = ""
	digest, _, err := canonicaljson.Digest(unsigned)
	if err != nil || digest != value.Digest {
		return admissionError("CAPABILITY_GRANT_INVALID", "Grant digest does not match content", err)
	}
	return nil
}

func normalizeWorkflowGrantRequest(value WorkflowGrantRequest) (WorkflowGrantRequest, AuthorizationTarget, []string, []string, error) {
	if !validWorkflowID(value.WorkflowID) || !validText(value.RequestID, 512) || !bundleIDPattern.MatchString(value.BundleID) || value.BundleGeneration == 0 ||
		!validDigest(value.BundleDigest) || execution.ValidateGraphCursor(value.Cursor) != nil || !validLocalID(value.HostID) || !validDigest(value.HostSessionDigest) ||
		(value.Topology != execution.TopologyCurrent && value.Topology != execution.TopologySubagent) || !validText(value.TerminationCondition, 2048) {
		return WorkflowGrantRequest{}, AuthorizationTarget{}, nil, nil, admissionError("WORKFLOW_GRANT_INVALID", "invalid Grant request identity", nil)
	}
	if (value.ProviderBinding == nil) == (value.HostAction == nil) || value.ProviderBinding != nil && value.Capability == nil || value.HostAction != nil && value.Capability != nil {
		return WorkflowGrantRequest{}, AuthorizationTarget{}, nil, nil, admissionError("WORKFLOW_GRANT_INVALID", "Grant request requires exactly one complete target", nil)
	}
	var target AuthorizationTarget
	var maximumEffects, resources []string
	if value.ProviderBinding != nil {
		authority, err := NewProviderBindingAuthority(*value.ProviderBinding, *value.Capability)
		if err != nil || value.Cursor != value.ProviderBinding.Cursor {
			return WorkflowGrantRequest{}, AuthorizationTarget{}, nil, nil, admissionError("WORKFLOW_GRANT_INVALID", "invalid Provider Binding target", err)
		}
		if !slices.Contains(value.ProviderBinding.SupportedTopologies, value.Topology) {
			return WorkflowGrantRequest{}, AuthorizationTarget{}, nil, nil, admissionError("CAPABILITY_TOPOLOGY_DENIED", "selected topology is not supported by the Provider Binding", nil)
		}
		target = AuthorizationTarget{TargetKind: GrantProviderBinding, ProviderBinding: &authority}
		maximumEffects = append([]string{}, value.ProviderBinding.MaximumEffects...)
		resources = append([]string{}, value.ProviderBinding.Resources...)
	} else {
		authority, err := NewHostActionAuthority(*value.HostAction)
		if err != nil || value.Cursor != value.HostAction.Cursor {
			return WorkflowGrantRequest{}, AuthorizationTarget{}, nil, nil, admissionError("WORKFLOW_GRANT_INVALID", "invalid Host action target", err)
		}
		target = AuthorizationTarget{TargetKind: GrantHostAction, HostAction: &authority}
		maximumEffects = append([]string{}, value.HostAction.MaximumEffects...)
		resources = append([]string{}, value.HostAction.Resources...)
	}
	var err error
	value.Effects, err = normalizeSet(value.Effects, knownEffect)
	if err != nil || len(value.Effects) == 0 {
		return WorkflowGrantRequest{}, AuthorizationTarget{}, nil, nil, admissionError("WORKFLOW_GRANT_INVALID", "invalid requested effects", err)
	}
	value.Resources, err = normalizeSet(value.Resources, knownResource)
	if err != nil || len(value.Resources) == 0 {
		return WorkflowGrantRequest{}, AuthorizationTarget{}, nil, nil, admissionError("WORKFLOW_GRANT_INVALID", "invalid requested resources", err)
	}
	value.Authority.Effects, err = normalizeSet(value.Authority.Effects, knownEffect)
	if err != nil {
		return WorkflowGrantRequest{}, AuthorizationTarget{}, nil, nil, admissionError("WORKFLOW_GRANT_INVALID", "invalid Engine effect ceiling", err)
	}
	value.Authority.Resources, err = normalizeSet(value.Authority.Resources, knownResource)
	if err != nil {
		return WorkflowGrantRequest{}, AuthorizationTarget{}, nil, nil, admissionError("WORKFLOW_GRANT_INVALID", "invalid Engine resource ceiling", err)
	}
	return value, target, maximumEffects, resources, nil
}

func validateRequestAuthorization(request WorkflowGrantRequest, target AuthorizationTarget) (string, error) {
	if request.Authorization == nil {
		if requiresNetworkAuthorization(request.Effects) {
			return "", admissionError("USER_AUTHORIZATION_REQUIRED", "network mutation requires exact current Host authorization", nil)
		}
		return "", nil
	}
	authorization := CloneUserAuthorization(*request.Authorization)
	if err := ValidateUserAuthorization(authorization); err != nil {
		return "", admissionError("USER_AUTHORIZATION_INVALID", "invalid User Authorization", err)
	}
	if authorization.Decision != AuthorizationAllowed || authorization.IssuerHostID != request.HostID || authorization.HostSessionDigest != request.HostSessionDigest ||
		authorization.WorkflowID != request.WorkflowID || authorization.BundleID != request.BundleID || authorization.BundleGeneration != request.BundleGeneration ||
		authorization.BundleDigest != request.BundleDigest || authorization.Cursor != request.Cursor || !reflect.DeepEqual(authorization.Target, target) ||
		!slices.Equal(authorization.Effects, request.Effects) || !slices.Equal(authorization.Resources, request.Resources) {
		return "", admissionError("USER_AUTHORIZATION_INVALID", "User Authorization does not exactly match Grant authority", nil)
	}
	return authorization.Digest, nil
}

func validateRequestInvocation(request WorkflowGrantRequest, target AuthorizationTarget) (string, error) {
	required := target.ProviderBinding != nil && target.ProviderBinding.RequiresExplicitInvocation
	if request.InvocationAttestation == nil {
		if required {
			return "", admissionError("EXPLICIT_INVOCATION_REQUIRED", "human-explicit Binding requires exact invocation attestation", nil)
		}
		return "", nil
	}
	if !required {
		return "", admissionError("EXPLICIT_INVOCATION_INVALID", "invocation attestation is not valid for this target", nil)
	}
	attestation := CloneExplicitInvocationAttestation(*request.InvocationAttestation)
	if err := ValidateExplicitInvocationAttestation(attestation); err != nil {
		return "", admissionError("EXPLICIT_INVOCATION_INVALID", "invalid Explicit Invocation Attestation", err)
	}
	if attestation.IssuerHostID != request.HostID || attestation.HostSessionDigest != request.HostSessionDigest || attestation.WorkflowID != request.WorkflowID ||
		attestation.BundleID != request.BundleID || attestation.BundleGeneration != request.BundleGeneration || attestation.BundleDigest != request.BundleDigest ||
		attestation.Cursor != request.Cursor || !reflect.DeepEqual(attestation.ProviderBinding, *target.ProviderBinding) {
		return "", admissionError("EXPLICIT_INVOCATION_INVALID", "Explicit Invocation Attestation does not exactly match Grant target", nil)
	}
	return attestation.Digest, nil
}

func normalizeAuthorizationTarget(value AuthorizationTarget) (AuthorizationTarget, error) {
	value = CloneAuthorizationTarget(value)
	switch value.TargetKind {
	case GrantProviderBinding:
		if value.ProviderBinding == nil || value.HostAction != nil {
			return AuthorizationTarget{}, admissionError("AUTHORIZATION_TARGET_INVALID", "provider-binding target requires exactly one Provider Binding", nil)
		}
		if err := validateProviderBindingAuthority(*value.ProviderBinding); err != nil {
			return AuthorizationTarget{}, err
		}
	case GrantHostAction:
		if value.HostAction == nil || value.ProviderBinding != nil {
			return AuthorizationTarget{}, admissionError("AUTHORIZATION_TARGET_INVALID", "host-action target requires exactly one Host action", nil)
		}
		if err := validateHostActionAuthority(*value.HostAction); err != nil {
			return AuthorizationTarget{}, err
		}
	default:
		return AuthorizationTarget{}, admissionError("AUTHORIZATION_TARGET_INVALID", "unknown target kind", nil)
	}
	return value, nil
}

func targetMatchesCursor(target AuthorizationTarget, cursor execution.GraphCursor) bool {
	return target.TargetKind == GrantProviderBinding && cursor.Kind == execution.CursorBinding ||
		target.TargetKind == GrantHostAction && cursor.Kind == execution.CursorHostAction
}

func validateResolvedBindingSource(value profile.ResolvedBinding) error {
	if execution.ValidateGraphCursor(value.Cursor) != nil || value.Cursor.Kind != execution.CursorBinding || value.Cursor.UnitID != value.UnitID ||
		value.Disposition != profile.DispatchByCoordinator || !validText(value.UnitID, 512) || !validText(value.StepID, 512) ||
		!validQualifiedID(value.ProviderID) || !validDigest(value.ProviderInstanceDigest) || !validLocalID(value.DistributionID) ||
		!revisionPattern.MatchString(value.DistributionRevision) || !treeDigestPattern.MatchString(value.DistributionTreeDigest) || !validLocalID(value.BindingID) ||
		!validText(value.Surface, 512) || !validText(value.Reference, 2048) || !validBindingKind(value.Kind) || !validInvocation(value.Invocation) ||
		value.Invocation == catalog.InvocationInternal || !treeDigestPattern.MatchString(value.BindingTreeDigest) || !validDigest(value.BindingEvidenceDigest) ||
		!validText(value.InputArtifact, 512) || !validText(value.OutputArtifact, 512) || value.RequiresExplicitInvocation != (value.Invocation == catalog.InvocationHumanExplicit) ||
		!canonicalTopologySet(value.SupportedTopologies) || !validSortedSet(value.MaximumEffects, knownEffect) || !validSortedSet(value.Resources, knownResource) {
		return admissionError("WORKFLOW_GRANT_INVALID", "invalid or non-dispatchable Provider Binding", nil)
	}
	return nil
}

func validateCompiledHostAction(value profile.CompiledHostAction) error {
	if execution.ValidateGraphCursor(value.Cursor) != nil || value.Cursor.Kind != execution.CursorHostAction || value.Cursor.UnitID != value.ID ||
		!validText(value.ID, 512) || !validText(value.InputArtifact, 512) || !validText(value.OutputArtifact, 512) ||
		!validText(value.InputSchema, 512) || !validText(value.OutcomeSchema, 512) || !validSortedSet(value.MaximumEffects, knownEffect) ||
		!validSortedSet(value.Resources, knownResource) || !validDigest(value.ObservationDigest) {
		return admissionError("WORKFLOW_GRANT_INVALID", "invalid compiled Host action", nil)
	}
	return nil
}

func validateProviderBindingAuthority(value ProviderBindingAuthority) error {
	if !validQualifiedID(value.ProviderID) || !validDigest(value.ProviderInstanceDigest) || !validLocalID(value.DistributionID) ||
		!revisionPattern.MatchString(value.DistributionRevision) || !treeDigestPattern.MatchString(value.DistributionTreeDigest) || !validLocalID(value.BindingID) ||
		!validText(value.Surface, 512) || !validText(value.Reference, 2048) || !validBindingKind(value.Kind) || !validInvocation(value.Invocation) ||
		value.Invocation == catalog.InvocationInternal || !treeDigestPattern.MatchString(value.BindingTreeDigest) || !validDigest(value.BindingEvidenceDigest) ||
		!validText(value.InputArtifact, 512) || !validText(value.OutputArtifact, 512) || !validText(value.InputSchema, 512) || !validText(value.OutcomeSchema, 512) ||
		value.RequiresExplicitInvocation != (value.Invocation == catalog.InvocationHumanExplicit) {
		return admissionError("AUTHORIZATION_TARGET_INVALID", "invalid Provider Binding authority", nil)
	}
	return nil
}

func validateHostActionAuthority(value HostActionAuthority) error {
	if !validText(value.ID, 512) || !validText(value.InputArtifact, 512) || !validText(value.OutputArtifact, 512) ||
		!validText(value.InputSchema, 512) || !validText(value.OutcomeSchema, 512) || !validSortedSet(value.MaximumEffects, knownEffect) ||
		!validSortedSet(value.Resources, knownResource) || !validDigest(value.ObservationDigest) {
		return admissionError("AUTHORIZATION_TARGET_INVALID", "invalid Host action authority", nil)
	}
	return nil
}

func validHostAuthorityIdentity(hostID, sessionDigest, handleDigest, nonce, workflowID, bundleID string, generation uint64, bundleDigest string, cursor execution.GraphCursor) bool {
	return validLocalID(hostID) && validDigest(sessionDigest) && validDigest(handleDigest) && validText(nonce, 512) && validWorkflowID(workflowID) &&
		bundleIDPattern.MatchString(bundleID) && generation > 0 && validDigest(bundleDigest) && execution.ValidateGraphCursor(cursor) == nil &&
		(cursor.Kind == execution.CursorBinding || cursor.Kind == execution.CursorHostAction)
}

func normalizeEvidence(values []host.EvidenceReference) ([]host.EvidenceReference, error) {
	result := append([]host.EvidenceReference{}, values...)
	if len(result) > 128 {
		return nil, admissionError("AUTHORITY_EVIDENCE_INVALID", "too many evidence references", nil)
	}
	sort.Slice(result, func(left, right int) bool { return evidenceKey(result[left]) < evidenceKey(result[right]) })
	for index, value := range result {
		if !validText(value.Kind, 128) || !validText(value.Reference, 2048) || !strings.HasPrefix(value.Reference, "evidence://") || !validDigest(value.Digest) ||
			index > 0 && evidenceIdentityKey(result[index-1]) == evidenceIdentityKey(value) {
			return nil, admissionError("AUTHORITY_EVIDENCE_INVALID", "invalid or duplicate evidence reference", nil)
		}
	}
	return result, nil
}

func evidenceKey(value host.EvidenceReference) string {
	return value.Kind + "\x00" + value.Reference + "\x00" + value.Digest
}

func evidenceIdentityKey(value host.EvidenceReference) string {
	return value.Kind + "\x00" + value.Reference
}

func normalizeSet(values []string, known func(string) bool) ([]string, error) {
	result := append([]string{}, values...)
	for _, value := range result {
		if !validText(value, 128) || !known(value) {
			return nil, admissionError("WORKFLOW_GRANT_INVALID", "unknown authority value", nil)
		}
	}
	sort.Strings(result)
	for index := 1; index < len(result); index++ {
		if result[index-1] == result[index] {
			return nil, admissionError("WORKFLOW_GRANT_INVALID", "duplicate authority value", nil)
		}
	}
	return result, nil
}

func validSortedSet(values []string, known func(string) bool) bool {
	if len(values) == 0 || len(values) > 128 {
		return false
	}
	for index, value := range values {
		if !validText(value, 128) || !known(value) || index > 0 && values[index-1] >= value {
			return false
		}
	}
	return true
}

func canonicalBindingRefs(values []string, wanted string) bool {
	if len(values) == 0 {
		return false
	}
	found := false
	for index, value := range values {
		if !validLocalID(value) || index > 0 && values[index-1] >= value {
			return false
		}
		if value == wanted {
			found = true
		}
	}
	return found
}

func canonicalTopologySet(values []execution.Topology) bool {
	normalized, err := execution.NormalizeTopologies(values)
	return err == nil && slices.Equal(normalized, values)
}

func subset(values, allowed []string) bool {
	for _, value := range values {
		if !contains(allowed, value) {
			return false
		}
	}
	return true
}

func contains(values []string, wanted string) bool { return slices.Contains(values, wanted) }

func requiresResourceLease(effects []string) bool {
	return contains(effects, "write-project") || contains(effects, "git-local")
}

func requiresNetworkAuthorization(effects []string) bool {
	return contains(effects, "network-write") || contains(effects, "network-mutation")
}

func validWorkflowID(value string) bool { return workflowIDPattern.MatchString(value) }

func validLocalID(value string) bool {
	_, err := catalog.ParseLocalID(value)
	return err == nil
}

func validQualifiedID(value string) bool {
	_, err := catalog.ParseQualifiedID(value)
	return err == nil
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func optionalDigest(value string) bool { return value == "" || validDigest(value) }

func validText(value string, maximum int) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum && strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}

func validBindingKind(value catalog.BindingKind) bool {
	return value == catalog.BindingSkill || value == catalog.BindingAgent || value == catalog.BindingRole || value == catalog.BindingInstruction || value == catalog.BindingTool
}

func validInvocation(value catalog.InvocationDisposition) bool {
	return value == catalog.InvocationHumanExplicit || value == catalog.InvocationModel || value == catalog.InvocationHost || value == catalog.InvocationInternal
}

func knownEffect(value string) bool {
	switch value {
	case "read-project", "write-project", "run-process", "git-local", "network-read", "network-write", "network-mutation":
		return true
	default:
		return false
	}
}

func knownResource(value string) bool {
	switch value {
	case "project", "project-worktree", "git-repository", "network", "credentials", "security", "data", "deployment":
		return true
	default:
		return false
	}
}
