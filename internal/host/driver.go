package host

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

type DispatchOutcome string

const (
	DispatchSucceeded DispatchOutcome = "SUCCEEDED"
	DispatchFailed    DispatchOutcome = "FAILED"
)

type DispatchRequest struct {
	GrantID                string              `json:"grant_id"`
	InvocationID           string              `json:"invocation_id"`
	ExecutorID             string              `json:"executor_id"`
	BundleDigest           string              `json:"bundle_digest"`
	ProviderID             string              `json:"provider_id,omitempty"`
	CapabilityID           string              `json:"capability_id,omitempty"`
	ProviderInstanceDigest string              `json:"provider_instance_digest,omitempty"`
	Binding                catalog.HostBinding `json:"binding"`
	Effects                []string            `json:"effects"`
	Resources              []string            `json:"resources"`
}

type DispatchEvidence struct {
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
}

type DispatchResult struct {
	GrantID      string             `json:"grant_id"`
	InvocationID string             `json:"invocation_id"`
	ExecutorID   string             `json:"executor_id"`
	ExecutionID  string             `json:"execution_id"`
	Outcome      DispatchOutcome    `json:"outcome"`
	Evidence     []DispatchEvidence `json:"evidence"`
}

type Driver interface {
	Prepare(context.Context, DispatchRequest) error
	Invoke(context.Context, DispatchRequest) (DispatchResult, error)
	Cancel(context.Context, string) error
}

func ValidateDispatchRequest(value DispatchRequest) error {
	if !safeDriverIdentifier(value.GrantID) || !safeDriverIdentifier(value.InvocationID) || !safeDriverIdentifier(value.ExecutorID) || !isDigest(value.BundleDigest) {
		return errors.New("HOST_DISPATCH_REQUEST_INVALID: identity is invalid")
	}
	if (value.ProviderID == "") != (value.CapabilityID == "") || (value.ProviderID == "" && value.ProviderInstanceDigest != "") || (value.ProviderID != "" && !isDigest(value.ProviderInstanceDigest)) || (value.ProviderID != "" && !safeDriverIdentifier(value.ProviderID)) || (value.CapabilityID != "" && !safeDriverIdentifier(value.CapabilityID)) {
		return errors.New("HOST_DISPATCH_REQUEST_INVALID: Provider identity is invalid")
	}
	if value.Binding.Host == "" || value.Binding.Reference == "" || (value.Binding.Kind != "agent" && value.Binding.Kind != "skill" && value.Binding.Kind != "tool") || strings.IndexFunc(value.Binding.Host, unicode.IsControl) >= 0 || strings.IndexFunc(value.Binding.Reference, unicode.IsControl) >= 0 {
		return errors.New("HOST_DISPATCH_REQUEST_INVALID: Binding is invalid")
	}
	if !validDriverSet(value.Effects) || !validDriverSet(value.Resources) {
		return errors.New("HOST_DISPATCH_REQUEST_INVALID: effects and resources must be non-empty sorted sets")
	}
	return nil
}

func CloneDispatchRequest(value DispatchRequest) DispatchRequest {
	value.Effects = append([]string{}, value.Effects...)
	value.Resources = append([]string{}, value.Resources...)
	return value
}

func EqualDispatchRequest(left, right DispatchRequest) bool {
	return left.GrantID == right.GrantID && left.InvocationID == right.InvocationID && left.ExecutorID == right.ExecutorID && left.BundleDigest == right.BundleDigest && left.ProviderID == right.ProviderID && left.CapabilityID == right.CapabilityID && left.ProviderInstanceDigest == right.ProviderInstanceDigest && left.Binding == right.Binding && slices.Equal(left.Effects, right.Effects) && slices.Equal(left.Resources, right.Resources)
}

func NormalizeDispatchResult(request DispatchRequest, value DispatchResult) (DispatchResult, error) {
	if err := ValidateDispatchRequest(request); err != nil {
		return DispatchResult{}, err
	}
	if value.GrantID != request.GrantID || value.InvocationID != request.InvocationID || value.ExecutorID != request.ExecutorID || !safeDriverIdentifier(value.ExecutionID) {
		return DispatchResult{}, errors.New("HOST_DISPATCH_RESULT_INVALID: result identity does not match request")
	}
	if value.Outcome != DispatchSucceeded && value.Outcome != DispatchFailed {
		return DispatchResult{}, errors.New("HOST_DISPATCH_RESULT_INVALID: outcome is not closed")
	}
	evidence := append([]DispatchEvidence{}, value.Evidence...)
	for index := range evidence {
		evidence[index].Reference = strings.TrimSpace(evidence[index].Reference)
		if !safeDriverIdentifier(evidence[index].Reference) || !isDigest(evidence[index].Digest) {
			return DispatchResult{}, errors.New("HOST_DISPATCH_RESULT_INVALID: evidence is invalid")
		}
	}
	sort.Slice(evidence, func(left, right int) bool {
		if evidence[left].Reference == evidence[right].Reference {
			return evidence[left].Digest < evidence[right].Digest
		}
		return evidence[left].Reference < evidence[right].Reference
	})
	for index := 1; index < len(evidence); index++ {
		if evidence[index-1] == evidence[index] {
			return DispatchResult{}, errors.New("HOST_DISPATCH_RESULT_INVALID: duplicate evidence")
		}
	}
	if len(evidence) == 0 {
		return DispatchResult{}, errors.New("HOST_DISPATCH_RESULT_INVALID: evidence is required")
	}
	return DispatchResult{GrantID: value.GrantID, InvocationID: value.InvocationID, ExecutorID: value.ExecutorID, ExecutionID: value.ExecutionID, Outcome: value.Outcome, Evidence: evidence}, nil
}

func safeDriverIdentifier(value string) bool {
	return value != "" && len(value) <= 256 && strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}

func validDriverSet(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for index, value := range values {
		if !safeDriverIdentifier(value) || index > 0 && values[index-1] >= value {
			return false
		}
	}
	return true
}

func isDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func driverError(format string, args ...any) error {
	return fmt.Errorf("HOST_DRIVER_ERROR: "+format, args...)
}
