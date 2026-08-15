package codexbridge

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/assurance"
	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/profileinspect"
)

type Observer interface {
	Observe(context.Context, string) (appserver.MetadataObservation, error)
}

type ServiceOptions struct {
	Observer          Observer
	Catalog           *catalog.Catalog
	ProfileConfigHome string
	UserHome          string
}

type Service struct {
	observer          Observer
	catalog           catalog.Catalog
	profileConfigHome string
	userHome          string
}

type verifiedObservation struct {
	provider     catalog.ProviderDescriptorRecord
	distribution catalog.DistributionRecord
	binding      catalog.BindingRecord
	candidate    discovery.Candidate
	observation  BindingObservation
}

func NewService(options ServiceOptions) (*Service, error) {
	if options.Observer == nil {
		return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "Codex Observer is required", nil)
	}
	value := catalog.Catalog{}
	if options.Catalog == nil {
		loaded, err := builtin.Load()
		if err != nil {
			return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "load built-in Provider identity", err)
		}
		value = loaded
	} else {
		value = *options.Catalog
	}
	return &Service{
		observer: options.Observer, catalog: value,
		profileConfigHome: options.ProfileConfigHome, userHome: options.UserHome,
	}, nil
}

func (service *Service) ObserveProfile(ctx context.Context, input ObserveProfileInput, hostContext HookContext) (ObserveProfileOutput, error) {
	if strings.TrimSpace(input.Profile) == "" {
		return ObserveProfileOutput{}, NewError("PROFILE_SELECTION_INVALID", "a source-qualified Profile selector is required", nil)
	}
	if err := ValidateHookContext(hostContext); err != nil {
		return ObserveProfileOutput{}, err
	}
	profile, err := service.resolveProfile(hostContext.CWD, input.Profile)
	if err != nil {
		code := profileinspect.SelectionErrorCode(err)
		if code == "" {
			code = "PROFILE_SELECTION_INVALID"
		}
		return ObserveProfileOutput{}, NewError(code, "resolve selected Markdown Profile", err)
	}
	index, err := assurance.Inspect(profile)
	if err != nil {
		return ObserveProfileOutput{}, assuranceBridgeError("inspect selected Markdown Profile", err)
	}

	metadata, err := service.observer.Observe(ctx, hostContext.CWD)
	if err != nil {
		return ObserveProfileOutput{}, bridgeErrorFromAppServer(err)
	}
	report, err := service.loadInputs()
	if err != nil {
		return ObserveProfileOutput{}, err
	}
	inventory, diagnostics, err := BuildBindingInventory(service.catalog, report, metadata, hostContext.CWD)
	if err != nil {
		return ObserveProfileOutput{}, err
	}
	claims, err := claimsForProfile(index, service.catalog, report, inventory)
	if err != nil {
		return ObserveProfileOutput{}, err
	}
	overlay, err := assurance.Issue(profile, assurance.IssueRequest{
		SchemaVersion: assurance.IssueRequestSchemaV1,
		Issuer:        BridgeIntegrationID,
		Claims:        claims,
	})
	if err != nil {
		return ObserveProfileOutput{}, assuranceBridgeError("issue Assurance Overlay", err)
	}
	if err := assurance.Verify(profile, overlay); err != nil {
		return ObserveProfileOutput{}, assuranceBridgeError("verify issued Assurance Overlay", err)
	}
	return ObserveProfileOutput{
		Overlay: overlay, Diagnostics: normalizeDiagnostics(diagnostics, maximumObserveProfileDiagnostics),
	}, nil
}

func (service *Service) resolveProfile(projectRoot, selector string) (profileinspect.Profile, error) {
	inventory, err := profileinspect.Discover(profileinspect.Environment{
		WorkingDir: projectRoot, Home: service.userHome, ConfigHome: service.profileConfigHome,
	})
	if err != nil {
		return profileinspect.Profile{}, err
	}
	return profileinspect.ResolveQualified(inventory, selector)
}

func (service *Service) loadInputs() (discovery.Report, error) {
	userHome := service.userHome
	var err error
	if userHome == "" {
		userHome, err = os.UserHomeDir()
		if err != nil {
			return discovery.Report{}, NewError("HOST_OBSERVATION_FAILED", "resolve current user home", err)
		}
	}
	report, err := discovery.Discover(service.catalog, discovery.Options{HostID: "codex", UserHome: userHome})
	if err != nil {
		return discovery.Report{}, NewError("HOST_OBSERVATION_FAILED", "discover current Codex Providers", err)
	}
	return report, nil
}

func claimsForProfile(index assurance.ReferenceIndex, value catalog.Catalog, report discovery.Report, inventory BindingInventory) ([]assurance.BindingClaim, error) {
	byReference := make(map[string][]verifiedObservation)
	for _, provider := range value.Providers() {
		for _, candidate := range report.Candidates(provider.ID) {
			distribution, found := distributionByID(provider.Distributions, candidate.DistributionID)
			if !found || !candidateMatchesDistribution(provider.ID, candidate, distribution) {
				continue
			}
			for _, binding := range provider.Bindings {
				if !bindingMatchesCandidate(binding, candidate) || !candidateAttestsBinding(candidate, binding) {
					continue
				}
				observation, found := exactObservation(provider.ID, candidate, binding, inventory)
				if !found {
					continue
				}
				byReference[binding.Reference] = append(byReference[binding.Reference], verifiedObservation{
					provider: provider, distribution: distribution, binding: binding,
					candidate: candidate, observation: observation,
				})
			}
		}
	}

	claims := make([]assurance.BindingClaim, 0, len(index.Occurrences))
	for _, occurrence := range index.Occurrences {
		matches := byReference[occurrence.BindingReference]
		if len(matches) != 1 {
			return nil, NewError(
				"ASSURANCE_BINDING_UNAVAILABLE",
				fmt.Sprintf("Profile occurrence %s has %d exact current Codex Bindings", occurrence.OccurrenceRef, len(matches)),
				nil,
			)
		}
		match := matches[0]
		claims = append(claims, assurance.BindingClaim{
			OccurrenceRef: occurrence.OccurrenceRef,
			ProviderID:    match.provider.ID, DistributionID: match.binding.DistributionID,
			DistributionRevision: match.distribution.Revision, DistributionTreeDigest: match.distribution.TreeDigest,
			HostID: "codex", Surface: match.binding.Surface, BindingID: match.binding.ID,
			BindingKind: string(match.binding.Kind), BindingReference: match.binding.Reference,
			Invocation: string(match.binding.Invocation), BindingContentDigest: match.binding.TreeDigest,
			Evidence: []assurance.EvidenceReference{
				{Kind: "codex-host-observation", Reference: match.observation.EvidenceReference, Digest: assuranceDigest(match.observation.Digest)},
				{Kind: "provider-discovery", Reference: "evidence://discovery/" + match.candidate.EvidenceDigest, Digest: assuranceDigest(match.candidate.EvidenceDigest)},
			},
		})
	}
	return claims, nil
}

func exactObservation(providerID string, candidate discovery.Candidate, binding catalog.BindingRecord, inventory BindingInventory) (BindingObservation, bool) {
	var match BindingObservation
	found := false
	for _, observation := range inventory.Observations {
		if observation.HostID != candidate.HostID || observation.ProviderID != providerID ||
			observation.InstallationKey != candidate.InstallationKey || observation.DistributionID != binding.DistributionID ||
			observation.BindingID != binding.ID || observation.Surface != binding.Surface || observation.Kind != binding.Kind ||
			observation.Reference != binding.Reference || observation.Invocation != binding.Invocation ||
			observation.BindingTreeDigest != binding.TreeDigest {
			continue
		}
		if found {
			return BindingObservation{}, false
		}
		match, found = observation, true
	}
	return match, found
}

func distributionByID(values []catalog.DistributionRecord, id string) (catalog.DistributionRecord, bool) {
	for _, value := range values {
		if value.ID == id {
			return value, true
		}
	}
	return catalog.DistributionRecord{}, false
}

func candidateMatchesDistribution(providerID string, candidate discovery.Candidate, distribution catalog.DistributionRecord) bool {
	if candidate.ProviderID != providerID || candidate.HostID != "codex" || candidate.DistributionID != distribution.ID ||
		candidate.Surface == "" || candidate.InstallationKey == "" || candidate.EvidenceDigest == "" {
		return false
	}
	switch candidate.Provenance {
	case discovery.ProvenanceDistributionAttested:
		return candidate.ObservedRevision == distribution.Revision && candidate.DistributionTreeDigest == distribution.TreeDigest
	case discovery.ProvenanceContentEquivalent:
		return candidate.ObservedRevision == "" && candidate.DistributionTreeDigest == ""
	default:
		return false
	}
}

func bindingMatchesCandidate(binding catalog.BindingRecord, candidate discovery.Candidate) bool {
	return binding.Host == candidate.HostID && binding.Surface == candidate.Surface &&
		binding.DistributionID == candidate.DistributionID
}

func candidateAttestsBinding(candidate discovery.Candidate, binding catalog.BindingRecord) bool {
	root, found := bindingRoot(candidate.BindingRoots, binding.ID)
	return found && root.ContentRoot == binding.ContentRoot && root.InstallRoot == binding.InstallRoot &&
		root.Tree.RootDigest == binding.TreeDigest
}

func assuranceDigest(value string) string {
	if strings.HasPrefix(value, "sha256:") {
		return value
	}
	return "sha256:" + value
}

func assuranceBridgeError(detail string, err error) error {
	code := assurance.ErrorCode(err)
	if code == "" {
		code = "ASSURANCE_FAILED"
	}
	return NewError(code, detail, err)
}

func bridgeErrorFromAppServer(err error) error {
	if err == nil {
		return nil
	}
	code := appserver.Code(err)
	if code == "" {
		code = "HOST_OBSERVATION_FAILED"
	}
	return NewError(code, "Codex metadata observation failed", err)
}
