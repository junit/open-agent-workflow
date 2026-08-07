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
	facts, err := AssembleFacts(hostContext, metadata, snapshot, report, inventory, resolved)
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
	result, err := core.Compile(compilationRequest(input.DeliverableID, input.InputDigest, 1, decision, facts, nil))
	if err != nil {
		return CoreInspectOutput{}, err
	}
	output.Compilation = &result
	return output, nil
}

func (service *Service) CoreCompile(_ context.Context, input CoreCompileInput) (core.CompilationResult, error) {
	facts, err := service.getFacts(input.HostEvidenceHandle)
	if err != nil {
		return core.CompilationResult{}, err
	}
	decision, err := core.Classify(&input.Proposal, service.rules)
	if err != nil {
		return core.CompilationResult{}, err
	}
	if decision.RequestMode != classification.RequestModeWorkflow {
		return core.CompilationResult{}, NewError("WORKFLOW_CLASSIFICATION_REQUIRED", "Lifecycle compilation requires WORKFLOW classification", nil)
	}
	return core.Compile(compilationRequest(input.DeliverableID, input.InputDigest, 1, decision, facts, &input.Selection))
}

func (service *Service) WorkflowExchange(_ context.Context, input WorkflowExchangeInput) (coordinator.Result, error) {
	facts, err := service.getFacts(input.HostEvidenceHandle)
	if err != nil {
		return coordinator.Result{}, err
	}
	if err := validateCommandHostFacts(input.Command, facts); err != nil {
		return coordinator.Result{}, err
	}
	engine, err := coordinator.NewEngine(coordinator.Options{
		StateRoot: service.stateRoot, PhysicalProjectRoot: facts.Configuration.Record().ProjectRoot,
		Rules: service.rules, Configuration: facts.Configuration, Resolutions: facts.Resolutions,
		Registry: facts.Registry, Authority: service.authority,
	})
	if err != nil {
		return coordinator.Result{}, err
	}
	if requiresPinnedPreflight(input.Command.Kind) {
		current, err := engine.Exchange(coordinator.Command{
			SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandInspect, WorkflowID: input.Command.WorkflowID,
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
	return engine.Exchange(input.Command)
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

func compilationRequest(deliverableID, inputDigest string, generation uint64, decision classification.ClassificationDecision, facts Facts, selection *core.Selection) core.CompilationRequest {
	return core.CompilationRequest{
		DeliverableID: deliverableID, InputDigest: inputDigest, Generation: generation, Classification: decision,
		Configuration: facts.Configuration, Resolutions: facts.Resolutions, Registry: facts.Registry, HostID: facts.Session.HostID,
		HostSessionDigest: facts.Session.Digest, HostEnvironmentReportDigest: facts.Environment.Digest,
		HostProviderInventoryDigest: facts.Inventory.Digest,
		HostTopologies:              append([]execution.Topology{}, facts.Session.SupportedTopologies...),
		EnvironmentObservations:     append([]execution.EnvironmentObservation{}, facts.Environment.Observations...),
		Selection:                   selection,
	}
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
		EnvironmentDigest: shortDigest(facts.Environment.Digest), Providers: providers, Diagnostics: projected, DirectAvailable: true,
	}
}

func shortDigest(value string) string {
	if len(value) <= 16 {
		return value
	}
	return value[:16]
}
