package admission

import (
	"errors"
	"fmt"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

const CapabilityGrantSchemaV1 = "oaw.capability-grant/v1"

type ExecutorKind string

const (
	ExecutorMainAgent ExecutorKind = "main-agent"
	ExecutorIsolated  ExecutorKind = "isolated"
)

type ExecutorRegistration struct {
	ID   string       `json:"id"`
	Kind ExecutorKind `json:"kind"`
}

type AuthorityCeiling struct {
	Effects         []string `json:"effects"`
	Resources       []string `json:"resources"`
	ResourceLeases  bool     `json:"resource_leases"`
	AllowDelegation bool     `json:"allow_delegation"`
}

type CatalogSource interface {
	Providers() []catalog.ProviderDescriptorRecord
	Digest() string
}

type VerifiedRegistry interface {
	Provider(id string) (registry.ProviderInstance, bool)
	Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool)
	Digest() string
}

type GrantRequest struct {
	RunID                string
	RequestID            string
	DeliverableID        string
	InputDigest          string
	IssuedRevision       uint64
	Selector             classification.CapabilitySelector
	Effects              []string
	Resources            []string
	TerminationCondition string
	Executor             ExecutorRegistration
	DelegationAllowList  []string
	Catalog              CatalogSource
	Registry             VerifiedRegistry
	Authority            AuthorityCeiling
	Executors            []ExecutorRegistration
}

type ChildGrantRequest struct {
	Parent  CapabilityGrant
	Request GrantRequest
}

type CapabilityGrant struct {
	SchemaVersion          string               `json:"schema_version"`
	ID                     string               `json:"id"`
	InvocationID           string               `json:"invocation_id"`
	RunID                  string               `json:"run_id"`
	RequestID              string               `json:"request_id"`
	DeliverableID          string               `json:"deliverable_id"`
	InputDigest            string               `json:"input_digest"`
	IssuedRevision         uint64               `json:"issued_revision"`
	Generation             uint64               `json:"generation"`
	ProviderID             string               `json:"provider_id"`
	ProviderInstanceDigest string               `json:"provider_instance_digest"`
	DescriptorDigest       string               `json:"descriptor_digest"`
	RegistryDigest         string               `json:"registry_digest"`
	CatalogDigest          string               `json:"catalog_digest"`
	CapabilityID           string               `json:"capability_id"`
	Binding                catalog.HostBinding  `json:"binding"`
	Executor               ExecutorRegistration `json:"executor"`
	Effects                []string             `json:"effects"`
	Resources              []string             `json:"resources"`
	TerminationCondition   string               `json:"termination_condition"`
	DelegationAllowList    []string             `json:"delegation_allow_list"`
	ParentGrantID          string               `json:"parent_grant_id,omitempty"`
	Digest                 string               `json:"digest"`
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
	value.Effects = append([]string{}, value.Effects...)
	value.Resources = append([]string{}, value.Resources...)
	value.DelegationAllowList = append([]string{}, value.DelegationAllowList...)
	return value
}

func CloneAuthority(value AuthorityCeiling) AuthorityCeiling {
	value.Effects = append([]string{}, value.Effects...)
	value.Resources = append([]string{}, value.Resources...)
	return value
}

func CloneExecutors(values []ExecutorRegistration) []ExecutorRegistration {
	return append([]ExecutorRegistration{}, values...)
}
