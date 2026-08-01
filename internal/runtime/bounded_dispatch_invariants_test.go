package runtime

import (
	"sort"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

func TestBoundedHandshakeRevisionValidationCoversTerminalStates(t *testing.T) {
	base := internalBoundedGrantRevision(t)
	tests := []struct {
		name        string
		status      RunStatus
		event       string
		replyKind   ReplyKind
		reason      string
		recovery    []string
		observation *CapabilityObservation
	}{
		{name: "in flight", status: RunInFlight, event: "BOUNDED_DISPATCH_AUTHORIZED", replyKind: ReplyDispatchAuthorized},
		{name: "finished", status: RunFinished, event: "BOUNDED_CAPABILITY_FINISHED", replyKind: ReplyFinished, observation: &CapabilityObservation{GrantID: base.Snapshot.Grants[0].ID, InvocationID: base.Snapshot.Grants[0].InvocationID, ExecutorID: base.Snapshot.Grants[0].Executor.ID, Outcome: ObservationSucceeded, EvidenceReferences: []EvidenceReference{{Reference: "evidence://finished", Digest: strings.Repeat("1", 64)}}}},
		{name: "failed pause", status: RunPaused, event: "BOUNDED_CAPABILITY_FAILED", replyKind: ReplyPaused, reason: ReasonModeEscalationRequired, recovery: []string{RecoveryStartSuccessorRun}, observation: &CapabilityObservation{GrantID: base.Snapshot.Grants[0].ID, InvocationID: base.Snapshot.Grants[0].InvocationID, ExecutorID: base.Snapshot.Grants[0].Executor.ID, Outcome: ObservationFailed, EvidenceReferences: []EvidenceReference{{Reference: "evidence://failed", Digest: strings.Repeat("2", 64)}}}},
		{name: "mode pause", status: RunPaused, event: "BOUNDED_SCOPE_EXPANDED", replyKind: ReplyPaused, reason: ReasonModeEscalationRequired, recovery: []string{RecoveryStartSuccessorRun}},
		{name: "uncertain pause", status: RunPaused, event: "BOUNDED_EXECUTION_UNCERTAIN", replyKind: ReplyPaused, reason: ReasonExecutionUncertain, recovery: []string{RecoveryReconcileInvocation}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := boundedTransitionRevision(t, base, test.status, test.event, test.replyKind, test.reason, test.recovery, test.observation)
			if err := validateRevision(record, record.RunID, record.Revision); err != nil {
				t.Fatalf("valid %s revision rejected: %v", test.name, err)
			}
		})
	}
}

func TestBoundedHandshakeRevisionValidationRejectsObservationAndReplyTampering(t *testing.T) {
	base := internalBoundedGrantRevision(t)
	validFinished := boundedTransitionRevision(t, base, RunFinished, "BOUNDED_CAPABILITY_FINISHED", ReplyFinished, "", nil, &CapabilityObservation{
		GrantID: base.Snapshot.Grants[0].ID, InvocationID: base.Snapshot.Grants[0].InvocationID, ExecutorID: base.Snapshot.Grants[0].Executor.ID,
		Outcome: ObservationSucceeded, EvidenceReferences: []EvidenceReference{{Reference: "evidence://tamper", Digest: strings.Repeat("1", 64)}},
	})
	for _, test := range []struct {
		name   string
		mutate func(*revisionRecord)
	}{
		{"raw observation", func(value *revisionRecord) { value.Snapshot.Observations[0].RawOutput = "raw" }},
		{"observation Grant", func(value *revisionRecord) { value.Snapshot.Observations[0].GrantID = "grant-other" }},
		{"observation evidence", func(value *revisionRecord) { value.Snapshot.Observations[0].EvidenceReferences[0].Digest = "bad" }},
		{"observation evidence order", func(value *revisionRecord) {
			value.Snapshot.Observations[0].EvidenceReferences = []EvidenceReference{
				{Reference: "evidence://z", Digest: strings.Repeat("2", 64)},
				{Reference: "evidence://a", Digest: strings.Repeat("1", 64)},
			}
		}},
		{"finished outcome", func(value *revisionRecord) { value.Snapshot.Observations[0].Outcome = ObservationFailed }},
		{"finished event", func(value *revisionRecord) { value.Event = "BOUNDED_SCOPE_EXPANDED" }},
		{"finished reply", func(value *revisionRecord) { value.Reply.Kind = ReplyPaused }},
		{"paused event", func(value *revisionRecord) {
			value.Snapshot.Status = RunPaused
			value.Event = "BOUNDED_UNKNOWN"
			value.Reply.Kind = ReplyPaused
			value.Reply.Reason = ReasonModeEscalationRequired
			value.Reply.RecoveryActions = []string{RecoveryStartSuccessorRun}
		}},
		{"paused control event", func(value *revisionRecord) {
			value.Snapshot.Status = RunPaused
			value.Snapshot.Observations = nil
			value.Event = "BOUNDED_UNKNOWN"
			value.Reply.Kind = ReplyPaused
			value.Reply.Reason = ReasonModeEscalationRequired
			value.Reply.RecoveryActions = []string{RecoveryStartSuccessorRun}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := validFinished
			candidate.Snapshot = cloneSnapshot(validFinished.Snapshot)
			candidate.Reply = cloneReply(validFinished.Reply)
			test.mutate(&candidate)
			resignStateRevision(t, &candidate)
			assertInternalErrorCode(t, validateRevision(candidate, candidate.RunID, candidate.Revision), "RUN_STATE_REVISION_INVALID")
		})
	}
}

func TestBoundedGrantAndHistoryRemainImmutableAcrossRevisions(t *testing.T) {
	previous := internalBoundedGrantRevision(t)
	valid := boundedTransitionRevision(t, previous, RunInFlight, "BOUNDED_DISPATCH_AUTHORIZED", ReplyDispatchAuthorized, "", nil, nil)
	if err := validateRevision(valid, valid.RunID, valid.Revision); err != nil {
		t.Fatalf("valid current revision rejected: %v", err)
	}
	if err := validateRevisionTransition(previous, valid); err != nil {
		t.Fatalf("valid transition rejected: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*revisionRecord)
	}{
		{"Grant issue revision", func(value *revisionRecord) {
			value.Snapshot.Grants[0].IssuedRevision = 1
			resignCapabilityGrant(t, &value.Snapshot.Grants[0])
		}},
		{"Bounded termination", func(value *revisionRecord) {
			value.Snapshot.Bounded.Input.TerminationCondition = "changed report"
			value.Snapshot.Grants[0].TerminationCondition = "changed report"
			resignCapabilityGrant(t, &value.Snapshot.Grants[0])
		}},
		{"processed history", func(value *revisionRecord) {
			for index := range value.Snapshot.ProcessedMessages {
				if value.Snapshot.ProcessedMessages[index].Revision == 1 {
					value.Snapshot.ProcessedMessages[index].ContentDigest = strings.Repeat("0", 64)
				}
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Snapshot = cloneSnapshot(valid.Snapshot)
			candidate.Reply = cloneReply(valid.Reply)
			test.mutate(&candidate)
			resignStateRevision(t, &candidate)
			if err := validateRevision(candidate, candidate.RunID, candidate.Revision); err != nil {
				t.Fatalf("candidate did not isolate transition validation: %v", err)
			}
			assertInternalErrorCode(t, validateRevisionTransition(previous, candidate), "RUN_STATE_REVISION_INVALID")
		})
	}
}

func TestRevisionTransitionValidationRejectsIllegalBoundedEdges(t *testing.T) {
	previous := internalBoundedGrantRevision(t)
	current := boundedTransitionRevision(t, previous, RunInFlight, "BOUNDED_DISPATCH_AUTHORIZED", ReplyDispatchAuthorized, "", nil, nil)
	for _, test := range []struct {
		name   string
		mutate func(*revisionRecord, *revisionRecord)
	}{
		{"Run identity", func(_ *revisionRecord, value *revisionRecord) { value.Snapshot.RequestID = "changed-request" }},
		{"missing Bounded state", func(_ *revisionRecord, value *revisionRecord) { value.Snapshot.Bounded = nil }},
		{"Ready skips Grant", func(left *revisionRecord, _ *revisionRecord) { left.Snapshot.Status = RunReady }},
		{"Granted skips authorization", func(_ *revisionRecord, value *revisionRecord) { value.Snapshot.Status = RunFinished }},
		{"InFlight regresses", func(left *revisionRecord, value *revisionRecord) {
			left.Snapshot.Status = RunInFlight
			value.Snapshot.Status = RunReady
		}},
		{"terminal continues", func(left *revisionRecord, _ *revisionRecord) { left.Snapshot.Status = RunFinished }},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidatePrevious := previous
			candidatePrevious.Snapshot = cloneSnapshot(previous.Snapshot)
			candidateCurrent := current
			candidateCurrent.Snapshot = cloneSnapshot(current.Snapshot)
			test.mutate(&candidatePrevious, &candidateCurrent)
			assertInternalErrorCode(t, validateRevisionTransition(candidatePrevious, candidateCurrent), "RUN_STATE_REVISION_INVALID")
		})
	}
}

func resignCapabilityGrant(t *testing.T, grant *admission.CapabilityGrant) {
	t.Helper()
	grant.Digest = ""
	digest, _, err := canonicaljson.Digest(*grant)
	if err != nil {
		t.Fatal(err)
	}
	grant.Digest = digest
}

func boundedTransitionRevision(t *testing.T, base revisionRecord, status RunStatus, event string, replyKind ReplyKind, reason string, recovery []string, observation *CapabilityObservation) revisionRecord {
	t.Helper()
	record := base
	record.Snapshot = cloneSnapshot(base.Snapshot)
	record.Reply = cloneReply(base.Reply)
	record.Revision = base.Revision + 1
	record.Snapshot.Revision = record.Revision
	record.Reply.Revision = record.Revision
	record.PredecessorDigest = strings.Repeat("e", 64)
	record.MessageID = "handshake-" + strings.ToLower(string(status))
	record.IdempotencyKey = "handshake-" + strings.ToLower(string(status))
	record.MessageDigest = strings.Repeat("f", 64)
	record.Event = event
	record.Snapshot.Status = status
	record.Snapshot.Observations = nil
	if observation != nil {
		record.Snapshot.Observations = []CapabilityObservation{*observation}
	}
	record.Snapshot.ProcessedMessages = append(record.Snapshot.ProcessedMessages, ProcessedMessage{IdempotencyKey: record.IdempotencyKey, ContentDigest: record.MessageDigest, Revision: record.Revision})
	sort.Slice(record.Snapshot.ProcessedMessages, func(i, j int) bool {
		return record.Snapshot.ProcessedMessages[i].IdempotencyKey < record.Snapshot.ProcessedMessages[j].IdempotencyKey
	})
	record.Reply.Kind = replyKind
	record.Reply.Reason = reason
	record.Reply.RecoveryActions = append([]string{}, recovery...)
	record.Reply.Diagnostics = []Diagnostic{}
	resignStateRevision(t, &record)
	return record
}
