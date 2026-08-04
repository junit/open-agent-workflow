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
	Installations        []InstallationHint
}

type InstallationHint struct {
	ProviderID       string
	HostID           string
	SurfaceID        string
	Location         string
	DiscoveryProbeID string
}

type candidateAccumulator struct {
	providerID   string
	hostID       string
	surfaceID    string
	distribution string
	location     string
	version      string
	direct       bool
	evidence     []Evidence
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
				if err := discoverDirect(accumulators, root, maximum, provider.ID, options.HostID, probe, ""); err != nil {
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
	if err := discoverInstallations(accumulators, providers, options.HostID, maximum, options.Installations); err != nil {
		return Report{}, err
	}

	keys := make([]string, 0, len(accumulators))
	for key := range accumulators {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	candidates := make([]Candidate, 0, len(keys))
	for _, key := range keys {
		candidate, buildErr := buildCandidate(accumulators[key])
		if buildErr != nil {
			return Report{}, buildErr
		}
		candidates = append(candidates, candidate)
	}
	return newReport(options.HostID, candidates)
}

func discoverDirect(accumulators map[string]*candidateAccumulator, root string, maximum int64, providerID, hostID string, probe catalog.DiscoveryProbe, explicitLocation string) error {
	location := explicitLocation
	if location == "" {
		candidate, found, err := resolveContained(root, probe.CandidatePath)
		if err != nil || !found {
			return err
		}
		location = candidate
	}
	if err := requireDirectory(location, providerID, probe.ID); err != nil {
		return err
	}
	data, physical, found, err := readEvidence(location, probe.EvidencePath, maximum)
	if err != nil || !found {
		return err
	}
	accumulator := ensureAccumulator(accumulators, providerID, hostID, probe.Surface, probe.Distribution, location, "", true)
	accumulator.evidence = append(accumulator.evidence, Evidence{
		ProviderID: providerID, HostID: hostID, SurfaceID: probe.Surface,
		ProbeID: probe.ID, Kind: probe.Kind, Path: physical, ContentDigest: canonicaljson.DigestBytes(data),
	})
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
		data, physical, found, err := readEvidence(versionDirectory, probe.EvidencePath, maximum)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		accumulator := ensureAccumulator(accumulators, providerID, hostID, probe.Surface, probe.Distribution, versionDirectory, entry.Name(), false)
		accumulator.evidence = append(accumulator.evidence, Evidence{
			ProviderID: providerID, HostID: hostID, SurfaceID: probe.Surface,
			ProbeID: probe.ID, Kind: probe.Kind, Path: physical, Version: entry.Name(), ContentDigest: canonicaljson.DigestBytes(data),
		})
	}
	return nil
}

func discoverInstallations(accumulators map[string]*candidateAccumulator, providers []catalog.ProviderDescriptorRecord, hostID string, maximum int64, hints []InstallationHint) error {
	providerIndex := make(map[string]catalog.ProviderDescriptorRecord, len(providers))
	for _, provider := range providers {
		providerIndex[provider.ID] = provider
	}
	for _, hint := range hints {
		if hint.HostID != hostID {
			continue
		}
		provider, found := providerIndex[hint.ProviderID]
		if !found {
			return fmt.Errorf("DISCOVERY_INSTALLATION_INVALID: provider %q is not registered", hint.ProviderID)
		}
		probe, found := findProbe(provider.Discovery, hint.DiscoveryProbeID)
		if !found || !contains(probe.Hosts, hostID) || probe.Surface != hint.SurfaceID {
			return fmt.Errorf("DISCOVERY_INSTALLATION_INVALID: probe %q does not match host installation", hint.DiscoveryProbeID)
		}
		location, err := physicalInstallation(hint.Location)
		if err != nil {
			return err
		}
		if probe.Kind == "path-exists" {
			if err := discoverDirect(accumulators, "", maximum, provider.ID, hostID, probe, location); err != nil {
				return err
			}
			continue
		}
		if probe.Kind != "one-level-version-path-exists" {
			return fmt.Errorf("DISCOVERY_PROBE_UNSUPPORTED: %s/%s kind %q", provider.ID, probe.ID, probe.Kind)
		}
		data, physical, found, err := readEvidence(location, probe.EvidencePath, maximum)
		if err != nil || !found {
			return err
		}
		version := filepath.Base(location)
		accumulator := ensureAccumulator(accumulators, provider.ID, hostID, probe.Surface, probe.Distribution, location, version, false)
		accumulator.evidence = append(accumulator.evidence, Evidence{
			ProviderID: provider.ID, HostID: hostID, SurfaceID: probe.Surface,
			ProbeID: probe.ID, Kind: probe.Kind, Path: physical, Version: version, ContentDigest: canonicaljson.DigestBytes(data),
		})
	}
	return nil
}

func findProbe(probes []catalog.DiscoveryProbe, id string) (catalog.DiscoveryProbe, bool) {
	for _, probe := range probes {
		if probe.ID == id {
			return probe, true
		}
	}
	return catalog.DiscoveryProbe{}, false
}

func ensureAccumulator(values map[string]*candidateAccumulator, providerID, hostID, surfaceID, distribution, location, version string, direct bool) *candidateAccumulator {
	mapKey := strings.Join([]string{providerID, hostID, surfaceID, distribution, location, version}, "\x00")
	if value, found := values[mapKey]; found {
		return value
	}
	value := &candidateAccumulator{
		providerID: providerID, hostID: hostID, surfaceID: surfaceID, distribution: distribution,
		location: location, version: version, direct: direct, evidence: []Evidence{},
	}
	values[mapKey] = value
	return value
}

func buildCandidate(value *candidateAccumulator) (Candidate, error) {
	evidence := normalizeEvidence(value.evidence)
	evidenceDigest, err := digestEvidence(evidence)
	if err != nil {
		return Candidate{}, err
	}
	version := value.version
	if value.direct {
		version = "content-" + evidenceDigest
	}
	distributionKey, err := deriveDistributionKey(value.providerID, value.distribution, value.location, version, evidenceDigest)
	if err != nil {
		return Candidate{}, err
	}
	installationKey, err := deriveInstallationKey(value.hostID, value.surfaceID, distributionKey)
	if err != nil {
		return Candidate{}, err
	}
	for i := range evidence {
		evidence[i].DistributionKey = distributionKey
		evidence[i].InstallationKey = installationKey
	}
	return Candidate{
		ProviderID: value.providerID, HostID: value.hostID, SurfaceID: value.surfaceID,
		DistributionKey: distributionKey, InstallationKey: installationKey,
		Location: value.location, Version: version, EvidenceDigest: evidenceDigest, Evidence: evidence,
	}, nil
}

func deriveDistributionKey(providerID, distribution, location, version, evidenceDigest string) (string, error) {
	digest, _, err := canonicaljson.Digest(struct {
		ProviderID     string `json:"provider_id"`
		Distribution   string `json:"distribution"`
		Location       string `json:"location"`
		Version        string `json:"version"`
		EvidenceDigest string `json:"evidence_digest"`
	}{providerID, distribution, location, version, evidenceDigest})
	if err != nil {
		return "", err
	}
	return "distribution-" + digest, nil
}

func deriveInstallationKey(hostID, surfaceID, distributionKey string) (string, error) {
	digest, _, err := canonicaljson.Digest(struct {
		HostID          string `json:"host_id"`
		SurfaceID       string `json:"surface_id"`
		DistributionKey string `json:"distribution_key"`
	}{hostID, surfaceID, distributionKey})
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

func physicalInstallation(value string) (string, error) {
	if value == "" || !filepath.IsAbs(value) || filepath.Clean(value) != value || hasControl(value) {
		return "", fmt.Errorf("DISCOVERY_INSTALLATION_INVALID: unsafe location")
	}
	physical, err := physicalDirectory(value)
	if err != nil {
		return "", fmt.Errorf("DISCOVERY_INSTALLATION_INVALID: %w", err)
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
	if value == "" || path.IsAbs(value) || path.Clean(value) != value || strings.ContainsAny(value, `\*?[]{}()`) {
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
