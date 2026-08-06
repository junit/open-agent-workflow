package dogfood

import (
	"encoding/json"

	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
)

const (
	pilotSchema      = "oaw.current-dogfood/v1"
	providerID       = "local/open-code-review"
	capabilityID     = "review"
	profileID        = "local/ocr-readonly-workflow"
	integrationID    = "local/codex-current-dogfood"
	hostID           = "codex"
	bindingReference = "open-code-review:review"
	providerFile     = "provider.json"
	profileFile      = "profile.json"
	integrationFile  = "host-integration.toml"
	userConfigFile   = "config.toml"
)

type repositoryFingerprint struct {
	Root         string `json:"root"`
	Commit       string `json:"commit"`
	StatusDigest string `json:"status_digest"`
	SkillPath    string `json:"skill_path"`
	SkillDigest  string `json:"skill_digest"`
}

type pilotRecord struct {
	SchemaVersion           string                `json:"schema_version"`
	EvidenceRoot            string                `json:"evidence_root"`
	ConfigRoot              string                `json:"config_root"`
	StateRoot               string                `json:"state_root"`
	WorkflowID              string                `json:"workflow_id"`
	Profile                 string                `json:"profile"`
	Repository              repositoryFingerprint `json:"repository"`
	HostSessionDigest       string                `json:"host_session_digest"`
	EnvironmentReportDigest string                `json:"environment_report_digest"`
	InventoryDigest         string                `json:"inventory_digest"`
	ConfigurationDigest     string                `json:"configuration_digest"`
	ResolutionDigest        string                `json:"resolution_digest"`
	RegistryDigest          string                `json:"registry_digest"`
	BundleDigest            string                `json:"bundle_digest"`
	Digest                  string                `json:"digest"`
}

type rawReceiptPayload struct {
	Receipt        json.RawMessage `json:"receipt"`
	Signal         string          `json:"signal"`
	StableBoundary string          `json:"stable_boundary"`
}

type rawReceiptCommand struct {
	SchemaVersion    string                  `json:"schema_version"`
	Kind             coordinator.CommandKind `json:"kind"`
	MessageID        string                  `json:"message_id"`
	IdempotencyKey   string                  `json:"idempotency_key"`
	WorkflowID       string                  `json:"workflow_id"`
	ExpectedRevision uint64                  `json:"expected_revision"`
	Receipt          rawReceiptPayload       `json:"receipt"`
}
