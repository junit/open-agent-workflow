package host_test

import (
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestNormalizeDispatchResultClosesAndSortsHostEvidence(t *testing.T) {
	request := host.DispatchRequest{GrantID: "grant", InvocationID: "invocation", ExecutorID: "executor", BundleDigest: strings.Repeat("a", 64), Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "to-spec"}}
	result, err := host.NormalizeDispatchResult(request, host.DispatchResult{
		GrantID: "grant", InvocationID: "invocation", ExecutorID: "executor", ExecutionID: "execution", Outcome: host.DispatchSucceeded,
		Evidence: []host.DispatchEvidence{{Reference: "evidence://z", Digest: strings.Repeat("b", 64)}, {Reference: "evidence://a", Digest: strings.Repeat("c", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Evidence[0].Reference != "evidence://a" || result.Evidence[1].Reference != "evidence://z" {
		t.Fatalf("evidence order = %#v", result.Evidence)
	}
}

func TestNormalizeDispatchResultRejectsForgedOrUntrustedValues(t *testing.T) {
	request := host.DispatchRequest{GrantID: "grant", InvocationID: "invocation", ExecutorID: "executor", BundleDigest: strings.Repeat("a", 64), Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "to-spec"}}
	for _, mutate := range []func(*host.DispatchResult){
		func(value *host.DispatchResult) { value.InvocationID = "other" },
		func(value *host.DispatchResult) { value.Outcome = "invented" },
		func(value *host.DispatchResult) { value.Evidence[0].Digest = "bad" },
		func(value *host.DispatchResult) { value.Evidence = nil },
	} {
		candidate := host.DispatchResult{GrantID: "grant", InvocationID: "invocation", ExecutorID: "executor", ExecutionID: "execution", Outcome: host.DispatchSucceeded, Evidence: []host.DispatchEvidence{{Reference: "evidence://a", Digest: strings.Repeat("b", 64)}}}
		mutate(&candidate)
		if _, err := host.NormalizeDispatchResult(request, candidate); err == nil {
			t.Fatal("NormalizeDispatchResult accepted forged value")
		}
	}
}
