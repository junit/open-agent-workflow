package codex

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/BurntSushi/toml"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const defaultMaximumBindingEvidenceBytes int64 = 4 << 20

type InventoryOptions struct {
	UserHome             string
	CodexConfigRoot      string
	MaximumEvidenceBytes int64
}

func ObserveBindings(value catalog.Catalog, report discovery.Report, options InventoryOptions) (host.BindingInventory, error) {
	if report.HostID() != "codex" {
		return host.BindingInventory{}, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: discovery Host %q is not codex", report.HostID())
	}
	maximum := options.MaximumEvidenceBytes
	if maximum <= 0 || maximum > defaultMaximumBindingEvidenceBytes {
		maximum = defaultMaximumBindingEvidenceBytes
	}
	codexRoot := options.CodexConfigRoot
	if codexRoot == "" && options.UserHome != "" {
		codexRoot = filepath.Join(options.UserHome, ".codex")
	}
	agents, err := loadAgentRegistry(codexRoot)
	if err != nil {
		return host.BindingInventory{}, err
	}
	descriptors := make(map[string]catalog.ProviderDescriptorRecord)
	for _, descriptor := range value.Providers() {
		descriptors[descriptor.ID] = descriptor
	}
	observations := make(map[string]host.BindingObservation)
	for providerID, descriptor := range descriptors {
		for _, candidate := range report.Candidates(providerID) {
			if candidate.HostID != "codex" {
				return host.BindingInventory{}, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: foreign Candidate in Codex report")
			}
			for _, capability := range descriptor.Capabilities {
				for _, binding := range capability.HostBindings {
					if binding.Host != "codex" {
						continue
					}
					observation, found, observeErr := observeBinding(candidate, binding, agents, maximum)
					if observeErr != nil {
						return host.BindingInventory{}, observeErr
					}
					if found {
						observations[observationKey(observation)] = observation
					}
				}
			}
		}
	}
	values := make([]host.BindingObservation, 0, len(observations))
	for _, observation := range observations {
		values = append(values, observation)
	}
	return host.NewBindingInventory("codex", values)
}

func observeBinding(candidate discovery.Candidate, binding catalog.HostBinding, agents map[string]string, maximum int64) (host.BindingObservation, bool, error) {
	switch binding.Kind {
	case "skill":
		return observeSkill(candidate, binding, maximum)
	case "agent":
		return observeAgent(candidate, binding, agents, maximum)
	case "tool":
		return host.BindingObservation{}, false, nil
	default:
		return host.BindingObservation{}, false, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: unsupported Binding kind %q", binding.Kind)
	}
}

func observeSkill(candidate discovery.Candidate, binding catalog.HostBinding, maximum int64) (host.BindingObservation, bool, error) {
	referenceName := binding.Reference
	relative := ""
	if prefix, suffix, found := strings.Cut(binding.Reference, ":"); found {
		if prefix != pluginNamespace(candidate) {
			return host.BindingObservation{}, false, nil
		}
		referenceName = suffix
		relative = filepath.ToSlash(filepath.Join("skills", suffix, "SKILL.md"))
	} else {
		relative = filepath.ToSlash(filepath.Join(referenceName, "SKILL.md"))
	}
	data, physical, found, err := readContainedBindingFile(candidate.Location, relative, maximum)
	if err != nil || !found {
		return host.BindingObservation{}, false, err
	}
	name, err := parseSkillName(data)
	if err != nil {
		return host.BindingObservation{}, false, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: %w", err)
	}
	if name != referenceName {
		return host.BindingObservation{}, false, nil
	}
	return host.BindingObservation{
		HostID: "codex", InstallationKey: candidate.InstallationKey, Binding: binding,
		Source: "host-filesystem", EvidenceReference: physical, Digest: canonicaljson.DigestBytes(data),
	}, true, nil
}

func observeAgent(candidate discovery.Candidate, binding catalog.HostBinding, agents map[string]string, maximum int64) (host.BindingObservation, bool, error) {
	configured, found := agents[binding.Reference]
	if !found {
		return host.BindingObservation{}, false, nil
	}
	physical, err := physicalContainedFile(candidate.Location, configured)
	if err != nil {
		return host.BindingObservation{}, false, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: %w", err)
	}
	data, err := readBoundedRegularFile(physical, maximum)
	if err != nil {
		return host.BindingObservation{}, false, err
	}
	return host.BindingObservation{
		HostID: "codex", InstallationKey: candidate.InstallationKey, Binding: binding,
		Source: "host-index", EvidenceReference: physical, Digest: canonicaljson.DigestBytes(data),
	}, true, nil
}

func pluginNamespace(candidate discovery.Candidate) string {
	if strings.Contains(candidate.SurfaceID, "plugin-cache") {
		return filepath.Base(filepath.Dir(candidate.Location))
	}
	if strings.Contains(candidate.SurfaceID, "plugin") {
		return filepath.Base(candidate.Location)
	}
	return ""
}

type codexAgentEntry struct {
	Description string `toml:"description"`
	ConfigFile  string `toml:"config_file"`
}

func loadAgentRegistry(codexRoot string) (map[string]string, error) {
	result := map[string]string{}
	if codexRoot == "" {
		return result, nil
	}
	configPath := filepath.Join(codexRoot, "config.toml")
	var document struct {
		Agents map[string]toml.Primitive `toml:"agents"`
	}
	metadata, err := toml.DecodeFile(configPath, &document)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("HOST_BINDING_REGISTRY_INVALID: %w", err)
	}
	for reference, primitive := range document.Agents {
		if isCodexAgentPoolOption(reference) {
			var value int
			if err := metadata.PrimitiveDecode(primitive, &value); err != nil || value < 1 {
				return nil, fmt.Errorf("HOST_BINDING_REGISTRY_INVALID: agent option %s must be a positive integer", reference)
			}
			continue
		}
		if _, err := catalog.ParseLocalID(reference); err != nil {
			return nil, fmt.Errorf("HOST_BINDING_REGISTRY_INVALID: invalid agent reference %q", reference)
		}
		var entry codexAgentEntry
		if err := metadata.PrimitiveDecode(primitive, &entry); err != nil {
			return nil, fmt.Errorf("HOST_BINDING_REGISTRY_INVALID: agent %s: %w", reference, err)
		}
		if entry.ConfigFile == "" || hasUnsafeBindingControl(entry.ConfigFile) {
			return nil, fmt.Errorf("HOST_BINDING_REGISTRY_INVALID: agent %s has invalid config_file", reference)
		}
		configured := entry.ConfigFile
		if !filepath.IsAbs(configured) {
			configured = filepath.Join(filepath.Dir(configPath), configured)
		}
		result[reference] = filepath.Clean(configured)
	}
	for _, key := range metadata.Undecoded() {
		parts := key.String()
		if strings.HasPrefix(parts, "agents.") && !isCodexAgentPoolOption(strings.TrimPrefix(parts, "agents.")) {
			return nil, fmt.Errorf("HOST_BINDING_REGISTRY_INVALID: unknown agent field %s", parts)
		}
	}
	return result, nil
}

func isCodexAgentPoolOption(value string) bool {
	return value == "max_threads" || value == "max_depth"
}

func parseSkillName(data []byte) (string, error) {
	if !utf8.Valid(data) || hasUnsafeBindingContent(string(data)) {
		return "", errors.New("SKILL.md is not valid bounded UTF-8 text")
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", errors.New("SKILL.md frontmatter is missing")
	}
	name := ""
	closed := false
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			closed = true
			break
		}
		key, value, found := strings.Cut(trimmed, ":")
		if !found || strings.TrimSpace(key) != "name" {
			continue
		}
		if name != "" {
			return "", errors.New("SKILL.md frontmatter has duplicate name")
		}
		name = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	if !closed || name == "" {
		return "", errors.New("SKILL.md frontmatter name is missing")
	}
	if _, err := catalog.ParseLocalID(name); err != nil {
		return "", fmt.Errorf("invalid SKILL.md name: %w", err)
	}
	return name, nil
}

func readContainedBindingFile(root, relative string, maximum int64) ([]byte, string, bool, error) {
	physical, found, err := resolveContainedBindingPath(root, relative)
	if err != nil || !found {
		return nil, "", found, err
	}
	data, err := readBoundedRegularFile(physical, maximum)
	return data, physical, true, err
}

func resolveContainedBindingPath(root, relative string) (string, bool, error) {
	if relative == "" || filepath.IsAbs(relative) || filepath.Clean(relative) != relative || hasUnsafeBindingControl(relative) {
		return "", false, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: unsafe relative path")
	}
	current := root
	for _, segment := range strings.Split(filepath.FromSlash(relative), string(filepath.Separator)) {
		if segment == "" || segment == "." || segment == ".." {
			return "", false, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: unsafe relative path")
		}
		candidate := filepath.Join(current, segment)
		info, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			candidate, err = filepath.EvalSymlinks(candidate)
			if err != nil || !bindingPathContained(root, candidate) {
				return "", false, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: symlink escapes Candidate")
			}
		}
		current = candidate
	}
	if !bindingPathContained(root, current) {
		return "", false, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: path escapes Candidate")
	}
	return filepath.Clean(current), true, nil
}

func physicalContainedFile(root, value string) (string, error) {
	physical, err := filepath.EvalSymlinks(value)
	if err != nil {
		return "", err
	}
	physical = filepath.Clean(physical)
	if !bindingPathContained(root, physical) {
		return "", errors.New("agent config is outside Candidate")
	}
	return physical, nil
}

func readBoundedRegularFile(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: evidence is not a regular file")
	}
	if info.Size() > maximum {
		return nil, fmt.Errorf("HOST_BINDING_EVIDENCE_TOO_LARGE: %s", path)
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: %w", err)
	}
	if int64(len(data)) > maximum {
		return nil, fmt.Errorf("HOST_BINDING_EVIDENCE_TOO_LARGE: %s", path)
	}
	if !utf8.Valid(data) || hasUnsafeBindingContent(string(data)) {
		return nil, fmt.Errorf("HOST_BINDING_EVIDENCE_INVALID: evidence is not bounded UTF-8 text")
	}
	return data, nil
}

func bindingPathContained(root, candidate string) bool {
	relation, err := filepath.Rel(root, candidate)
	return err == nil && relation != ".." && !strings.HasPrefix(relation, ".."+string(filepath.Separator)) && !filepath.IsAbs(relation)
}

func observationKey(value host.BindingObservation) string {
	return value.HostID + "\x00" + value.InstallationKey + "\x00" + value.Binding.Kind + "\x00" + value.Binding.Reference
}

func hasUnsafeBindingControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func hasUnsafeBindingContent(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return true
		}
	}
	return false
}

func sortedKeys[T any](values map[string]T) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
