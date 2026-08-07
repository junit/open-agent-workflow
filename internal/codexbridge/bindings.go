package codexbridge

import (
	"io"
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
)

const maximumSkillEvidenceBytes int64 = 4 << 20

func BuildBindingInventory(value catalog.Catalog, report discovery.Report, metadata appserver.MetadataObservation, cwd string) (host.BindingInventory, []Diagnostic, error) {
	diagnostics := make([]Diagnostic, 0)
	add := func(code, detail string) {
		diagnostics = append(diagnostics, NewDiagnostic(code, "binding", detail, true))
	}
	if report.HostID() != "codex" || !validCanonicalPath(cwd) || metadata.Skills.CWD != cwd {
		inventory, err := host.NewBindingInventory("codex", nil)
		return inventory, append(diagnostics, NewDiagnostic("HOST_OBSERVATION_FAILED", "binding", "Skill metadata CWD does not match the requested canonical CWD", true)), err
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
			add("HOST_OBSERVATION_FAILED", "Skill path is not a canonical regular SKILL.md")
			continue
		}
		candidates := candidatesContaining(value, report, path)
		if len(candidates) == 0 {
			add("HOST_SKILL_ORPHAN", "enabled Skill is outside every discovered Candidate")
			continue
		}
		if len(candidates) != 1 {
			add("HOST_SKILL_INSTALLATION_AMBIGUOUS", "Skill path belongs to more than one Candidate")
			continue
		}
		candidate := candidates[0]
		bindings := skillBindings(value, candidate.ProviderID, entry.Name)
		if len(bindings) == 0 {
			add("HOST_BINDING_EVIDENCE_REQUIRED", "no declared Skill binding matches the enabled Skill")
			continue
		}
		type availableBinding struct {
			binding    catalog.HostBinding
			topologies []execution.Topology
		}
		available := make([]availableBinding, 0, len(bindings))
		for _, binding := range bindings {
			topologies := intersectTopologies(binding.Topologies, []execution.Topology{execution.TopologyCurrent})
			if len(topologies) == 0 {
				add("HOST_BINDING_TOPOLOGY_UNAVAILABLE", "declared Skill binding does not support CURRENT")
				continue
			}
			available = append(available, availableBinding{binding: binding, topologies: topologies})
		}
		if len(available) == 0 {
			continue
		}
		content, err := readSkillEvidence(path)
		if err != nil {
			add("HOST_OBSERVATION_FAILED", "enabled Skill content could not be read")
			continue
		}
		for _, admitted := range available {
			record := struct {
				Name            string `json:"name"`
				Scope           string `json:"scope"`
				Path            string `json:"path"`
				ContentDigest   string `json:"content_digest"`
				InstallationKey string `json:"installation_key"`
				Source          string `json:"source"`
				Enabled         bool   `json:"enabled"`
			}{
				Name: entry.Name, Scope: entry.Scope, Path: path,
				ContentDigest: canonicaljson.DigestBytes(content), InstallationKey: candidate.InstallationKey,
				Source: "native-probe", Enabled: true,
			}
			digest, _, err := canonicaljson.Digest(record)
			if err != nil {
				return host.BindingInventory{}, diagnostics, NewError("HOST_OBSERVATION_FAILED", "Skill evidence cannot be canonicalized", err)
			}
			observations = append(observations, host.BindingObservation{
				HostID: candidate.HostID, InstallationKey: candidate.InstallationKey, Binding: admitted.binding,
				Topologies: admitted.topologies, Source: "native-probe",
				EvidenceReference: "evidence://codex/skills-list/" + digest, Digest: digest,
			})
		}
	}
	inventory, err := host.NewBindingInventory("codex", observations)
	if err != nil {
		return host.BindingInventory{}, diagnostics, NewError("HOST_OBSERVATION_FAILED", "Skill inventory cannot be normalized", err)
	}
	return inventory, diagnostics, nil
}

func canonicalSkillPath(value string) (string, error) {
	if !validCanonicalPath(value) {
		return "", NewError("HOST_OBSERVATION_FAILED", "invalid absolute Skill path", nil)
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
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() || filepath.Base(resolved) != "SKILL.md" {
		return "", NewError("HOST_OBSERVATION_FAILED", "Skill path is not a regular SKILL.md", err)
	}
	return resolved, nil
}

func readSkillEvidence(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, NewError("HOST_OBSERVATION_FAILED", "open Skill evidence", err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maximumSkillEvidenceBytes {
		return nil, NewError("HOST_OBSERVATION_FAILED", "Skill evidence is unavailable or oversized", err)
	}
	content, err := io.ReadAll(io.LimitReader(file, maximumSkillEvidenceBytes+1))
	if err != nil || int64(len(content)) > maximumSkillEvidenceBytes {
		return nil, NewError("HOST_OBSERVATION_FAILED", "read Skill evidence", err)
	}
	return content, nil
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
	if reportHost != "codex" || candidate.HostID != "codex" || candidate.HostID != reportHost || !validCanonicalPath(path) {
		return false
	}
	root, err := filepath.EvalSymlinks(candidate.Location)
	if err != nil {
		return false
	}
	root, err = filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false
	}
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
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

func skillBindings(value catalog.Catalog, providerID, skillName string) []catalog.HostBinding {
	result := make([]catalog.HostBinding, 0)
	for _, provider := range value.Providers() {
		if provider.ID != providerID {
			continue
		}
		for _, capability := range provider.Capabilities {
			for _, binding := range capability.HostBindings {
				if binding.Host == "codex" && binding.Kind == "skill" && binding.Reference == skillName &&
					!slices.ContainsFunc(result, func(existing catalog.HostBinding) bool {
						return existing.Host == binding.Host && existing.Kind == binding.Kind && existing.Reference == binding.Reference
					}) {
					result = append(result, binding)
				}
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
