package host

import (
	"errors"
	"fmt"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

const (
	HostManifestSchemaV3              = "oaw.host-manifest/v3"
	HostIntegrationSchemaV3           = "oaw.host-integration/v3"
	HostSessionSchemaV3               = "oaw.host-session/v3"
	HostEnvironmentReportSchemaV2     = "oaw.host-environment-report/v2"
	HostInvocationReceiptSchemaV2     = "oaw.host-invocation-receipt/v2"
	HostConformanceTranscriptSchemaV3 = "oaw.host-conformance-transcript/v3"
	HostConformanceReportSchemaV3     = "oaw.host-conformance-report/v3"
	WorkflowProtocolV1                = "oaw.workflow/v1"
)

type ControlSurface string

const (
	SurfacePolicy     ControlSurface = "policy"
	SurfaceHostNative ControlSurface = "host-native"
)

type Feature string

const (
	FeaturePause                    Feature = "pause"
	FeatureInvocationDedup          Feature = "invocation-deduplication"
	FeatureCancellation             Feature = "cancellation"
	FeatureProviderBindingInventory Feature = "provider-binding-inventory"
	FeatureNormalizedReceipts       Feature = "normalized-receipts"
	FeatureEnvironmentReporting     Feature = "environment-reporting"
)

type Manifest struct {
	SchemaVersion       string                `json:"schema_version" toml:"schema_version"`
	ManifestVersion     string                `json:"manifest_version" toml:"manifest_version"`
	HostID              string                `json:"host_id" toml:"host_id"`
	ControlSurface      ControlSurface        `json:"control_surface" toml:"control_surface"`
	Protocols           []string              `json:"protocols" toml:"protocols"`
	BindingKinds        []catalog.BindingKind `json:"binding_kinds" toml:"binding_kinds"`
	SupportedTopologies []execution.Topology  `json:"supported_topologies" toml:"supported_topologies"`
	Features            []Feature             `json:"features" toml:"features"`
	DelegationFeatures  []FeatureID           `json:"delegation_features" toml:"delegation_features"`
	HostActions         []HostActionContract  `json:"host_actions" toml:"host_actions"`
	Digest              string                `json:"digest" toml:"digest"`
}

type SessionSnapshot struct {
	SchemaVersion           string                  `json:"schema_version"`
	HostID                  string                  `json:"host_id"`
	IntegrationID           string                  `json:"integration_id"`
	IntegrationVersion      string                  `json:"integration_version"`
	SessionID               string                  `json:"session_id"`
	ManifestDigest          string                  `json:"manifest_digest"`
	SupportedTopologies     []execution.Topology    `json:"supported_topologies"`
	ProviderInventoryDigest string                  `json:"provider_inventory_digest"`
	FeatureObservations     []FeatureObservation    `json:"feature_observations"`
	FeatureDigest           string                  `json:"feature_digest"`
	HostActionObservations  []HostActionObservation `json:"host_action_observations"`
	HostActionDigest        string                  `json:"host_action_digest"`
	EnvironmentReportDigest string                  `json:"environment_report_digest"`
	SandboxPolicyDigest     string                  `json:"sandbox_policy_digest"`
	ApprovalPolicyDigest    string                  `json:"approval_policy_digest"`
	Digest                  string                  `json:"digest"`
}

type EnvironmentReport struct {
	SchemaVersion   string                             `json:"schema_version"`
	SessionID       string                             `json:"session_id"`
	ParentSessionID string                             `json:"parent_session_id"`
	Topology        execution.Topology                 `json:"topology"`
	Observations    []execution.EnvironmentObservation `json:"observations"`
	Digest          string                             `json:"digest"`
}

type ReceiptKind string

const (
	ReceiptStarted   ReceiptKind = "STARTED"
	ReceiptPaused    ReceiptKind = "PAUSED"
	ReceiptCompleted ReceiptKind = "COMPLETED"
	ReceiptFailed    ReceiptKind = "FAILED"
	ReceiptCancelled ReceiptKind = "CANCELLED"

	ContextShared = "shared"
	ContextFresh  = "fresh"
)

type EvidenceReference struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
}

type InvocationReceipt struct {
	SchemaVersion           string              `json:"schema_version"`
	Kind                    ReceiptKind         `json:"kind"`
	WorkflowID              string              `json:"workflow_id"`
	BundleGeneration        uint64              `json:"bundle_generation"`
	BundleDigest            string              `json:"bundle_digest"`
	NodeID                  string              `json:"node_id"`
	Topology                execution.Topology  `json:"topology"`
	HostSessionDigest       string              `json:"host_session_digest"`
	DispatchDigest          string              `json:"dispatch_digest"`
	InvocationHandle        string              `json:"invocation_handle"`
	ContextFreshness        string              `json:"context_freshness"`
	EnvironmentReportDigest string              `json:"environment_report_digest"`
	Outcome                 string              `json:"outcome"`
	FailureCode             string              `json:"failure_code"`
	Evidence                []EvidenceReference `json:"evidence"`
	Digest                  string              `json:"digest"`
}

type InvocationRecord struct {
	IdempotencyKey string `json:"idempotency_key"`
	DispatchDigest string `json:"dispatch_digest"`
	ReceiptDigest  string `json:"receipt_digest"`
}

type ConformanceTranscript struct {
	SchemaVersion      string              `json:"schema_version"`
	Session            SessionSnapshot     `json:"session"`
	Inventory          BindingInventory    `json:"inventory"`
	EnvironmentReports []EnvironmentReport `json:"environment_reports"`
	Receipts           []InvocationReceipt `json:"receipts"`
	Invocations        []InvocationRecord  `json:"invocations"`
	Digest             string              `json:"digest"`
}

type AuditStatus string

const (
	AuditPending AuditStatus = "pending"
	AuditPassed  AuditStatus = "passed"
)

type AuditEvidenceReference struct {
	Reference string `json:"reference" toml:"reference"`
	Digest    string `json:"digest" toml:"digest"`
}

type AuditEvidence struct {
	Status     AuditStatus              `json:"status" toml:"status"`
	References []AuditEvidenceReference `json:"references" toml:"references"`
	Digest     string                   `json:"digest" toml:"digest"`
}

type ConformanceReport struct {
	SchemaVersion              string      `json:"schema_version" toml:"schema_version"`
	ManifestDigest             string      `json:"manifest_digest" toml:"manifest_digest"`
	TranscriptDigest           string      `json:"transcript_digest" toml:"transcript_digest"`
	VerifiedFeatures           []Feature   `json:"verified_features" toml:"verified_features"`
	VerifiedDelegationFeatures []FeatureID `json:"verified_delegation_features" toml:"verified_delegation_features"`
	VerifiedHostActionIDs      []string    `json:"verified_host_action_ids" toml:"verified_host_action_ids"`
	Diagnostics                []string    `json:"diagnostics" toml:"diagnostics"`
	Digest                     string      `json:"digest" toml:"digest"`
}

type IntegrationRecord struct {
	SchemaVersion      string             `json:"schema_version" toml:"schema_version"`
	IntegrationVersion string             `json:"integration_version" toml:"integration_version"`
	ID                 string             `json:"id" toml:"id"`
	Manifest           Manifest           `json:"manifest" toml:"manifest"`
	ManifestDigest     string             `json:"manifest_digest" toml:"manifest_digest"`
	Audit              AuditEvidence      `json:"audit" toml:"audit"`
	Conformance        *ConformanceReport `json:"conformance,omitempty" toml:"conformance"`
	Digest             string             `json:"digest" toml:"digest"`
}

type Error struct {
	Code   string
	Detail string
	Cause  error
}

func (value *Error) Error() string {
	if value.Detail == "" {
		return value.Code
	}
	return fmt.Sprintf("%s: %s", value.Code, value.Detail)
}

func (value *Error) Unwrap() error { return value.Cause }

func ErrorCode(err error) string {
	var hostErr *Error
	if errors.As(err, &hostErr) {
		return hostErr.Code
	}
	return ""
}

func hostError(code, detail string, cause error) error {
	return &Error{Code: code, Detail: detail, Cause: cause}
}

func (value Manifest) ContentDigest() string {
	value = CloneManifest(value)
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		return ""
	}
	return digest
}

func CloneManifest(value Manifest) Manifest {
	value.Protocols = append([]string{}, value.Protocols...)
	value.BindingKinds = append([]catalog.BindingKind{}, value.BindingKinds...)
	value.SupportedTopologies = append([]execution.Topology{}, value.SupportedTopologies...)
	value.Features = append([]Feature{}, value.Features...)
	value.DelegationFeatures = append([]FeatureID{}, value.DelegationFeatures...)
	value.HostActions = cloneHostActionContracts(value.HostActions)
	return value
}

func CloneSessionSnapshot(value SessionSnapshot) SessionSnapshot {
	value.SupportedTopologies = append([]execution.Topology{}, value.SupportedTopologies...)
	value.FeatureObservations = cloneFeatureObservations(value.FeatureObservations)
	value.HostActionObservations = cloneHostActionObservations(value.HostActionObservations)
	return value
}

func CloneEnvironmentReport(value EnvironmentReport) EnvironmentReport {
	value.Observations = append([]execution.EnvironmentObservation{}, value.Observations...)
	return value
}

func NewAuditEvidence(value AuditEvidence) (AuditEvidence, error) {
	providedDigest := value.Digest
	value.Digest = ""
	value.References = append([]AuditEvidenceReference{}, value.References...)
	sort.Slice(value.References, func(left, right int) bool {
		return value.References[left].Reference+"\x00"+value.References[left].Digest < value.References[right].Reference+"\x00"+value.References[right].Digest
	})
	if err := validateAuditEvidence(value); err != nil {
		return AuditEvidence{}, err
	}
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		return AuditEvidence{}, hostError("HOST_AUDIT_INVALID", "audit evidence cannot be canonicalized", err)
	}
	if providedDigest != "" && providedDigest != digest {
		return AuditEvidence{}, hostError("HOST_AUDIT_INVALID", "audit evidence digest mismatch", nil)
	}
	value.Digest = digest
	return value, nil
}

func CloneAuditEvidence(value AuditEvidence) AuditEvidence {
	value.References = append([]AuditEvidenceReference{}, value.References...)
	return value
}

func NewConformanceReport(value ConformanceReport) (ConformanceReport, error) {
	if value.SchemaVersion != HostConformanceReportSchemaV3 {
		return ConformanceReport{}, hostError("HOST_SCHEMA_UNSUPPORTED", "unsupported Conformance Report schema", nil)
	}
	providedDigest := value.Digest
	value.Digest = ""
	value = CloneConformanceReport(value)
	sort.Slice(value.VerifiedFeatures, func(left, right int) bool { return value.VerifiedFeatures[left] < value.VerifiedFeatures[right] })
	sort.Slice(value.VerifiedDelegationFeatures, func(left, right int) bool {
		return value.VerifiedDelegationFeatures[left] < value.VerifiedDelegationFeatures[right]
	})
	sort.Strings(value.VerifiedHostActionIDs)
	sort.Strings(value.Diagnostics)
	if err := validateConformanceReport(value); err != nil {
		return ConformanceReport{}, err
	}
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		return ConformanceReport{}, hostError("HOST_CONFORMANCE_INVALID", "Conformance Report cannot be canonicalized", err)
	}
	if providedDigest != "" && providedDigest != digest {
		return ConformanceReport{}, hostError("HOST_CONFORMANCE_INVALID", "Conformance Report digest mismatch", nil)
	}
	value.Digest = digest
	return value, nil
}

func CloneConformanceReport(value ConformanceReport) ConformanceReport {
	value.VerifiedFeatures = append([]Feature{}, value.VerifiedFeatures...)
	value.VerifiedDelegationFeatures = append([]FeatureID{}, value.VerifiedDelegationFeatures...)
	value.VerifiedHostActionIDs = append([]string{}, value.VerifiedHostActionIDs...)
	value.Diagnostics = append([]string{}, value.Diagnostics...)
	return value
}

func NewIntegration(value IntegrationRecord) (IntegrationRecord, error) {
	if value.SchemaVersion != HostIntegrationSchemaV3 {
		return IntegrationRecord{}, hostError("HOST_SCHEMA_UNSUPPORTED", "unsupported Host Integration schema", nil)
	}
	providedDigest := value.Digest
	value.Digest = ""
	manifest, err := NewManifest(value.Manifest)
	if err != nil {
		if ErrorCode(err) == "HOST_SCHEMA_UNSUPPORTED" {
			return IntegrationRecord{}, err
		}
		return IntegrationRecord{}, hostError("HOST_INTEGRATION_INVALID", "invalid Manifest", err)
	}
	value.Manifest = manifest
	if value.ManifestDigest != manifest.Digest {
		return IntegrationRecord{}, hostError("HOST_INTEGRATION_INVALID", "Manifest digest mismatch", nil)
	}
	audit, err := NewAuditEvidence(value.Audit)
	if err != nil {
		return IntegrationRecord{}, hostError("HOST_INTEGRATION_INVALID", "invalid audit evidence", err)
	}
	value.Audit = audit
	if value.Conformance != nil {
		report, reportErr := NewConformanceReport(*value.Conformance)
		if reportErr != nil {
			if ErrorCode(reportErr) == "HOST_SCHEMA_UNSUPPORTED" {
				return IntegrationRecord{}, reportErr
			}
			return IntegrationRecord{}, hostError("HOST_INTEGRATION_INVALID", "invalid Conformance Report", reportErr)
		}
		value.Conformance = &report
	}
	if err := validateIntegration(value); err != nil {
		return IntegrationRecord{}, err
	}
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		return IntegrationRecord{}, hostError("HOST_INTEGRATION_INVALID", "Integration cannot be canonicalized", err)
	}
	if providedDigest != "" && providedDigest != digest {
		return IntegrationRecord{}, hostError("HOST_INTEGRATION_INVALID", "Integration digest mismatch", nil)
	}
	value.Digest = digest
	return value, nil
}

func CloneIntegration(value IntegrationRecord) IntegrationRecord {
	value.Manifest = CloneManifest(value.Manifest)
	value.Audit = CloneAuditEvidence(value.Audit)
	if value.Conformance != nil {
		report := CloneConformanceReport(*value.Conformance)
		value.Conformance = &report
	}
	return value
}
