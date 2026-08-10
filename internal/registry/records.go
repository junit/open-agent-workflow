package registry

import (
	"fmt"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

const resolutionReportSchemaV4 = "oaw.provider-resolution-report/v4"

type ProviderState string

const (
	ProviderNotFound           ProviderState = "not-found"
	ProviderCandidate          ProviderState = "candidate"
	ProviderVerified           ProviderState = "verified"
	ProviderAmbiguous          ProviderState = "ambiguous"
	ProviderIncompatible       ProviderState = "incompatible"
	ProviderBindingUnavailable ProviderState = "binding-unavailable"
	ProviderDisabled           ProviderState = "disabled"
	ProviderUntrusted          ProviderState = "untrusted"
)

type VerifiedBinding struct {
	BindingID              string                          `json:"binding_id"`
	DistributionID         string                          `json:"distribution_id"`
	DistributionRevision   string                          `json:"distribution_revision"`
	DistributionTreeDigest string                          `json:"distribution_tree_digest"`
	Surface                string                          `json:"surface"`
	Kind                   catalog.BindingKind             `json:"kind"`
	Reference              string                          `json:"reference"`
	Invocation             catalog.InvocationDisposition   `json:"invocation"`
	BindingTreeDigest      string                          `json:"binding_tree_digest"`
	SupportedTopologies    []execution.Topology            `json:"supported_topologies"`
	Delegation             catalog.DelegationRequirements  `json:"delegation"`
	Provenance             discovery.ProvenanceDisposition `json:"provenance"`
	BindingEvidenceDigest  string                          `json:"binding_evidence_digest"`
}

type VerifiedCapability struct {
	ID                 string   `json:"id"`
	BindingIDs         []string `json:"binding_ids"`
	PreferredBindingID string   `json:"preferred_binding_id,omitempty"`
}

type ProviderInstance struct {
	ProviderID             string               `json:"provider_id"`
	HostID                 string               `json:"host_id"`
	DescriptorDigest       string               `json:"descriptor_digest"`
	DistributionID         string               `json:"distribution_id"`
	DistributionRevision   string               `json:"distribution_revision"`
	DistributionTreeDigest string               `json:"distribution_tree_digest"`
	InstallationKey        string               `json:"installation_key"`
	ConfigurationDigest    string               `json:"configuration_digest"`
	BindingInventoryDigest string               `json:"binding_inventory_digest"`
	EvidenceDigest         string               `json:"evidence_digest"`
	Bindings               []VerifiedBinding    `json:"bindings"`
	Capabilities           []VerifiedCapability `json:"capabilities"`
	Digest                 string               `json:"digest"`
}

type ProviderResolution struct {
	ProviderID string                `json:"provider_id"`
	State      ProviderState         `json:"state"`
	Reason     string                `json:"reason"`
	Instance   *ProviderInstance     `json:"instance,omitempty"`
	Candidates []discovery.Candidate `json:"candidates"`
}

type ResolutionReport struct {
	hostID      string
	resolutions []ProviderResolution
	digest      string
}

func (report ResolutionReport) HostID() string { return report.hostID }

func (report ResolutionReport) Resolutions() []ProviderResolution {
	return cloneResolutions(report.resolutions)
}

func (report ResolutionReport) Resolution(providerID string) (ProviderResolution, bool) {
	index := sort.Search(len(report.resolutions), func(i int) bool { return report.resolutions[i].ProviderID >= providerID })
	if index == len(report.resolutions) || report.resolutions[index].ProviderID != providerID {
		return ProviderResolution{}, false
	}
	return cloneResolution(report.resolutions[index]), true
}

func (report ResolutionReport) Digest() string { return report.digest }

func newResolutionReport(hostID string, values []ProviderResolution) (ResolutionReport, error) {
	if _, err := catalog.ParseLocalID(hostID); err != nil {
		return ResolutionReport{}, fmt.Errorf("HOST_PROVIDER_SCOPE_MISMATCH: invalid Host ID %q: %w", hostID, err)
	}
	resolutions := cloneResolutions(values)
	sort.Slice(resolutions, func(i, j int) bool { return resolutions[i].ProviderID < resolutions[j].ProviderID })
	for i := 1; i < len(resolutions); i++ {
		if resolutions[i-1].ProviderID == resolutions[i].ProviderID {
			return ResolutionReport{}, fmt.Errorf("DUPLICATE_PROVIDER_RESOLUTION: %s", resolutions[i].ProviderID)
		}
	}
	record := struct {
		SchemaVersion string               `json:"schema_version"`
		HostID        string               `json:"host_id"`
		Resolutions   []ProviderResolution `json:"resolutions"`
	}{resolutionReportSchemaV4, hostID, resolutions}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return ResolutionReport{}, err
	}
	return ResolutionReport{hostID: hostID, resolutions: resolutions, digest: digest}, nil
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
	if values == nil {
		return nil
	}
	result := make([]discovery.Candidate, len(values))
	for index, value := range values {
		result[index] = value
		result[index].BindingRoots = cloneBindingRoots(value.BindingRoots)
		result[index].Evidence = cloneEvidence(value.Evidence)
	}
	return result
}

func cloneEvidence(values []discovery.Evidence) []discovery.Evidence {
	if values == nil {
		return nil
	}
	result := make([]discovery.Evidence, len(values))
	for index, value := range values {
		result[index] = value
		result[index].BindingRoots = cloneBindingRoots(value.BindingRoots)
	}
	return result
}

func cloneBindingRoots(values []discovery.BindingRootEvidence) []discovery.BindingRootEvidence {
	if values == nil {
		return nil
	}
	result := make([]discovery.BindingRootEvidence, len(values))
	for index, value := range values {
		result[index] = value
		result[index].Tree.Entries = append([]integrity.TreeEntry{}, value.Tree.Entries...)
	}
	return result
}

func cloneProviderInstance(value ProviderInstance) ProviderInstance {
	value.Bindings = cloneVerifiedBindings(value.Bindings)
	value.Capabilities = cloneVerifiedCapabilities(value.Capabilities)
	return value
}

func cloneVerifiedBindings(values []VerifiedBinding) []VerifiedBinding {
	result := make([]VerifiedBinding, len(values))
	for index, value := range values {
		result[index] = value
		result[index].SupportedTopologies = append([]execution.Topology{}, value.SupportedTopologies...)
	}
	return result
}

func cloneVerifiedCapabilities(values []VerifiedCapability) []VerifiedCapability {
	result := make([]VerifiedCapability, len(values))
	for index, value := range values {
		result[index] = value
		result[index].BindingIDs = append([]string{}, value.BindingIDs...)
	}
	return result
}
