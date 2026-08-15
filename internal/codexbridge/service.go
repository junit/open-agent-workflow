package codexbridge

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/assurance"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profileinspect"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

type Observer interface {
	Observe(context.Context, string) (appserver.MetadataObservation, error)
}

type ServiceOptions struct {
	Observer          Observer
	UserConfigRoot    string
	ProfileConfigHome string
	UserHome          string
}

type Service struct {
	observer          Observer
	userConfigRoot    string
	profileConfigHome string
	userHome          string
}

type verifiedObservation struct {
	provider    registry.ProviderInstance
	binding     registry.VerifiedBinding
	observation host.BindingObservation
}

func NewService(options ServiceOptions) (*Service, error) {
	if options.Observer == nil {
		return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "Codex Observer is required", nil)
	}
	return &Service{
		observer: options.Observer, userConfigRoot: options.UserConfigRoot,
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
	snapshot, report, err := service.loadInputs(hostContext.CWD)
	if err != nil {
		return ObserveProfileOutput{}, err
	}
	inventory, diagnostics, err := BuildBindingInventory(snapshot.Catalog(), report, metadata, hostContext.CWD)
	if err != nil {
		return ObserveProfileOutput{}, err
	}
	resolutions, _, err := registry.Resolve(snapshot, "codex", report, &inventory)
	if err != nil {
		return ObserveProfileOutput{}, NewError("HOST_OBSERVATION_FAILED", "resolve exact Provider Bindings", err)
	}
	claims, err := claimsForProfile(index, resolutions, inventory)
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

func (service *Service) loadInputs(projectRoot string) (config.Snapshot, discovery.Report, error) {
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: service.userConfigRoot, ProjectRoot: projectRoot})
	if err != nil {
		return config.Snapshot{}, discovery.Report{}, NewError("HOST_OBSERVATION_FAILED", "load current Provider configuration", err)
	}
	userHome := service.userHome
	if userHome == "" {
		userHome, err = os.UserHomeDir()
		if err != nil {
			return config.Snapshot{}, discovery.Report{}, NewError("HOST_OBSERVATION_FAILED", "resolve current user home", err)
		}
	}
	hints := make([]discovery.InstallationHint, 0)
	for _, installation := range snapshot.ProviderInstallations() {
		if installation.HostID != "codex" {
			continue
		}
		hints = append(hints, discovery.InstallationHint{
			ProviderID: installation.ProviderID, HostID: installation.HostID, SurfaceID: installation.SurfaceID,
			Location: installation.Location, DiscoveryProbeID: installation.DiscoveryProbeID,
		})
	}
	report, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: userHome, Installations: hints})
	if err != nil {
		return config.Snapshot{}, discovery.Report{}, NewError("HOST_OBSERVATION_FAILED", "discover current Codex Providers", err)
	}
	return snapshot, report, nil
}

func claimsForProfile(index assurance.ReferenceIndex, resolutions registry.ResolutionReport, inventory host.BindingInventory) ([]assurance.BindingClaim, error) {
	byReference := make(map[string][]verifiedObservation)
	for _, resolution := range resolutions.Resolutions() {
		if resolution.State != registry.ProviderVerified || resolution.Instance == nil {
			continue
		}
		for _, binding := range resolution.Instance.Bindings {
			observation, found := exactObservation(*resolution.Instance, binding, inventory)
			if !found {
				continue
			}
			byReference[binding.Reference] = append(byReference[binding.Reference], verifiedObservation{
				provider: *resolution.Instance, binding: binding, observation: observation,
			})
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
			ProviderID:    match.provider.ProviderID, DistributionID: match.binding.DistributionID,
			DistributionRevision: match.binding.DistributionRevision, DistributionTreeDigest: match.binding.DistributionTreeDigest,
			HostID: "codex", Surface: match.binding.Surface, BindingID: match.binding.BindingID,
			BindingKind: string(match.binding.Kind), BindingReference: match.binding.Reference,
			Invocation: string(match.binding.Invocation), BindingContentDigest: match.binding.BindingTreeDigest,
			Evidence: []assurance.EvidenceReference{
				{Kind: "codex-host-observation", Reference: match.observation.EvidenceReference, Digest: assuranceDigest(match.observation.Digest)},
				{Kind: "provider-discovery", Reference: "evidence://discovery/" + match.provider.EvidenceDigest, Digest: assuranceDigest(match.provider.EvidenceDigest)},
			},
		})
	}
	return claims, nil
}

func exactObservation(instance registry.ProviderInstance, binding registry.VerifiedBinding, inventory host.BindingInventory) (host.BindingObservation, bool) {
	var match host.BindingObservation
	found := false
	for _, observation := range inventory.Observations {
		if observation.HostID != instance.HostID || observation.ProviderID != instance.ProviderID ||
			observation.InstallationKey != instance.InstallationKey || observation.DistributionID != binding.DistributionID ||
			observation.BindingID != binding.BindingID || observation.Surface != binding.Surface || observation.Kind != binding.Kind ||
			observation.Reference != binding.Reference || observation.Invocation != binding.Invocation ||
			observation.BindingTreeDigest != binding.BindingTreeDigest || observation.Digest != binding.BindingEvidenceDigest {
			continue
		}
		if found {
			return host.BindingObservation{}, false
		}
		match, found = observation, true
	}
	return match, found
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
