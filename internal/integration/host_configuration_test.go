package integration_test

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestTicket09DefaultConfigurationPinsSelectedCodexAndInstructionHosts(t *testing.T) {
	snapshot, err := config.Load(config.LoadOptions{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	records := snapshot.HostIntegrations()
	if len(records) != 9 || len(snapshot.Record().HostIntegrations) != 9 {
		t.Fatalf("default Host Integration count = %d", len(records))
	}
	for _, record := range records {
		if record.Manifest.HostID == "codex" {
			if record.ID != "oaw/codex-runner" || record.Manifest.IntegrationLevel != host.RunnerManaged || record.Conformance == nil || !record.Conformance.Passed {
				t.Fatalf("selected Codex is not Runtime-admitted: %#v", record)
			}
			continue
		}
		if record.Manifest.IntegrationLevel != host.InstructionOnly || record.Conformance != nil || record.Digest == "" {
			t.Fatalf("default Integration claims Runtime guarantees: %#v", record)
		}
	}
	_, err = host.AdmitWorkflow(records, host.RuntimeFrame{HostID: "claude", IntegrationID: "oaw/claude-instruction"}, []catalog.HostBinding{{Host: "claude", Kind: "skill", Reference: "fixture"}})
	if host.ErrorCode(err) != "HOST_INTEGRATION_NOT_ADMITTED" {
		t.Fatalf("instruction-only fallback admission error = %v", err)
	}
}
