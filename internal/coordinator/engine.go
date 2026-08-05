package coordinator

import (
	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

type CoreCompiler interface {
	Classify(*classification.ClassificationProposal, classification.ClassificationRules) (classification.ClassificationDecision, error)
	Compile(core.CompilationRequest) (core.CompilationResult, error)
}

type ProjectionRecord struct {
	SchemaVersion    string                   `json:"schema_version"`
	WorkflowID       string                   `json:"workflow_id"`
	Revision         uint64                   `json:"revision"`
	BundleGeneration uint64                   `json:"bundle_generation"`
	BundleDigest     string                   `json:"bundle_digest"`
	NodeID           string                   `json:"node_id"`
	Ticket           string                   `json:"ticket,omitempty"`
	Topology         execution.Topology       `json:"topology"`
	Evidence         []host.EvidenceReference `json:"evidence"`
	Digest           string                   `json:"digest"`
}

type ProjectionSink interface {
	WriteProjection(ProjectionRecord) error
}

type Options struct {
	StateRoot           string
	PhysicalProjectRoot string
	Rules               classification.ClassificationRules
	Configuration       config.Snapshot
	Resolutions         registry.ResolutionReport
	Registry            registry.Registry
	Authority           admission.AuthorityCeiling
	Core                CoreCompiler
	Projection          ProjectionSink
}

type Engine struct {
	core       CoreCompiler
	options    Options
	journal    *journal
	projection ProjectionSink
}

type defaultCore struct{}

func (defaultCore) Classify(proposal *classification.ClassificationProposal, rules classification.ClassificationRules) (classification.ClassificationDecision, error) {
	return core.Classify(proposal, rules)
}

func (defaultCore) Compile(request core.CompilationRequest) (core.CompilationResult, error) {
	return core.Compile(request)
}

func NewEngine(options Options) (*Engine, error) {
	journal, err := newJournal(options.StateRoot)
	if err != nil {
		return nil, err
	}
	if options.Core == nil {
		options.Core = defaultCore{}
	}
	options.Rules = cloneClassificationRules(options.Rules)
	options.Authority.Effects = append([]string{}, options.Authority.Effects...)
	options.Authority.Resources = append([]string{}, options.Authority.Resources...)
	return &Engine{core: options.Core, options: options, journal: journal, projection: options.Projection}, nil
}

func (engine *Engine) Exchange(command Command) (Result, error) {
	if engine == nil {
		return Result{}, coordinatorError("WORKFLOW_ENGINE_UNAVAILABLE", "Workflow Engine is required", nil)
	}
	if command.SchemaVersion != WorkflowCommandSchemaV1 {
		return Result{}, coordinatorError("SCHEMA_UNSUPPORTED", "unsupported Workflow Command schema", nil)
	}
	normalized, err := normalizeCommand(command)
	if err != nil {
		return Result{}, err
	}
	if err := validateCommand(normalized); err != nil {
		return Result{}, err
	}
	var result Result
	switch normalized.Kind {
	case CommandStart:
		result, err = engine.start(normalized)
	case CommandInspect:
		return engine.inspect(normalized.WorkflowID)
	case CommandPrepare:
		result, err = engine.prepare(normalized)
	case CommandReceipt:
		result, err = engine.receipt(normalized)
	case CommandSwitch:
		result, err = engine.switchWorkflow(normalized)
	case CommandCancel:
		result, err = engine.cancel(normalized)
	default:
		return Result{}, coordinatorError("WORKFLOW_COMMAND_UNSUPPORTED", "Workflow command is not implemented in this Coordinator transition", nil)
	}
	if err != nil {
		return Result{}, err
	}
	if !result.Replayed {
		engine.projectResult(result)
	}
	return result, nil
}

func (engine *Engine) inspect(workflowID string) (Result, error) {
	revision, err := engine.journal.inspect(workflowID)
	if err != nil {
		return Result{}, err
	}
	return revision.Result, nil
}

func cloneClassificationRules(value classification.ClassificationRules) classification.ClassificationRules {
	value.User.ProtectedResources = append([]classification.Resource{}, value.User.ProtectedResources...)
	value.User.RequiredEvidence = append([]classification.EvidenceKind{}, value.User.RequiredEvidence...)
	value.Project.ProtectedResources = append([]classification.Resource{}, value.Project.ProtectedResources...)
	value.Project.RequiredEvidence = append([]classification.EvidenceKind{}, value.Project.RequiredEvidence...)
	return value
}
