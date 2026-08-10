package host

import (
	"slices"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func NewEnvironmentReport(input EnvironmentReport) (EnvironmentReport, error) {
	providedDigest := input.Digest
	input.Digest = ""
	if input.SchemaVersion != HostEnvironmentReportSchemaV2 || !validSessionID(input.SessionID) {
		return EnvironmentReport{}, hostError("HOST_ENVIRONMENT_REPORT_INVALID", "invalid environment report identity", nil)
	}
	if _, err := execution.NormalizeTopologies([]execution.Topology{input.Topology}); err != nil {
		return EnvironmentReport{}, hostError("HOST_ENVIRONMENT_REPORT_INVALID", "invalid environment report topology", err)
	}
	if input.Topology == execution.TopologyCurrent && input.ParentSessionID != "" {
		return EnvironmentReport{}, hostError("HOST_ENVIRONMENT_REPORT_INVALID", "CURRENT report has a parent session", nil)
	}
	if input.Topology == execution.TopologySubagent && (!validSessionID(input.ParentSessionID) || input.ParentSessionID == input.SessionID) {
		return EnvironmentReport{}, hostError("HOST_ENVIRONMENT_REPORT_INVALID", "SUBAGENT report has an invalid parent session", nil)
	}
	input.Observations = append([]execution.EnvironmentObservation{}, input.Observations...)
	for _, observation := range input.Observations {
		if !validHostText(observation.Surface, 128) || !validHostText(observation.Source, 512) {
			return EnvironmentReport{}, hostError("HOST_ENVIRONMENT_REPORT_INVALID", "invalid environment observation identity", nil)
		}
	}
	if err := execution.RequirementsSatisfied(nil, input.Observations); err != nil {
		return EnvironmentReport{}, hostError("HOST_ENVIRONMENT_REPORT_INVALID", "invalid environment observations", err)
	}
	sort.Slice(input.Observations, func(left, right int) bool {
		return input.Observations[left].Surface < input.Observations[right].Surface
	})
	digest, _, err := canonicaljson.Digest(input)
	if err != nil {
		return EnvironmentReport{}, hostError("HOST_ENVIRONMENT_REPORT_INVALID", "environment report cannot be canonicalized", err)
	}
	if providedDigest != "" && providedDigest != digest {
		return EnvironmentReport{}, hostError("HOST_ENVIRONMENT_REPORT_INVALID", "environment report digest mismatch", nil)
	}
	input.Digest = digest
	return input, nil
}

func ValidateEnvironmentReport(session SessionSnapshot, report EnvironmentReport) error {
	if err := validateStoredSessionSnapshot(session); err != nil {
		return err
	}
	normalized, err := NewEnvironmentReport(report)
	if err != nil {
		return err
	}
	if !environmentReportsEqual(normalized, report) {
		return hostError("HOST_ENVIRONMENT_REPORT_INVALID", "environment report is not canonical", nil)
	}
	if session.EnvironmentReportDigest != report.Digest || !slices.Contains(session.SupportedTopologies, report.Topology) {
		return hostError("HOST_SESSION_CHANGED", "environment report is not pinned to the Host session", nil)
	}
	switch report.Topology {
	case execution.TopologyCurrent:
		if report.SessionID != session.SessionID || report.ParentSessionID != "" {
			return hostError("HOST_SESSION_CHANGED", "CURRENT report does not describe the active session", nil)
		}
	case execution.TopologySubagent:
		if report.ParentSessionID != session.SessionID || report.SessionID == session.SessionID {
			return hostError("HOST_SESSION_CHANGED", "SUBAGENT report does not describe a child of the active session", nil)
		}
	}
	return nil
}

func ValidateRequirements(requirements []execution.EnvironmentRequirement, report EnvironmentReport) error {
	normalized, err := NewEnvironmentReport(report)
	if err != nil || !environmentReportsEqual(normalized, report) {
		return hostError("HOST_ENVIRONMENT_REPORT_INVALID", "environment report is not canonical", err)
	}
	if err := execution.RequirementsSatisfied(requirements, report.Observations); err != nil {
		return hostError("HOST_ENVIRONMENT_REQUIREMENT_UNMET", "environment requirements are not satisfied", err)
	}
	return nil
}

func environmentReportsEqual(left, right EnvironmentReport) bool {
	return left.SchemaVersion == right.SchemaVersion && left.SessionID == right.SessionID && left.ParentSessionID == right.ParentSessionID &&
		left.Topology == right.Topology && slices.Equal(left.Observations, right.Observations) && left.Digest == right.Digest
}
