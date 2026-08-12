package profile

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

var recordDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func NewHostEvidence(manifest host.Manifest, session host.SessionSnapshot, inventory host.BindingInventory, environment host.EnvironmentReport) (HostEvidence, error) {
	reporterIdentity, _, err := canonicaljson.Digest(struct {
		SchemaVersion      string `json:"schema_version"`
		HostID             string `json:"host_id"`
		IntegrationID      string `json:"integration_id"`
		IntegrationVersion string `json:"integration_version"`
		SessionID          string `json:"session_id"`
		ManifestDigest     string `json:"manifest_digest"`
	}{"oaw.host-reporter-identity/v1", session.HostID, session.IntegrationID, session.IntegrationVersion, session.SessionID, manifest.Digest})
	if err != nil {
		return HostEvidence{}, fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: reporter identity cannot be canonicalized")
	}
	return NewHostEvidenceWithReporterIdentity(manifest, session, inventory, environment, reporterIdentity)
}

func NewHostEvidenceWithReporterIdentity(manifest host.Manifest, session host.SessionSnapshot, inventory host.BindingInventory, environment host.EnvironmentReport, reporterIdentity string) (HostEvidence, error) {
	normalizedManifest, err := host.NewManifest(manifest)
	if err != nil || !reflect.DeepEqual(normalizedManifest, manifest) {
		return HostEvidence{}, fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: invalid Host Manifest")
	}
	normalizedSession, err := host.NewSessionSnapshot(normalizedManifest, session)
	if err != nil || !reflect.DeepEqual(normalizedSession, session) {
		return HostEvidence{}, fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: invalid Host Session")
	}
	normalizedInventory, err := host.ValidateBindingInventory(inventory)
	if err != nil || !reflect.DeepEqual(normalizedInventory, inventory) {
		return HostEvidence{}, fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: invalid Host Binding Inventory")
	}
	if err := host.ValidateEnvironmentReport(normalizedSession, environment); err != nil {
		return HostEvidence{}, fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: invalid Host Environment Report")
	}
	if normalizedManifest.HostID != normalizedSession.HostID || normalizedSession.HostID != normalizedInventory.HostID ||
		normalizedSession.ManifestDigest != normalizedManifest.Digest || normalizedSession.ProviderInventoryDigest != normalizedInventory.Digest ||
		normalizedSession.EnvironmentReportDigest != environment.Digest {
		return HostEvidence{}, fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: Host evidence identity changed")
	}
	if !recordDigestPattern.MatchString(reporterIdentity) {
		return HostEvidence{}, fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: invalid reporter identity")
	}
	record := HostEvidenceRecord{
		SchemaVersion: HostEvidenceSchemaV1, HostID: normalizedSession.HostID, Topology: environment.Topology,
		FeatureObservations:     append([]host.FeatureObservation{}, normalizedSession.FeatureObservations...),
		ActionObservations:      cloneHostEvidenceRecord(HostEvidenceRecord{ActionObservations: normalizedSession.HostActionObservations}).ActionObservations,
		EnvironmentObservations: append([]execution.EnvironmentObservation{}, environment.Observations...),
		SessionDigest:           normalizedSession.Digest, ReporterIdentityDigest: reporterIdentity,
		ManifestDigest: normalizedManifest.Digest, InventoryDigest: normalizedInventory.Digest,
		FeatureDigest: normalizedSession.FeatureDigest, ActionDigest: normalizedSession.HostActionDigest, EnvironmentDigest: environment.Digest,
	}
	if record.FeatureObservations == nil {
		record.FeatureObservations = []host.FeatureObservation{}
	}
	if record.ActionObservations == nil {
		record.ActionObservations = []host.HostActionObservation{}
	}
	if record.EnvironmentObservations == nil {
		record.EnvironmentObservations = []execution.EnvironmentObservation{}
	}
	record.Digest = record.ContentDigest()
	if err := ValidateHostEvidenceRecord(record); err != nil {
		return HostEvidence{}, err
	}
	return HostEvidence{record: cloneHostEvidenceRecord(record)}, nil
}

func ValidateHostEvidenceRecord(record HostEvidenceRecord) error {
	if record.SchemaVersion != HostEvidenceSchemaV1 || record.FeatureObservations == nil || record.ActionObservations == nil || record.EnvironmentObservations == nil {
		return fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: incomplete record")
	}
	if !recordDigestPattern.MatchString(record.SessionDigest) || !recordDigestPattern.MatchString(record.ReporterIdentityDigest) ||
		!recordDigestPattern.MatchString(record.ManifestDigest) || !recordDigestPattern.MatchString(record.InventoryDigest) ||
		!recordDigestPattern.MatchString(record.FeatureDigest) || !recordDigestPattern.MatchString(record.ActionDigest) || !recordDigestPattern.MatchString(record.EnvironmentDigest) {
		return fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: invalid evidence pin")
	}
	if !recordDigestPattern.MatchString(record.Digest) || record.ContentDigest() != record.Digest {
		return fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: record digest mismatch")
	}
	if _, err := catalog.ParseLocalID(record.HostID); err != nil {
		return fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: invalid Host identity")
	}
	if _, err := execution.NormalizeTopologies([]execution.Topology{record.Topology}); err != nil {
		return fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: invalid topology")
	}
	features := append([]host.FeatureObservation{}, record.FeatureObservations...)
	for index, observation := range features {
		normalized, err := host.NewFeatureObservation(observation)
		if err != nil || !reflect.DeepEqual(normalized, observation) || observation.State == host.AvailabilityAvailable && !liveSource(observation.Source) {
			return fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: invalid feature observation")
		}
		if index > 0 && features[index-1].Feature >= observation.Feature {
			return fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: feature order changed")
		}
	}
	actions := cloneHostEvidenceRecord(record).ActionObservations
	for index, observation := range actions {
		normalized, err := host.NewHostActionObservation(observation)
		if err != nil || !reflect.DeepEqual(normalized, observation) || observation.State == host.AvailabilityAvailable && !liveSource(observation.Source) {
			return fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: invalid action observation")
		}
		if index > 0 && actions[index-1].Action.ID >= observation.Action.ID {
			return fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: action order changed")
		}
	}
	if err := execution.RequirementsSatisfied(nil, record.EnvironmentObservations); err != nil {
		return fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: invalid environment observations")
	}
	for index, observation := range record.EnvironmentObservations {
		if index > 0 && record.EnvironmentObservations[index-1].Surface >= observation.Surface {
			return fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: environment order changed")
		}
	}
	featureDigest, _, err := canonicaljson.Digest(struct {
		SchemaVersion string                    `json:"schema_version"`
		Observations  []host.FeatureObservation `json:"observations"`
	}{"oaw.host-feature-observations/v1", features})
	if err != nil || featureDigest != record.FeatureDigest {
		return fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: feature digest mismatch")
	}
	actionDigest, _, err := canonicaljson.Digest(struct {
		SchemaVersion string                       `json:"schema_version"`
		Observations  []host.HostActionObservation `json:"observations"`
	}{"oaw.host-action-observations/v1", actions})
	if err != nil || actionDigest != record.ActionDigest {
		return fmt.Errorf("PROFILE_HOST_EVIDENCE_INVALID: action digest mismatch")
	}
	return nil
}

func liveSource(source host.ObservationSource) bool {
	return source == host.SourceNativeAPI || source == host.SourceLiveHostIndex || source == host.SourceLiveFilesystem
}

func hostFeature(record HostEvidenceRecord, feature host.FeatureID) (host.FeatureObservation, bool) {
	index, found := slices.BinarySearchFunc(record.FeatureObservations, feature, func(value host.FeatureObservation, target host.FeatureID) int {
		if value.Feature < target {
			return -1
		}
		if value.Feature > target {
			return 1
		}
		return 0
	})
	if !found {
		return host.FeatureObservation{}, false
	}
	return record.FeatureObservations[index], true
}

func hostAction(record HostEvidenceRecord, id string) (host.HostActionObservation, bool) {
	index := sort.Search(len(record.ActionObservations), func(index int) bool { return record.ActionObservations[index].Action.ID >= id })
	if index == len(record.ActionObservations) || record.ActionObservations[index].Action.ID != id {
		return host.HostActionObservation{}, false
	}
	value := cloneHostEvidenceRecord(HostEvidenceRecord{ActionObservations: []host.HostActionObservation{record.ActionObservations[index]}}).ActionObservations[0]
	return value, true
}
