package host

import (
	"errors"
	"fmt"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

const (
	HostManifestSchemaV1          = "oaw.host-manifest/v1"
	HostIntegrationSchemaV1       = "oaw.host-integration/v1"
	HostManifestSchemaV2          = "oaw.host-manifest/v2"
	HostIntegrationSchemaV2       = "oaw.host-integration/v2"
	HostSessionSchemaV2           = "oaw.host-session/v2"
	HostEnvironmentReportSchemaV2 = "oaw.host-environment-report/v2"
	ConformanceReportSchemaV1     = "oaw.host-conformance-report/v1"
	ConformanceSuiteV1            = "oaw.host-conformance/v1"
	RuntimeProtocolV1             = "oaw.runtime/v1"
	WorkflowProtocolV1            = "oaw.workflow/v1"
)

type IntegrationLevel string

const (
	InstructionOnly IntegrationLevel = "instruction-only"
	RunnerManaged   IntegrationLevel = "runner-managed"
	NativeManaged   IntegrationLevel = "native-managed"
)

type ControlSurface string

const (
	SurfacePolicy     ControlSurface = "policy"
	SurfaceHostNative ControlSurface = "host-native"
)

type Feature string

const (
	FeatureIsolatedExecutor         Feature = "isolated-executor"
	FeatureExactBindingInvocation   Feature = "exact-binding-invocation"
	FeaturePause                    Feature = "pause"
	FeatureBundleInheritance        Feature = "bundle-inheritance"
	FeatureEvidenceReturn           Feature = "evidence-return"
	FeatureInvocationDedup          Feature = "invocation-deduplication"
	FeatureCancellation             Feature = "cancellation"
	FeatureNormalizedObservation    Feature = "normalized-observation"
	FeatureProviderBindingInventory Feature = "provider-binding-inventory"
	FeatureNativeInvocation         Feature = "native-invocation"
	FeatureNormalizedReceipts       Feature = "normalized-receipts"
	FeatureEnvironmentReporting     Feature = "environment-reporting"
)

type Manifest struct {
	SchemaVersion       string               `json:"schema_version" toml:"schema_version"`
	ManifestVersion     string               `json:"manifest_version" toml:"manifest_version"`
	HostID              string               `json:"host_id" toml:"host_id"`
	IntegrationLevel    IntegrationLevel     `json:"integration_level,omitempty" toml:"integration_level"`
	ControlSurface      ControlSurface       `json:"control_surface" toml:"control_surface"`
	Protocols           []string             `json:"protocols" toml:"protocols"`
	BindingKinds        []string             `json:"binding_kinds" toml:"binding_kinds"`
	SupportedTopologies []execution.Topology `json:"supported_topologies" toml:"supported_topologies"`
	Features            []Feature            `json:"features" toml:"features"`
}

type SessionSnapshot struct {
	SchemaVersion           string               `json:"schema_version"`
	HostID                  string               `json:"host_id"`
	IntegrationID           string               `json:"integration_id"`
	IntegrationVersion      string               `json:"integration_version"`
	SessionID               string               `json:"session_id"`
	SupportedTopologies     []execution.Topology `json:"supported_topologies"`
	ProviderInventoryDigest string               `json:"provider_inventory_digest"`
	EnvironmentReportDigest string               `json:"environment_report_digest"`
	SandboxPolicyDigest     string               `json:"sandbox_policy_digest"`
	ApprovalPolicyDigest    string               `json:"approval_policy_digest"`
	Digest                  string               `json:"digest"`
}

type EnvironmentReport struct {
	SchemaVersion   string                             `json:"schema_version"`
	SessionID       string                             `json:"session_id"`
	ParentSessionID string                             `json:"parent_session_id"`
	Topology        execution.Topology                 `json:"topology"`
	Observations    []execution.EnvironmentObservation `json:"observations"`
	Digest          string                             `json:"digest"`
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

type CheckID string

const (
	CheckIsolatedExecutor         CheckID = "isolated-executor"
	CheckExactBindingInvocation   CheckID = "exact-binding-invocation"
	CheckPause                    CheckID = "pause"
	CheckBundleInheritance        CheckID = "bundle-inheritance"
	CheckEvidenceReturn           CheckID = "evidence-return"
	CheckInvocationDedup          CheckID = "invocation-deduplication"
	CheckCancellation             CheckID = "cancellation"
	CheckNormalizedObservation    CheckID = "normalized-observation"
	CheckProviderBindingInventory CheckID = "provider-binding-inventory"
	CheckNativeInvocation         CheckID = "native-invocation"
)

type ConformanceCheck struct {
	ID       CheckID `json:"id" toml:"id"`
	Passed   bool    `json:"passed" toml:"passed"`
	Evidence string  `json:"evidence" toml:"evidence"`
}

type ConformanceReport struct {
	SchemaVersion    string             `json:"schema_version" toml:"schema_version"`
	SuiteVersion     string             `json:"suite_version" toml:"suite_version"`
	IntegrationID    string             `json:"integration_id" toml:"integration_id"`
	ManifestDigest   string             `json:"manifest_digest" toml:"manifest_digest"`
	Checks           []ConformanceCheck `json:"checks" toml:"checks"`
	TranscriptDigest string             `json:"transcript_digest" toml:"transcript_digest"`
	Passed           bool               `json:"passed" toml:"passed"`
	Digest           string             `json:"digest" toml:"digest"`
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

type RuntimeFrame struct {
	HostID              string    `json:"host_id"`
	IntegrationID       string    `json:"integration_id"`
	UnavailableFeatures []Feature `json:"unavailable_features"`
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
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		return ""
	}
	return digest
}

func CloneManifest(value Manifest) Manifest {
	value.Protocols = append([]string{}, value.Protocols...)
	value.BindingKinds = append([]string{}, value.BindingKinds...)
	value.SupportedTopologies = append([]execution.Topology{}, value.SupportedTopologies...)
	value.Features = append([]Feature{}, value.Features...)
	return value
}

func CloneSessionSnapshot(value SessionSnapshot) SessionSnapshot {
	value.SupportedTopologies = append([]execution.Topology{}, value.SupportedTopologies...)
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
	providedDigest := value.Digest
	value.Digest = ""
	value.Checks = append([]ConformanceCheck{}, value.Checks...)
	sort.Slice(value.Checks, func(left, right int) bool { return value.Checks[left].ID < value.Checks[right].ID })
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
	value.Checks = append([]ConformanceCheck{}, value.Checks...)
	return value
}

func NewIntegration(value IntegrationRecord) (IntegrationRecord, error) {
	if value.SchemaVersion != HostIntegrationSchemaV2 {
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
	if value.ManifestDigest != manifest.ContentDigest() {
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

func CloneRuntimeFrame(value RuntimeFrame) RuntimeFrame {
	value.UnavailableFeatures = append([]Feature{}, value.UnavailableFeatures...)
	return value
}
