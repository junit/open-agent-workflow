package registry

import (
	"fmt"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
)

const resolutionReportSchemaV1 = "oaw.provider-resolution-report/v1"

type ProviderState string

const (
	NotFound           ProviderState = "not-found"
	CandidateState     ProviderState = "candidate"
	Verified           ProviderState = "verified"
	Ambiguous          ProviderState = "ambiguous"
	Incompatible       ProviderState = "incompatible"
	BindingUnavailable ProviderState = "binding-unavailable"
	Disabled           ProviderState = "disabled"
	Untrusted          ProviderState = "untrusted"
)

type BindingInventory struct {
	Host     string                `json:"host"`
	Bindings []catalog.HostBinding `json:"bindings"`
}

type VerifiedCapability struct {
	ID      string              `json:"id"`
	Binding catalog.HostBinding `json:"binding"`
}

type ProviderInstance struct {
	ProviderID          string               `json:"provider_id"`
	DescriptorDigest    string               `json:"descriptor_digest"`
	Location            string               `json:"location"`
	Version             string               `json:"version"`
	ConfigurationDigest string               `json:"configuration_digest"`
	BindingDigest       string               `json:"binding_digest"`
	EvidenceDigest      string               `json:"evidence_digest"`
	Capabilities        []VerifiedCapability `json:"capabilities"`
	Digest              string               `json:"digest"`
}

type ProviderResolution struct {
	ProviderID string                `json:"provider_id"`
	State      ProviderState         `json:"state"`
	Reason     string                `json:"reason"`
	Instance   *ProviderInstance     `json:"instance"`
	Candidates []discovery.Candidate `json:"candidates"`
}

type ResolutionReport struct {
	resolutions []ProviderResolution
	digest      string
}

func (report ResolutionReport) Resolutions() []ProviderResolution {
	return cloneResolutions(report.resolutions)
}

func (report ResolutionReport) Resolution(providerID string) (ProviderResolution, bool) {
	index := sort.Search(len(report.resolutions), func(i int) bool {
		return report.resolutions[i].ProviderID >= providerID
	})
	if index == len(report.resolutions) || report.resolutions[index].ProviderID != providerID {
		return ProviderResolution{}, false
	}
	return cloneResolution(report.resolutions[index]), true
}

func (report ResolutionReport) Digest() string { return report.digest }

func newResolutionReport(values []ProviderResolution) (ResolutionReport, error) {
	resolutions := cloneResolutions(values)
	sort.Slice(resolutions, func(i, j int) bool { return resolutions[i].ProviderID < resolutions[j].ProviderID })
	for i := 1; i < len(resolutions); i++ {
		if resolutions[i-1].ProviderID == resolutions[i].ProviderID {
			return ResolutionReport{}, fmt.Errorf("DUPLICATE_PROVIDER_RESOLUTION: %s", resolutions[i].ProviderID)
		}
	}
	record := struct {
		SchemaVersion string               `json:"schema_version"`
		Resolutions   []ProviderResolution `json:"resolutions"`
	}{resolutionReportSchemaV1, resolutions}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return ResolutionReport{}, err
	}
	return ResolutionReport{resolutions: resolutions, digest: digest}, nil
}

func cloneResolutions(values []ProviderResolution) []ProviderResolution {
	result := make([]ProviderResolution, len(values))
	for i, value := range values {
		result[i] = cloneResolution(value)
	}
	return result
}

func cloneResolution(value ProviderResolution) ProviderResolution {
	if value.Instance != nil {
		instance := cloneProviderInstance(*value.Instance)
		value.Instance = &instance
	}
	value.Candidates = cloneCandidates(value.Candidates)
	return value
}

func cloneCandidates(values []discovery.Candidate) []discovery.Candidate {
	result := make([]discovery.Candidate, len(values))
	for i, value := range values {
		result[i] = value
		result[i].Evidence = append([]discovery.Evidence{}, value.Evidence...)
	}
	return result
}

func cloneProviderInstance(value ProviderInstance) ProviderInstance {
	value.Capabilities = append([]VerifiedCapability{}, value.Capabilities...)
	return value
}
