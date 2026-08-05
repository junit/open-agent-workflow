package admission

import (
	"errors"
	"fmt"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

const CapabilityGrantSchemaV2 = "oaw.capability-grant/v2"

type AuthorityCeiling struct {
	Effects         []string `json:"effects"`
	Resources       []string `json:"resources"`
	ResourceLeases  bool     `json:"resource_leases"`
	AllowDelegation bool     `json:"allow_delegation"`
}

type WorkflowGrantRequest struct {
	WorkflowID           string
	RequestID            string
	BundleID             string
	BundleGeneration     uint64
	BundleDigest         string
	Node                 profile.GraphNode
	Topology             execution.Topology
	HostSessionDigest    string
	Effects              []string
	Resources            []string
	TerminationCondition string
	Authority            AuthorityCeiling
}

type CapabilityGrant struct {
	SchemaVersion          string              `json:"schema_version"`
	ID                     string              `json:"id"`
	WorkflowID             string              `json:"workflow_id"`
	RequestID              string              `json:"request_id"`
	BundleID               string              `json:"bundle_id"`
	BundleGeneration       uint64              `json:"bundle_generation"`
	BundleDigest           string              `json:"bundle_digest"`
	NodeID                 string              `json:"node_id"`
	Topology               execution.Topology  `json:"topology"`
	HostSessionDigest      string              `json:"host_session_digest"`
	ProviderID             string              `json:"provider_id"`
	ProviderInstanceDigest string              `json:"provider_instance_digest"`
	CapabilityID           string              `json:"capability_id"`
	Binding                catalog.HostBinding `json:"binding"`
	Effects                []string            `json:"effects"`
	Resources              []string            `json:"resources"`
	TerminationCondition   string              `json:"termination_condition"`
	Digest                 string              `json:"digest"`
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
	var admissionErr *Error
	if errors.As(err, &admissionErr) {
		return admissionErr.Code
	}
	return ""
}

func admissionError(code, detail string, cause error) error {
	return &Error{Code: code, Detail: detail, Cause: cause}
}

func CloneGrant(value CapabilityGrant) CapabilityGrant {
	value.Binding.Topologies = append([]execution.Topology{}, value.Binding.Topologies...)
	value.Effects = append([]string{}, value.Effects...)
	value.Resources = append([]string{}, value.Resources...)
	return value
}

func CloneAuthority(value AuthorityCeiling) AuthorityCeiling {
	value.Effects = append([]string{}, value.Effects...)
	value.Resources = append([]string{}, value.Resources...)
	return value
}
