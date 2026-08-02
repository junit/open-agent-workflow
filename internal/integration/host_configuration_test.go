package integration_test

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestTicket08DefaultConfigurationPinsOnlyInstructionHostRecords(t *testing.T) {
	snapshot, err := config.Load(config.LoadOptions{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	records := snapshot.HostIntegrations()
	if len(records) != 9 || len(snapshot.Record().HostIntegrations) != 9 {
		t.Fatalf("default Host Integration count = %d", len(records))
	}
	for _, record := range records {
		if record.Manifest.IntegrationLevel != host.InstructionOnly || record.Conformance != nil || record.Digest == "" {
			t.Fatalf("default Integration claims Runtime guarantees: %#v", record)
		}
	}
	_, err = host.AdmitWorkflow(records, host.RuntimeFrame{IntegrationID: "oaw/codex-instruction"}, []catalog.HostBinding{{Host: "codex", Kind: "skill", Reference: "fixture"}})
	if host.ErrorCode(err) != "HOST_INTEGRATION_NOT_ADMITTED" {
		t.Fatalf("instruction-only fallback admission error = %v", err)
	}
}
