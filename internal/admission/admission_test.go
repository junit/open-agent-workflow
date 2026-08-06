package admission

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestIssueWorkflowGrantPinsTopologyAndNarrowedAuthority(t *testing.T) {
	request := validWorkflowGrantRequest()
	grant, err := IssueWorkflowGrant(request)
	if err != nil {
		t.Fatalf("IssueWorkflowGrant() error = %v", err)
	}
	if grant.SchemaVersion != CapabilityGrantSchemaV2 || grant.WorkflowID != request.WorkflowID || grant.RequestID != request.RequestID ||
		grant.BundleID != request.BundleID || grant.BundleGeneration != request.BundleGeneration || grant.BundleDigest != request.BundleDigest ||
		grant.NodeID != request.Node.ID || grant.Topology != request.Topology || grant.HostSessionDigest != request.HostSessionDigest ||
		grant.ProviderID != request.Node.ProviderID || grant.ProviderInstanceDigest != request.Node.ProviderInstanceDigest ||
		grant.CapabilityID != request.Node.CapabilityID || !reflect.DeepEqual(grant.Binding, request.Node.Binding) ||
		!reflect.DeepEqual(grant.Effects, []string{"read-project", "run-process"}) || !reflect.DeepEqual(grant.Resources, []string{"project-worktree"}) ||
		grant.TerminationCondition != request.TerminationCondition || grant.ID == "" || grant.Digest == "" {
		t.Fatalf("Grant = %#v", grant)
	}
	if err := ValidateGrant(grant); err != nil {
		t.Fatalf("ValidateGrant() error = %v", err)
	}
	raw, err := json.Marshal(grant)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"executor", "invocation_id", "run_id", "command"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("Grant contains retired field %q: %s", forbidden, raw)
		}
	}
}

func TestGrantRejectsTopologyAndSessionMismatch(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*WorkflowGrantRequest)
		code   string
	}{
		{name: "invalid Host session digest", mutate: func(value *WorkflowGrantRequest) { value.HostSessionDigest = "not-a-digest" }, code: "WORKFLOW_GRANT_INVALID"},
		{name: "effect outside node", mutate: func(value *WorkflowGrantRequest) { value.Effects = []string{"network-read"} }, code: "CAPABILITY_AUTHORITY_EXCEEDED"},
		{name: "effect outside ceiling", mutate: func(value *WorkflowGrantRequest) { value.Authority.Effects = []string{"read-project"} }, code: "CAPABILITY_AUTHORITY_EXCEEDED"},
		{name: "resource outside node", mutate: func(value *WorkflowGrantRequest) { value.Resources = []string{"git-repository"} }, code: "CAPABILITY_AUTHORITY_EXCEEDED"},
		{name: "resource outside ceiling", mutate: func(value *WorkflowGrantRequest) { value.Authority.Resources = []string{"project"} }, code: "CAPABILITY_AUTHORITY_EXCEEDED"},
		{name: "topology outside node", mutate: func(value *WorkflowGrantRequest) { value.Topology = execution.TopologySubagent }, code: "CAPABILITY_TOPOLOGY_DENIED"},
		{name: "write without leases", mutate: func(value *WorkflowGrantRequest) {
			value.Effects = []string{"write-project"}
			value.Authority.ResourceLeases = false
		}, code: "RESOURCE_LEASE_REQUIRED"},
		{name: "write without worktree", mutate: func(value *WorkflowGrantRequest) {
			value.Effects = []string{"write-project"}
			value.Resources = []string{"project"}
			value.Node.Resources = []string{"project", "project-worktree"}
			value.Authority.Resources = []string{"project", "project-worktree"}
		}, code: "RESOURCE_LEASE_REQUIRED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := validWorkflowGrantRequest()
			test.mutate(&request)
			_, err := IssueWorkflowGrant(request)
			if ErrorCode(err) != test.code {
				t.Fatalf("IssueWorkflowGrant() error = %v", err)
			}
		})
	}
}

func TestCapabilityGrantV2IdentityAndValidationAreContentBound(t *testing.T) {
	request := validWorkflowGrantRequest()
	first, err := IssueWorkflowGrant(request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := IssueWorkflowGrant(request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same request produced different Grants: %#v, %#v", first, second)
	}
	changed := request
	changed.TerminationCondition = "different"
	third, err := IssueWorkflowGrant(changed)
	if err != nil {
		t.Fatal(err)
	}
	if third.ID == first.ID || third.Digest == first.Digest {
		t.Fatalf("changed request reused identity: %#v, %#v", first, third)
	}

	for _, mutate := range []func(*CapabilityGrant){
		func(value *CapabilityGrant) { value.SchemaVersion = "oaw.capability-grant/v1" },
		func(value *CapabilityGrant) { value.ID = "grant-bad" },
		func(value *CapabilityGrant) { value.WorkflowID = "bad workflow" },
		func(value *CapabilityGrant) { value.BundleGeneration = 0 },
		func(value *CapabilityGrant) { value.BundleDigest = "bad" },
		func(value *CapabilityGrant) { value.Topology = execution.TopologySubagent },
		func(value *CapabilityGrant) { value.ProviderID = "bad" },
		func(value *CapabilityGrant) { value.Binding.Reference = "changed" },
		func(value *CapabilityGrant) { value.Effects = nil },
		func(value *CapabilityGrant) { value.Digest = strings.Repeat("0", 64) },
	} {
		candidate := CloneGrant(first)
		mutate(&candidate)
		if ErrorCode(ValidateGrant(candidate)) != "CAPABILITY_GRANT_INVALID" {
			t.Fatalf("ValidateGrant(%#v) did not reject", candidate)
		}
	}
}

func TestIssueWorkflowGrantCopiesAndValidatesHostBinding(t *testing.T) {
	request := validWorkflowGrantRequest()
	grant, err := IssueWorkflowGrant(request)
	if err != nil {
		t.Fatal(err)
	}
	request.Node.Binding.Topologies[0] = execution.TopologySubagent
	if grant.Binding.Topologies[0] != execution.TopologyCurrent {
		t.Fatalf("Grant Binding aliases request: %#v", grant.Binding)
	}

	invalid := validWorkflowGrantRequest()
	invalid.Node.Binding.Topologies = []execution.Topology{execution.TopologyCurrent, execution.TopologyCurrent}
	if ErrorCode(issueWorkflowGrantError(invalid)) != "WORKFLOW_GRANT_INVALID" {
		t.Fatalf("duplicate Binding topologies were accepted: %#v", invalid.Node.Binding)
	}
	invalid = validWorkflowGrantRequest()
	invalid.Node.Binding.Kind = ""
	if ErrorCode(issueWorkflowGrantError(invalid)) != "WORKFLOW_GRANT_INVALID" {
		t.Fatalf("empty Binding kind was accepted: %#v", invalid.Node.Binding)
	}
}

func issueWorkflowGrantError(request WorkflowGrantRequest) error {
	_, err := IssueWorkflowGrant(request)
	return err
}

func TestAdmissionErrorAndAuthorityCopies(t *testing.T) {
	cause := errors.New("cause")
	err := admissionError("CODE", "detail", cause)
	if ErrorCode(err) != "CODE" || !errors.Is(err, cause) || err.Error() != "CODE: detail" {
		t.Fatalf("error contract = %v", err)
	}
	authority := AuthorityCeiling{Effects: []string{"read-project"}, Resources: []string{"project"}}
	cloned := CloneAuthority(authority)
	cloned.Effects[0], cloned.Resources[0] = "changed", "changed"
	if authority.Effects[0] != "read-project" || authority.Resources[0] != "project" {
		t.Fatalf("CloneAuthority() aliased input: %#v", authority)
	}
}

func validWorkflowGrantRequest() WorkflowGrantRequest {
	binding := catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "superpowers:executing-plans", Topologies: []execution.Topology{execution.TopologyCurrent}}
	node := profile.GraphNode{
		ID: "implementation", Kind: catalog.PhaseNode, Responsibility: "implementation", Phase: "implementation",
		ProviderID: "oaw/superpowers", ProviderInstanceDigest: strings.Repeat("b", 64), CapabilityID: "implementation", Binding: binding,
		InputSchema: "oaw.capability-input/v1", OutcomeSchema: "oaw.capability-outcome/v1",
		MaximumEffects: []string{"read-project", "run-process", "write-project"}, Resources: []string{"project-worktree"},
		RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		DelegationAllowList: []string{}, Transitions: []profile.GraphTransition{},
	}
	return WorkflowGrantRequest{
		WorkflowID: "workflow-0123456789abcdef0123456789abcdef", RequestID: "request-1", BundleID: "bundle-0123456789abcdef0123456789abcdef",
		BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64), Topology: execution.TopologyCurrent,
		HostSessionDigest: strings.Repeat("c", 64), Node: node, Effects: []string{"run-process", "read-project"},
		Resources: []string{"project-worktree"}, TerminationCondition: "complete implementation",
		Authority: AuthorityCeiling{Effects: []string{"read-project", "run-process", "write-project"}, Resources: []string{"project-worktree"}, ResourceLeases: true},
	}
}
