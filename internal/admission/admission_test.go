package admission

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestGrantV3PinsProviderBindingAuthorityAndCursor(t *testing.T) {
	request := validProviderGrantRequest(t, catalog.InvocationModel, []string{"read-project", "run-process"})
	grant, err := IssueWorkflowGrant(request)
	if err != nil {
		t.Fatalf("IssueWorkflowGrant() error = %v", err)
	}
	wantTarget, err := NewProviderBindingAuthority(*request.ProviderBinding, *request.Capability)
	if err != nil {
		t.Fatal(err)
	}
	if grant.SchemaVersion != CapabilityGrantSchemaV3 || grant.Cursor != request.Cursor || grant.Target.TargetKind != GrantProviderBinding ||
		grant.Target.ProviderBinding == nil || grant.Target.HostAction != nil || !reflect.DeepEqual(*grant.Target.ProviderBinding, wantTarget) ||
		grant.AuthorizationDigest != "" || grant.InvocationAttestationDigest != "" || grant.ID == "" || grant.Digest == "" {
		t.Fatalf("Grant = %#v", grant)
	}
	if err := ValidateGrant(grant); err != nil {
		t.Fatalf("ValidateGrant() error = %v", err)
	}
	raw, err := json.Marshal(grant.Target)
	if err != nil {
		t.Fatal(err)
	}
	for _, retired := range []string{"node_id", "capability_id", `"binding":`} {
		if strings.Contains(string(raw), retired) {
			t.Fatalf("Grant target contains retired field %q: %s", retired, raw)
		}
	}
}

func TestGrantV3PinsHostActionAuthority(t *testing.T) {
	request := validHostActionGrantRequest(t)
	grant, err := IssueWorkflowGrant(request)
	if err != nil {
		t.Fatalf("IssueWorkflowGrant() error = %v", err)
	}
	if grant.Target.TargetKind != GrantHostAction || grant.Target.HostAction == nil || grant.Target.ProviderBinding != nil ||
		grant.Target.HostAction.ID != request.HostAction.ID || grant.Target.HostAction.OutputArtifact != request.HostAction.OutputArtifact ||
		grant.Target.HostAction.OutcomeSchema != request.HostAction.OutcomeSchema || grant.Cursor != request.Cursor {
		t.Fatalf("Grant = %#v", grant)
	}
	if err := ValidateGrant(grant); err != nil {
		t.Fatalf("ValidateGrant() error = %v", err)
	}
}

func TestGrantV3RejectsNonDispatchableAndInconsistentContracts(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*WorkflowGrantRequest)
		code   string
	}{
		{name: "cursor mismatch", mutate: func(value *WorkflowGrantRequest) { value.Cursor.Ordinal++ }, code: "WORKFLOW_GRANT_INVALID"},
		{name: "credited unit", mutate: func(value *WorkflowGrantRequest) { value.ProviderBinding.Disposition = profile.CreditInternalOnly }, code: "WORKFLOW_GRANT_INVALID"},
		{name: "omitted unit", mutate: func(value *WorkflowGrantRequest) { value.ProviderBinding.Disposition = profile.OmittedBySelection }, code: "WORKFLOW_GRANT_INVALID"},
		{name: "missing capability", mutate: func(value *WorkflowGrantRequest) { value.Capability = nil }, code: "WORKFLOW_GRANT_INVALID"},
		{name: "capability does not bind target", mutate: func(value *WorkflowGrantRequest) { value.Capability.BindingRefs = []string{"another-binding"} }, code: "WORKFLOW_GRANT_INVALID"},
		{name: "provider target plus Host target", mutate: func(value *WorkflowGrantRequest) { action := validHostAction(); value.HostAction = &action }, code: "WORKFLOW_GRANT_INVALID"},
		{name: "effect outside unit", mutate: func(value *WorkflowGrantRequest) { value.Effects = []string{"git-local"} }, code: "CAPABILITY_AUTHORITY_EXCEEDED"},
		{name: "resource outside ceiling", mutate: func(value *WorkflowGrantRequest) { value.Authority.Resources = []string{"project"} }, code: "CAPABILITY_AUTHORITY_EXCEEDED"},
		{name: "topology outside unit", mutate: func(value *WorkflowGrantRequest) { value.Topology = execution.TopologySubagent }, code: "CAPABILITY_TOPOLOGY_DENIED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := validProviderGrantRequest(t, catalog.InvocationModel, []string{"read-project"})
			test.mutate(&request)
			_, err := IssueWorkflowGrant(request)
			if ErrorCode(err) != test.code {
				t.Fatalf("IssueWorkflowGrant() error = %v", err)
			}
		})
	}
}

func TestAuthorizationV1RequiresExactAllowedNetworkAuthority(t *testing.T) {
	request := validProviderGrantRequest(t, catalog.InvocationModel, []string{"network-write", "read-project"})
	if _, err := IssueWorkflowGrant(request); ErrorCode(err) != "USER_AUTHORIZATION_REQUIRED" {
		t.Fatalf("IssueWorkflowGrant(no authorization) error = %v", err)
	}
	target, err := NewProviderBindingAuthority(*request.ProviderBinding, *request.Capability)
	if err != nil {
		t.Fatal(err)
	}
	authorization := validAuthorization(t, request, AuthorizationTarget{TargetKind: GrantProviderBinding, ProviderBinding: &target})
	request.Authorization = &authorization
	grant, err := IssueWorkflowGrant(request)
	if err != nil || grant.AuthorizationDigest != authorization.Digest {
		t.Fatalf("IssueWorkflowGrant(authorized) = %#v, %v", grant, err)
	}

	for _, mutate := range []func(*UserAuthorization){
		func(value *UserAuthorization) { value.Decision = AuthorizationDenied },
		func(value *UserAuthorization) { value.Cursor.Ordinal++ },
		func(value *UserAuthorization) { value.BundleGeneration++ },
		func(value *UserAuthorization) { value.HostSessionDigest = strings.Repeat("9", 64) },
		func(value *UserAuthorization) { value.Effects = []string{"read-project"} },
		func(value *UserAuthorization) { value.Target.ProviderBinding.Reference = "changed" },
	} {
		candidate := CloneUserAuthorization(authorization)
		mutate(&candidate)
		candidate.Digest = ""
		candidate.ID = ""
		candidate, err = NewUserAuthorization(candidate)
		if err != nil {
			// A malformed mutation is also correctly rejected before Grant issuance.
			continue
		}
		request.Authorization = &candidate
		if _, err := IssueWorkflowGrant(request); ErrorCode(err) != "USER_AUTHORIZATION_INVALID" {
			t.Fatalf("IssueWorkflowGrant(mismatched authorization) error = %v", err)
		}
	}
}

func TestExplicitInvocationAttestationV1IsRequiredAndExact(t *testing.T) {
	request := validProviderGrantRequest(t, catalog.InvocationHumanExplicit, []string{"read-project"})
	if _, err := IssueWorkflowGrant(request); ErrorCode(err) != "EXPLICIT_INVOCATION_REQUIRED" {
		t.Fatalf("IssueWorkflowGrant(no attestation) error = %v", err)
	}
	target, err := NewProviderBindingAuthority(*request.ProviderBinding, *request.Capability)
	if err != nil {
		t.Fatal(err)
	}
	attestation := validInvocationAttestation(t, request, target)
	request.InvocationAttestation = &attestation
	grant, err := IssueWorkflowGrant(request)
	if err != nil || grant.InvocationAttestationDigest != attestation.Digest {
		t.Fatalf("IssueWorkflowGrant(attested) = %#v, %v", grant, err)
	}

	changed := CloneExplicitInvocationAttestation(attestation)
	changed.Cursor.Ordinal++
	changed.ID = ""
	changed.Digest = ""
	changed, err = NewExplicitInvocationAttestation(changed)
	if err != nil {
		t.Fatal(err)
	}
	request.InvocationAttestation = &changed
	if _, err := IssueWorkflowGrant(request); ErrorCode(err) != "EXPLICIT_INVOCATION_INVALID" {
		t.Fatalf("IssueWorkflowGrant(mismatched attestation) error = %v", err)
	}
}

func TestAuthorizationAndExplicitInvocationRecordsOwnNestedStorage(t *testing.T) {
	request := validProviderGrantRequest(t, catalog.InvocationHumanExplicit, []string{"network-write", "read-project"})
	target, err := NewProviderBindingAuthority(*request.ProviderBinding, *request.Capability)
	if err != nil {
		t.Fatal(err)
	}
	authorization := validAuthorization(t, request, AuthorizationTarget{TargetKind: GrantProviderBinding, ProviderBinding: &target})
	attestation := validInvocationAttestation(t, request, target)
	authorization.Effects[0] = "changed"
	authorization.Evidence[0].Reference = "changed"
	attestation.Evidence[0].Reference = "changed"
	validAuthorizationRecord := validAuthorization(t, request, AuthorizationTarget{TargetKind: GrantProviderBinding, ProviderBinding: &target})
	validInvocationRecord := validInvocationAttestation(t, request, target)
	request.Authorization = &validAuthorizationRecord
	request.InvocationAttestation = &validInvocationRecord
	grant, err := IssueWorkflowGrant(request)
	if err != nil {
		t.Fatal(err)
	}
	cloned := CloneGrant(grant)
	cloned.Effects[0] = "changed"
	cloned.Target.ProviderBinding.Reference = "changed"
	if grant.Effects[0] == "changed" || grant.Target.ProviderBinding.Reference == "changed" {
		t.Fatal("CloneGrant aliases Grant storage")
	}
}

func TestOldAuthoritySchemasAreRejected(t *testing.T) {
	request := validProviderGrantRequest(t, catalog.InvocationModel, []string{"read-project"})
	grant, err := IssueWorkflowGrant(request)
	if err != nil {
		t.Fatal(err)
	}
	grant.SchemaVersion = "oaw.capability-grant/v2"
	if ErrorCode(ValidateGrant(grant)) != "CAPABILITY_GRANT_SCHEMA_UNSUPPORTED" {
		t.Fatalf("ValidateGrant(v2) was not rejected")
	}
	authorization := UserAuthorization{SchemaVersion: "oaw.user-authorization/v0"}
	if _, err := NewUserAuthorization(authorization); ErrorCode(err) != "USER_AUTHORIZATION_SCHEMA_UNSUPPORTED" {
		t.Fatalf("NewUserAuthorization(v0) error = %v", err)
	}
	attestation := ExplicitInvocationAttestation{SchemaVersion: "oaw.explicit-invocation-attestation/v0"}
	if _, err := NewExplicitInvocationAttestation(attestation); ErrorCode(err) != "EXPLICIT_INVOCATION_SCHEMA_UNSUPPORTED" {
		t.Fatalf("NewExplicitInvocationAttestation(v0) error = %v", err)
	}
}

func TestGrantV3ValidationRejectsContentAndTargetTampering(t *testing.T) {
	request := validProviderGrantRequest(t, catalog.InvocationModel, []string{"read-project"})
	grant, err := IssueWorkflowGrant(request)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*CapabilityGrant){
		"identity":       func(value *CapabilityGrant) { value.ID = "grant-bad" },
		"workflow":       func(value *CapabilityGrant) { value.WorkflowID = "bad" },
		"cursor":         func(value *CapabilityGrant) { value.Cursor.Ordinal = 0 },
		"target oneOf":   func(value *CapabilityGrant) { action := HostActionAuthority{}; value.Target.HostAction = &action },
		"target kind":    func(value *CapabilityGrant) { value.Target.TargetKind = GrantHostAction },
		"target content": func(value *CapabilityGrant) { value.Target.ProviderBinding.Reference = "changed" },
		"digest":         func(value *CapabilityGrant) { value.Digest = strings.Repeat("0", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := CloneGrant(grant)
			mutate(&candidate)
			if ErrorCode(ValidateGrant(candidate)) != "CAPABILITY_GRANT_INVALID" {
				t.Fatalf("ValidateGrant(%#v) did not reject", candidate)
			}
		})
	}
}

func TestAuthorityConstructorsNormalizeEvidenceAndRejectMalformedRecords(t *testing.T) {
	request := validProviderGrantRequest(t, catalog.InvocationModel, []string{"read-project"})
	target, err := NewProviderBindingAuthority(*request.ProviderBinding, *request.Capability)
	if err != nil {
		t.Fatal(err)
	}
	authorizationInput := UserAuthorization{
		SchemaVersion: UserAuthorizationSchemaV1, IssuerHostID: request.HostID, HostSessionDigest: request.HostSessionDigest,
		EvidenceHandleDigest: strings.Repeat("7", 64), AuthorizationNonce: "nonce", WorkflowID: request.WorkflowID,
		BundleID: request.BundleID, BundleGeneration: request.BundleGeneration, BundleDigest: request.BundleDigest, Cursor: request.Cursor,
		Target: AuthorizationTarget{TargetKind: GrantProviderBinding, ProviderBinding: &target}, Decision: AuthorizationAllowed,
		Effects: []string{"read-project"}, Resources: []string{"project-worktree"}, Evidence: []host.EvidenceReference{
			{Kind: "z", Reference: "evidence://z", Digest: strings.Repeat("8", 64)},
			{Kind: "a", Reference: "evidence://a", Digest: strings.Repeat("9", 64)},
		},
	}
	authorization, err := NewUserAuthorization(authorizationInput)
	if err != nil || authorization.Evidence[0].Kind != "a" || ValidateUserAuthorization(authorization) != nil {
		t.Fatalf("NewUserAuthorization() = %#v, %v", authorization, err)
	}
	tampered := CloneUserAuthorization(authorization)
	tampered.Digest = strings.Repeat("0", 64)
	if err := ValidateUserAuthorization(tampered); ErrorCode(err) != "USER_AUTHORIZATION_INVALID" {
		t.Fatalf("ValidateUserAuthorization(tampered) error = %v", err)
	}

	action := validHostAction()
	action.Cursor.Kind = execution.CursorBinding
	if _, err := NewHostActionAuthority(action); ErrorCode(err) != "WORKFLOW_GRANT_INVALID" {
		t.Fatalf("NewHostActionAuthority(malformed) error = %v", err)
	}
	invalidTarget := authorizationInput
	invalidTarget.Target.TargetKind = "invented"
	if _, err := NewUserAuthorization(invalidTarget); ErrorCode(err) != "USER_AUTHORIZATION_INVALID" {
		t.Fatalf("NewUserAuthorization(invalid target) error = %v", err)
	}
	hostAction, err := NewHostActionAuthority(validHostAction())
	if err != nil {
		t.Fatal(err)
	}
	mismatchedCursor := authorizationInput
	mismatchedCursor.Target = AuthorizationTarget{TargetKind: GrantHostAction, HostAction: &hostAction}
	if _, err := NewUserAuthorization(mismatchedCursor); ErrorCode(err) != "USER_AUTHORIZATION_INVALID" {
		t.Fatalf("NewUserAuthorization(cursor/target mismatch) error = %v", err)
	}
	duplicateEvidence := authorizationInput
	duplicateEvidence.Evidence = append(duplicateEvidence.Evidence, duplicateEvidence.Evidence[0])
	if _, err := NewUserAuthorization(duplicateEvidence); ErrorCode(err) != "USER_AUTHORIZATION_INVALID" {
		t.Fatalf("NewUserAuthorization(duplicate evidence) error = %v", err)
	}
	rewrittenEvidence := authorizationInput
	rewritten := rewrittenEvidence.Evidence[0]
	rewritten.Digest = strings.Repeat("0", 64)
	rewrittenEvidence.Evidence = append(rewrittenEvidence.Evidence, rewritten)
	if _, err := NewUserAuthorization(rewrittenEvidence); ErrorCode(err) != "USER_AUTHORIZATION_INVALID" {
		t.Fatalf("NewUserAuthorization(rewritten evidence digest) error = %v", err)
	}

	clonedCeiling := CloneAuthority(request.Authority)
	clonedCeiling.Effects[0] = "changed"
	if request.Authority.Effects[0] == "changed" {
		t.Fatal("CloneAuthority aliases caller storage")
	}
}

func validProviderGrantRequest(t *testing.T, invocation catalog.InvocationDisposition, effects []string) WorkflowGrantRequest {
	t.Helper()
	unit := validResolvedBinding(invocation)
	capability := catalog.CapabilityRecord{
		ID: "workflow", InputSchema: "oaw.workflow-input/v1", OutcomeSchema: "oaw.workflow-output/v1",
		RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, BindingRefs: []string{unit.BindingID},
	}
	return WorkflowGrantRequest{
		WorkflowID: "workflow-0123456789abcdef0123456789abcdef", RequestID: "request-1",
		BundleID: "bundle-0123456789abcdef0123456789abcdef", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64),
		Cursor: unit.Cursor, ProviderBinding: &unit, Capability: &capability, Topology: execution.TopologyCurrent,
		HostID: "codex", HostSessionDigest: strings.Repeat("b", 64), Effects: effects, Resources: []string{"project-worktree"},
		TerminationCondition: "complete", Authority: AuthorityCeiling{Effects: []string{"network-write", "read-project", "run-process", "write-project"}, Resources: []string{"project-worktree"}, ResourceLeases: true},
	}
}

func validHostActionGrantRequest(t *testing.T) WorkflowGrantRequest {
	t.Helper()
	action := validHostAction()
	request := WorkflowGrantRequest{
		WorkflowID: "workflow-0123456789abcdef0123456789abcdef", RequestID: "request-1",
		BundleID: "bundle-0123456789abcdef0123456789abcdef", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64),
		Cursor: action.Cursor, HostAction: &action, Topology: execution.TopologyCurrent, HostID: "codex", HostSessionDigest: strings.Repeat("b", 64),
		Effects: []string{"read-project", "run-process"}, Resources: []string{"project-worktree"}, TerminationCondition: "complete",
		Authority: AuthorityCeiling{Effects: []string{"read-project", "run-process"}, Resources: []string{"project-worktree"}},
	}
	return request
}

func validResolvedBinding(invocation catalog.InvocationDisposition) profile.ResolvedBinding {
	return profile.ResolvedBinding{
		Cursor: execution.GraphCursor{SlotID: "implementation", Kind: execution.CursorBinding, UnitID: "implementation-main", Ordinal: 1},
		UnitID: "implementation-main", StepID: "main", AnchorSlotID: catalog.SlotImplementation, SlotIDs: []catalog.SlotID{catalog.SlotImplementation},
		ProviderID: "acme/provider", ProviderInstanceDigest: strings.Repeat("1", 64), BindingID: "binding",
		DistributionID: "distribution", DistributionRevision: strings.Repeat("2", 40), DistributionTreeDigest: "sha256:" + strings.Repeat("3", 64),
		Surface: "codex-skills", Kind: catalog.BindingSkill, Reference: "acme:implementation", Invocation: invocation,
		BindingTreeDigest: "sha256:" + strings.Repeat("4", 64), InputArtifact: "workflow-input", OutputArtifact: "workflow-output",
		Responsibilities: []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipStage, Name: "implementation", SlotID: catalog.SlotImplementation, OutcomeOwner: true}},
		MaximumEffects:   []string{"network-write", "read-project", "run-process", "write-project"}, Resources: []string{"project-worktree"},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Disposition: profile.DispatchByCoordinator,
		RequiresExplicitInvocation: invocation == catalog.InvocationHumanExplicit, BindingEvidenceDigest: strings.Repeat("5", 64),
	}
}

func validHostAction() profile.CompiledHostAction {
	return profile.CompiledHostAction{
		Cursor: execution.GraphCursor{SlotID: "workspace-preparation", Kind: execution.CursorHostAction, UnitID: "workspace.prepare-or-confirm", Ordinal: 1},
		ID:     "workspace.prepare-or-confirm", InputArtifact: "workspace-input", OutputArtifact: "workspace-output",
		InputSchema: "oaw.host-action.workspace-input/v1", OutcomeSchema: "oaw.host-action.workspace-outcome/v1",
		MaximumEffects: []string{"read-project", "run-process", "write-project"}, Resources: []string{"project-worktree"}, ObservationDigest: strings.Repeat("6", 64),
	}
}

func validAuthorization(t *testing.T, request WorkflowGrantRequest, target AuthorizationTarget) UserAuthorization {
	t.Helper()
	value, err := NewUserAuthorization(UserAuthorization{
		SchemaVersion: UserAuthorizationSchemaV1, IssuerHostID: request.HostID, HostSessionDigest: request.HostSessionDigest,
		EvidenceHandleDigest: strings.Repeat("7", 64), AuthorizationNonce: "authorization-nonce-1", WorkflowID: request.WorkflowID,
		BundleID: request.BundleID, BundleGeneration: request.BundleGeneration, BundleDigest: request.BundleDigest, Cursor: request.Cursor,
		Target: target, Decision: AuthorizationAllowed, Effects: append([]string{}, request.Effects...), Resources: append([]string{}, request.Resources...),
		Evidence: []host.EvidenceReference{{Kind: "user-approval", Reference: "evidence://host/authorization/1", Digest: strings.Repeat("8", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func validInvocationAttestation(t *testing.T, request WorkflowGrantRequest, target ProviderBindingAuthority) ExplicitInvocationAttestation {
	t.Helper()
	value, err := NewExplicitInvocationAttestation(ExplicitInvocationAttestation{
		SchemaVersion: ExplicitInvocationAttestationSchemaV1, IssuerHostID: request.HostID, HostSessionDigest: request.HostSessionDigest,
		EvidenceHandleDigest: strings.Repeat("7", 64), InvocationNonce: "invocation-nonce-1", WorkflowID: request.WorkflowID,
		BundleID: request.BundleID, BundleGeneration: request.BundleGeneration, BundleDigest: request.BundleDigest, Cursor: request.Cursor,
		ProviderBinding: target, Evidence: []host.EvidenceReference{{Kind: "explicit-invocation", Reference: "evidence://host/invocation/1", Digest: strings.Repeat("9", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}
