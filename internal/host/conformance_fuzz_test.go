package host_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func FuzzConformanceReceiptFailsClosed(f *testing.F) {
	f.Add("execution", "evidence://host-conformance", "SUCCEEDED", "binding", "raw")
	f.Add("", "\n", "INVENTED", "", "secret")
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV1, ManifestVersion: "1.0.0", HostID: "codex",
		IntegrationLevel: host.RunnerManaged, Protocols: []string{host.RuntimeProtocolV1},
		BindingKinds: []string{"skill"}, Features: []host.Feature{
			host.FeatureBundleInheritance, host.FeatureCancellation, host.FeatureEvidenceReturn,
			host.FeatureExactBindingInvocation, host.FeatureInvocationDedup,
			host.FeatureIsolatedExecutor, host.FeatureNormalizedObservation, host.FeaturePause, host.FeatureProviderBindingInventory,
		},
	})
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, executionID, evidenceReference, outcome, bindingReference, rawValue string) {
		for _, value := range []string{executionID, evidenceReference, outcome, bindingReference, rawValue} {
			if len(value) > 4096 {
				t.Skip()
			}
		}
		rawSecret := "raw-secret:" + rawValue
		adapter := fuzzReceiptAdapter{
			executionID: executionID, evidenceReference: evidenceReference,
			outcome: host.FixtureOutcome(outcome), bindingReference: bindingReference, raw: rawSecret,
		}
		report, runErr := host.RunConformance("acme/fuzz-host", manifest, adapter)
		if runErr != nil {
			t.Fatalf("RunConformance() error = %v", runErr)
		}
		if _, validateErr := host.NewConformanceReport(report); validateErr != nil {
			t.Fatalf("generated Report is invalid: %v", validateErr)
		}
		encoded, marshalErr := json.Marshal(report)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if strings.Contains(string(encoded), rawSecret) {
			t.Fatalf("Report leaked raw receipt: %s", encoded)
		}
	})
}

type fuzzReceiptAdapter struct {
	executionID       string
	evidenceReference string
	outcome           host.FixtureOutcome
	bindingReference  string
	raw               string
}

func (adapter fuzzReceiptAdapter) CreateExecutor(request host.ExecutorFixtureRequest) (host.ExecutorFixtureReceipt, error) {
	return conformingAdapter{}.CreateExecutor(request)
}

func (adapter fuzzReceiptAdapter) Invoke(request host.InvocationFixtureRequest) (host.ObservationFixtureReceipt, error) {
	receipt, err := conformingAdapter{}.Invoke(request)
	receipt.ExecutionID = adapter.executionID
	receipt.Evidence[0].Reference = adapter.evidenceReference
	receipt.Outcome = adapter.outcome
	receipt.Binding.Reference = adapter.bindingReference
	receipt.RawOutput = adapter.raw
	return receipt, err
}

func (fuzzReceiptAdapter) ObserveProviderBindings(request host.BindingInventoryFixtureRequest) (host.BindingInventory, error) {
	return conformingAdapter{}.ObserveProviderBindings(request)
}

func (adapter fuzzReceiptAdapter) Pause(request host.PauseFixtureRequest) (host.PauseFixtureReceipt, error) {
	return conformingAdapter{}.Pause(request)
}

func (adapter fuzzReceiptAdapter) Cancel(request host.CancelFixtureRequest) (host.CancelFixtureReceipt, error) {
	return conformingAdapter{}.Cancel(request)
}
