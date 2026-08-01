package discovery

import (
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

const (
	discoveryEvidenceSchemaV1 = "oaw.discovery-evidence/v1"
	discoveryReportSchemaV1   = "oaw.discovery-report/v1"
)

type Evidence struct {
	ProviderID    string `json:"provider_id"`
	CandidateKey  string `json:"candidate_key"`
	ProbeID       string `json:"probe_id"`
	Kind          string `json:"kind"`
	Path          string `json:"path"`
	Version       string `json:"version"`
	ContentDigest string `json:"content_digest"`
}

type Candidate struct {
	ProviderID     string     `json:"provider_id"`
	Key            string     `json:"key"`
	Location       string     `json:"location"`
	Version        string     `json:"version"`
	EvidenceDigest string     `json:"evidence_digest"`
	Evidence       []Evidence `json:"evidence"`
}

type Report struct {
	candidates []Candidate
	digest     string
}

func (report Report) Candidates(providerID string) []Candidate {
	start := sort.Search(len(report.candidates), func(i int) bool {
		return report.candidates[i].ProviderID >= providerID
	})
	end := start
	for end < len(report.candidates) && report.candidates[end].ProviderID == providerID {
		end++
	}
	return cloneCandidates(report.candidates[start:end])
}

func (report Report) Digest() string { return report.digest }

func newReport(candidates []Candidate) (Report, error) {
	values := cloneCandidates(candidates)
	sort.Slice(values, func(i, j int) bool {
		return candidateSortKey(values[i]) < candidateSortKey(values[j])
	})
	record := struct {
		SchemaVersion string      `json:"schema_version"`
		Candidates    []Candidate `json:"candidates"`
	}{discoveryReportSchemaV1, values}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return Report{}, err
	}
	return Report{candidates: values, digest: digest}, nil
}

func prepareEvidence(values []Evidence) ([]Evidence, string, error) {
	result := append([]Evidence{}, values...)
	sort.Slice(result, func(i, j int) bool {
		return evidenceSortKey(result[i]) < evidenceSortKey(result[j])
	})
	result = deduplicateEvidence(result)
	record := struct {
		SchemaVersion string     `json:"schema_version"`
		Evidence      []Evidence `json:"evidence"`
	}{discoveryEvidenceSchemaV1, result}
	digest, _, err := canonicaljson.Digest(record)
	return result, digest, err
}

func deduplicateEvidence(values []Evidence) []Evidence {
	if len(values) == 0 {
		return []Evidence{}
	}
	result := make([]Evidence, 0, len(values))
	last := ""
	for _, value := range values {
		key := evidenceIdentity(value)
		if len(result) != 0 && key == last {
			continue
		}
		result = append(result, value)
		last = key
	}
	return result
}

func cloneCandidates(values []Candidate) []Candidate {
	result := make([]Candidate, len(values))
	for i, value := range values {
		result[i] = value
		result[i].Evidence = append([]Evidence{}, value.Evidence...)
	}
	return result
}

func candidateSortKey(value Candidate) string {
	return value.ProviderID + "\x00" + value.Key
}

func evidenceSortKey(value Evidence) string {
	return value.ProviderID + "\x00" + value.CandidateKey + "\x00" + value.ProbeID + "\x00" + value.Path
}

func evidenceIdentity(value Evidence) string {
	return evidenceSortKey(value) + "\x00" + value.Kind + "\x00" + value.Version + "\x00" + value.ContentDigest
}
