package policyrun

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"
	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
)

const (
	RunSchemaV6  = "oaw.policy-engagement/v6"
	maximumBytes = 4 << 20
)

const (
	AssurancePolicyCooperative = "policy-cooperative"
	ActivationUserExplicit     = "user-explicit"
	ModeWorkflow               = "WORKFLOW"
	SelectionUserExplicit      = "user-explicit"
	TopologyCurrent            = "CURRENT"
	AddOnNone                  = "NONE"
	StatusActive               = "active"
	StatusCompleted            = "completed"
	StatusStopped              = "stopped"
)

type Responsibility struct {
	Route string `json:"route"`
	Mode  string `json:"mode"`
	Kind  string `json:"kind"`
}

type ResponsibilityMap map[string][]Responsibility

// Plan is cooperative operating context. It is deliberately separate from
// machine Lifecycle Bundle, Grant, Lease, Receipt, and revision records.
type Plan struct {
	Assurance                string            `json:"assurance"`
	ActivationSource         string            `json:"activation_source"`
	Deliverable              string            `json:"deliverable"`
	Mode                     string            `json:"mode"`
	Complexity               string            `json:"complexity"`
	Risk                     string            `json:"risk"`
	SelectedProfileCandidate string            `json:"selected_profile_candidate"`
	SelectionSource          string            `json:"selection_source"`
	Topology                 string            `json:"topology"`
	ResponsibilityMap        ResponsibilityMap `json:"responsibility_map"`
	AcceptedLimitations      []string          `json:"accepted_limitations"`
	AddOn                    string            `json:"add_on"`
	Status                   string            `json:"status"`
}

type Run struct {
	SchemaVersion string                    `json:"schema_version"`
	ID            string                    `json:"id"`
	ProjectRoot   string                    `json:"project_root"`
	Plan          Plan                      `json:"plan"`
	Inventory     policyflow.RouteInventory `json:"route_inventory"`
	State         policyflow.State          `json:"state"`
}

type Store struct{ root string }

func NewStore(root string) (*Store, error) {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root || strings.IndexAny(root, "\r\n\x00") >= 0 {
		return nil, errors.New("POLICY_STATE_ROOT_INVALID: root must be a clean absolute path")
	}
	return &Store{root: root}, nil
}

func (store *Store) Start(run Run) (Run, error) {
	if store == nil {
		return Run{}, errors.New("POLICY_STATE_ROOT_INVALID: store is nil")
	}
	if err := validateRun(run); err != nil {
		return Run{}, err
	}
	return run, store.withLock(run.ID, func() error {
		if _, err := os.Stat(store.runPath(run.ID)); err == nil {
			current, err := store.load(run.ID)
			if err != nil {
				return err
			}
			progress, err := current.State.Progress()
			if err != nil {
				return err
			}
			switch progress.Next.(type) {
			case policyflow.Done, policyflow.Stopped, policyflow.Blocked:
				return store.write(run)
			default:
				return errors.New("POLICY_ENGAGEMENT_ACTIVE: the current project already has an active Engagement")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return store.write(run)
	})
}

func (store *Store) Load(id string) (Run, error) {
	if store == nil {
		return Run{}, errors.New("POLICY_STATE_ROOT_INVALID: store is nil")
	}
	if err := validateRunID(id); err != nil {
		return Run{}, err
	}
	return store.load(id)
}

func (store *Store) Mutate(id string, action func(Run) (Run, error)) (Run, error) {
	if store == nil {
		return Run{}, errors.New("POLICY_STATE_ROOT_INVALID: store is nil")
	}
	if err := validateRunID(id); err != nil {
		return Run{}, err
	}
	var result Run
	err := store.withLock(id, func() error {
		run, err := store.load(id)
		if err != nil {
			return err
		}
		result, err = action(run)
		if err != nil {
			return err
		}
		if err := validateRun(result); err != nil {
			return err
		}
		return store.write(result)
	})
	return result, err
}

func (store *Store) load(id string) (Run, error) {
	file, err := os.Open(store.runPath(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Run{}, errors.New("POLICY_RUN_NOT_FOUND: run does not exist")
		}
		return Run{}, fmt.Errorf("POLICY_STATE_READ_FAILED: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maximumBytes {
		return Run{}, errors.New("POLICY_STATE_READ_FAILED: invalid run file")
	}
	raw, err := io.ReadAll(io.LimitReader(file, maximumBytes+1))
	if err != nil || len(raw) > maximumBytes {
		return Run{}, errors.New("POLICY_STATE_READ_FAILED: run file is too large")
	}
	var header struct {
		SchemaVersion string `json:"schema_version"`
		Plan          *struct {
			SchemaVersion string `json:"schema_version"`
		} `json:"plan"`
	}
	if err := json.Unmarshal(raw, &header); err == nil && isLegacySchema(header.SchemaVersion, header.Plan) {
		return Run{}, errors.New("POLICY_ONLY_CONTEXT_UNCERTAIN: legacy Policy state cannot be mapped to the current project Engagement; start it again explicitly")
	}
	var run Run
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&run); err != nil {
		return Run{}, fmt.Errorf("POLICY_STATE_INVALID: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Run{}, errors.New("POLICY_STATE_INVALID: trailing data")
	}
	if err := validateRun(run); err != nil {
		return Run{}, err
	}
	return run, nil
}

func isLegacySchema(schema string, plan *struct {
	SchemaVersion string `json:"schema_version"`
}) bool {
	if plan != nil && plan.SchemaVersion == "oaw.policy-workflow-plan/v1" {
		return true
	}
	switch schema {
	case "oaw.policy-run/v3", "oaw.policy-engagement/v4", "oaw.policy-engagement/v5":
		return true
	default:
		return false
	}
}

func validateRun(run Run) error {
	if err := validateRunID(run.ID); err != nil {
		return err
	}
	if run.SchemaVersion != RunSchemaV6 ||
		run.ProjectRoot == "" || !filepath.IsAbs(run.ProjectRoot) || filepath.Clean(run.ProjectRoot) != run.ProjectRoot ||
		strings.IndexAny(run.ProjectRoot, "\r\n\x00") >= 0 || run.State.SchemaVersion != policyflow.StateSchemaV4 ||
		validatePlan(run.Plan) != nil {
		return errors.New("POLICY_RUN_INVALID: missing Policy-only state fields")
	}
	module := policyflow.New()
	progress, err := module.Restore(run.Inventory, run.State)
	if err != nil {
		return fmt.Errorf("POLICY_RUN_INVALID: %w", err)
	}
	if run.Plan.SelectedProfileCandidate != string(progress.Plan.Profile) ||
		run.Plan.Topology != progress.Plan.Topology || run.Plan.AddOn != progress.Plan.AddOn ||
		run.Plan.Status != statusFor(progress.Next) {
		return errors.New("POLICY_RUN_INVALID: Policy Workflow Plan does not match reducer state")
	}
	expected, err := expectedResponsibilities(run.Inventory, progress.Plan.Profile)
	if err != nil || !equalResponsibilities(run.Plan.ResponsibilityMap, expected) {
		return errors.New("POLICY_RUN_INVALID: Policy responsibility map does not match reducer state")
	}
	return nil
}

func validatePlan(plan Plan) error {
	if plan.Assurance != AssurancePolicyCooperative || plan.ActivationSource != ActivationUserExplicit ||
		plan.Deliverable == "" || strings.TrimSpace(plan.Deliverable) != plan.Deliverable || strings.IndexAny(plan.Deliverable, "\r\n\x00") >= 0 ||
		plan.Mode != ModeWorkflow || !oneOf(plan.Complexity, "ordinary", "complex") ||
		!oneOf(plan.Risk, "normal", "elevated", "critical") || plan.SelectedProfileCandidate == "" ||
		plan.SelectionSource != SelectionUserExplicit || plan.Topology != TopologyCurrent || plan.AddOn != AddOnNone ||
		!oneOf(plan.Status, StatusActive, StatusCompleted, StatusStopped) || len(plan.AcceptedLimitations) == 0 {
		return errors.New("invalid Policy Workflow Plan")
	}
	if len(plan.ResponsibilityMap) != len(policySlots) {
		return errors.New("invalid Policy responsibility map")
	}
	for _, slot := range policySlots {
		responsibilities, ok := plan.ResponsibilityMap[slot]
		if !ok || len(responsibilities) == 0 {
			return errors.New("invalid Policy responsibility map")
		}
		for _, responsibility := range responsibilities {
			if responsibility.Route == "" || !oneOf(responsibility.Mode, "host-visible", "user-explicit", "host-controlled", "unavailable", "conditional-unavailable") ||
				!oneOf(responsibility.Kind, "skill", "credited-skill", "host-action", "user-gate", "host-gate", "incident-handler") {
				return errors.New("invalid Policy responsibility")
			}
		}
	}
	for _, limitation := range plan.AcceptedLimitations {
		if limitation == "" || strings.TrimSpace(limitation) != limitation || strings.IndexAny(limitation, "\r\n\x00") >= 0 {
			return errors.New("invalid Policy limitation")
		}
	}
	return nil
}

func statusFor(next policyflow.NextWork) string {
	switch next.(type) {
	case policyflow.Done:
		return StatusCompleted
	case policyflow.Stopped, policyflow.Blocked:
		return StatusStopped
	default:
		return StatusActive
	}
}

func oneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func (store *Store) write(run Run) error {
	if err := ensurePrivateDirectory(store.root); err != nil {
		return err
	}
	raw, err := json.Marshal(run)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(store.root, ".policy-run-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		_ = temporary.Close()
		if !keep {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := replaceRunFile(temporaryPath, store.runPath(run.ID)); err != nil {
		return err
	}
	keep = true
	return syncRunDirectory(store.root)
}

func (store *Store) withLock(id string, action func() error) error {
	if err := ensurePrivateDirectory(store.root); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(store.root, id+".lock"))
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("POLICY_STATE_LOCK_FAILED: %w", err)
	}
	defer func() {
		_ = lock.Unlock()
		_ = lock.Close()
	}()
	return action()
}

func (store *Store) runPath(id string) string { return filepath.Join(store.root, id+".json") }

func validateRunID(id string) error {
	if id == "" || strings.TrimSpace(id) != id || strings.ContainsAny(id, "/\\:\x00\r\n") {
		return errors.New("POLICY_RUN_ID_INVALID: id contains unsafe characters")
	}
	return nil
}

func ensurePrivateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("path is not a private directory")
	}
	return os.Chmod(path, 0o700)
}
