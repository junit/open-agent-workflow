package admission

import (
	"errors"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestIssueBoundedGrantPinsVerifiedNarrowedAuthority(t *testing.T) {
	fixture := admissionFixture(t)
	request := fixture.request("review")
	request.Effects = []string{"run-process", "read-project"}
	request.Resources = []string{"project"}
	grant, err := IssueBoundedGrant(request)
	if err != nil {
		t.Fatalf("IssueBoundedGrant() error = %v", err)
	}
	if grant.SchemaVersion != CapabilityGrantSchemaV1 || grant.ID == "" || grant.InvocationID == "" || grant.Digest == "" {
		t.Fatalf("grant identity = %#v", grant)
	}
	if grant.RunID != request.RunID || grant.RequestID != request.RequestID || grant.DeliverableID != request.DeliverableID || grant.IssuedRevision != 2 || grant.Generation != 0 {
		t.Fatalf("grant run identity = %#v", grant)
	}
	if grant.ProviderID != "acme/suite" || grant.CapabilityID != "review" || grant.ProviderInstanceDigest != strings.Repeat("e", 64) || grant.DescriptorDigest == "" {
		t.Fatalf("grant capability identity = %#v", grant)
	}
	if grant.Binding.Reference != "acme:review" || grant.Executor.ID != "executor-review" || grant.Executor.Kind != ExecutorIsolated {
		t.Fatalf("grant dispatch identity = %#v", grant)
	}
	if !equalStrings(grant.Effects, []string{"read-project", "run-process"}) || !equalStrings(grant.Resources, []string{"project"}) || len(grant.DelegationAllowList) != 0 {
		t.Fatalf("grant authority = %#v", grant)
	}
	if err := ValidateGrant(grant); err != nil {
		t.Fatalf("ValidateGrant() error = %v", err)
	}

	request.Effects[0] = "changed"
	request.Resources[0] = "changed"
	grant.Effects[0] = "changed"
	grant.Resources[0] = "changed"
	fresh, err := IssueBoundedGrant(fixture.request("review"))
	if err != nil || fresh.Effects[0] == "changed" || fresh.Resources[0] == "changed" {
		t.Fatalf("grant or request aliases mutable state: %#v, %v", fresh, err)
	}
}

func TestIssueBoundedGrantFailsClosedAtEveryAuthorityBoundary(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*GrantRequest, *admissionTestFixture)
		code   string
	}{
		{"invalid request", func(value *GrantRequest, _ *admissionTestFixture) { value.InputDigest = "bad" }, "BOUNDED_REQUEST_INVALID"},
		{"missing provider", func(value *GrantRequest, _ *admissionTestFixture) { value.Selector.ProviderID = "missing/provider" }, "CAPABILITY_NOT_VERIFIED"},
		{"missing capability", func(value *GrantRequest, _ *admissionTestFixture) { value.Selector.CapabilityID = "missing" }, "CAPABILITY_NOT_VERIFIED"},
		{"descriptor mismatch", func(_ *GrantRequest, fixture *admissionTestFixture) {
			fixture.registry.provider.DescriptorDigest = strings.Repeat("0", 64)
		}, "CAPABILITY_NOT_VERIFIED"},
		{"binding mismatch", func(_ *GrantRequest, fixture *admissionTestFixture) {
			fixture.registry.capability.Binding.Reference = "wrong"
		}, "CAPABILITY_NOT_VERIFIED"},
		{"mode denied", func(_ *GrantRequest, fixture *admissionTestFixture) {
			fixture.catalog.provider.Capabilities[0].RequestModes = []catalog.RequestMode{catalog.RequestModeWorkflow}
		}, "CAPABILITY_MODE_NOT_ALLOWED"},
		{"unknown effect", func(value *GrantRequest, _ *admissionTestFixture) { value.Effects = []string{"unknown"} }, "CAPABILITY_EFFECT_NOT_ALLOWED"},
		{"capability effect", func(value *GrantRequest, _ *admissionTestFixture) { value.Effects = []string{"network-read"} }, "CAPABILITY_EFFECT_NOT_ALLOWED"},
		{"authority effect", func(value *GrantRequest, fixture *admissionTestFixture) {
			value.Effects = []string{"run-process"}
			fixture.authority.Effects = []string{"read-project"}
		}, "CAPABILITY_AUTHORITY_EXCEEDED"},
		{"write lease", func(value *GrantRequest, fixture *admissionTestFixture) {
			value.Effects = []string{"write-project"}
			value.Resources = []string{"project-worktree"}
			fixture.authority.Effects = append(fixture.authority.Effects, "write-project")
			fixture.authority.Resources = append(fixture.authority.Resources, "project-worktree")
		}, "RESOURCE_LEASE_REQUIRED"},
		{"unknown resource", func(value *GrantRequest, _ *admissionTestFixture) { value.Resources = []string{"unknown"} }, "CAPABILITY_RESOURCE_NOT_ALLOWED"},
		{"capability resource", func(value *GrantRequest, _ *admissionTestFixture) { value.Resources = []string{"git-repository"} }, "CAPABILITY_RESOURCE_NOT_ALLOWED"},
		{"authority resource", func(value *GrantRequest, fixture *admissionTestFixture) {
			fixture.authority.Resources = []string{}
			value.Resources = []string{"project"}
		}, "CAPABILITY_AUTHORITY_EXCEEDED"},
		{"executor missing", func(value *GrantRequest, _ *admissionTestFixture) { value.Executor.ID = "missing" }, "EXECUTOR_NOT_REGISTERED"},
		{"main agent denied", func(value *GrantRequest, fixture *admissionTestFixture) {
			value.Executor = ExecutorRegistration{ID: "main", Kind: ExecutorMainAgent}
			fixture.executors = append(fixture.executors, value.Executor)
		}, "EXECUTOR_TOPOLOGY_DENIED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := admissionFixture(t)
			request := fixture.request("review")
			test.mutate(&request, &fixture)
			if test.name != "descriptor mismatch" {
				fixture.pinDescriptor(t)
			}
			request.Catalog = fixture.catalog
			request.Registry = fixture.registry
			request.Authority = fixture.authority
			request.Executors = fixture.executors
			_, err := IssueBoundedGrant(request)
			assertAdmissionCode(t, err, test.code)
		})
	}
}

func TestIssueBoundedGrantAllowsRegisteredMainAgentOnlyWhenDeclared(t *testing.T) {
	fixture := admissionFixture(t)
	fixture.catalog.provider.Capabilities[0].ExecutorTopology = catalog.MainAgentAllowed
	fixture.pinDescriptor(t)
	main := ExecutorRegistration{ID: "main-agent", Kind: ExecutorMainAgent}
	fixture.executors = append(fixture.executors, main)
	request := fixture.request("review")
	request.Executor = main
	request.Executors = fixture.executors
	grant, err := IssueBoundedGrant(request)
	if err != nil {
		t.Fatalf("IssueBoundedGrant(main agent) error = %v", err)
	}
	if grant.Executor != main {
		t.Fatalf("grant executor = %#v", grant.Executor)
	}
}

func TestIssueBoundedGrantRejectsMalformedAuthorityInputs(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*GrantRequest, *admissionTestFixture)
		code   string
	}{
		{"invalid provider selector", func(value *GrantRequest, _ *admissionTestFixture) { value.Selector.ProviderID = "invalid" }, "BOUNDED_REQUEST_INVALID"},
		{"invalid capability selector", func(value *GrantRequest, _ *admissionTestFixture) { value.Selector.CapabilityID = "Invalid Capability" }, "BOUNDED_REQUEST_INVALID"},
		{"invalid selector source", func(value *GrantRequest, _ *admissionTestFixture) { value.Selector.Source = "host-claim" }, "BOUNDED_REQUEST_INVALID"},
		{"empty effects", func(value *GrantRequest, _ *admissionTestFixture) { value.Effects = nil }, "BOUNDED_REQUEST_INVALID"},
		{"duplicate effects", func(value *GrantRequest, _ *admissionTestFixture) {
			value.Effects = []string{"read-project", "read-project"}
		}, "BOUNDED_REQUEST_INVALID"},
		{"empty resources", func(value *GrantRequest, _ *admissionTestFixture) { value.Resources = nil }, "BOUNDED_REQUEST_INVALID"},
		{"duplicate resources", func(value *GrantRequest, _ *admissionTestFixture) { value.Resources = []string{"project", "project"} }, "BOUNDED_REQUEST_INVALID"},
		{"duplicate delegation", func(value *GrantRequest, _ *admissionTestFixture) {
			value.DelegationAllowList = []string{"review", "review"}
		}, "BOUNDED_REQUEST_INVALID"},
		{"duplicate authority effects", func(_ *GrantRequest, fixture *admissionTestFixture) {
			fixture.authority.Effects = []string{"read-project", "read-project"}
		}, "BOUNDED_REQUEST_INVALID"},
		{"duplicate authority resources", func(_ *GrantRequest, fixture *admissionTestFixture) {
			fixture.authority.Resources = []string{"project", "project"}
		}, "BOUNDED_REQUEST_INVALID"},
		{"invalid executor registration", func(_ *GrantRequest, fixture *admissionTestFixture) {
			fixture.executors = append(fixture.executors, ExecutorRegistration{ID: "bad", Kind: "unknown"})
		}, "BOUNDED_REQUEST_INVALID"},
		{"duplicate executor registration", func(_ *GrantRequest, fixture *admissionTestFixture) {
			fixture.executors = append(fixture.executors, fixture.executors[0])
		}, "BOUNDED_REQUEST_INVALID"},
		{"executor registration mismatch", func(value *GrantRequest, fixture *admissionTestFixture) {
			value.Executor.Kind = ExecutorMainAgent
			fixture.executors[0].ID = value.Executor.ID
		}, "EXECUTOR_NOT_REGISTERED"},
		{"delegation denied by authority", func(value *GrantRequest, fixture *admissionTestFixture) {
			value.DelegationAllowList = []string{"security-review"}
			fixture.catalog.provider.Capabilities[0].DelegationAllowList = []string{"security-review"}
			fixture.authority.AllowDelegation = false
		}, "CAPABILITY_AUTHORITY_EXCEEDED"},
		{"delegation denied by capability", func(value *GrantRequest, _ *admissionTestFixture) {
			value.DelegationAllowList = []string{"security-review"}
		}, "CAPABILITY_AUTHORITY_EXCEEDED"},
		{"git completion forbidden", func(value *GrantRequest, fixture *admissionTestFixture) {
			value.Effects = []string{"git-local"}
			fixture.catalog.provider.Capabilities[0].MaximumEffects = append(fixture.catalog.provider.Capabilities[0].MaximumEffects, "git-local")
			fixture.authority.Effects = append(fixture.authority.Effects, "git-local")
		}, "CAPABILITY_EFFECT_NOT_ALLOWED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := admissionFixture(t)
			request := fixture.request("review")
			test.mutate(&request, &fixture)
			fixture.pinDescriptor(t)
			request.Catalog = fixture.catalog
			request.Registry = fixture.registry
			request.Authority = fixture.authority
			request.Executors = fixture.executors
			_, err := IssueBoundedGrant(request)
			assertAdmissionCode(t, err, test.code)
		})
	}
}

func TestCapabilityGrantIdentityIsDeterministicAndContentBound(t *testing.T) {
	fixture := admissionFixture(t)
	first, err := IssueBoundedGrant(fixture.request("review"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := IssueBoundedGrant(fixture.request("review"))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.InvocationID != second.InvocationID || first.Digest != second.Digest {
		t.Fatalf("same request produced different identities: %#v, %#v", first, second)
	}
	changedRequest := fixture.request("review")
	changedRequest.InputDigest = strings.Repeat("2", 64)
	changed, err := IssueBoundedGrant(changedRequest)
	if err != nil {
		t.Fatal(err)
	}
	if changed.ID == first.ID || changed.InvocationID == first.InvocationID || changed.Digest == first.Digest {
		t.Fatalf("changed input reused Grant identity: %#v, %#v", first, changed)
	}
}

func TestValidateGrantRejectsMalformedOrTamperedContent(t *testing.T) {
	fixture := admissionFixture(t)
	grant, err := IssueBoundedGrant(fixture.request("review"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*CapabilityGrant)
	}{
		{"schema", func(value *CapabilityGrant) { value.SchemaVersion = "other" }},
		{"grant ID", func(value *CapabilityGrant) { value.ID = "grant-bad" }},
		{"invocation ID", func(value *CapabilityGrant) { value.InvocationID = "invocation-bad" }},
		{"run ID", func(value *CapabilityGrant) { value.RunID = "run-bad" }},
		{"request ID", func(value *CapabilityGrant) { value.RequestID = "bad\nrequest" }},
		{"input digest", func(value *CapabilityGrant) { value.InputDigest = "bad" }},
		{"issued revision", func(value *CapabilityGrant) { value.IssuedRevision = 0 }},
		{"generation", func(value *CapabilityGrant) { value.Generation = 1 }},
		{"provider ID", func(value *CapabilityGrant) { value.ProviderID = "bad" }},
		{"provider digest", func(value *CapabilityGrant) { value.ProviderInstanceDigest = strings.Repeat("G", 64) }},
		{"capability ID", func(value *CapabilityGrant) { value.CapabilityID = "Bad Capability" }},
		{"executor", func(value *CapabilityGrant) { value.Executor.Kind = "unknown" }},
		{"binding", func(value *CapabilityGrant) { value.Binding.Host = "" }},
		{"termination", func(value *CapabilityGrant) { value.TerminationCondition = "" }},
		{"empty effects", func(value *CapabilityGrant) { value.Effects = nil }},
		{"unsorted effects", func(value *CapabilityGrant) { value.Effects = []string{"run-process", "read-project"} }},
		{"invalid resource", func(value *CapabilityGrant) { value.Resources = []string{"unknown"} }},
		{"nil delegation", func(value *CapabilityGrant) { value.DelegationAllowList = nil }},
		{"invalid delegation", func(value *CapabilityGrant) { value.DelegationAllowList = []string{"Bad Capability"} }},
		{"invalid digest", func(value *CapabilityGrant) { value.Digest = "bad" }},
		{"content digest mismatch", func(value *CapabilityGrant) { value.TerminationCondition = "changed" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := CloneGrant(grant)
			test.mutate(&candidate)
			assertAdmissionCode(t, ValidateGrant(candidate), "PARENT_GRANT_INVALID")
		})
	}
}

func TestAdmissionErrorsAndAuthorityCopiesPreserveBoundaries(t *testing.T) {
	cause := errors.New("cause")
	err := admissionError("CODE", "detail", cause)
	if err.Error() != "CODE: detail" || !errors.Is(err, cause) || ErrorCode(err) != "CODE" {
		t.Fatalf("error contract = %v", err)
	}
	if (&Error{Code: "CODE"}).Error() != "CODE" || ErrorCode(errors.New("other")) != "" {
		t.Fatal("error fallback contract changed")
	}

	original := AuthorityCeiling{Effects: []string{"read-project"}, Resources: []string{"project"}}
	cloned := CloneAuthority(original)
	cloned.Effects[0] = "changed"
	cloned.Resources[0] = "changed"
	if original.Effects[0] != "read-project" || original.Resources[0] != "project" {
		t.Fatalf("CloneAuthority() aliased input: %#v", original)
	}
}

func TestDeriveChildGrantRequiresCapabilityAndStrictNarrowing(t *testing.T) {
	fixture := admissionFixture(t)
	fixture.catalog.provider.Capabilities[0].DelegationAllowList = []string{"security-review"}
	fixture.pinDescriptor(t)
	parentRequest := fixture.request("review")
	parentRequest.DelegationAllowList = []string{"security-review"}
	parent, err := IssueBoundedGrant(parentRequest)
	if err != nil {
		t.Fatalf("IssueBoundedGrant(parent) error = %v", err)
	}
	childRequest := fixture.childRequest(parent)
	child, err := DeriveChildGrant(childRequest)
	if err != nil {
		t.Fatalf("DeriveChildGrant() error = %v", err)
	}
	if child.ParentGrantID != parent.ID || child.CapabilityID != "security-review" || !equalStrings(child.Effects, []string{"read-project"}) || !equalStrings(child.Resources, []string{"project"}) {
		t.Fatalf("child Grant = %#v", child)
	}

	for _, test := range []struct {
		name   string
		mutate func(*ChildGrantRequest)
		code   string
	}{
		{"tampered parent", func(value *ChildGrantRequest) { value.Parent.Digest = strings.Repeat("0", 64) }, "PARENT_GRANT_INVALID"},
		{"capability not delegated", func(value *ChildGrantRequest) { value.Request.Selector.CapabilityID = "other" }, "CHILD_GRANT_NOT_ALLOWED"},
		{"effect widened", func(value *ChildGrantRequest) { value.Request.Effects = []string{"run-process"} }, "CHILD_GRANT_NOT_ALLOWED"},
		{"resource widened", func(value *ChildGrantRequest) { value.Request.Resources = []string{"project-worktree"} }, "CHILD_GRANT_NOT_ALLOWED"},
		{"onward delegation widened", func(value *ChildGrantRequest) { value.Request.DelegationAllowList = []string{"other"} }, "CHILD_GRANT_NOT_ALLOWED"},
		{"run changed", func(value *ChildGrantRequest) { value.Request.RunID = "run-ffffffffffffffffffffffffffffffff" }, "CHILD_GRANT_NOT_ALLOWED"},
		{"revision not advanced", func(value *ChildGrantRequest) { value.Request.IssuedRevision = value.Parent.IssuedRevision }, "CHILD_GRANT_NOT_ALLOWED"},
		{"child capability unverified", func(value *ChildGrantRequest) { value.Request.Registry = fixture.registry }, "CHILD_GRANT_NOT_ALLOWED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := fixture.childRequest(parent)
			test.mutate(&request)
			_, err := DeriveChildGrant(request)
			assertAdmissionCode(t, err, test.code)
		})
	}
}

type admissionTestCatalog struct {
	provider catalog.ProviderDescriptorRecord
	digest   string
}

func (value admissionTestCatalog) Providers() []catalog.ProviderDescriptorRecord {
	return []catalog.ProviderDescriptorRecord{value.provider}
}

func (value admissionTestCatalog) Digest() string { return value.digest }

type admissionTestRegistry struct {
	provider   registry.ProviderInstance
	capability registry.VerifiedCapability
	digest     string
}

func (value admissionTestRegistry) Provider(id string) (registry.ProviderInstance, bool) {
	return value.provider, id == value.provider.ProviderID
}

func (value admissionTestRegistry) Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool) {
	return value.capability, providerID == value.provider.ProviderID && capabilityID == value.capability.ID
}

func (value admissionTestRegistry) Digest() string { return value.digest }

type admissionTestFixture struct {
	catalog   admissionTestCatalog
	registry  admissionTestRegistry
	authority AuthorityCeiling
	executors []ExecutorRegistration
}

func (value *admissionTestFixture) pinDescriptor(t *testing.T) {
	t.Helper()
	digest, _, err := canonicaljson.Digest(value.catalog.provider)
	if err != nil {
		t.Fatal(err)
	}
	value.registry.provider.DescriptorDigest = digest
}

func admissionFixture(t *testing.T) admissionTestFixture {
	t.Helper()
	reviewBinding := catalog.HostBinding{Host: "codex", Kind: "agent", Reference: "acme:review"}
	securityBinding := catalog.HostBinding{Host: "codex", Kind: "agent", Reference: "acme:security-review"}
	provider := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV1, DescriptorVersion: "1.0.0", ID: "acme/suite", DisplayName: "Acme",
		Discovery: []catalog.DiscoveryProbe{},
		Capabilities: []catalog.CapabilityRecord{
			{ID: "review", InputSchema: "input/v1", OutcomeSchema: "outcome/v1", MaximumEffects: []string{"read-project", "run-process", "write-project"}, Resources: []string{"project", "project-worktree"}, RequestModes: []catalog.RequestMode{catalog.RequestModeBounded}, Responsibilities: []string{}, ExecutorTopology: catalog.IsolatedRequired, DelegationAllowList: []string{}, HostBindings: []catalog.HostBinding{reviewBinding}},
			{ID: "security-review", InputSchema: "input/v1", OutcomeSchema: "outcome/v1", MaximumEffects: []string{"read-project"}, Resources: []string{"project"}, RequestModes: []catalog.RequestMode{catalog.RequestModeBounded}, Responsibilities: []string{}, ExecutorTopology: catalog.IsolatedRequired, DelegationAllowList: []string{}, HostBindings: []catalog.HostBinding{securityBinding}},
		},
	}
	descriptorDigest, _, err := canonicaljson.Digest(provider)
	if err != nil {
		t.Fatal(err)
	}
	verified := registry.ProviderInstance{
		ProviderID: "acme/suite", DescriptorDigest: descriptorDigest, Location: "/verified/acme", Version: "1.0.0",
		ConfigurationDigest: strings.Repeat("a", 64), BindingDigest: strings.Repeat("b", 64), EvidenceDigest: strings.Repeat("c", 64),
		Capabilities: []registry.VerifiedCapability{{ID: "review", Binding: reviewBinding}, {ID: "security-review", Binding: securityBinding}}, Digest: strings.Repeat("e", 64),
	}
	return admissionTestFixture{
		catalog:   admissionTestCatalog{provider: provider, digest: strings.Repeat("f", 64)},
		registry:  admissionTestRegistry{provider: verified, capability: verified.Capabilities[0], digest: strings.Repeat("9", 64)},
		authority: AuthorityCeiling{Effects: []string{"read-project", "run-process"}, Resources: []string{"project"}, ResourceLeases: false, AllowDelegation: true},
		executors: []ExecutorRegistration{{ID: "executor-review", Kind: ExecutorIsolated}},
	}
}

func (value admissionTestFixture) request(capabilityID string) GrantRequest {
	return GrantRequest{
		RunID: "run-0123456789abcdef0123456789abcdef", RequestID: "request", DeliverableID: "deliverable", InputDigest: strings.Repeat("1", 64),
		IssuedRevision: 2, Selector: classification.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: capabilityID, Source: classification.SelectorUserIntent},
		Effects: []string{"read-project"}, Resources: []string{"project"}, TerminationCondition: "one normalized review report", Executor: value.executors[0],
		Catalog: value.catalog, Registry: value.registry, Authority: value.authority, Executors: value.executors, DelegationAllowList: []string{},
	}
}

func (value admissionTestFixture) childRequest(parent CapabilityGrant) ChildGrantRequest {
	registryValue := value.registry
	registryValue.capability = registry.VerifiedCapability{ID: "security-review", Binding: value.catalog.provider.Capabilities[1].HostBindings[0]}
	return ChildGrantRequest{
		Parent: parent,
		Request: GrantRequest{
			RunID: parent.RunID, RequestID: parent.RequestID, DeliverableID: parent.DeliverableID + "-child", InputDigest: strings.Repeat("2", 64),
			IssuedRevision: parent.IssuedRevision + 1, Selector: classification.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "security-review", Source: classification.SelectorUserIntent},
			Effects: []string{"read-project"}, Resources: []string{"project"}, TerminationCondition: "one security report", Executor: value.executors[0],
			Catalog: value.catalog, Registry: registryValue, Authority: value.authority, Executors: value.executors, DelegationAllowList: []string{},
		},
	}
}

func assertAdmissionCode(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil || ErrorCode(err) != expected {
		t.Fatalf("error = %v (code %q), want %q", err, ErrorCode(err), expected)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
