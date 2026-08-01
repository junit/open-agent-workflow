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
	UserHome             string
	MaximumEvidenceBytes int64
}

type candidateAccumulator struct {
	providerID string
	key        string
	location   string
	version    string
	direct     bool
	evidence   []Evidence
}

func Discover(value catalog.Catalog, options Options) (Report, error) {
	root, err := physicalUserHome(options.UserHome)
	if err != nil {
		return Report{}, err
	}
	maximum := options.MaximumEvidenceBytes
	if maximum <= 0 || maximum > defaultMaximumEvidenceBytes {
		maximum = defaultMaximumEvidenceBytes
	}
	accumulators := make(map[string]*candidateAccumulator)
	for _, provider := range value.Providers() {
		for _, probe := range provider.Discovery {
			if probe.Root != "user-home" {
				return Report{}, fmt.Errorf("DISCOVERY_PROBE_UNSUPPORTED: %s/%s root %q", provider.ID, probe.ID, probe.Root)
			}
			switch probe.Kind {
			case "path-exists":
				if err := discoverDirect(accumulators, root, maximum, provider.ID, probe); err != nil {
					return Report{}, err
				}
			case "one-level-version-path-exists":
				if err := discoverVersions(accumulators, root, maximum, provider.ID, probe); err != nil {
					return Report{}, err
				}
			default:
				return Report{}, fmt.Errorf("DISCOVERY_PROBE_UNSUPPORTED: %s/%s kind %q", provider.ID, probe.ID, probe.Kind)
			}
		}
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
	return newReport(candidates)
}

func discoverDirect(accumulators map[string]*candidateAccumulator, root string, maximum int64, providerID string, probe catalog.DiscoveryProbe) error {
	data, physical, found, err := readEvidence(root, probe.Path, maximum)
	if err != nil || !found {
		return err
	}
	key := "direct:" + root
	accumulator := ensureAccumulator(accumulators, providerID, key, root, "", true)
	accumulator.evidence = append(accumulator.evidence, Evidence{
		ProviderID: providerID, CandidateKey: key, ProbeID: probe.ID, Kind: probe.Kind,
		Path: physical, Version: "", ContentDigest: canonicaljson.DigestBytes(data),
	})
	return nil
}

func discoverVersions(accumulators map[string]*candidateAccumulator, root string, maximum int64, providerID string, probe catalog.DiscoveryProbe) error {
	prefix, found, err := resolveContained(root, probe.Prefix)
	if err != nil || !found {
		return err
	}
	info, err := os.Stat(prefix)
	if err != nil {
		return fmt.Errorf("DISCOVERY_ENUMERATION_FAILED: %s/%s: %w", providerID, probe.ID, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("DISCOVERY_VERSION_PREFIX_NOT_DIRECTORY: %s/%s", providerID, probe.ID)
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
		relativeDirectory, err := filepath.Rel(root, versionDirectory)
		if err != nil || !isContainedRelation(relativeDirectory) {
			return fmt.Errorf("DISCOVERY_PATH_ESCAPE: %s/%s", providerID, probe.ID)
		}
		relativeEvidence := filepath.ToSlash(filepath.Join(relativeDirectory, filepath.FromSlash(probe.Suffix)))
		data, physical, found, err := readEvidence(root, relativeEvidence, maximum)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		key := "version:" + versionDirectory
		accumulator := ensureAccumulator(accumulators, providerID, key, versionDirectory, entry.Name(), false)
		if accumulator.version != entry.Name() {
			return fmt.Errorf("DISCOVERY_VERSION_CONFLICT: %s/%s", providerID, versionDirectory)
		}
		accumulator.evidence = append(accumulator.evidence, Evidence{
			ProviderID: providerID, CandidateKey: key, ProbeID: probe.ID, Kind: probe.Kind,
			Path: physical, Version: entry.Name(), ContentDigest: canonicaljson.DigestBytes(data),
		})
	}
	return nil
}

func ensureAccumulator(values map[string]*candidateAccumulator, providerID, key, location, version string, direct bool) *candidateAccumulator {
	mapKey := providerID + "\x00" + key
	if value, found := values[mapKey]; found {
		return value
	}
	value := &candidateAccumulator{
		providerID: providerID, key: key, location: location, version: version, direct: direct, evidence: []Evidence{},
	}
	values[mapKey] = value
	return value
}

func buildCandidate(value *candidateAccumulator) (Candidate, error) {
	evidence, digest, err := prepareEvidence(value.evidence)
	if err != nil {
		return Candidate{}, err
	}
	version := value.version
	if value.direct {
		version = "content-" + digest
	}
	return Candidate{
		ProviderID: value.providerID, Key: value.key, Location: value.location, Version: version,
		EvidenceDigest: digest, Evidence: evidence,
	}, nil
}

func physicalUserHome(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("DISCOVERY_ROOT_INVALID: empty user home")
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("DISCOVERY_ROOT_INVALID: %w", err)
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("DISCOVERY_ROOT_INVALID: %w", err)
	}
	info, err := os.Stat(physical)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("DISCOVERY_ROOT_INVALID: user home is not a directory")
	}
	return filepath.Clean(physical), nil
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
