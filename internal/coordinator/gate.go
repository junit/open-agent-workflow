package coordinator

import (
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func cloneGateAttestation(value GateAttestation) GateAttestation {
	value.Evidence = append([]host.EvidenceReference{}, value.Evidence...)
	return value
}

func normalizeGateAttestation(value GateAttestation) (GateAttestation, error) {
	if value.SchemaVersion != GateAttestationSchemaV1 {
		return GateAttestation{}, coordinatorError("GATE_ATTESTATION_SCHEMA_UNSUPPORTED", "unsupported Gate Attestation schema", nil)
	}
	providedDigest := value.Digest
	value = cloneGateAttestation(value)
	value.Digest = ""
	sort.Slice(value.Evidence, func(left, right int) bool {
		return evidenceReferenceKey(value.Evidence[left]) < evidenceReferenceKey(value.Evidence[right])
	})
	if !validWorkflowID(value.WorkflowID) || !validStableID("bundle-", value.BundleID) || value.BundleGeneration == 0 ||
		!validDigest(value.BundleDigest) || execution.ValidateGraphCursor(value.Cursor) != nil || value.Cursor.Kind != execution.CursorGate ||
		!validText(value.GateID, 512) || value.Cursor.UnitID != value.GateID ||
		(value.Authority != catalog.GateOAWCore && value.Authority != catalog.GateHost && value.Authority != catalog.GateUser) ||
		(value.Decision != GateSatisfied && value.Decision != GateRejected) || len(value.Evidence) == 0 {
		return GateAttestation{}, coordinatorError("GATE_ATTESTATION_INVALID", "invalid Gate Attestation identity or decision", nil)
	}
	for index, evidence := range value.Evidence {
		if !validText(evidence.Kind, 128) || !validText(evidence.Reference, 2048) || !strings.HasPrefix(evidence.Reference, "evidence://") || !validDigest(evidence.Digest) ||
			index > 0 && evidenceReferenceIdentityKey(value.Evidence[index-1]) == evidenceReferenceIdentityKey(evidence) {
			return GateAttestation{}, coordinatorError("GATE_ATTESTATION_INVALID", "invalid or duplicate Gate evidence", nil)
		}
	}
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		return GateAttestation{}, coordinatorError("GATE_ATTESTATION_INVALID", "digest Gate Attestation", err)
	}
	if providedDigest != "" && providedDigest != digest {
		return GateAttestation{}, coordinatorError("GATE_ATTESTATION_INVALID", "Gate Attestation digest mismatch", nil)
	}
	value.Digest = digest
	return value, nil
}

func evidenceReferenceKey(value host.EvidenceReference) string {
	return value.Kind + "\x00" + value.Reference + "\x00" + value.Digest
}

func evidenceReferenceIdentityKey(value host.EvidenceReference) string {
	return value.Kind + "\x00" + value.Reference
}
