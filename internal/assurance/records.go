// Package assurance issues and verifies machine claims attached to one Policy
// Profile without defining workflow semantics.
package assurance

import (
	"errors"
	"fmt"
)

const (
	IssueRequestSchemaV1   = "oaw.assurance-issue-request/v1"
	OverlaySchemaV1        = "oaw.assurance-overlay/v1"
	ReferenceIndexSchemaV1 = "oaw.assurance-reference-index/v1"
)

type IssueRequest struct {
	SchemaVersion string         `json:"schema_version"`
	Issuer        string         `json:"issuer"`
	Claims        []BindingClaim `json:"claims"`
}

type ProfileReference struct {
	Source        string `json:"source"`
	ID            string `json:"id"`
	ContentDigest string `json:"content_digest"`
}

type EvidenceReference struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
}

type BindingClaim struct {
	OccurrenceRef          string              `json:"occurrence_ref"`
	ProviderID             string              `json:"provider_id"`
	DistributionID         string              `json:"distribution_id"`
	DistributionRevision   string              `json:"distribution_revision"`
	DistributionTreeDigest string              `json:"distribution_tree_digest"`
	HostID                 string              `json:"host_id"`
	Surface                string              `json:"surface"`
	BindingID              string              `json:"binding_id"`
	BindingKind            string              `json:"binding_kind"`
	BindingReference       string              `json:"binding_reference"`
	Invocation             string              `json:"invocation"`
	BindingContentDigest   string              `json:"binding_content_digest"`
	Evidence               []EvidenceReference `json:"evidence"`
}

type Overlay struct {
	SchemaVersion string           `json:"schema_version"`
	Profile       ProfileReference `json:"profile"`
	Issuer        string           `json:"issuer"`
	Claims        []BindingClaim   `json:"claims"`
	Digest        string           `json:"digest"`
}

type ReferenceOccurrence struct {
	OccurrenceRef    string `json:"occurrence_ref"`
	BindingReference string `json:"binding_reference"`
}

type ReferenceIndex struct {
	SchemaVersion string                `json:"schema_version"`
	Profile       ProfileReference      `json:"profile"`
	Occurrences   []ReferenceOccurrence `json:"occurrences"`
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
	var assuranceErr *Error
	if errors.As(err, &assuranceErr) {
		return assuranceErr.Code
	}
	return ""
}

func assuranceError(code, detail string, cause error) error {
	return &Error{Code: code, Detail: detail, Cause: cause}
}
