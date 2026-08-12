package codexbridge

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

const maximumSkillEvidenceBytes int64 = 4 << 20

func BuildBindingInventory(value catalog.Catalog, report discovery.Report, metadata appserver.MetadataObservation, cwd string) (host.BindingInventory, []Diagnostic, error) {
	diagnostics := make([]Diagnostic, 0)
	add := func(code, detail string, providerIDs ...string) {
		diagnostic := NewDiagnostic(code, "binding", detail, true)
		diagnostic.AffectedProviders, diagnostic.AffectedProfiles = diagnosticOwnership(value, providerIDs)
		diagnostics = append(diagnostics, diagnostic)
	}
	empty := func() (host.BindingInventory, []Diagnostic, error) {
		inventory, err := host.BuildBindingInventoryV3("codex", nil)
		return inventory, aggregateDiagnostics(diagnostics), err
	}
	if report.HostID() != "codex" || !validCanonicalPath(cwd) || metadata.Skills.CWD != cwd {
		add("HOST_OBSERVATION_FAILED", "Skill metadata CWD does not match the requested canonical CWD")
		return empty()
	}

	observations := make([]host.BindingObservation, 0)
	for _, entry := range metadata.Skills.Skills {
		if !entry.Enabled {
			continue
		}
		if err := validateSkillIdentity(entry.Name, entry.Scope); err != nil {
			add("HOST_OBSERVATION_FAILED", "enabled Skill has an invalid name or scope")
			continue
		}
		path, err := canonicalSkillPath(entry.Path)
		if err != nil {
			add("PROVIDER_BINDING_CONTENT_MISMATCH", "enabled Skill path is not a physical regular SKILL.md", providerIDsForSkill(value, entry.Name)...)
			continue
		}
		candidates := candidatesContaining(value, report, path)
		if len(candidates) == 0 {
			add("HOST_SKILL_ORPHAN", "enabled Skill is outside every discovered Candidate", providerIDsForSkill(value, entry.Name)...)
			continue
		}
		if len(candidates) != 1 {
			add("HOST_SKILL_INSTALLATION_AMBIGUOUS", "Skill path belongs to more than one Candidate", candidateProviderIDs(candidates)...)
			continue
		}
		candidate := candidates[0]
		bindings := skillBindings(value, candidate, entry.Name)
		if len(bindings) == 0 {
			add("HOST_BINDING_EVIDENCE_REQUIRED", "no declared Skill binding matches the enabled Skill", candidate.ProviderID)
			continue
		}

		matchedRoot := false
		for _, binding := range bindings {
			topologies := intersectTopologies(binding.SupportedTopologies, []execution.Topology{execution.TopologyCurrent})
			if len(topologies) == 0 {
				add("HOST_BINDING_TOPOLOGY_UNAVAILABLE", "declared Skill binding does not support CURRENT", candidate.ProviderID)
				continue
			}
			rootEvidence, found := bindingRoot(candidate.BindingRoots, binding.ID)
			if !found {
				add("PROVIDER_BINDING_CONTENT_MISMATCH", "discovery did not attest the declared Binding tree", candidate.ProviderID)
				continue
			}
			root, err := physicalBindingRoot(candidate, binding)
			if err != nil {
				add("PROVIDER_BINDING_CONTENT_MISMATCH", "declared Binding InstallRoot is not a physical tree", candidate.ProviderID)
				continue
			}
			if path != filepath.Join(root, "SKILL.md") {
				add("HOST_BINDING_INSTALL_ROOT_MISMATCH", "enabled Skill does not belong to the exact declared InstallRoot", candidate.ProviderID)
				continue
			}
			matchedRoot = true
			tree, err := digestLiveBindingRoot(candidate, binding)
			if err != nil || rootEvidence.BindingID != binding.ID || rootEvidence.ContentRoot != binding.ContentRoot ||
				rootEvidence.InstallRoot != binding.InstallRoot || tree.RootDigest != rootEvidence.Tree.RootDigest || tree.RootDigest != binding.TreeDigest {
				add("PROVIDER_BINDING_CONTENT_MISMATCH", "live Binding tree differs from Descriptor and discovery evidence", candidate.ProviderID)
				continue
			}
			evidenceDigest, _, err := canonicaljson.Digest(struct {
				HostID            string                        `json:"host_id"`
				ProviderID        string                        `json:"provider_id"`
				InstallationKey   string                        `json:"installation_key"`
				DistributionID    string                        `json:"distribution_id"`
				BindingID         string                        `json:"binding_id"`
				Surface           string                        `json:"surface"`
				Kind              catalog.BindingKind           `json:"kind"`
				Reference         string                        `json:"reference"`
				Invocation        catalog.InvocationDisposition `json:"invocation"`
				BindingTreeDigest string                        `json:"binding_tree_digest"`
				Scope             string                        `json:"scope"`
				Enabled           bool                          `json:"enabled"`
				Source            host.ObservationSource        `json:"source"`
			}{
				HostID: candidate.HostID, ProviderID: candidate.ProviderID, InstallationKey: candidate.InstallationKey,
				DistributionID: binding.DistributionID, BindingID: binding.ID, Surface: binding.Surface,
				Kind: binding.Kind, Reference: binding.Reference, Invocation: binding.Invocation,
				BindingTreeDigest: tree.RootDigest, Scope: entry.Scope, Enabled: true, Source: host.SourceNativeAPI,
			})
			if err != nil {
				return host.BindingInventory{}, diagnostics, NewError("HOST_OBSERVATION_FAILED", "Skill evidence cannot be canonicalized", err)
			}
			observations = append(observations, host.BindingObservation{
				HostID: candidate.HostID, ProviderID: candidate.ProviderID, InstallationKey: candidate.InstallationKey,
				DistributionID: binding.DistributionID, BindingID: binding.ID, Surface: binding.Surface,
				Kind: binding.Kind, Reference: binding.Reference, Invocation: binding.Invocation,
				BindingTreeDigest: tree.RootDigest, Topologies: topologies, Source: host.SourceNativeAPI,
				EvidenceReference: "evidence://codex/skills-list/" + evidenceDigest,
			})
		}
		if !matchedRoot && len(bindings) > 0 {
			// Exact-root diagnostics above deliberately replace any same-name shortcut.
			continue
		}
	}
	inventory, err := host.BuildBindingInventoryV3("codex", observations)
	if err != nil {
		return host.BindingInventory{}, diagnostics, NewError("HOST_OBSERVATION_FAILED", "Skill inventory cannot be normalized", err)
	}
	return inventory, aggregateDiagnostics(diagnostics), nil
}

func digestLiveBindingRoot(candidate discovery.Candidate, binding catalog.BindingRecord) (integrity.TreeEvidence, error) {
	if !validCanonicalPath(candidate.DiagnosticLocation) {
		return integrity.TreeEvidence{}, errors.New("invalid Binding installation root")
	}
	return integrity.DigestBindingRoot(candidate.DiagnosticLocation, binding.InstallRoot, binding.ContentRoot)
}

func providerIDsForSkill(value catalog.Catalog, skillName string) []string {
	providerIDs := make([]string, 0)
	for _, provider := range value.Providers() {
		for _, binding := range provider.Bindings {
			if binding.Host == "codex" && binding.Kind == catalog.BindingSkill && binding.Reference == skillName {
				providerIDs = append(providerIDs, provider.ID)
				break
			}
		}
	}
	return sortedUniqueDiagnosticOwners(providerIDs)
}

func candidateProviderIDs(values []discovery.Candidate) []string {
	providerIDs := make([]string, 0, len(values))
	for _, value := range values {
		providerIDs = append(providerIDs, value.ProviderID)
	}
	return sortedUniqueDiagnosticOwners(providerIDs)
}

func diagnosticOwnership(value catalog.Catalog, providerIDs []string) ([]string, []string) {
	providers := sortedUniqueDiagnosticOwners(providerIDs)
	providerSet := make(map[string]struct{}, len(providers))
	for _, providerID := range providers {
		providerSet[providerID] = struct{}{}
	}
	recipeProfiles := make(map[string][]string)
	for _, alias := range value.Aliases() {
		recipeProfiles[alias.RecipeID] = append(recipeProfiles[alias.RecipeID], alias.Alias)
	}
	profiles := make([]string, 0)
	for _, recipe := range value.Recipes() {
		if recipeUsesProvider(recipe, providerSet) {
			aliases := recipeProfiles[recipe.ID]
			profiles = append(profiles, aliases...)
			if len(aliases) == 0 && !strings.HasPrefix(recipe.ID, "oaw/") {
				profiles = append(profiles, "USER-DEFINED")
			}
		}
	}
	return providers, sortedUniqueDiagnosticOwners(profiles)
}

func recipeUsesProvider(recipe catalog.ProfileRecipeRecord, providers map[string]struct{}) bool {
	uses := func(providerID string) bool {
		_, found := providers[providerID]
		return found
	}
	for _, slot := range recipe.Slots {
		for _, step := range slot.Pipeline {
			if uses(step.Selector.ProviderID) {
				return true
			}
		}
	}
	for _, addOn := range recipe.AddOns {
		if uses(addOn.Selector.ProviderID) {
			return true
		}
	}
	for _, route := range recipe.IncidentRoutes {
		if uses(route.Handler.ProviderID) {
			return true
		}
	}
	for _, overlay := range recipe.Overlays {
		for _, paused := range overlay.PausedBindings {
			if uses(paused.ProviderID) {
				return true
			}
		}
	}
	return false
}

func canonicalSkillPath(value string) (string, error) {
	if !validCanonicalPath(value) {
		return "", NewError("HOST_OBSERVATION_FAILED", "invalid absolute Skill path", nil)
	}
	info, err := os.Lstat(value)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || filepath.Base(value) != "SKILL.md" {
		return "", NewError("HOST_OBSERVATION_FAILED", "Skill path is not a physical regular SKILL.md", err)
	}
	resolved, err := filepath.EvalSymlinks(value)
	if err != nil {
		return "", NewError("HOST_OBSERVATION_FAILED", "resolve Skill path", err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", NewError("HOST_OBSERVATION_FAILED", "canonicalize Skill path", err)
	}
	resolved = filepath.Clean(resolved)
	return resolved, nil
}

func candidatesContaining(value catalog.Catalog, report discovery.Report, path string) []discovery.Candidate {
	result := make([]discovery.Candidate, 0)
	for _, provider := range value.Providers() {
		for _, candidate := range report.Candidates(provider.ID) {
			if candidate.ProviderID == provider.ID && candidateContainsPath(candidate, report.HostID(), path) {
				result = append(result, candidate)
			}
		}
	}
	return result
}

func candidateContainsPath(candidate discovery.Candidate, reportHost, path string) bool {
	if reportHost != "codex" || candidate.HostID != reportHost || !validCanonicalPath(path) {
		return false
	}
	root := candidate.DiagnosticLocation
	if !validCanonicalPath(root) {
		return false
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || filepath.Clean(resolved) != root {
		return false
	}
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

func physicalBindingRoot(candidate discovery.Candidate, binding catalog.BindingRecord) (string, error) {
	root := candidate.DiagnosticLocation
	if !validCanonicalPath(root) || binding.InstallRoot == "" {
		return "", errors.New("invalid Binding root")
	}
	current := root
	for _, segment := range strings.Split(filepath.FromSlash(binding.InstallRoot), string(filepath.Separator)) {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("Binding root is missing or symlinked")
		}
	}
	current = filepath.Clean(current)
	if current == root || !strings.HasPrefix(current, root+string(filepath.Separator)) {
		return "", errors.New("Binding root escapes installation")
	}
	info, err := os.Lstat(current)
	if err != nil || !info.IsDir() {
		return "", errors.New("Binding root is not a directory")
	}
	return current, nil
}

func bindingRoot(values []discovery.BindingRootEvidence, bindingID string) (discovery.BindingRootEvidence, bool) {
	for _, value := range values {
		if value.BindingID == bindingID {
			return value, true
		}
	}
	return discovery.BindingRootEvidence{}, false
}

func validateSkillIdentity(name, scope string) error {
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) == 0 || utf8.RuneCountInString(name) > 512 ||
		name != strings.TrimSpace(name) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return NewError("HOST_OBSERVATION_FAILED", "invalid Skill name", nil)
	}
	switch scope {
	case "user", "repo", "system", "admin":
		return nil
	default:
		return NewError("HOST_OBSERVATION_FAILED", "invalid Skill scope", nil)
	}
}

func skillBindings(value catalog.Catalog, candidate discovery.Candidate, skillName string) []catalog.BindingRecord {
	result := make([]catalog.BindingRecord, 0)
	for _, provider := range value.Providers() {
		if provider.ID != candidate.ProviderID {
			continue
		}
		for _, binding := range provider.Bindings {
			if binding.Host == candidate.HostID && binding.Surface == candidate.Surface && binding.DistributionID == candidate.DistributionID &&
				binding.Kind == catalog.BindingSkill && binding.Reference == skillName &&
				!slices.ContainsFunc(result, func(existing catalog.BindingRecord) bool { return existing.ID == binding.ID }) {
				result = append(result, binding)
			}
		}
	}
	return result
}

func intersectTopologies(left, right []execution.Topology) []execution.Topology {
	result := make([]execution.Topology, 0, len(left))
	for _, topology := range left {
		if slices.Contains(right, topology) && !slices.Contains(result, topology) {
			result = append(result, topology)
		}
	}
	return result
}

func validCanonicalPath(value string) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= 4096 && filepath.IsAbs(value) &&
		filepath.Clean(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}
