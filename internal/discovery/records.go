package discovery

import (
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

const (
	discoveryEvidenceSchemaV3 = "oaw.discovery-evidence/v3"
	discoveryReportSchemaV3   = "oaw.discovery-report/v3"
)

type ProvenanceDisposition string

const (
	ProvenanceDistributionAttested ProvenanceDisposition = "distribution-attested"
	ProvenanceContentEquivalent    ProvenanceDisposition = "content-equivalent"
)

type BindingRootEvidence struct {
	BindingID   string                 `json:"binding_id"`
	ContentRoot string                 `json:"content_root"`
	InstallRoot string                 `json:"install_root"`
	Tree        integrity.TreeEvidence `json:"tree"`
}

type Evidence struct {
	ProviderID        string                `json:"provider_id"`
	HostID            string                `json:"host_id"`
	Surface           string                `json:"surface"`
	DistributionID    string                `json:"distribution_id"`
	ObservedRevision  string                `json:"observed_revision,omitempty"`
	InstallationKey   string                `json:"installation_key"`
	ProbeID           string                `json:"probe_id"`
	Kind              string                `json:"kind"`
	BindingRoots      []BindingRootEvidence `json:"binding_roots"`
	EvidenceReference string                `json:"evidence_reference"`
	Digest            string                `json:"digest"`
}

type Candidate struct {
	ProviderID             string                `json:"provider_id"`
	HostID                 string                `json:"host_id"`
	Surface                string                `json:"surface"`
	DistributionID         string                `json:"distribution_id"`
	InstallationKey        string                `json:"installation_key"`
	DiagnosticLocation     string                `json:"diagnostic_location,omitempty"`
	ObservedRevision       string                `json:"observed_revision,omitempty"`
	DistributionTreeDigest string                `json:"distribution_tree_digest,omitempty"`
	Provenance             ProvenanceDisposition `json:"provenance,omitempty"`
	BindingRoots           []BindingRootEvidence `json:"binding_roots"`
	EvidenceDigest         string                `json:"evidence_digest"`
	Evidence               []Evidence            `json:"evidence"`
}

type Report struct {
	hostID     string
	candidates []Candidate
	digest     string
}

func (report Report) HostID() string { return report.hostID }

func (report Report) Candidates(providerID string) []Candidate {
	start := sort.Search(len(report.candidates), func(i int) bool { return report.candidates[i].ProviderID >= providerID })
	end := start
	for end < len(report.candidates) && report.candidates[end].ProviderID == providerID {
		end++
	}
	return cloneCandidates(report.candidates[start:end])
}

func (report Report) Digest() string { return report.digest }

func newReport(hostID string, candidates []Candidate) (Report, error) {
	values := cloneCandidates(candidates)
	sort.Slice(values, func(i, j int) bool { return candidateSortKey(values[i]) < candidateSortKey(values[j]) })
	record := struct {
		SchemaVersion string      `json:"schema_version"`
		HostID        string      `json:"host_id"`
		Candidates    []Candidate `json:"candidates"`
	}{discoveryReportSchemaV3, hostID, values}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return Report{}, err
	}
	return Report{hostID: hostID, candidates: values, digest: digest}, nil
}

func normalizeEvidence(values []Evidence) []Evidence {
	result := cloneEvidence(values)
	sort.Slice(result, func(i, j int) bool { return evidenceSortKey(result[i]) < evidenceSortKey(result[j]) })
	for index := range result {
		result[index].BindingRoots = normalizeBindingRoots(result[index].BindingRoots)
		result[index].Digest = evidenceDigest(result[index])
		result[index].EvidenceReference = "evidence://discovery/" + result[index].Digest
	}
	return result
}

func evidenceDigest(value Evidence) string {
	value.Digest = ""
	value.EvidenceReference = ""
	digest, _, err := canonicaljson.Digest(struct {
		SchemaVersion string   `json:"schema_version"`
		Evidence      Evidence `json:"evidence"`
	}{discoveryEvidenceSchemaV3, value})
	if err != nil {
		return ""
	}
	return digest
}

func digestEvidence(values []Evidence) (string, error) {
	digest, _, err := canonicaljson.Digest(struct {
		SchemaVersion string     `json:"schema_version"`
		Evidence      []Evidence `json:"evidence"`
	}{discoveryEvidenceSchemaV3, normalizeEvidence(values)})
	return digest, err
}

func normalizeBindingRoots(values []BindingRootEvidence) []BindingRootEvidence {
	result := cloneBindingRoots(values)
	sort.Slice(result, func(i, j int) bool { return result[i].BindingID < result[j].BindingID })
	for index := range result {
		result[index].Tree.Entries = append([]integrity.TreeEntry{}, result[index].Tree.Entries...)
	}
	return result
}

func cloneCandidates(values []Candidate) []Candidate {
	if values == nil {
		return nil
	}
	result := make([]Candidate, len(values))
	for index, value := range values {
		result[index] = value
		result[index].BindingRoots = cloneBindingRoots(value.BindingRoots)
		for rootIndex := range result[index].BindingRoots {
			result[index].BindingRoots[rootIndex].Tree.Entries = append([]integrity.TreeEntry{}, value.BindingRoots[rootIndex].Tree.Entries...)
		}
		result[index].Evidence = cloneEvidence(value.Evidence)
	}
	return result
}

func cloneEvidence(values []Evidence) []Evidence {
	if values == nil {
		return nil
	}
	result := make([]Evidence, len(values))
	for index, value := range values {
		result[index] = value
		result[index].BindingRoots = cloneBindingRoots(value.BindingRoots)
		for rootIndex := range result[index].BindingRoots {
			result[index].BindingRoots[rootIndex].Tree.Entries = append([]integrity.TreeEntry{}, value.BindingRoots[rootIndex].Tree.Entries...)
		}
	}
	return result
}

func cloneBindingRoots(values []BindingRootEvidence) []BindingRootEvidence {
	if values == nil {
		return nil
	}
	result := make([]BindingRootEvidence, len(values))
	copy(result, values)
	for index := range result {
		result[index].Tree.Entries = append([]integrity.TreeEntry{}, values[index].Tree.Entries...)
	}
	return result
}

func candidateSortKey(value Candidate) string {
	return value.ProviderID + "\x00" + value.HostID + "\x00" + value.Surface + "\x00" + value.DistributionID + "\x00" + value.InstallationKey + "\x00" + value.DiagnosticLocation
}

func evidenceSortKey(value Evidence) string {
	return value.ProviderID + "\x00" + value.HostID + "\x00" + value.Surface + "\x00" + value.DistributionID + "\x00" + value.ProbeID + "\x00" + value.InstallationKey
}
