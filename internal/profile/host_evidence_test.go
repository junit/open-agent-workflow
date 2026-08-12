package profile_test

import (
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestNewHostEvidencePinsOneValidatedSession(t *testing.T) {
	manifest, session, inventory, environment := hostEvidenceRecords(t, true)
	evidence, err := profile.NewHostEvidence(manifest, session, inventory, environment)
	if err != nil {
		t.Fatal(err)
	}
	record := evidence.Record()
	if record.HostID != session.HostID || record.Topology != environment.Topology || record.SessionDigest != session.Digest ||
		record.ManifestDigest != manifest.Digest || record.InventoryDigest != inventory.Digest || record.EnvironmentDigest != environment.Digest ||
		record.Digest != evidence.Digest() {
		t.Fatalf("Host evidence pins = %#v", record)
	}
	if err := profile.ValidateHostEvidenceRecord(record); err != nil {
		t.Fatal(err)
	}
}

func TestNewHostEvidenceWithReporterIdentityPinsExplicitDigest(t *testing.T) {
	manifest, session, inventory, environment := hostEvidenceRecords(t, true)
	reporterIdentity := strings.Repeat("9", 64)
	evidence, err := profile.NewHostEvidenceWithReporterIdentity(manifest, session, inventory, environment, reporterIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Record().ReporterIdentityDigest != reporterIdentity {
		t.Fatalf("reporter identity = %q", evidence.Record().ReporterIdentityDigest)
	}
}

func TestNewHostEvidenceRejectsHostDigestEnvironmentOrSourceDrift(t *testing.T) {
	manifest, session, inventory, environment := hostEvidenceRecords(t, true)
	tests := []struct {
		name string
		run  func() error
	}{
		{"host", func() error {
			changed := inventory
			changed.HostID = "claude"
			_, err := profile.NewHostEvidence(manifest, session, changed, environment)
			return err
		}},
		{"session digest", func() error {
			changed := session
			changed.ManifestDigest = strings.Repeat("f", 64)
			_, err := profile.NewHostEvidence(manifest, changed, inventory, environment)
			return err
		}},
		{"environment", func() error {
			changed := environment
			changed.SessionID = "another-session"
			_, err := profile.NewHostEvidence(manifest, session, inventory, changed)
			return err
		}},
		{"static available source", func() error {
			changed := session
			changed.FeatureObservations = append([]host.FeatureObservation{}, session.FeatureObservations...)
			changed.FeatureObservations[0].Source = host.SourceStaticConfig
			changed.FeatureObservations[0].Digest = ""
			_, err := profile.NewHostEvidence(manifest, changed, inventory, environment)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); err == nil {
				t.Fatal("NewHostEvidence accepted drifted evidence")
			}
		})
	}
}

func TestNewHostEvidenceRejectsAbsoluteEvidenceReferences(t *testing.T) {
	manifest, session, inventory, environment := hostEvidenceRecords(t, true)
	changed := session
	changed.FeatureObservations = append([]host.FeatureObservation{}, session.FeatureObservations...)
	changed.FeatureObservations[0].EvidenceReference = "/private/host/delegation"
	changed.FeatureObservations[0].Digest = ""
	if _, err := profile.NewHostEvidence(manifest, changed, inventory, environment); err == nil {
		t.Fatal("NewHostEvidence accepted an absolute evidence reference")
	}
}

func TestValidateHostEvidenceRecordRejectsManualTopologyAndDigestDrift(t *testing.T) {
	record := hostEvidence(t, true).Record()
	changed := record
	changed.Topology = execution.Topology("MANUAL")
	changed.Digest = changed.ContentDigest()
	if err := profile.ValidateHostEvidenceRecord(changed); err == nil {
		t.Fatal("ValidateHostEvidenceRecord accepted an unknown topology")
	}
	changed = record
	changed.SessionDigest = strings.Repeat("f", 64)
	if err := profile.ValidateHostEvidenceRecord(changed); err == nil {
		t.Fatal("ValidateHostEvidenceRecord accepted digest drift")
	}
}

func TestHostEvidenceRecordIsDeeplyImmutable(t *testing.T) {
	evidence := hostEvidence(t, true)
	first := evidence.Record()
	first.FeatureObservations[0].EvidenceReference = "evidence://changed"
	first.ActionObservations[0].Action.MaximumEffects[0] = "changed"
	second := evidence.Record()
	if second.FeatureObservations[0].EvidenceReference == "evidence://changed" || second.ActionObservations[0].Action.MaximumEffects[0] == "changed" {
		t.Fatal("HostEvidence.Record exposed internal storage")
	}
}
