package integration_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const externalTranscriptMaximumBytes = 4 << 20

func TestExternalHostNativeTranscript(t *testing.T) {
	path := os.Getenv("OAW_HOST_NATIVE_TRANSCRIPT")
	if path == "" {
		t.Skip("Host-native SUBAGENT transcript unavailable")
	}
	transcript, err := loadExternalHostNativeTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateExternalHostNativeTranscript(transcript); err != nil {
		t.Fatal(err)
	}
}

func TestExternalHostNativeTranscriptRequiresNativeProof(t *testing.T) {
	transcript := externalHostNativeTranscriptFixture(t)
	if err := validateExternalHostNativeTranscript(transcript); err != nil {
		t.Fatalf("valid native transcript rejected: %v", err)
	}

	t.Run("accepted environment disposition", func(t *testing.T) {
		invalid := host.CloneConformanceTranscript(transcript)
		invalid.EnvironmentReports[0].Observations[0].Disposition = execution.DispositionUnavailable
		if err := validateExternalHostNativeTranscript(invalid); err == nil {
			t.Fatal("unavailable child environment unexpectedly accepted")
		}
	})

	t.Run("exact subagent binding visibility", func(t *testing.T) {
		invalid := host.CloneConformanceTranscript(transcript)
		invalid.Inventory.Observations[0].Topologies = []execution.Topology{execution.TopologyCurrent}
		if err := validateExternalHostNativeTranscript(invalid); err == nil {
			t.Fatal("CURRENT-only binding visibility unexpectedly accepted")
		}
	})

	t.Run("paired normalized receipts", func(t *testing.T) {
		invalid := host.CloneConformanceTranscript(transcript)
		invalid.Receipts = invalid.Receipts[1:]
		if err := validateExternalHostNativeTranscript(invalid); err == nil {
			t.Fatal("terminal Receipt without STARTED unexpectedly accepted")
		}
	})
}

func TestLoadExternalHostNativeTranscriptIsStrictAndSecretFree(t *testing.T) {
	transcript := externalHostNativeTranscriptFixture(t)
	raw, err := canonicaljson.Marshal(transcript)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "transcript.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadExternalHostNativeTranscript(path)
	if err != nil || !reflect.DeepEqual(loaded, transcript) {
		t.Fatalf("loadExternalHostNativeTranscript() = %#v, %v", loaded, err)
	}

	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	document["token"] = "must-not-cross-boundary"
	secretRaw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, secretRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadExternalHostNativeTranscript(path); err == nil {
		t.Fatal("secret-bearing transcript field unexpectedly accepted")
	}
}

func loadExternalHostNativeTranscript(path string) (host.ConformanceTranscript, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return host.ConformanceTranscript{}, errors.New("Host-native transcript path must be a clean absolute path")
	}
	entry, err := os.Lstat(path)
	if err != nil {
		return host.ConformanceTranscript{}, fmt.Errorf("Host-native transcript: %w", err)
	}
	if entry.Mode()&os.ModeSymlink != 0 || !entry.Mode().IsRegular() {
		return host.ConformanceTranscript{}, errors.New("Host-native transcript must be a regular non-symlinked file")
	}
	if entry.Size() > externalTranscriptMaximumBytes {
		return host.ConformanceTranscript{}, errors.New("Host-native transcript is too large")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return host.ConformanceTranscript{}, fmt.Errorf("read Host-native transcript: %w", err)
	}
	if !utf8.Valid(raw) {
		return host.ConformanceTranscript{}, errors.New("Host-native transcript is not valid UTF-8")
	}
	if err := rejectSecretBearingFields(raw); err != nil {
		return host.ConformanceTranscript{}, err
	}
	var transcript host.ConformanceTranscript
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&transcript); err != nil {
		return host.ConformanceTranscript{}, fmt.Errorf("Host-native transcript JSON is invalid: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("trailing JSON value")
		}
		return host.ConformanceTranscript{}, fmt.Errorf("Host-native transcript has trailing content: %w", err)
	}
	if transcript.Digest == "" {
		return host.ConformanceTranscript{}, errors.New("Host-native transcript digest is required")
	}
	normalized, err := host.NewConformanceTranscript(transcript)
	if err != nil {
		return host.ConformanceTranscript{}, fmt.Errorf("Host-native transcript is invalid: %w", err)
	}
	if !reflect.DeepEqual(normalized, transcript) {
		return host.ConformanceTranscript{}, errors.New("Host-native transcript is not canonical")
	}
	return transcript, nil
}

func rejectSecretBearingFields(raw []byte) error {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("Host-native transcript JSON is invalid: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("Host-native transcript has trailing content")
	}
	forbidden := map[string]struct{}{
		"api_key": {}, "authorization": {}, "client_secret": {}, "command": {}, "cookie": {},
		"credential": {}, "credentials": {}, "password": {}, "private_key": {}, "process": {},
		"raw_output": {}, "secret": {}, "token": {},
	}
	var walk func(any) error
	walk = func(current any) error {
		switch value := current.(type) {
		case map[string]any:
			for key, child := range value {
				if _, found := forbidden[strings.ToLower(key)]; found {
					return fmt.Errorf("Host-native transcript contains forbidden field %q", key)
				}
				if err := walk(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range value {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(value)
}

func validateExternalHostNativeTranscript(transcript host.ConformanceTranscript) error {
	normalized, err := host.NewConformanceTranscript(transcript)
	if err != nil {
		return fmt.Errorf("Host-native transcript is invalid: %w", err)
	}
	if !reflect.DeepEqual(normalized, transcript) {
		return errors.New("Host-native transcript is not canonical")
	}
	if !slices.Contains(transcript.Session.SupportedTopologies, execution.TopologySubagent) {
		return errors.New("Host session does not declare SUBAGENT")
	}
	report := transcript.EnvironmentReports[0]
	if report.Topology != execution.TopologySubagent || report.ParentSessionID != transcript.Session.SessionID || report.SessionID == transcript.Session.SessionID {
		return errors.New("SUBAGENT Environment Report does not prove a distinct child session")
	}
	if len(report.Observations) == 0 {
		return errors.New("SUBAGENT Environment Report has no surface observations")
	}
	for _, observation := range report.Observations {
		switch observation.Disposition {
		case execution.DispositionInherited, execution.DispositionHostConfigured, execution.DispositionRestricted:
		default:
			return fmt.Errorf("SUBAGENT Environment Report has unaccepted %q disposition", observation.Disposition)
		}
	}
	bindingVisible := false
	for _, observation := range transcript.Inventory.Observations {
		if slices.Contains(observation.Binding.Topologies, execution.TopologySubagent) && slices.Contains(observation.Topologies, execution.TopologySubagent) {
			bindingVisible = true
			break
		}
	}
	if !bindingVisible {
		return errors.New("Host Binding Inventory does not expose an exact SUBAGENT binding")
	}
	if len(transcript.Receipts) < 2 {
		return errors.New("Host-native transcript requires STARTED and terminal Receipts")
	}
	started := make(map[string]bool)
	terminal := make(map[string]bool)
	for _, receipt := range transcript.Receipts {
		if receipt.Topology != execution.TopologySubagent || receipt.ContextFreshness != host.ContextFresh || receipt.HostSessionDigest != transcript.Session.Digest || receipt.EnvironmentReportDigest != report.Digest {
			return errors.New("Invocation Receipt is not pinned to the SUBAGENT session environment")
		}
		key := externalReceiptExecutionKey(receipt)
		switch receipt.Kind {
		case host.ReceiptStarted:
			started[key] = true
		case host.ReceiptCompleted, host.ReceiptFailed, host.ReceiptCancelled:
			terminal[key] = true
		}
	}
	if len(started) == 0 || len(terminal) == 0 {
		return errors.New("Host-native transcript does not contain a paired STARTED and terminal Receipt")
	}
	for key := range terminal {
		if !started[key] {
			return errors.New("terminal Receipt has no matching STARTED Receipt")
		}
	}
	return nil
}

func externalReceiptExecutionKey(receipt host.InvocationReceipt) string {
	return strings.Join([]string{
		receipt.WorkflowID, fmt.Sprint(receipt.BundleGeneration), receipt.BundleDigest,
		receipt.NodeID, receipt.DispatchDigest, receipt.InvocationHandle,
	}, "\x00")
}

func externalHostNativeTranscriptFixture(t *testing.T) host.ConformanceTranscript {
	t.Helper()
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "2.0.0", HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []string{"skill"},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		Features:            []host.Feature{host.FeatureProviderBindingInventory, host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts},
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := host.NewBindingInventory("codex", []host.BindingObservation{{
		HostID: "codex", InstallationKey: "installation-codex", Binding: catalog.HostBinding{
			Host: "codex", Kind: "skill", Reference: "acme:implementation",
			Topologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		}, Topologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}, Source: "native-probe",
		EvidenceReference: "evidence://codex/implementation", Digest: strings.Repeat("1", 64),
	}})
	if err != nil {
		t.Fatal(err)
	}
	report, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-child", ParentSessionID: "session-parent",
		Topology: execution.TopologySubagent, Observations: []execution.EnvironmentObservation{{
			Surface: "skills", Disposition: execution.DispositionInherited, Source: "codex-subagent", Digest: strings.Repeat("2", 64),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV2, HostID: "codex", IntegrationID: "acme/codex-host", IntegrationVersion: "2.0.0",
		SessionID: "session-parent", SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		ProviderInventoryDigest: inventory.Digest, EnvironmentReportDigest: report.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	started, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptStarted,
		WorkflowID: "workflow-external", BundleGeneration: 1, BundleDigest: strings.Repeat("3", 64), NodeID: "implementation",
		Topology: execution.TopologySubagent, HostSessionDigest: session.Digest, InvocationHandle: "child-invocation-1",
		ContextFreshness: host.ContextFresh, EnvironmentReportDigest: report.Digest, DispatchDigest: strings.Repeat("8", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptCompleted,
		WorkflowID: started.WorkflowID, BundleGeneration: started.BundleGeneration, BundleDigest: started.BundleDigest, NodeID: started.NodeID,
		Topology: execution.TopologySubagent, HostSessionDigest: session.Digest, InvocationHandle: started.InvocationHandle,
		ContextFreshness: host.ContextFresh, EnvironmentReportDigest: report.Digest, DispatchDigest: started.DispatchDigest,
		Outcome: "succeeded", Evidence: []host.EvidenceReference{{Kind: "report", Reference: "evidence://result", Digest: strings.Repeat("4", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	transcript, err := host.NewConformanceTranscript(host.ConformanceTranscript{
		SchemaVersion: host.HostConformanceTranscriptSchemaV2, Session: session, Inventory: inventory,
		EnvironmentReports: []host.EnvironmentReport{report}, Receipts: []host.InvocationReceipt{started, completed},
	})
	if err != nil {
		t.Fatal(err)
	}
	return transcript
}
