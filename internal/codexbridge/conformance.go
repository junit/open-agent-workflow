package codexbridge

import "github.com/wifibaby4u/open-agent-workflow/internal/host"

func BuildConformanceTranscript(facts Facts, receipts []host.InvocationReceipt) (host.ConformanceTranscript, error) {
	return host.NewConformanceTranscript(host.ConformanceTranscript{
		SchemaVersion:      host.HostConformanceTranscriptSchemaV4,
		Session:            facts.Session,
		Inventory:          facts.Inventory,
		EnvironmentReports: []host.EnvironmentReport{facts.Environment},
		Receipts:           append([]host.InvocationReceipt{}, receipts...),
		Invocations:        []host.InvocationRecord{},
	})
}
