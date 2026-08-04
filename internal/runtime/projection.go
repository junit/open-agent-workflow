package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

const (
	workflowProjectionSchemaV2 = "oaw.workflow-projection/v2"
	projectionLagSchemaV1      = "oaw.projection-lag/v1"
	projectionFailureReason    = "PROJECTION_WRITE_FAILED"
	ProjectionLagCurrent       = ProjectionLagStatus("current")
	ProjectionLagging          = ProjectionLagStatus("lagging")
)

type ProjectionLagStatus string

type ProjectionSink interface {
	WriteProjection(WorkflowProjection) error
}

type WorkflowProjection struct {
	SchemaVersion          string              `json:"schema_version"`
	RunID                  string              `json:"run_id"`
	Revision               uint64              `json:"revision"`
	RevisionDigest         string              `json:"revision_digest"`
	StateDigest            string              `json:"state_digest"`
	Status                 RunStatus           `json:"status"`
	Event                  string              `json:"event"`
	ConfigurationDigest    string              `json:"configuration_digest"`
	ActiveTicket           string              `json:"active_ticket"`
	LagStatus              ProjectionLagStatus `json:"lag_status"`
	EvidenceReferences     []EvidenceReference `json:"evidence_references"`
	Profile                string              `json:"profile,omitempty"`
	BundleID               string              `json:"bundle_id,omitempty"`
	BundleDigest           string              `json:"bundle_digest,omitempty"`
	BundleGeneration       uint64              `json:"bundle_generation,omitempty"`
	Stage                  string              `json:"stage,omitempty"`
	GraphDigest            string              `json:"graph_digest,omitempty"`
	HostID                 string              `json:"host_id,omitempty"`
	BindingInventoryDigest string              `json:"binding_inventory_digest,omitempty"`
	HostIntegrationID      string              `json:"host_integration_id,omitempty"`
	HostIntegrationDigest  string              `json:"host_integration_digest,omitempty"`
	HostManifestDigest     string              `json:"host_manifest_digest,omitempty"`
	HostAuditDigest        string              `json:"host_audit_digest,omitempty"`
	HostConformanceDigest  string              `json:"host_conformance_digest,omitempty"`
	Digest                 string              `json:"digest"`
}

type FilesystemProjectionSink struct {
	root string
}

type projectionLagMarker struct {
	SchemaVersion  string              `json:"schema_version"`
	RunID          string              `json:"run_id"`
	RevisionDigest string              `json:"revision_digest"`
	Status         ProjectionLagStatus `json:"status"`
	Lag            ProjectionLag       `json:"lag"`
	Digest         string              `json:"digest"`
}

func projectionSinkFromOptions(options ProjectionOptions) (ProjectionSink, error) {
	if options.Sink != nil && options.Root != "" {
		return nil, runtimeError("PROJECTION_CONFIGURATION_INVALID", "projection Root and Sink are mutually exclusive", nil)
	}
	if options.Sink != nil {
		return options.Sink, nil
	}
	if options.Root == "" {
		return nil, nil
	}
	return NewFilesystemProjectionSink(options.Root)
}

func NewFilesystemProjectionSink(root string) (*FilesystemProjectionSink, error) {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, runtimeError("PROJECTION_DESTINATION_INVALID", "projection root must be a clean absolute path", nil)
	}
	if err := ensurePrivateProjectionRoot(root); err != nil {
		return nil, runtimeError("PROJECTION_DESTINATION_INVALID", "prepare projection root", err)
	}
	return &FilesystemProjectionSink{root: root}, nil
}

func (sink *FilesystemProjectionSink) WriteProjection(value WorkflowProjection) error {
	if sink == nil || validateWorkflowProjection(value) != nil {
		return runtimeError("PROJECTION_INVALID", "Workflow projection is invalid", nil)
	}
	if err := ensurePrivateProjectionRoot(sink.root); err != nil {
		return runtimeError("PROJECTION_WRITE_FAILED", "validate projection root", err)
	}
	runRoot := filepath.Join(sink.root, value.RunID)
	if err := ensurePrivateProjectionRunRoot(sink.root, runRoot); err != nil {
		return runtimeError("PROJECTION_WRITE_FAILED", "prepare projection Run root", err)
	}
	rawJSON, err := canonicaljson.Marshal(value)
	if err != nil {
		return runtimeError("PROJECTION_WRITE_FAILED", "encode Workflow projection", err)
	}
	if err := atomicWriteFile(filepath.Join(runRoot, "workflow.json"), rawJSON, 0o600); err != nil {
		return runtimeError("PROJECTION_WRITE_FAILED", "write JSON projection", err)
	}
	if err := atomicWriteFile(filepath.Join(runRoot, "workflow.md"), renderWorkflowProjectionMarkdown(value), 0o600); err != nil {
		return runtimeError("PROJECTION_WRITE_FAILED", "write Markdown projection", err)
	}
	return nil
}

func ensurePrivateProjectionRoot(root string) error {
	if err := rejectProjectionSymlinkComponents(root); err != nil {
		return err
	}
	if info, err := os.Lstat(root); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("projection root is not a physical directory")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := ensurePrivateDir(root); err != nil {
		return err
	}
	physical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	if filepath.Clean(physical) != root {
		return errors.New("projection root resolves through a symlink")
	}
	return nil
}

func rejectProjectionSymlinkComponents(path string) error {
	volume := filepath.VolumeName(path)
	remainder := strings.TrimPrefix(path, volume)
	current := volume + string(os.PathSeparator)
	for _, component := range strings.Split(strings.TrimPrefix(remainder, string(os.PathSeparator)), string(os.PathSeparator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("projection path contains a symlink")
		}
	}
	return nil
}

func ensurePrivateProjectionRunRoot(root, runRoot string) error {
	if filepath.Dir(runRoot) != root || !runIDPattern.MatchString(filepath.Base(runRoot)) {
		return errors.New("projection Run root escapes the configured root")
	}
	info, err := os.Lstat(runRoot)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(runRoot, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(runRoot)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("projection Run root is not a physical directory")
	}
	return os.Chmod(runRoot, 0o700)
}

func newWorkflowProjection(record revisionRecord) (WorkflowProjection, error) {
	if record.Snapshot.Workflow == nil || record.Snapshot.RunID != record.RunID || record.Snapshot.Revision != record.Revision || !validDigest(record.Digest) || !validDigest(record.StateDigest) {
		return WorkflowProjection{}, runtimeError("PROJECTION_INVALID", "committed Workflow revision is invalid", nil)
	}
	workflow := record.Snapshot.Workflow
	evidence, err := workflowProjectionEvidence(workflow.Observations)
	if err != nil {
		return WorkflowProjection{}, err
	}
	value := WorkflowProjection{
		SchemaVersion: workflowProjectionSchemaV2, RunID: record.RunID, Revision: record.Revision,
		RevisionDigest: record.Digest, StateDigest: record.StateDigest, Status: record.Snapshot.Status,
		Event: record.Event, ConfigurationDigest: workflow.ConfigurationDigest,
		ActiveTicket: workflow.ActiveTicket, LagStatus: ProjectionLagCurrent,
		EvidenceReferences: evidence,
	}
	if workflow.ActiveGeneration > 0 {
		bundle, err := workflowActiveBundle(record.Snapshot)
		if err != nil {
			return WorkflowProjection{}, err
		}
		value.BundleID = bundle.ID
		value.BundleDigest = bundle.Digest
		value.Profile = bundle.Selection.Profile
		value.BundleGeneration = bundle.Generation
		value.Stage = workflow.ActiveNodeID
		value.GraphDigest = bundle.GraphDigest
		value.HostID = bundle.HostID
		value.BindingInventoryDigest = bundle.BindingInventoryDigest
		value.HostIntegrationID = bundle.HostIntegrationID
		value.HostIntegrationDigest = bundle.HostIntegrationDigest
		value.HostManifestDigest = bundle.HostManifestDigest
		value.HostAuditDigest = bundle.HostAuditDigest
		value.HostConformanceDigest = bundle.HostConformanceDigest
	}
	digestValue := value
	digestValue.Digest = ""
	digest, _, err := canonicaljson.Digest(digestValue)
	if err != nil {
		return WorkflowProjection{}, runtimeError("PROJECTION_INVALID", "digest Workflow projection", err)
	}
	value.Digest = digest
	return value, nil
}

func workflowProjectionEvidence(observations []StageObservation) ([]EvidenceReference, error) {
	evidence := make([]EvidenceReference, 0)
	for _, observation := range observations {
		for _, reference := range observation.EvidenceReferences {
			if validateIdentifier(reference.Reference) != nil || !validDigest(reference.Digest) {
				return nil, runtimeError("PROJECTION_INVALID", "Workflow evidence reference is invalid", nil)
			}
			evidence = append(evidence, reference)
		}
	}
	sort.Slice(evidence, func(left, right int) bool {
		if evidence[left].Reference == evidence[right].Reference {
			return evidence[left].Digest < evidence[right].Digest
		}
		return evidence[left].Reference < evidence[right].Reference
	})
	result := make([]EvidenceReference, 0, len(evidence))
	for _, reference := range evidence {
		if len(result) == 0 || result[len(result)-1] != reference {
			result = append(result, reference)
		}
	}
	return result, nil
}

func validateWorkflowProjection(value WorkflowProjection) error {
	if value.SchemaVersion != workflowProjectionSchemaV2 || !runIDPattern.MatchString(value.RunID) || value.Revision == 0 || !validDigest(value.RevisionDigest) || !validDigest(value.StateDigest) || !validDigest(value.ConfigurationDigest) || !validDigest(value.Digest) || value.Status == "" || value.Event == "" || value.ActiveTicket != "" && validateIdentifier(value.ActiveTicket) != nil || value.LagStatus != ProjectionLagCurrent || value.EvidenceReferences == nil {
		return runtimeError("PROJECTION_INVALID", "invalid Workflow projection identity", nil)
	}
	for index, reference := range value.EvidenceReferences {
		if validateIdentifier(reference.Reference) != nil || !validDigest(reference.Digest) || index > 0 && !projectionEvidenceLess(value.EvidenceReferences[index-1], reference) {
			return runtimeError("PROJECTION_INVALID", "invalid Workflow projection evidence", nil)
		}
	}
	if value.BundleGeneration == 0 {
		if value.Profile != "" || value.BundleID != "" || value.BundleDigest != "" || value.Stage != "" || value.GraphDigest != "" || value.HostID != "" || value.BindingInventoryDigest != "" || value.HostIntegrationID != "" || value.HostIntegrationDigest != "" || value.HostManifestDigest != "" || value.HostAuditDigest != "" || value.HostConformanceDigest != "" || len(value.EvidenceReferences) != 0 {
			return runtimeError("PROJECTION_INVALID", "unselected Workflow projection contains Bundle state", nil)
		}
	} else if validateIdentifier(value.Profile) != nil || !bundleIDPattern.MatchString(value.BundleID) || !validDigest(value.BundleDigest) || validateIdentifier(value.Stage) != nil || !validDigest(value.GraphDigest) || invalidProjectionHostID(value.HostID) || !validDigest(value.BindingInventoryDigest) || validateIdentifier(value.HostIntegrationID) != nil || !validDigest(value.HostIntegrationDigest) || !validDigest(value.HostManifestDigest) || !validDigest(value.HostAuditDigest) || !validDigest(value.HostConformanceDigest) {
		return runtimeError("PROJECTION_INVALID", "selected Workflow projection is incomplete", nil)
	}
	stored := value.Digest
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(value)
	if err != nil || digest != stored {
		return runtimeError("PROJECTION_INVALID", "Workflow projection digest mismatch", err)
	}
	return nil
}

func invalidProjectionHostID(value string) bool {
	_, err := catalog.ParseLocalID(value)
	return err != nil
}

func projectionEvidenceLess(left, right EvidenceReference) bool {
	if left.Reference == right.Reference {
		return left.Digest < right.Digest
	}
	return left.Reference < right.Reference
}

func renderWorkflowProjectionMarkdown(value WorkflowProjection) []byte {
	var output strings.Builder
	output.WriteString("# OAW Workflow Projection\n\n")
	fmt.Fprintf(&output, "- Run: `%s`\n", value.RunID)
	fmt.Fprintf(&output, "- Revision: `%d`\n", value.Revision)
	fmt.Fprintf(&output, "- Revision digest: `%s`\n", value.RevisionDigest)
	fmt.Fprintf(&output, "- State digest: `%s`\n", value.StateDigest)
	fmt.Fprintf(&output, "- Status: `%s`\n", value.Status)
	fmt.Fprintf(&output, "- Event: `%s`\n", value.Event)
	fmt.Fprintf(&output, "- Active ticket: `%s`\n", value.ActiveTicket)
	fmt.Fprintf(&output, "- Lag status: `%s`\n", value.LagStatus)
	if value.BundleID != "" {
		fmt.Fprintf(&output, "- Profile: `%s`\n", value.Profile)
		fmt.Fprintf(&output, "- Bundle: `%s`\n", value.BundleID)
		fmt.Fprintf(&output, "- Bundle digest: `%s`\n", value.BundleDigest)
		fmt.Fprintf(&output, "- Bundle generation: `%d`\n", value.BundleGeneration)
		fmt.Fprintf(&output, "- Stage: `%s`\n", value.Stage)
		fmt.Fprintf(&output, "- Graph digest: `%s`\n", value.GraphDigest)
		fmt.Fprintf(&output, "- Host: `%s`\n", value.HostID)
		fmt.Fprintf(&output, "- Binding Inventory digest: `%s`\n", value.BindingInventoryDigest)
		fmt.Fprintf(&output, "- Host Integration: `%s`\n", value.HostIntegrationID)
		fmt.Fprintf(&output, "- Host Integration digest: `%s`\n", value.HostIntegrationDigest)
		fmt.Fprintf(&output, "- Host Manifest digest: `%s`\n", value.HostManifestDigest)
		fmt.Fprintf(&output, "- Host audit digest: `%s`\n", value.HostAuditDigest)
		fmt.Fprintf(&output, "- Host conformance digest: `%s`\n", value.HostConformanceDigest)
	}
	if len(value.EvidenceReferences) > 0 {
		output.WriteString("\n## Evidence References\n\n")
		for _, reference := range value.EvidenceReferences {
			fmt.Fprintf(&output, "- `%s` (`%s`)\n", reference.Reference, reference.Digest)
		}
	}
	return []byte(output.String())
}

func (engine *Engine) projectCommittedWorkflow(record revisionRecord) {
	if engine.projection == nil || record.Snapshot.Workflow == nil {
		return
	}
	value, err := newWorkflowProjection(record)
	if err == nil {
		err = writeProjectionSafely(engine.projection, value)
	}
	if err != nil {
		_ = engine.journal.recordProjectionLag(record)
	}
}

func writeProjectionSafely(sink ProjectionSink, value WorkflowProjection) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("projection sink panicked")
		}
	}()
	return sink.WriteProjection(value)
}

func (value *journal) recordProjectionLag(record revisionRecord) error {
	marker := projectionLagMarker{
		SchemaVersion: projectionLagSchemaV1, RunID: record.RunID, RevisionDigest: record.Digest,
		Status: ProjectionLagging,
		Lag:    ProjectionLag{Revision: record.Revision, Digest: record.Digest, Reason: projectionFailureReason},
	}
	digestValue := marker
	digestValue.Digest = ""
	digest, _, err := canonicaljson.Digest(digestValue)
	if err != nil {
		return err
	}
	marker.Digest = digest
	raw, err := canonicaljson.Marshal(marker)
	if err != nil {
		return err
	}
	lagRoot := filepath.Join(value.stateRoot, "projection-lag")
	if err := ensurePrivateDir(lagRoot); err != nil {
		return err
	}
	runRoot := filepath.Join(lagRoot, record.RunID)
	if err := ensurePrivateDir(runRoot); err != nil {
		return err
	}
	path := filepath.Join(runRoot, revisionFileName(record.Revision))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	remove := true
	defer func() {
		_ = file.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	if _, err := file.Write(raw); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := syncDirectory(runRoot); err != nil {
		return err
	}
	remove = false
	return nil
}
