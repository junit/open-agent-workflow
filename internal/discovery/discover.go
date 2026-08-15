package discovery

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

const defaultMaximumEvidenceBytes int64 = 4 << 20

type Options struct {
	HostID               string
	UserHome             string
	MaximumEvidenceBytes int64
}

type evidenceSeed struct {
	probeID string
	kind    string
}

type candidateAccumulator struct {
	providerID   string
	hostID       string
	surface      string
	distribution string
	location     string
	version      string
	evidence     []evidenceSeed
}

func Discover(value catalog.Catalog, options Options) (Report, error) {
	if _, err := catalog.ParseLocalID(options.HostID); err != nil {
		return Report{}, fmt.Errorf("DISCOVERY_HOST_INVALID: %w", err)
	}
	root, err := physicalUserHome(options.UserHome)
	if err != nil {
		return Report{}, err
	}
	maximum := boundedMaximum(options.MaximumEvidenceBytes)
	accumulators := make(map[string]*candidateAccumulator)
	providers := value.Providers()
	for _, provider := range providers {
		for _, probe := range provider.Discovery {
			if !contains(probe.Hosts, options.HostID) {
				continue
			}
			if probe.Root != "user-home" {
				return Report{}, fmt.Errorf("DISCOVERY_PROBE_UNSUPPORTED: %s/%s root %q", provider.ID, probe.ID, probe.Root)
			}
			switch probe.Kind {
			case "path-exists":
				if err := discoverDirect(accumulators, root, maximum, provider.ID, options.HostID, probe); err != nil {
					return Report{}, err
				}
			case "one-level-version-path-exists":
				if err := discoverVersions(accumulators, root, maximum, provider.ID, options.HostID, probe); err != nil {
					return Report{}, err
				}
			default:
				return Report{}, fmt.Errorf("DISCOVERY_PROBE_UNSUPPORTED: %s/%s kind %q", provider.ID, probe.ID, probe.Kind)
			}
		}
	}
	providerIndex := make(map[string]catalog.ProviderDescriptorRecord, len(providers))
	for _, provider := range providers {
		providerIndex[provider.ID] = provider
	}
	keys := make([]string, 0, len(accumulators))
	for key := range accumulators {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	candidates := make([]Candidate, 0, len(keys))
	for _, key := range keys {
		candidate, buildErr := buildCandidate(accumulators[key], providerIndex)
		if buildErr != nil {
			return Report{}, buildErr
		}
		candidates = append(candidates, candidate)
	}
	return newReport(options.HostID, candidates)
}

func discoverDirect(accumulators map[string]*candidateAccumulator, root string, maximum int64, providerID, hostID string, probe catalog.DiscoveryProbe) error {
	location, found, err := resolveContained(root, probe.CandidatePath)
	if err != nil || !found {
		return err
	}
	if err := requireDirectory(location, providerID, probe.ID); err != nil {
		return err
	}
	if _, _, found, err := readEvidence(location, probe.EvidencePath, maximum); err != nil || !found {
		return err
	}
	accumulator := ensureAccumulator(accumulators, providerID, hostID, probe.Surface, probe.DistributionID, location, "")
	accumulator.evidence = append(accumulator.evidence, evidenceSeed{probeID: probe.ID, kind: probe.Kind})
	return nil
}

func discoverVersions(accumulators map[string]*candidateAccumulator, root string, maximum int64, providerID, hostID string, probe catalog.DiscoveryProbe) error {
	prefix, found, err := resolveContained(root, probe.Prefix)
	if err != nil || !found {
		return err
	}
	if err := requireDirectory(prefix, providerID, probe.ID); err != nil {
		return err
	}
	entries, err := os.ReadDir(prefix)
	if err != nil {
		return fmt.Errorf("DISCOVERY_ENUMERATION_FAILED: %s/%s: %w", providerID, probe.ID, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		versionDirectory, err := resolveImmediateChild(root, prefix, entry.Name())
		if err != nil {
			return err
		}
		if versionDirectory == "" {
			continue
		}
		if !utf8.ValidString(entry.Name()) || hasControl(entry.Name()) {
			return fmt.Errorf("DISCOVERY_VERSION_INVALID: %s/%s", providerID, probe.ID)
		}
		_, _, found, err := readEvidence(versionDirectory, probe.EvidencePath, maximum)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		accumulator := ensureAccumulator(accumulators, providerID, hostID, probe.Surface, probe.DistributionID, versionDirectory, entry.Name())
		accumulator.evidence = append(accumulator.evidence, evidenceSeed{probeID: probe.ID, kind: probe.Kind})
	}
	return nil
}

func ensureAccumulator(values map[string]*candidateAccumulator, providerID, hostID, surface, distribution, location, version string) *candidateAccumulator {
	mapKey := strings.Join([]string{providerID, hostID, surface, distribution, location, version}, "\x00")
	if value, found := values[mapKey]; found {
		return value
	}
	value := &candidateAccumulator{providerID: providerID, hostID: hostID, surface: surface, distribution: distribution, location: location, version: version, evidence: []evidenceSeed{}}
	values[mapKey] = value
	return value
}

func buildCandidate(value *candidateAccumulator, providers map[string]catalog.ProviderDescriptorRecord) (Candidate, error) {
	provider, exists := providers[value.providerID]
	if !exists {
		return Candidate{}, errors.New("PROVIDER_PROVENANCE_MISMATCH: Provider descriptor is unavailable")
	}
	distribution, exists := findDistribution(provider.Distributions, value.distribution)
	if !exists {
		return Candidate{}, errors.New("PROVIDER_PROVENANCE_MISMATCH: Distribution is unavailable")
	}
	installationKey, err := deriveInstallationKey(value.providerID, value.hostID, value.surface, value.distribution, value.location)
	if err != nil {
		return Candidate{}, err
	}
	attestation := attestCandidate(provider, distribution, value.hostID, value.surface, value.location, value.version)
	evidence := make([]Evidence, len(value.evidence))
	for index, seed := range value.evidence {
		evidence[index] = Evidence{ProviderID: value.providerID, HostID: value.hostID, Surface: value.surface, DistributionID: value.distribution, ObservedRevision: attestation.observedRevision, InstallationKey: installationKey, ProbeID: seed.probeID, Kind: seed.kind, BindingRoots: attestation.bindingRoots}
	}
	normalizedEvidence := normalizeEvidence(evidence)
	evidenceDigest, err := digestEvidence(normalizedEvidence)
	if err != nil {
		return Candidate{}, err
	}
	for index := range normalizedEvidence {
		normalizedEvidence[index].InstallationKey = installationKey
	}
	return Candidate{ProviderID: value.providerID, HostID: value.hostID, Surface: value.surface, DistributionID: value.distribution, InstallationKey: installationKey, DiagnosticLocation: value.location, ObservedRevision: attestation.observedRevision, DistributionTreeDigest: attestation.distributionTreeDigest, Provenance: attestation.provenance, BindingRoots: normalizeBindingRoots(attestation.bindingRoots), EvidenceDigest: evidenceDigest, Evidence: normalizedEvidence}, nil
}

func findDistribution(values []catalog.DistributionRecord, id string) (catalog.DistributionRecord, bool) {
	for _, value := range values {
		if value.ID == id {
			return value, true
		}
	}
	return catalog.DistributionRecord{}, false
}

func deriveInstallationKey(providerID, hostID, surface, distributionID, location string) (string, error) {
	digest, _, err := canonicaljson.Digest(struct {
		ProviderID     string `json:"provider_id"`
		HostID         string `json:"host_id"`
		Surface        string `json:"surface"`
		DistributionID string `json:"distribution_id"`
		Location       string `json:"location"`
	}{providerID, hostID, surface, distributionID, location})
	if err != nil {
		return "", err
	}
	return "installation-" + digest, nil
}

func boundedMaximum(value int64) int64 {
	if value <= 0 || value > defaultMaximumEvidenceBytes {
		return defaultMaximumEvidenceBytes
	}
	return value
}

func physicalUserHome(value string) (string, error) {
	physical, err := physicalDirectory(value)
	if err != nil {
		return "", fmt.Errorf("DISCOVERY_ROOT_INVALID: %w", err)
	}
	return physical, nil
}

func physicalDirectory(value string) (string, error) {
	if value == "" {
		return "", errors.New("empty directory")
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(physical)
	if err != nil || !info.IsDir() {
		return "", errors.New("path is not a directory")
	}
	return filepath.Clean(physical), nil
}

func requireDirectory(value, providerID, probeID string) error {
	info, err := os.Stat(value)
	if err != nil {
		return fmt.Errorf("DISCOVERY_ENUMERATION_FAILED: %s/%s: %w", providerID, probeID, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("DISCOVERY_CANDIDATE_NOT_DIRECTORY: %s/%s", providerID, probeID)
	}
	return nil
}

func readEvidence(root, relative string, maximum int64) ([]byte, string, bool, error) {
	physical, found, err := resolveContained(root, relative)
	if err != nil || !found {
		return nil, "", found, err
	}
	file, err := os.Open(physical)
	if err != nil {
		return nil, "", false, fmt.Errorf("DISCOVERY_EVIDENCE_READ_FAILED: %s: %w", relative, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, "", false, fmt.Errorf("DISCOVERY_EVIDENCE_READ_FAILED: %s: %w", relative, err)
	}
	if !info.Mode().IsRegular() {
		return nil, "", false, fmt.Errorf("DISCOVERY_EVIDENCE_NOT_REGULAR: %s", relative)
	}
	if info.Size() > maximum {
		return nil, "", false, fmt.Errorf("DISCOVERY_EVIDENCE_TOO_LARGE: %s", relative)
	}
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, "", false, fmt.Errorf("DISCOVERY_EVIDENCE_READ_FAILED: %s: %w", relative, err)
	}
	if int64(len(data)) > maximum {
		return nil, "", false, fmt.Errorf("DISCOVERY_EVIDENCE_TOO_LARGE: %s", relative)
	}
	if !utf8.Valid(data) || hasUnsafeContentControl(string(data)) {
		return nil, "", false, fmt.Errorf("DISCOVERY_EVIDENCE_INVALID: %s", relative)
	}
	return data, physical, true, nil
}

func resolveContained(root, relative string) (string, bool, error) {
	if err := validateProbePath(relative); err != nil {
		return "", false, err
	}
	current := root
	segments := strings.Split(filepath.FromSlash(relative), string(filepath.Separator))
	for _, segment := range segments {
		candidate := filepath.Join(current, segment)
		info, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("DISCOVERY_EVIDENCE_READ_FAILED: %s: %w", relative, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			candidate, err = filepath.EvalSymlinks(candidate)
			if err != nil {
				return "", false, fmt.Errorf("DISCOVERY_EVIDENCE_READ_FAILED: %s: %w", relative, err)
			}
			candidate = filepath.Clean(candidate)
			if !pathContained(root, candidate) {
				return "", false, fmt.Errorf("DISCOVERY_PATH_ESCAPE: %s", relative)
			}
		}
		current = candidate
	}
	if !pathContained(root, current) {
		return "", false, fmt.Errorf("DISCOVERY_PATH_ESCAPE: %s", relative)
	}
	return filepath.Clean(current), true, nil
}

func resolveImmediateChild(root, prefix, name string) (string, error) {
	candidate := filepath.Join(prefix, name)
	info, err := os.Lstat(candidate)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("DISCOVERY_ENUMERATION_FAILED: %s: %w", candidate, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		candidate, err = filepath.EvalSymlinks(candidate)
		if err != nil {
			return "", fmt.Errorf("DISCOVERY_ENUMERATION_FAILED: %s: %w", candidate, err)
		}
		if !pathContained(root, candidate) {
			return "", fmt.Errorf("DISCOVERY_PATH_ESCAPE: %s", candidate)
		}
		info, err = os.Stat(candidate)
		if err != nil {
			return "", fmt.Errorf("DISCOVERY_ENUMERATION_FAILED: %s: %w", candidate, err)
		}
	}
	if !info.IsDir() {
		return "", nil
	}
	if !pathContained(root, candidate) {
		return "", fmt.Errorf("DISCOVERY_PATH_ESCAPE: %s", candidate)
	}
	return filepath.Clean(candidate), nil
}

func validateProbePath(value string) error {
	if value == "" || path.IsAbs(value) || path.Clean(value) != value || strings.ContainsAny(value, `\*?[]{}():`) {
		return fmt.Errorf("DISCOVERY_PATH_INVALID: %q", value)
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("DISCOVERY_PATH_INVALID: %q", value)
		}
	}
	if hasControl(value) {
		return fmt.Errorf("DISCOVERY_PATH_INVALID: %q", value)
	}
	return nil
}

func pathContained(root, candidate string) bool {
	relation, err := filepath.Rel(root, candidate)
	return err == nil && isContainedRelation(relation)
}

func isContainedRelation(value string) bool {
	return value != ".." && !strings.HasPrefix(value, ".."+string(filepath.Separator)) && !filepath.IsAbs(value)
}

func hasControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func hasUnsafeContentControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return true
		}
	}
	return false
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
