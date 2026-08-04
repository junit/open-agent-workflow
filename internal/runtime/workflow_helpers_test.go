package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestWorkflowGrantAndSelectionNormalizationFailsClosed(t *testing.T) {
	validGrant := &StageGrantRequest{
		ExecutorID: " executor ", RequestedEffects: []string{"write-project", "read-project"},
		RequestedResources: []string{"project-worktree"}, TerminationCondition: " complete ",
	}
	if normalized, err := normalizeStageGrantRequest(validGrant); err != nil || normalized.ExecutorID != "executor" || normalized.TerminationCondition != "complete" || normalized.RequestedEffects[0] != "read-project" {
		t.Fatalf("valid Stage Grant normalization = %#v, %v", normalized, err)
	}
	for _, test := range []struct {
		name  string
		value *StageGrantRequest
	}{
		{"nil", nil},
		{"blank termination", &StageGrantRequest{TerminationCondition: " "}},
		{"invalid termination", &StageGrantRequest{TerminationCondition: "bad\ncondition", RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project"}}},
		{"empty effects", &StageGrantRequest{TerminationCondition: "complete", RequestedResources: []string{"project"}}},
		{"duplicate effects", &StageGrantRequest{TerminationCondition: "complete", RequestedEffects: []string{"read-project", "read-project"}, RequestedResources: []string{"project"}}},
		{"empty resources", &StageGrantRequest{TerminationCondition: "complete", RequestedEffects: []string{"read-project"}}},
		{"duplicate resources", &StageGrantRequest{TerminationCondition: "complete", RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project", "project"}}},
	} {
		t.Run("grant "+test.name, func(t *testing.T) {
			if _, err := normalizeStageGrantRequest(test.value); ErrorCode(err) != "WORKFLOW_GRANT_INVALID" {
				t.Fatalf("normalizeStageGrantRequest() error = %v", err)
			}
		})
	}

	validSelection := ProfileSelection{Profile: " MATT-SP-HYBRID ", Bindings: []profile.ProfileBinding{{
		Selector: catalog.CapabilitySelector{ProviderID: "oaw/matt", CapabilityID: "tdd"}, PreferredProviderID: "oaw/matt",
	}}}
	if normalized, err := normalizeProfileSelection(validSelection); err != nil || normalized.Profile != "MATT-SP-HYBRID" {
		t.Fatalf("valid Profile selection = %#v, %v", normalized, err)
	}
	duplicate := validSelection
	duplicate.Bindings = append(duplicate.Bindings, duplicate.Bindings[0])
	for _, value := range []ProfileSelection{
		{},
		{Profile: "bad\nprofile"},
		duplicate,
		{Profile: "profile", Bindings: []profile.ProfileBinding{{}}},
	} {
		if _, err := normalizeProfileSelection(value); ErrorCode(err) != "PROFILE_SELECTION_INVALID" {
			t.Fatalf("normalizeProfileSelection(%#v) error = %v", value, err)
		}
	}

	validSwitch := &StableBoundarySwitch{Boundary: " boundary ", Selection: ProfileSelection{Profile: "SP-FULL"}}
	if normalized, err := normalizeStableBoundarySwitch(validSwitch); err != nil || normalized.Boundary != "boundary" {
		t.Fatalf("valid Stable Boundary switch = %#v, %v", normalized, err)
	}
	for _, value := range []*StableBoundarySwitch{nil, {Boundary: "bad\nboundary", Selection: ProfileSelection{Profile: "SP-FULL"}}, {Boundary: "boundary"}} {
		if _, err := normalizeStableBoundarySwitch(value); err == nil {
			t.Fatalf("normalizeStableBoundarySwitch(%#v) error = nil", value)
		}
	}
}

func TestWorkflowObservationNormalizationFailsClosed(t *testing.T) {
	valid := StageObservation{CapabilityObservation: CapabilityObservation{
		GrantID: "grant", InvocationID: "invocation", ExecutorID: "executor", Outcome: ObservationSucceeded,
		EvidenceReferences: []EvidenceReference{{Reference: " evidence://one ", Digest: strings.Repeat("1", 64)}},
	}, Signal: " succeeded ", StableBoundary: " boundary "}
	if normalized, err := normalizeStageObservation(&valid); err != nil || normalized.Signal != workflowSignalSucceeded || normalized.StableBoundary != "boundary" || normalized.EvidenceReferences[0].Reference != "evidence://one" {
		t.Fatalf("valid Stage Observation = %#v, %v", normalized, err)
	}

	tests := []struct {
		name   string
		mutate func(*StageObservation)
	}{
		{"missing identity", func(value *StageObservation) { value.GrantID = "" }},
		{"invalid outcome", func(value *StageObservation) { value.Outcome = "UNKNOWN" }},
		{"raw output", func(value *StageObservation) { value.RawOutput = "raw" }},
		{"missing evidence", func(value *StageObservation) { value.EvidenceReferences = nil }},
		{"invalid reference", func(value *StageObservation) { value.EvidenceReferences[0].Reference = "bad\nreference" }},
		{"invalid evidence digest", func(value *StageObservation) { value.EvidenceReferences[0].Digest = "bad" }},
		{"duplicate evidence", func(value *StageObservation) {
			value.EvidenceReferences = append(value.EvidenceReferences, value.EvidenceReferences[0])
		}},
		{"unknown signal", func(value *StageObservation) { value.Signal = "invented" }},
		{"failed success", func(value *StageObservation) { value.Outcome = ObservationFailed }},
		{"successful incident", func(value *StageObservation) { value.Signal = workflowSignalBuildFailure }},
		{"invalid boundary", func(value *StageObservation) { value.StableBoundary = "bad\nboundary" }},
	}
	if _, err := normalizeStageObservation(nil); ErrorCode(err) != "OBSERVATION_INVALID" {
		t.Fatalf("nil Stage Observation error = %v", err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneStageObservation(valid)
			test.mutate(&candidate)
			if _, err := normalizeStageObservation(&candidate); ErrorCode(err) != "OBSERVATION_INVALID" {
				t.Fatalf("normalizeStageObservation() error = %v", err)
			}
		})
	}
	for _, signal := range []string{
		workflowSignalSucceeded, workflowSignalFinding, workflowSignalRemediated, workflowSignalFunctionalFailure,
		workflowSignalBuildFailure, workflowSignalDependencyFailure, workflowSignalTypeFailure, workflowSignalSecurityFinding,
	} {
		if !workflowObservationSignalAllowed(signal) {
			t.Fatalf("closed Workflow signal %q rejected", signal)
		}
	}
	if workflowObservationSignalAllowed("unknown") {
		t.Fatal("unknown Workflow signal accepted")
	}
}

func TestWorkflowExecutorAndLeaseHelpersCoverEveryDecision(t *testing.T) {
	normal := profile.GraphNode{ID: "implementation", Responsibility: "implementation", MaximumEffects: []string{"read-project", "write-project"}, ExecutorTopology: catalog.IsolatedRequired}
	review := profile.GraphNode{ID: "review", Responsibility: "review", MaximumEffects: []string{"read-project"}, ExecutorTopology: catalog.IsolatedRequired}
	isolated := WorkflowExecutorRegistration{Registration: admission.ExecutorRegistration{ID: "isolated", Kind: admission.ExecutorIsolated}}
	freshReview := WorkflowExecutorRegistration{Registration: admission.ExecutorRegistration{ID: "reviewer", Kind: admission.ExecutorIsolated}, ReadOnly: true, Fresh: true}
	mainAgent := WorkflowExecutorRegistration{Registration: admission.ExecutorRegistration{ID: "main", Kind: admission.ExecutorMainAgent}, ReadOnly: true, Fresh: true}
	if selected, err := selectWorkflowExecutor(normal, "isolated", []WorkflowExecutorRegistration{isolated}, nil); err != nil || selected.ID != "isolated" {
		t.Fatalf("normal Executor selection = %#v, %v", selected, err)
	}
	if selected, err := selectWorkflowExecutor(review, "", []WorkflowExecutorRegistration{mainAgent, isolated, freshReview}, nil); err != nil || selected.ID != "reviewer" {
		t.Fatalf("automatic Review Executor selection = %#v, %v", selected, err)
	}
	prior := []admission.CapabilityGrant{{Executor: freshReview.Registration}}
	for _, test := range []struct {
		name      string
		node      profile.GraphNode
		requested string
		available []WorkflowExecutorRegistration
		prior     []admission.CapabilityGrant
		code      string
	}{
		{"main Agent", normal, "main", []WorkflowExecutorRegistration{mainAgent}, nil, "EXECUTOR_NOT_REGISTERED"},
		{"unknown", normal, "missing", []WorkflowExecutorRegistration{isolated}, nil, "EXECUTOR_NOT_REGISTERED"},
		{"missing normal", normal, "", []WorkflowExecutorRegistration{isolated}, nil, "EXECUTOR_NOT_REGISTERED"},
		{"Review not read-only", review, "isolated", []WorkflowExecutorRegistration{isolated}, nil, "REVIEW_EXECUTOR_REQUIRED"},
		{"Review reused", review, "reviewer", []WorkflowExecutorRegistration{freshReview}, prior, "REVIEW_EXECUTOR_REQUIRED"},
		{"Review unavailable", review, "", []WorkflowExecutorRegistration{mainAgent, isolated}, nil, "REVIEW_EXECUTOR_REQUIRED"},
		{"Review all used", review, "", []WorkflowExecutorRegistration{freshReview}, prior, "REVIEW_EXECUTOR_REQUIRED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := selectWorkflowExecutor(test.node, test.requested, test.available, test.prior); ErrorCode(err) != test.code {
				t.Fatalf("selectWorkflowExecutor() error = %v", err)
			}
		})
	}
	if !workflowNodeAllowsWrites(normal) || workflowNodeReadOnly(normal) || workflowNodeAllowsWrites(review) || !workflowNodeReadOnly(review) {
		t.Fatal("Workflow node write/read-only classification is inconsistent")
	}
	if workflowNodeReadOnly(profile.GraphNode{ExecutorTopology: catalog.MainAgentAllowed}) {
		t.Fatal("Main Agent node classified as isolated read-only")
	}

	if workflowStageNeedsResourceLease(nil) || workflowStageNeedsResourceLease(&StageGrantRequest{RequestedEffects: []string{"read-project"}}) || !workflowStageNeedsResourceLease(&StageGrantRequest{RequestedEffects: []string{"write-project"}}) || !workflowStageNeedsResourceLease(&StageGrantRequest{RequestedEffects: []string{"git-local"}}) {
		t.Fatal("Stage Grant lease classification is inconsistent")
	}
	if workflowGrantNeedsResourceLease(admission.CapabilityGrant{Effects: []string{"read-project"}}) || !workflowGrantNeedsResourceLease(admission.CapabilityGrant{Effects: []string{"write-project"}}) {
		t.Fatal("committed Grant lease classification is inconsistent")
	}
	if workflowPauseEvent(SignalAdditionalCapabilityRequired) != "WORKFLOW_ADDITIONAL_CAPABILITY_REQUIRED" || workflowPauseEvent(SignalRemediationRequired) != "WORKFLOW_REMEDIATION_REQUIRED" || workflowPauseEvent(SignalArchitectureRequired) != "WORKFLOW_ARCHITECTURE_REQUIRED" || workflowPauseEvent("UNKNOWN") != "WORKFLOW_PAUSED" {
		t.Fatal("Workflow pause event mapping is inconsistent")
	}

	graph := profile.ExecutionGraphRecord{Nodes: []profile.GraphNode{normal}}
	if node, found := workflowGraphNode(graph, normal.ID); !found || node.ID != normal.ID {
		t.Fatal("Workflow graph node lookup failed")
	}
	if _, found := workflowGraphNode(graph, "missing"); found {
		t.Fatal("missing Workflow graph node found")
	}
	lease := ResourceLease{ID: "lease"}
	if found, ok := workflowResourceLease([]ResourceLease{lease}, "lease"); !ok || found.ID != "lease" {
		t.Fatal("Workflow lease lookup failed")
	}
	if _, ok := workflowResourceLease([]ResourceLease{lease}, "missing"); ok || containsWorkflowValue([]string{"one"}, "missing") || !containsWorkflowValue([]string{"one"}, "one") {
		t.Fatal("Workflow lease/value lookup is inconsistent")
	}
	bundle := LifecycleBundle{ID: "bundle"}
	if found, ok := workflowBundleByID([]LifecycleBundle{bundle}, "bundle"); !ok || found.ID != "bundle" {
		t.Fatal("Workflow Bundle lookup failed")
	}
	if _, ok := workflowBundleByID([]LifecycleBundle{bundle}, "missing"); ok {
		t.Fatal("missing Workflow Bundle found")
	}
}

func TestResourceLeaseRecordsRejectInvalidIdentityAndRoots(t *testing.T) {
	projectRoot := t.TempDir()
	request := resourceLeaseRequest{
		RunID: "run-0123456789abcdef0123456789abcdef", GrantID: "grant-0123456789abcdef0123456789abcdef",
		BundleID: "bundle-0123456789abcdef0123456789abcdef", Generation: 1,
		Resource: resourceLeaseProjectWorktree, PhysicalRoot: projectRoot, AcquiredRevision: 3,
	}
	lease, err := newResourceLease(request)
	if err != nil || validateResourceLease(lease) != nil {
		t.Fatalf("valid Resource Lease = %#v, %v", lease, err)
	}
	invalidRoot := request
	invalidRoot.PhysicalRoot = "relative"
	if _, err := newResourceLease(invalidRoot); ErrorCode(err) != "RESOURCE_LEASE_INVALID" {
		t.Fatalf("relative Resource Lease root error = %v", err)
	}
	invalidIdentity := request
	invalidIdentity.Generation = 0
	if _, err := newResourceLease(invalidIdentity); ErrorCode(err) != "RESOURCE_LEASE_INVALID" {
		t.Fatalf("invalid Resource Lease identity error = %v", err)
	}
	for _, mutate := range []func(*ResourceLease){
		func(value *ResourceLease) { value.SchemaVersion = "wrong" },
		func(value *ResourceLease) { value.Digest = strings.Repeat("0", 64) },
	} {
		candidate := lease
		mutate(&candidate)
		if ErrorCode(validateResourceLease(candidate)) != "RUN_STATE_REVISION_INVALID" {
			t.Fatalf("invalid Resource Lease accepted: %#v", candidate)
		}
	}
	alias := filepath.Join(t.TempDir(), "project-alias")
	if err := os.Symlink(projectRoot, alias); err == nil {
		candidate := lease
		candidate.PhysicalRoot = alias
		if ErrorCode(validateResourceLease(candidate)) != "RUN_STATE_REVISION_INVALID" {
			t.Fatal("non-canonical Resource Lease root accepted")
		}
	}
	fileRoot := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(fileRoot, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{"relative", filepath.Join(t.TempDir(), "missing"), fileRoot} {
		if _, err := canonicalPhysicalRoot(root); err == nil {
			t.Fatalf("canonicalPhysicalRoot(%q) error = nil", root)
		}
	}
}

func TestWorkflowProjectionValidationAndFilesystemBoundaries(t *testing.T) {
	valid := validInternalWorkflowProjection(t)
	if err := validateWorkflowProjection(valid); err != nil {
		t.Fatalf("valid Workflow projection rejected: %v", err)
	}
	for _, mutate := range []func(*WorkflowProjection){
		func(value *WorkflowProjection) { value.SchemaVersion = "wrong" },
		func(value *WorkflowProjection) { value.RunID = "bad" },
		func(value *WorkflowProjection) { value.Revision = 0 },
		func(value *WorkflowProjection) { value.RevisionDigest = "bad" },
		func(value *WorkflowProjection) { value.StateDigest = "bad" },
		func(value *WorkflowProjection) { value.Status = "" },
		func(value *WorkflowProjection) { value.Event = "" },
		func(value *WorkflowProjection) { value.ConfigurationDigest = "bad" },
		func(value *WorkflowProjection) { value.ActiveTicket = "" },
		func(value *WorkflowProjection) { value.LagStatus = "" },
		func(value *WorkflowProjection) { value.Profile = "" },
		func(value *WorkflowProjection) { value.BundleGeneration = 0 },
		func(value *WorkflowProjection) { value.Stage = "" },
		func(value *WorkflowProjection) { value.BundleID = "bad" },
		func(value *WorkflowProjection) { value.EvidenceReferences[0].Reference = "bad\nreference" },
		func(value *WorkflowProjection) { value.EvidenceReferences[0].Digest = "bad" },
		func(value *WorkflowProjection) {
			value.EvidenceReferences = append(value.EvidenceReferences, value.EvidenceReferences[0])
		},
		func(value *WorkflowProjection) { value.Digest = strings.Repeat("0", 64) },
	} {
		candidate := valid
		candidate.EvidenceReferences = append([]EvidenceReference{}, valid.EvidenceReferences...)
		mutate(&candidate)
		if ErrorCode(validateWorkflowProjection(candidate)) != "PROJECTION_INVALID" {
			t.Fatalf("invalid projection accepted: %#v", candidate)
		}
	}
	unselected := valid
	unselected.Profile, unselected.BundleGeneration, unselected.BundleID, unselected.BundleDigest, unselected.Stage, unselected.GraphDigest = "", 0, "", "", "", ""
	unselected.HostID, unselected.BindingInventoryDigest = "", ""
	unselected.HostIntegrationID, unselected.HostIntegrationDigest, unselected.HostManifestDigest, unselected.HostAuditDigest, unselected.HostConformanceDigest = "", "", "", "", ""
	unselected.EvidenceReferences = []EvidenceReference{}
	finalizeInternalProjection(t, &unselected)
	if err := validateWorkflowProjection(unselected); err != nil {
		t.Fatalf("valid unselected projection rejected: %v", err)
	}

	root := internalPhysicalDirectory(t, filepath.Join(t.TempDir(), "projection"))
	sink, err := NewFilesystemProjectionSink(root)
	if err != nil || sink.WriteProjection(valid) != nil {
		t.Fatalf("valid filesystem projection = %v", err)
	}
	if (*FilesystemProjectionSink)(nil).WriteProjection(valid) == nil || sink.WriteProjection(WorkflowProjection{}) == nil {
		t.Fatal("invalid filesystem projection accepted")
	}
	if _, err := projectionSinkFromOptions(ProjectionOptions{Root: root, Sink: sink}); ErrorCode(err) != "PROJECTION_CONFIGURATION_INVALID" {
		t.Fatalf("ambiguous projection options error = %v", err)
	}
	if selected, err := projectionSinkFromOptions(ProjectionOptions{Sink: sink}); err != nil || selected != sink {
		t.Fatalf("injected projection sink = %#v, %v", selected, err)
	}
	if selected, err := projectionSinkFromOptions(ProjectionOptions{}); err != nil || selected != nil {
		t.Fatalf("empty projection sink = %#v, %v", selected, err)
	}
	for _, invalid := range []string{"relative", filepath.Join(t.TempDir(), "missing", "..", "projection")} {
		if _, err := NewFilesystemProjectionSink(invalid); ErrorCode(err) != "PROJECTION_DESTINATION_INVALID" {
			t.Fatalf("NewFilesystemProjectionSink(%q) error = %v", invalid, err)
		}
	}
	fileRoot := filepath.Join(t.TempDir(), "projection-file")
	if err := os.WriteFile(fileRoot, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFilesystemProjectionSink(fileRoot); ErrorCode(err) != "PROJECTION_DESTINATION_INVALID" {
		t.Fatalf("projection file root error = %v", err)
	}

	runRoot := filepath.Join(root, valid.RunID)
	if err := os.RemoveAll(runRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runRoot, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sink.WriteProjection(valid); ErrorCode(err) != "PROJECTION_WRITE_FAILED" {
		t.Fatalf("blocked projection Run root error = %v", err)
	}

	jsonBlockedRoot := internalPhysicalDirectory(t, filepath.Join(t.TempDir(), "projection-json-blocked"))
	jsonBlockedSink, err := NewFilesystemProjectionSink(jsonBlockedRoot)
	if err != nil {
		t.Fatal(err)
	}
	jsonRunRoot := filepath.Join(jsonBlockedRoot, valid.RunID)
	if err := os.Mkdir(jsonRunRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(jsonRunRoot, "workflow.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := jsonBlockedSink.WriteProjection(valid); ErrorCode(err) != "PROJECTION_WRITE_FAILED" {
		t.Fatalf("blocked JSON projection error = %v", err)
	}

	markdownBlockedRoot := internalPhysicalDirectory(t, filepath.Join(t.TempDir(), "projection-markdown-blocked"))
	markdownBlockedSink, err := NewFilesystemProjectionSink(markdownBlockedRoot)
	if err != nil {
		t.Fatal(err)
	}
	markdownRunRoot := filepath.Join(markdownBlockedRoot, valid.RunID)
	if err := os.Mkdir(markdownRunRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(markdownRunRoot, "workflow.md"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := markdownBlockedSink.WriteProjection(valid); ErrorCode(err) != "PROJECTION_WRITE_FAILED" {
		t.Fatalf("blocked Markdown projection error = %v", err)
	}
}

func TestWorkflowProjectionEvidenceIsSortedDeduplicatedAndPinned(t *testing.T) {
	observations := []StageObservation{
		{CapabilityObservation: CapabilityObservation{EvidenceReferences: []EvidenceReference{
			{Reference: "evidence://z", Digest: strings.Repeat("2", 64)},
			{Reference: "evidence://a", Digest: strings.Repeat("1", 64)},
		}}},
		{CapabilityObservation: CapabilityObservation{EvidenceReferences: []EvidenceReference{
			{Reference: "evidence://a", Digest: strings.Repeat("1", 64)},
		}}},
	}
	got, err := workflowProjectionEvidence(observations)
	if err != nil || len(got) != 2 || got[0].Reference != "evidence://a" || got[1].Reference != "evidence://z" {
		t.Fatalf("workflowProjectionEvidence() = %#v, %v", got, err)
	}
	observations[0].EvidenceReferences[0].Digest = "bad"
	if _, err := workflowProjectionEvidence(observations); ErrorCode(err) != "PROJECTION_INVALID" {
		t.Fatalf("invalid projected evidence error = %v", err)
	}
}

func TestWorkflowRecordAndProjectionHelpersFailClosed(t *testing.T) {
	validInput := &WorkflowInput{DeliverableID: "deliverable", InputDigest: strings.Repeat("1", 64), ActiveTicket: "ticket-10"}
	if normalized, err := normalizeWorkflowInput(validInput); err != nil || normalized != *validInput {
		t.Fatalf("valid Workflow input = %#v, %v", normalized, err)
	}
	for _, value := range []*WorkflowInput{nil, {DeliverableID: "bad\ndeliverable", InputDigest: strings.Repeat("1", 64)}, {DeliverableID: "deliverable", InputDigest: "bad"}, {DeliverableID: "deliverable", InputDigest: strings.Repeat("1", 64), ActiveTicket: "bad\nticket"}} {
		if _, err := normalizeWorkflowInput(value); ErrorCode(err) != "WORKFLOW_REQUEST_INVALID" {
			t.Fatalf("normalizeWorkflowInput(%#v) error = %v", value, err)
		}
	}

	providers := []profile.GraphProviderInstance{{ProviderID: "one", InstanceDigest: strings.Repeat("1", 64)}}
	if !equalGraphProviders(providers, append([]profile.GraphProviderInstance{}, providers...)) || equalGraphProviders(providers, nil) || equalGraphProviders(providers, []profile.GraphProviderInstance{{ProviderID: "two"}}) {
		t.Fatal("Graph Provider equality is inconsistent")
	}
	bindings := []profile.ProfileBinding{{Selector: catalog.CapabilitySelector{ProviderID: "one", CapabilityID: "capability"}, PreferredProviderID: "one"}}
	if !equalBindings(bindings, append([]profile.ProfileBinding{}, bindings...)) || equalBindings(bindings, nil) || equalBindings(bindings, []profile.ProfileBinding{{PreferredProviderID: "two"}}) {
		t.Fatal("Profile Binding equality is inconsistent")
	}

	snapshot, err := config.Load(config.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	identity := bundleRequest{
		RunID: "run-0123456789abcdef0123456789abcdef", DeliverableID: "deliverable", InputDigest: strings.Repeat("1", 64),
		Generation: 1, CreatedRevision: 2, Selection: ProfileSelection{Profile: "SP-FULL"},
		Configuration: snapshot.Record(), Registry: grantTestRegistry{},
	}
	if _, err := newLifecycleBundle(bundleRequest{}); ErrorCode(err) != "PROFILE_SELECTION_INVALID" {
		t.Fatalf("empty Lifecycle Bundle request error = %v", err)
	}
	for _, request := range []bundleRequest{
		{Selection: ProfileSelection{Profile: "SP-FULL"}},
		{RunID: identity.RunID, DeliverableID: identity.DeliverableID, InputDigest: identity.InputDigest, Generation: 1, CreatedRevision: 2, Selection: identity.Selection, Registry: identity.Registry},
		identity,
	} {
		if _, err := newLifecycleBundle(request); ErrorCode(err) != "WORKFLOW_BUNDLE_INVALID" {
			t.Fatalf("newLifecycleBundle(%#v) error = %v", request, err)
		}
	}

	grant := admission.CapabilityGrant{ID: "grant", Executor: admission.ExecutorRegistration{ID: "executor"}}
	if _, err := workflowActiveGrant(RunSnapshot{Workflow: &WorkflowState{}}); ErrorCode(err) != "RUN_STATE_REVISION_INVALID" {
		t.Fatalf("missing active Grant error = %v", err)
	}
	if _, err := workflowActiveGrant(RunSnapshot{Workflow: &WorkflowState{ActiveGrantID: "missing"}, Grants: []admission.CapabilityGrant{grant}}); ErrorCode(err) != "RUN_STATE_REVISION_INVALID" {
		t.Fatalf("unknown active Grant error = %v", err)
	}
	if _, err := workflowActiveBundle(RunSnapshot{Workflow: &WorkflowState{}}); ErrorCode(err) != "RUN_STATE_REVISION_INVALID" {
		t.Fatalf("missing active Bundle error = %v", err)
	}
	if _, err := workflowActiveBundle(RunSnapshot{Workflow: &WorkflowState{ActiveGeneration: 2}, LifecycleBundles: []string{"bundle"}}); ErrorCode(err) != "RUN_STATE_REVISION_INVALID" {
		t.Fatalf("unknown active Bundle error = %v", err)
	}

	leftObservation := StageObservation{CapabilityObservation: CapabilityObservation{GrantID: "grant", EvidenceReferences: []EvidenceReference{{Reference: "one", Digest: strings.Repeat("1", 64)}}}, Signal: workflowSignalSucceeded}
	rightObservation := cloneStageObservation(leftObservation)
	if !equalStageObservation(leftObservation, rightObservation) {
		t.Fatal("identical Stage Observations differ")
	}
	rightObservation.GrantID = "other"
	if equalStageObservation(leftObservation, rightObservation) {
		t.Fatal("Stage Observation identity mismatch accepted")
	}
	rightObservation = cloneStageObservation(leftObservation)
	rightObservation.EvidenceReferences[0].Digest = strings.Repeat("2", 64)
	if equalStageObservation(leftObservation, rightObservation) {
		t.Fatal("Stage Observation evidence mismatch accepted")
	}

	if appendOnlyWorkflowGrants([]admission.CapabilityGrant{grant}, []admission.CapabilityGrant{{ID: "changed"}, {ID: "new"}}) {
		t.Fatal("Workflow Grant prefix rewrite accepted")
	}
	bundle := LifecycleBundle{ID: "bundle"}
	if appendOnlyLifecycleBundles([]LifecycleBundle{bundle}, []LifecycleBundle{{ID: "changed"}, {ID: "new"}}) {
		t.Fatal("Lifecycle Bundle prefix rewrite accepted")
	}

	if _, err := newWorkflowProjection(revisionRecord{}); ErrorCode(err) != "PROJECTION_INVALID" {
		t.Fatalf("invalid committed projection record error = %v", err)
	}
	incomplete := revisionRecord{
		RunID: "run-0123456789abcdef0123456789abcdef", Revision: 1, Digest: strings.Repeat("1", 64), StateDigest: strings.Repeat("2", 64),
		Snapshot: RunSnapshot{RunID: "run-0123456789abcdef0123456789abcdef", Revision: 1, ConfigurationDigest: strings.Repeat("3", 64), Workflow: &WorkflowState{ActiveGeneration: 1}},
	}
	if _, err := newWorkflowProjection(incomplete); ErrorCode(err) != "RUN_STATE_REVISION_INVALID" {
		t.Fatalf("incomplete committed Bundle projection error = %v", err)
	}

	journal, err := newJournal(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	lagRecord := revisionRecord{RunID: incomplete.RunID, Revision: 1, Digest: strings.Repeat("4", 64)}
	if err := journal.recordProjectionLag(lagRecord); err != nil {
		t.Fatal(err)
	}
	if err := journal.recordProjectionLag(lagRecord); err == nil {
		t.Fatal("duplicate immutable projection lag marker accepted")
	}
}

func validInternalWorkflowProjection(t *testing.T) WorkflowProjection {
	t.Helper()
	value := WorkflowProjection{
		SchemaVersion: workflowProjectionSchemaV2, RunID: "run-0123456789abcdef0123456789abcdef", Revision: 2,
		RevisionDigest: strings.Repeat("1", 64), StateDigest: strings.Repeat("2", 64), Status: RunReady,
		Event: "WORKFLOW_BUNDLE_CREATED", ConfigurationDigest: strings.Repeat("3", 64), ActiveTicket: "ticket-10", LagStatus: "current",
		BundleID: "bundle-0123456789abcdef0123456789abcdef", BundleDigest: strings.Repeat("4", 64),
		Profile: "MATT-SP-HYBRID", BundleGeneration: 1, Stage: "requirements", GraphDigest: strings.Repeat("5", 64),
		HostID: "codex", BindingInventoryDigest: strings.Repeat("b", 64),
		EvidenceReferences: []EvidenceReference{{Reference: "evidence://projection", Digest: strings.Repeat("a", 64)}},
		HostIntegrationID:  "acme/codex-runtime", HostIntegrationDigest: strings.Repeat("6", 64),
		HostManifestDigest: strings.Repeat("7", 64), HostAuditDigest: strings.Repeat("8", 64),
		HostConformanceDigest: strings.Repeat("9", 64),
	}
	finalizeInternalProjection(t, &value)
	return value
}

func finalizeInternalProjection(t *testing.T, value *WorkflowProjection) {
	t.Helper()
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(*value)
	if err != nil {
		t.Fatal(err)
	}
	value.Digest = digest
}

func internalPhysicalDirectory(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(physical)
}
