package coordinator_test

import (
	"bytes"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
)

func TestExchangeJSONRejectsMissingEngineWithoutWriting(t *testing.T) {
	var output bytes.Buffer
	err := coordinator.ExchangeJSON(bytes.NewReader(mustMarshal(t, validStartCommand(t))), &output, nil)
	if err == nil || output.Len() != 0 {
		t.Fatalf("ExchangeJSON() = %v, output %q", err, output.String())
	}
}
