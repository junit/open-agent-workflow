package codexbridge

import (
	"context"
	"os"
	"reflect"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

type Observer interface {
	Observe(context.Context, string) (appserver.MetadataObservation, error)
}

type ServiceOptions struct {
	Observer       Observer
	Store          EvidenceStore
	StateRoot      string
	ProjectRoot    string
	UserConfigRoot string
	UserHome       string
	BridgeVersion  string
	Rules          classification.ClassificationRules
	Authority      admission.AuthorityCeiling
}

type Service struct {
	observer       Observer
	store          EvidenceStore
	stateRoot      string
	projectRoot    string
	userConfigRoot string
	userHome       string
	rules          classification.ClassificationRules
	authority      admission.AuthorityCeiling
	bridgeVersion  string
}

func NewService(options ServiceOptions) (*Service, error) {
	if options.Observer == nil || options.Store == nil {
		return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "Observer and EvidenceStore are required", nil)
	}
	return &Service{
		observer: options.Observer, store: options.Store, stateRoot: options.StateRoot,
		projectRoot: options.ProjectRoot, userConfigRoot: options.UserConfigRoot, userHome: options.UserHome,
		rules: cloneServiceClassificationRules(options.Rules), authority: admission.CloneAuthority(options.Authority),
		bridgeVersion: options.BridgeVersion,
	}, nil
}

func cloneServiceClassificationRules(value classification.ClassificationRules) classification.ClassificationRules {
	value.User.ProtectedResources = append([]classification.Resource{}, value.User.ProtectedResources...)
	value.User.RequiredEvidence = append([]classification.EvidenceKind{}, value.User.RequiredEvidence...)
	value.Project.ProtectedResources = append([]classification.Resource{}, value.Project.ProtectedResources...)
	value.Project.RequiredEvidence = append([]classification.EvidenceKind{}, value.Project.RequiredEvidence...)
	return value
}

func (service *Service) ObserveCurrent(ctx context.Context, _ ObserveCurrentInput, hostContext HookContext) (ObserveCurrentOutput, error) {
	if _, _, err := ContextDigestHeaders(hostContext); err != nil {
		return ObserveCurrentOutput{}, err
	}
	metadata, err := service.observer.Observe(ctx, hostContext.CWD)
	if err != nil {
		return ObserveCurrentOutput{}, bridgeErrorFromAppServer(err)
	}
	snapshot, report, err := service.loadInputs(hostContext.CWD)
	if err != nil {
		return ObserveCurrentOutput{}, err
	}
	inventory, diagnostics, err := BuildBindingInventory(snapshot.Catalog(), report, metadata, hostContext.CWD)
	if err != nil {
		return ObserveCurrentOutput{}, err
	}
	diagnostics = append(diagnostics, projectObservationDiagnostics(metadata.Diagnostics)...)
	resolved, err := core.Resolve(core.ResolutionRequest{
		Configuration: snapshot, HostID: "codex", Discovery: report, Inventory: &inventory,
	})
	if err != nil {
		return ObserveCurrentOutput{}, err
	}
	facts, err := AssembleFacts(hostContext, metadata, snapshot, report, inventory, resolved, service.bridgeVersion)
	if err != nil {
		return ObserveCurrentOutput{}, err
	}
	handle, err := service.store.Put(hostContext, facts)
	if err != nil {
		return ObserveCurrentOutput{}, err
	}
	return ObserveCurrentOutput{HostEvidenceHandle: handle, HostSummary: secretFreeSummary(facts, diagnostics)}, nil
}

func (service *Service) CoreInspect(_ context.Context, input CoreInspectInput) (CoreInspectOutput, error) {
	facts, err := service.getFacts(input.HostEvidenceHandle)
	if err != nil {
		return CoreInspectOutput{}, err
	}
	decision, err := core.Classify(&input.Proposal, service.rules)
	if err != nil {
		return CoreInspectOutput{}, err
	}
	output := CoreInspectOutput{Classification: decision, HostSummary: secretFreeSummary(facts, nil)}
	if decision.RequestMode != classification.RequestModeWorkflow {
		return output, nil
	}
	request, err := compilationRequest(input.DeliverableID, input.InputDigest, 1, decision, facts, nil)
	if err != nil {
		return CoreInspectOutput{}, err
	}
	result, err := core.Compile(request)
	if err != nil {
		return CoreInspectOutput{}, err
	}
	output.Compilation = &result
	draft, err := profile.NewRecipe("oaw-user/bridge-draft", "1.0.0")
	if err != nil {
		return CoreInspectOutput{}, err
	}
	evidence := request.Host
	builder, err := profile.BuildProjection(
		facts.Configuration.Catalog(), facts.Registry, evidence, draft, profile.BuilderBaseCanonical, "",
		profile.BuilderSelectionRequest{Profile: core.UserDefinedProfile, Topology: execution.TopologyCurrent, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}},
	)
	if err != nil {
		return CoreInspectOutput{}, err
	}
	output.Builder = &builder
	return output, nil
}

func (service *Service) CoreCompile(_ context.Context, input CoreCompileInput) (core.LifecycleBundle, error) {
	facts, err := service.getFacts(input.HostEvidenceHandle)
	if err != nil {
		return core.LifecycleBundle{}, err
	}
	decision, err := core.Classify(&input.Proposal, service.rules)
	if err != nil {
		return core.LifecycleBundle{}, err
	}
	if decision.RequestMode != classification.RequestModeWorkflow {
		return core.LifecycleBundle{}, NewError("WORKFLOW_CLASSIFICATION_REQUIRED", "Lifecycle compilation requires WORKFLOW classification", nil)
	}
	request, err := compilationRequest(input.DeliverableID, input.InputDigest, 1, decision, facts, &input.Selection)
	if err != nil {
		return core.LifecycleBundle{}, err
	}
	result, err := core.Compile(request)
	if err != nil {
		return core.LifecycleBundle{}, err
	}
	if result.Bundle == nil {
		return core.LifecycleBundle{}, NewError("PROFILE_SELECTION_INVALID", "exact trusted preview confirmation is required", nil)
	}
	return *result.Bundle, nil
}

func (service *Service) WorkflowExchange(_ context.Context, input WorkflowExchangeInput) (coordinator.Result, error) {
	facts, err := service.getFacts(input.HostEvidenceHandle)
	if err != nil {
		return coordinator.Result{}, err
	}
	if input.Command.SchemaVersion != coordinator.WorkflowCommandSchemaV2 {
		return coordinator.Result{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "Workflow Command schema is unsupported", nil)
	}
	hostEvidence, err := buildProfileHostEvidence(facts)
	if err != nil {
		return coordinator.Result{}, err
	}
	engine, err := coordinator.NewEngine(coordinator.Options{
		StateRoot: service.stateRoot, PhysicalProjectRoot: facts.Configuration.Record().ProjectRoot,
		Rules: service.rules, Configuration: facts.Configuration, Resolutions: facts.Resolutions,
		Registry: facts.Registry, Host: hostEvidence, Authority: service.authority,
	})
	if err != nil {
		return coordinator.Result{}, err
	}
	current := coordinator.Result{}
	if requiresPinnedPreflight(input.Command.Kind) {
		current, err = engine.Exchange(coordinator.Command{
			SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandInspect, WorkflowID: input.Command.WorkflowID,
		})
		if err != nil {
			return coordinator.Result{}, err
		}
		if err := compareActiveBundleFacts(current, facts); err != nil {
			return coordinator.Result{}, err
		}
		if input.Command.Kind == coordinator.CommandInspect {
			return current, nil
		}
	}
	command, err := input.Command.coordinatorCommand(facts, current)
	if err != nil {
		return coordinator.Result{}, err
	}
	if err := validateCommandHostFacts(command, facts); err != nil {
		return coordinator.Result{}, err
	}
	return engine.Exchange(command)
}

func validateCommandHostFacts(command coordinator.Command, facts Facts) error {
	switch command.Kind {
	case coordinator.CommandStart:
		if command.Start == nil {
			return NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "START payload is missing", nil)
		}
		if !reflect.DeepEqual(command.Start.HostSession, facts.Session) || !reflect.DeepEqual(command.Start.Environment, facts.Environment) {
			return NewError("HOST_SESSION_CHANGED", "START facts differ from observation", nil)
		}
	case coordinator.CommandSwitch:
		if command.Switch == nil {
			return NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "SWITCH payload is missing", nil)
		}
		if !reflect.DeepEqual(command.Switch.HostSession, facts.Session) || !reflect.DeepEqual(command.Switch.Environment, facts.Environment) {
			return NewError("HOST_SESSION_CHANGED", "SWITCH facts differ from observation", nil)
		}
	}
	return nil
}

func requiresPinnedPreflight(kind coordinator.CommandKind) bool {
	switch kind {
	case coordinator.CommandInspect, coordinator.CommandPrepare, coordinator.CommandReceipt, coordinator.CommandSwitch, coordinator.CommandCancel:
		return true
	default:
		return false
	}
}

func compareActiveBundleFacts(result coordinator.Result, facts Facts) error {
	if result.Snapshot == nil || len(result.Snapshot.Bundles) == 0 {
		return nil
	}
	found := false
	for _, bundle := range result.Snapshot.Bundles {
		if bundle.Generation != result.Snapshot.ActiveGeneration {
			continue
		}
		found = true
		if bundle.HostSessionDigest != facts.FactDigests.Session ||
			bundle.EnvironmentReportDigest != facts.FactDigests.Environment ||
			bundle.ProviderInventoryDigest != facts.FactDigests.Inventory ||
			bundle.HostFeatureDigest != facts.FactDigests.Features ||
			bundle.HostActionDigest != facts.FactDigests.Actions ||
			bundle.Configuration.Digest != facts.FactDigests.Configuration ||
			bundle.ResolutionDigest != facts.FactDigests.Resolution ||
			bundle.RegistryDigest != facts.FactDigests.Registry {
			return NewError("HOST_SESSION_CHANGED", "active Bundle facts differ from current observation", nil)
		}
	}
	if !found {
		return NewError("HOST_SESSION_CHANGED", "active Bundle generation is missing", nil)
	}
	return nil
}

func (service *Service) getFacts(handle HostEvidenceHandle) (Facts, error) {
	return service.store.Get(handle)
}

func compilationRequest(deliverableID, inputDigest string, generation uint64, decision classification.ClassificationDecision, facts Facts, selection *core.Selection) (core.CompilationRequest, error) {
	hostEvidence, err := buildProfileHostEvidence(facts)
	if err != nil {
		return core.CompilationRequest{}, err
	}
	return core.CompilationRequest{
		DeliverableID: deliverableID, InputDigest: inputDigest, Generation: generation, Classification: decision,
		Configuration: facts.Configuration, ResolutionDigest: facts.Resolutions.Digest(), Registry: facts.Registry, Host: hostEvidence,
		Selection: selection,
	}, nil
}

func buildProfileHostEvidence(facts Facts) (profile.HostEvidence, error) {
	manifest, err := CodexHostManifest()
	if err != nil {
		return profile.HostEvidence{}, NewError("HOST_EVIDENCE_HANDLE_INVALID", "Codex Host Manifest is invalid", err)
	}
	evidence, err := profile.NewHostEvidence(manifest, facts.Session, facts.Inventory, facts.Environment)
	if err != nil {
		return profile.HostEvidence{}, NewError("HOST_EVIDENCE_HANDLE_INVALID", "Host facts cannot form compiler evidence", err)
	}
	return evidence, nil
}

func (service *Service) loadInputs(projectRoot string) (config.Snapshot, discovery.Report, error) {
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: service.userConfigRoot, ProjectRoot: projectRoot})
	if err != nil {
		return config.Snapshot{}, discovery.Report{}, NewError("HOST_OBSERVATION_FAILED", "load current OAW configuration", err)
	}
	userHome := service.userHome
	if userHome == "" {
		userHome, err = os.UserHomeDir()
		if err != nil {
			return config.Snapshot{}, discovery.Report{}, NewError("HOST_OBSERVATION_FAILED", "resolve current user home", err)
		}
	}
	hints := make([]discovery.InstallationHint, 0)
	for _, installation := range snapshot.ProviderInstallations() {
		if installation.HostID != "codex" {
			continue
		}
		hints = append(hints, discovery.InstallationHint{
			ProviderID: installation.ProviderID, HostID: installation.HostID, SurfaceID: installation.SurfaceID,
			Location: installation.Location, DiscoveryProbeID: installation.DiscoveryProbeID,
		})
	}
	report, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: userHome, Installations: hints})
	if err != nil {
		return config.Snapshot{}, discovery.Report{}, NewError("HOST_OBSERVATION_FAILED", "discover current Codex Providers", err)
	}
	return snapshot, report, nil
}

func projectObservationDiagnostics(values []appserver.ObservationDiagnostic) []Diagnostic {
	result := make([]Diagnostic, 0, len(values))
	for _, value := range values {
		result = append(result, NewDiagnostic(value.Code, "observation", value.Detail, true))
	}
	return result
}

func bridgeErrorFromAppServer(err error) error {
	if err == nil {
		return nil
	}
	code := appserver.Code(err)
	if code == "" {
		code = "HOST_OBSERVATION_FAILED"
	}
	return NewError(code, "Codex metadata observation failed", err)
}

func secretFreeSummary(facts Facts, diagnostics []Diagnostic) HostSummary {
	providers := make([]ProviderStateSummary, 0)
	for _, resolution := range facts.Resolutions.Resolutions() {
		providers = append(providers, ProviderStateSummary{ProviderID: resolution.ProviderID, State: resolution.State})
	}
	projected := make([]Diagnostic, len(diagnostics))
	for index, diagnostic := range diagnostics {
		projected[index] = diagnostic
		projected[index].AffectedProviders = append([]string{}, diagnostic.AffectedProviders...)
		projected[index].AffectedProfiles = append([]string{}, diagnostic.AffectedProfiles...)
		projected[index].EvidenceDigest = shortDigest(facts.Session.Digest)
	}
	return HostSummary{
		SessionDigest: shortDigest(facts.Session.Digest), InventoryDigest: shortDigest(facts.Inventory.Digest),
		EnvironmentDigest: shortDigest(facts.Environment.Digest), VersionEvidenceDigest: shortDigest(facts.VersionEvidence.Digest),
		Providers: providers, Diagnostics: projected, DirectAvailable: true,
	}
}

func shortDigest(value string) string {
	if len(value) <= 16 {
		return value
	}
	return value[:16]
}
