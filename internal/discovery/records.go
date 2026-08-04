package discovery

import (
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

const (
	discoveryEvidenceSchemaV2 = "oaw.discovery-evidence/v2"
	discoveryReportSchemaV2   = "oaw.discovery-report/v2"
)

type Evidence struct {
	ProviderID      string `json:"provider_id"`
	HostID          string `json:"host_id"`
	SurfaceID       string `json:"surface_id"`
	DistributionKey string `json:"distribution_key"`
	InstallationKey string `json:"installation_key"`
	ProbeID         string `json:"probe_id"`
	Kind            string `json:"kind"`
	Path            string `json:"path"`
	Version         string `json:"version"`
	ContentDigest   string `json:"content_digest"`
}

type Candidate struct {
	ProviderID      string     `json:"provider_id"`
	HostID          string     `json:"host_id"`
	SurfaceID       string     `json:"surface_id"`
	DistributionKey string     `json:"distribution_key"`
	InstallationKey string     `json:"installation_key"`
	Location        string     `json:"location"`
	Version         string     `json:"version"`
	EvidenceDigest  string     `json:"evidence_digest"`
	Evidence        []Evidence `json:"evidence"`
}

type Report struct {
	hostID     string
	candidates []Candidate
	digest     string
}

func (report Report) HostID() string { return report.hostID }

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

func newReport(hostID string, candidates []Candidate) (Report, error) {
	values := cloneCandidates(candidates)
	sort.Slice(values, func(i, j int) bool {
		return candidateSortKey(values[i]) < candidateSortKey(values[j])
	})
	record := struct {
		SchemaVersion string      `json:"schema_version"`
		HostID        string      `json:"host_id"`
		Candidates    []Candidate `json:"candidates"`
	}{discoveryReportSchemaV2, hostID, values}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return Report{}, err
	}
	return Report{hostID: hostID, candidates: values, digest: digest}, nil
}

func normalizeEvidence(values []Evidence) []Evidence {
	result := append([]Evidence{}, values...)
	sort.Slice(result, func(i, j int) bool {
		return evidenceSortKey(result[i]) < evidenceSortKey(result[j])
	})
	return deduplicateEvidence(result)
}

func digestEvidence(values []Evidence) (string, error) {
	type digestRecord struct {
		ProviderID    string `json:"provider_id"`
		HostID        string `json:"host_id"`
		SurfaceID     string `json:"surface_id"`
		ProbeID       string `json:"probe_id"`
		Kind          string `json:"kind"`
		Path          string `json:"path"`
		Version       string `json:"version"`
		ContentDigest string `json:"content_digest"`
	}
	records := make([]digestRecord, len(values))
	for i, value := range values {
		records[i] = digestRecord{
			ProviderID: value.ProviderID, HostID: value.HostID, SurfaceID: value.SurfaceID,
			ProbeID: value.ProbeID, Kind: value.Kind, Path: value.Path,
			Version: value.Version, ContentDigest: value.ContentDigest,
		}
	}
	record := struct {
		SchemaVersion string         `json:"schema_version"`
		Evidence      []digestRecord `json:"evidence"`
	}{discoveryEvidenceSchemaV2, records}
	digest, _, err := canonicaljson.Digest(record)
	return digest, err
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
	return value.ProviderID + "\x00" + value.HostID + "\x00" + value.SurfaceID + "\x00" + value.Location + "\x00" + value.InstallationKey
}

func evidenceSortKey(value Evidence) string {
	return value.ProviderID + "\x00" + value.HostID + "\x00" + value.SurfaceID + "\x00" + value.ProbeID + "\x00" + value.Path
}

func evidenceIdentity(value Evidence) string {
	return evidenceSortKey(value) + "\x00" + value.Kind + "\x00" + value.Version + "\x00" + value.ContentDigest
}
