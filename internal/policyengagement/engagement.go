// Package policyengagement owns the current project's cooperative Policy
// Engagement. Its interface uses business actions; reducer references,
// persistence locations, and route observations stay inside the module.
package policyengagement

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
	"github.com/wifibaby4u/open-agent-workflow/internal/policyroute"
	"github.com/wifibaby4u/open-agent-workflow/internal/policyrun"
)

type Action string

const (
	ActionProfiles  Action = "profiles"
	ActionUse       Action = "use"
	ActionStatus    Action = "status"
	ActionComplete  Action = "complete"
	ActionReview    Action = "review"
	ActionApprove   Action = "approve"
	ActionSatisfy   Action = "satisfy"
	ActionIncident  Action = "incident"
	ActionSwitch    Action = "switch"
	ActionStop      Action = "stop"
	ActionUncertain Action = "uncertain"
)

type Command struct {
	Action     Action
	Profile    policyflow.ProfileID
	Intent     string
	Complexity string
	Risk       string
	Incident   policyflow.IncidentType
	Review     policyflow.ReviewOutcome
	Reason     string
}

type Profile struct {
	Name             string          `json:"name"`
	PolicySelectable bool            `json:"policy_selectable"`
	HostRoutable     bool            `json:"host_routable"`
	Missing          []string        `json:"missing"`
	IncidentRoutes   []IncidentRoute `json:"incident_routes"`
}

type IncidentRoute struct {
	Incident string `json:"incident"`
	Skill    string `json:"skill,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Status   string `json:"status"`
}

type Status struct {
	Deliverable string `json:"deliverable"`
	Profile     string `json:"profile"`
	State       string `json:"state"`
	Phase       string `json:"phase,omitempty"`
	Next        string `json:"next,omitempty"`
	Name        string `json:"name,omitempty"`
	CanSwitch   bool   `json:"can_switch"`
	Reason      string `json:"reason,omitempty"`
}

type Result struct {
	Assurance   string    `json:"assurance,omitempty"`
	Execution   string    `json:"execution,omitempty"`
	Limitations []string  `json:"limitations,omitempty"`
	Profiles    []Profile `json:"profiles,omitempty"`
	Status      *Status   `json:"status,omitempty"`
}

const policyLimitation = "No machine-backed authority or recovery guarantees."

// Module is a deep module around one project-scoped Policy Engagement.
// Execute is its only behavioural interface.
type Module struct {
	projectRoot string
	home        string
	id          string
	store       *policyrun.Store
}

func NewCurrent() (*Module, error) {
	projectRoot, err := currentPhysicalProject()
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, errors.New("POLICY_HOME_UNAVAILABLE: current user home is unavailable")
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	if !cleanAbsolutePath(stateHome) {
		return nil, errors.New("POLICY_STATE_HOME_INVALID: XDG_STATE_HOME must be a clean absolute path")
	}
	store, err := policyrun.NewStore(filepath.Join(stateHome, "open-agent-workflow", "policy-engagements"))
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(projectRoot))
	return &Module{
		projectRoot: projectRoot,
		home:        home,
		id:          "project-" + hex.EncodeToString(digest[:16]),
		store:       store,
	}, nil
}

func (module *Module) Execute(command Command) (Result, error) {
	if module == nil || module.store == nil {
		return Result{}, errors.New("POLICY_ENGAGEMENT_UNAVAILABLE: current project Engagement is unavailable")
	}
	switch command.Action {
	case ActionProfiles:
		return module.profiles()
	case ActionUse:
		return module.start(command)
	case ActionStatus:
		run, err := module.load()
		if err != nil {
			return Result{}, err
		}
		return statusResult(run)
	case ActionComplete, ActionReview, ActionApprove, ActionSatisfy, ActionIncident, ActionSwitch, ActionStop, ActionUncertain:
		return module.apply(command)
	default:
		return Result{}, fmt.Errorf("POLICY_ACTION_UNKNOWN: unknown action %q", command.Action)
	}
}

func (module *Module) profiles() (Result, error) {
	inventory, offer, err := module.inspect()
	_ = inventory
	if err != nil {
		return Result{}, err
	}
	result := Result{
		Assurance:   "policy-cooperative",
		Execution:   "current-host-session",
		Limitations: []string{policyLimitation},
		Profiles:    make([]Profile, 0, len(offer.Profiles)),
	}
	for _, candidate := range offer.Profiles {
		missing := make([]string, 0, len(candidate.Missing))
		for _, route := range candidate.Missing {
			missing = append(missing, route.Name)
		}
		incidents := make([]IncidentRoute, 0, len(candidate.IncidentRoutes))
		for _, route := range candidate.IncidentRoutes {
			status := "unavailable-if-triggered"
			if route.Available {
				status = "routable-if-triggered"
			}
			incidents = append(incidents, IncidentRoute{
				Incident: string(route.Incident), Skill: route.Skill,
				Mode: string(route.Mode), Status: status,
			})
		}
		result.Profiles = append(result.Profiles, Profile{
			Name:             string(candidate.ID),
			PolicySelectable: candidate.PolicySelectable,
			HostRoutable:     candidate.HostRoutable,
			Missing:          missing,
			IncidentRoutes:   incidents,
		})
	}
	return result, nil
}

func (module *Module) start(command Command) (Result, error) {
	intent := strings.TrimSpace(command.Intent)
	if intent == "" || intent != command.Intent || strings.IndexFunc(intent, unicode.IsControl) >= 0 {
		return Result{}, errors.New("POLICY_INTENT_INVALID: use requires one non-empty intent")
	}
	if command.Profile == "" {
		result, err := module.profiles()
		if err != nil {
			return Result{}, err
		}
		return result, errors.New("PROFILE_SELECTION_REQUIRED: choose an available Profile with --profile")
	}
	if command.Complexity != "ordinary" && command.Complexity != "complex" {
		return Result{}, errors.New("POLICY_ASSESSMENT_REQUIRED: use requires --complexity ordinary|complex")
	}
	if command.Risk != "normal" && command.Risk != "elevated" && command.Risk != "critical" {
		return Result{}, errors.New("POLICY_ASSESSMENT_REQUIRED: use requires --risk normal|elevated|critical")
	}
	inventory, offer, err := module.inspect()
	if err != nil {
		return Result{}, err
	}
	flow := policyflow.New()
	progress, err := flow.Start(inventory, policyflow.Selection{OfferRef: offer.Ref, Profile: command.Profile})
	if err != nil {
		return Result{}, err
	}
	state, err := flow.Export(progress)
	if err != nil {
		return Result{}, err
	}
	plan, err := policyPlan(intent, command.Complexity, command.Risk, progress, offer)
	if err != nil {
		return Result{}, err
	}
	run, err := module.store.Start(policyrun.Run{
		SchemaVersion: policyrun.RunSchemaV6,
		ID:            module.id,
		ProjectRoot:   module.projectRoot,
		Plan:          plan,
		Inventory:     inventory,
		State:         state,
	})
	if err != nil {
		return Result{}, err
	}
	return statusResult(run)
}

func (module *Module) apply(command Command) (Result, error) {
	run, err := module.store.Mutate(module.id, func(run policyrun.Run) (policyrun.Run, error) {
		if err := module.checkProject(run); err != nil {
			return policyrun.Run{}, err
		}
		inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: module.home})
		if err != nil {
			return policyrun.Run{}, err
		}
		flow := policyflow.New()
		progress, err := flow.Restore(run.Inventory, run.State)
		if err != nil {
			return policyrun.Run{}, err
		}
		event, err := eventFor(command, progress, flow, inventory)
		if err != nil {
			return policyrun.Run{}, err
		}
		progress, err = flow.Apply(inventory, event)
		if err != nil {
			return policyrun.Run{}, err
		}
		run.Inventory = inventory
		run.State, err = flow.Export(progress)
		if err == nil {
			offer, offerErr := flow.Offer(inventory)
			if offerErr != nil {
				return policyrun.Run{}, offerErr
			}
			responsibilities, projectionErr := policyrun.ProjectResponsibilities(progress.Plan.Profile, offer)
			if projectionErr != nil {
				return policyrun.Run{}, projectionErr
			}
			run.Plan.SelectedProfileCandidate = string(progress.Plan.Profile)
			run.Plan.Topology = progress.Plan.Topology
			run.Plan.AddOn = progress.Plan.AddOn
			run.Plan.Status = policyStatus(progress.Next)
			run.Plan.ResponsibilityMap = responsibilities
		}
		return run, err
	})
	if err != nil {
		return Result{}, err
	}
	return statusResult(run)
}

func eventFor(command Command, progress policyflow.Progress, flow *policyflow.Module, inventory policyflow.RouteInventory) (policyflow.Event, error) {
	switch command.Action {
	case ActionComplete:
		switch next := progress.Next.(type) {
		case policyflow.InvokeSkill:
			if next.Review {
				return nil, wrongAction("complete", progress.Next)
			}
			return policyflow.SkillCompleted{WorkRef: next.WorkRef}, nil
		case policyflow.AwaitUserSkill:
			if next.Review {
				return nil, wrongAction("complete", progress.Next)
			}
			return policyflow.SkillCompleted{WorkRef: next.WorkRef}, nil
		case policyflow.HostAction:
			if next.Review {
				return nil, wrongAction("complete", progress.Next)
			}
			return policyflow.HostActionCompleted{WorkRef: next.WorkRef}, nil
		default:
			return nil, wrongAction("complete", progress.Next)
		}
	case ActionReview:
		if command.Review != policyflow.ReviewClean && command.Review != policyflow.ReviewFindings {
			return nil, fmt.Errorf("POLICY_REVIEW_OUTCOME_UNKNOWN: unknown review outcome %q", command.Review)
		}
		switch next := progress.Next.(type) {
		case policyflow.InvokeSkill:
			if next.Review {
				return policyflow.ReviewCompleted{WorkRef: next.WorkRef, Outcome: command.Review}, nil
			}
		case policyflow.AwaitUserSkill:
			if next.Review {
				return policyflow.ReviewCompleted{WorkRef: next.WorkRef, Outcome: command.Review}, nil
			}
		case policyflow.HostAction:
			if next.Review {
				return policyflow.ReviewCompleted{WorkRef: next.WorkRef, Outcome: command.Review}, nil
			}
		}
		return nil, wrongAction("review", progress.Next)
	case ActionApprove:
		if next, ok := progress.Next.(policyflow.UserGate); ok {
			return policyflow.UserGateApproved{GateRef: next.GateRef}, nil
		}
		return nil, wrongAction("approve", progress.Next)
	case ActionSatisfy:
		if next, ok := progress.Next.(policyflow.HostGate); ok {
			return policyflow.HostGateSatisfied{GateRef: next.GateRef}, nil
		}
		return nil, wrongAction("satisfy", progress.Next)
	case ActionIncident:
		if !validIncident(command.Incident) {
			return nil, fmt.Errorf("POLICY_INCIDENT_UNKNOWN: unknown incident type %q", command.Incident)
		}
		ref, ok := executableRef(progress.Next)
		if !ok {
			return nil, wrongAction("incident", progress.Next)
		}
		return policyflow.IncidentReported{WorkRef: ref, Incident: command.Incident, Reason: strings.TrimSpace(command.Reason)}, nil
	case ActionSwitch:
		if command.Profile == "" {
			return nil, errors.New("PROFILE_SELECTION_REQUIRED: switch requires a Profile")
		}
		current, ok := currentRef(progress.Next)
		if !ok {
			return nil, wrongAction("switch", progress.Next)
		}
		offer, err := flow.Offer(inventory)
		if err != nil {
			return nil, err
		}
		return policyflow.ProfileSwitchRequested{Current: current, OfferRef: offer.Ref, Profile: command.Profile}, nil
	case ActionStop:
		current, ok := currentRef(progress.Next)
		if !ok {
			return nil, wrongAction("stop", progress.Next)
		}
		reason := strings.TrimSpace(command.Reason)
		if reason == "" {
			reason = "stopped by user"
		}
		return policyflow.StopRequested{Current: current, Reason: reason}, nil
	case ActionUncertain:
		if strings.TrimSpace(command.Reason) == "" {
			return nil, errors.New("POLICY_REASON_REQUIRED: uncertain requires --reason")
		}
		ref, ok := executableRef(progress.Next)
		if !ok {
			return nil, wrongAction("uncertain", progress.Next)
		}
		return policyflow.ExecutionUncertain{WorkRef: ref, Reason: strings.TrimSpace(command.Reason)}, nil
	default:
		return nil, fmt.Errorf("POLICY_ACTION_UNKNOWN: unknown action %q", command.Action)
	}
}

func (module *Module) inspect() (policyflow.RouteInventory, policyflow.Offer, error) {
	inventory, err := policyroute.Inspect(policyroute.Options{HostID: "codex", Home: module.home})
	if err != nil {
		return nil, policyflow.Offer{}, err
	}
	offer, err := policyflow.New().Offer(inventory)
	return inventory, offer, err
}

func (module *Module) load() (policyrun.Run, error) {
	run, err := module.store.Load(module.id)
	if err != nil {
		return policyrun.Run{}, err
	}
	if err := module.checkProject(run); err != nil {
		return policyrun.Run{}, err
	}
	return run, nil
}

func (module *Module) checkProject(run policyrun.Run) error {
	if run.ProjectRoot != module.projectRoot {
		return errors.New("POLICY_ONLY_CONTEXT_UNCERTAIN: saved Engagement belongs to a different physical project")
	}
	return nil
}

func statusResult(run policyrun.Run) (Result, error) {
	progress, err := run.State.Progress()
	if err != nil {
		return Result{}, err
	}
	status := &Status{Deliverable: run.Plan.Deliverable, Profile: string(progress.Plan.Profile)}
	status.CanSwitch = currentRefAvailable(progress.Next) && run.State.SwitchBoundary != ""
	switch next := progress.Next.(type) {
	case policyflow.InvokeSkill:
		status.State, status.Next, status.Name, status.Phase = "active", "invoke-skill", next.Skill, phase(next.Covers)
		if next.Review {
			status.Next = "review-skill"
		}
	case policyflow.AwaitUserSkill:
		status.State, status.Next, status.Name, status.Phase = "active", "invoke-user-skill", next.Skill, phase(next.Covers)
		if next.Review {
			status.Next = "review-user-skill"
		}
	case policyflow.HostAction:
		status.State, status.Next, status.Name, status.Phase = "active", "host-action", next.Action, phase(next.Covers)
		if next.Review {
			status.Next = "review-host-action"
		}
	case policyflow.UserGate:
		status.State, status.Next, status.Name = "active", "user-approval", next.Gate
	case policyflow.HostGate:
		status.State, status.Next, status.Name = "active", "host-confirmation", next.Gate
	case policyflow.Done:
		status.State = "completed"
	case policyflow.Stopped:
		status.State, status.Reason = "stopped", next.Reason
	case policyflow.Blocked:
		status.State, status.Reason = "blocked", next.Reason
	default:
		return Result{}, errors.New("POLICY_STATE_INVALID: current action is invalid")
	}
	return Result{
		Assurance: "policy-cooperative", Execution: "current-host-session",
		Limitations: []string{policyLimitation}, Status: status,
	}, nil
}

func policyPlan(deliverable, complexity, risk string, progress policyflow.Progress, offer policyflow.Offer) (policyrun.Plan, error) {
	responsibilities, err := policyrun.ProjectResponsibilities(progress.Plan.Profile, offer)
	if err != nil {
		return policyrun.Plan{}, err
	}
	return policyrun.Plan{
		Assurance:                policyrun.AssurancePolicyCooperative,
		ActivationSource:         policyrun.ActivationUserExplicit,
		Deliverable:              deliverable,
		Mode:                     policyrun.ModeWorkflow,
		Complexity:               complexity,
		Risk:                     risk,
		SelectedProfileCandidate: string(progress.Plan.Profile),
		SelectionSource:          policyrun.SelectionUserExplicit,
		Topology:                 progress.Plan.Topology,
		ResponsibilityMap:        responsibilities,
		AcceptedLimitations:      []string{policyLimitation},
		AddOn:                    progress.Plan.AddOn,
		Status:                   policyStatus(progress.Next),
	}, nil
}

func policyStatus(next policyflow.NextWork) string {
	switch next.(type) {
	case policyflow.Done:
		return policyrun.StatusCompleted
	case policyflow.Stopped, policyflow.Blocked:
		return policyrun.StatusStopped
	default:
		return policyrun.StatusActive
	}
}

func currentRefAvailable(next policyflow.NextWork) bool {
	_, ok := currentRef(next)
	return ok
}

func executableRef(next policyflow.NextWork) (policyflow.WorkRef, bool) {
	switch value := next.(type) {
	case policyflow.InvokeSkill:
		return value.WorkRef, true
	case policyflow.AwaitUserSkill:
		return value.WorkRef, true
	case policyflow.HostAction:
		return value.WorkRef, true
	default:
		return policyflow.WorkRef{}, false
	}
}

func currentRef(next policyflow.NextWork) (policyflow.CurrentRef, bool) {
	switch value := next.(type) {
	case policyflow.InvokeSkill:
		return value.WorkRef, true
	case policyflow.AwaitUserSkill:
		return value.WorkRef, true
	case policyflow.HostAction:
		return value.WorkRef, true
	case policyflow.UserGate:
		return value.GateRef, true
	case policyflow.HostGate:
		return value.GateRef, true
	default:
		return nil, false
	}
}

func wrongAction(action string, next policyflow.NextWork) error {
	return fmt.Errorf("POLICY_ACTION_NOT_APPLICABLE: %s cannot record the current %s", action, nextKind(next))
}

func nextKind(next policyflow.NextWork) string {
	switch value := next.(type) {
	case policyflow.InvokeSkill:
		if value.Review {
			return "review outcome"
		}
		return "skill action"
	case policyflow.AwaitUserSkill:
		if value.Review {
			return "review outcome"
		}
		return "skill action"
	case policyflow.HostAction:
		if value.Review {
			return "review outcome"
		}
		return "Host action"
	case policyflow.UserGate:
		return "user approval"
	case policyflow.HostGate:
		return "Host confirmation"
	case policyflow.Done:
		return "completed Engagement"
	case policyflow.Stopped:
		return "stopped Engagement"
	case policyflow.Blocked:
		return "blocked Engagement"
	default:
		return "unknown action"
	}
}

func phase(covers []policyflow.LifecycleSlot) string {
	if len(covers) == 0 {
		return ""
	}
	return string(covers[0])
}

func validIncident(value policyflow.IncidentType) bool {
	switch value {
	case policyflow.IncidentBuildFailure,
		policyflow.IncidentDependencyFailure,
		policyflow.IncidentFunctionalFailure,
		policyflow.IncidentHardBug,
		policyflow.IncidentPerformanceRegression,
		policyflow.IncidentTypeFailure:
		return true
	default:
		return false
	}
}

func currentPhysicalProject() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("POLICY_PROJECT_UNAVAILABLE: %w", err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("POLICY_PROJECT_UNAVAILABLE: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("POLICY_PROJECT_UNAVAILABLE: %w", err)
	}
	if !cleanAbsolutePath(root) {
		return "", errors.New("POLICY_PROJECT_UNAVAILABLE: current project path is invalid")
	}
	for candidate := root; ; candidate = filepath.Dir(candidate) {
		if _, err := os.Lstat(filepath.Join(candidate, ".git")); err == nil {
			return candidate, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("POLICY_PROJECT_UNAVAILABLE: inspect project root: %w", err)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return root, nil
		}
	}
}

func cleanAbsolutePath(value string) bool {
	return value != "" && filepath.IsAbs(value) && filepath.Clean(value) == value && strings.IndexAny(value, "\r\n\x00") < 0
}
