package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func generateCodexHost(root string) error {
	transcriptPath := filepath.Join(root, "internal", "assets", "conformance", "codex-host-v1.json")
	auditPath := filepath.Join(root, "internal", "assets", "audits", "codex-host-v1.json")
	integrationsPath := filepath.Join(root, "internal", "assets", "host-integrations.json")

	var transcript host.ConformanceTranscript
	if err := readStrictJSON(transcriptPath, &transcript); err != nil {
		return err
	}
	transcript, err := host.NewConformanceTranscript(transcript)
	if err != nil {
		return fmt.Errorf("normalize Codex Host transcript: %w", err)
	}
	var audit host.AuditEvidence
	if err := readStrictJSON(auditPath, &audit); err != nil {
		return err
	}
	audit, err = host.NewAuditEvidence(audit)
	if err != nil {
		return fmt.Errorf("normalize Codex Host audit: %w", err)
	}
	integration, err := buildCodexHostIntegration(transcript, audit)
	if err != nil {
		return err
	}

	raw, err := os.ReadFile(integrationsPath)
	if err != nil {
		return fmt.Errorf("read Host Integration set: %w", err)
	}
	set, err := host.DecodeIntegrationSetJSON(raw)
	if err != nil {
		return fmt.Errorf("decode Host Integration set: %w", err)
	}
	replaced := false
	values := make([]host.IntegrationRecord, 0, len(set.Integrations)+1)
	for _, value := range set.Integrations {
		if value.ID == codexbridge.BridgeIntegrationID {
			values = append(values, integration)
			replaced = true
			continue
		}
		values = append(values, value)
	}
	if !replaced {
		values = append(values, integration)
	}
	sort.Slice(values, func(left, right int) bool { return values[left].ID < values[right].ID })
	set.Integrations = values
	encoded, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Host Integration set: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(integrationsPath, encoded, 0o644); err != nil {
		return fmt.Errorf("write Host Integration set: %w", err)
	}
	return nil
}

func buildCodexHostIntegration(transcript host.ConformanceTranscript, audit host.AuditEvidence) (host.IntegrationRecord, error) {
	manifest, err := codexbridge.CodexHostManifest()
	if err != nil {
		return host.IntegrationRecord{}, err
	}
	report, err := host.ValidateConformanceTranscript(manifest, transcript)
	if err != nil {
		return host.IntegrationRecord{}, err
	}
	return host.NewIntegration(host.IntegrationRecord{
		SchemaVersion:      host.HostIntegrationSchemaV2,
		IntegrationVersion: codexbridge.BridgeIntegrationVersion,
		ID:                 codexbridge.BridgeIntegrationID,
		Manifest:           manifest,
		ManifestDigest:     manifest.ContentDigest(),
		Audit:              audit,
		Conformance:        &report,
	})
}

func readStrictJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("trailing JSON value")
		}
		return fmt.Errorf("decode trailing %s: %w", path, err)
	}
	return nil
}
