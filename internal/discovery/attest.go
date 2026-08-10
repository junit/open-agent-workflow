package discovery

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

const immutableSourceManifest = ".oaw-distribution.json"

type attestationResult struct {
	provenance             ProvenanceDisposition
	observedRevision       string
	distributionTreeDigest string
	bindingRoots           []BindingRootEvidence
}

type immutableSourceRecord struct {
	DistributionID string `json:"distribution_id"`
	Revision       string `json:"revision"`
	TreeDigest     string `json:"tree_digest"`
}

func attestCandidate(provider catalog.ProviderDescriptorRecord, distribution catalog.DistributionRecord, hostID, surface, root, version string) attestationResult {
	bindings := make([]catalog.BindingRecord, 0)
	for _, binding := range provider.Bindings {
		if binding.DistributionID == distribution.ID && binding.Host == hostID && binding.Surface == surface {
			bindings = append(bindings, binding)
		}
	}
	if len(bindings) == 0 {
		return attestationResult{}
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].ID < bindings[j].ID })
	roots := make([]BindingRootEvidence, 0, len(bindings))
	for _, binding := range bindings {
		path, found, err := resolvePhysicalBindingRoot(root, binding.InstallRoot)
		if err != nil || !found {
			return attestationResult{}
		}
		tree, err := integrity.DigestTree(path)
		if err != nil || tree.RootDigest != binding.TreeDigest {
			return attestationResult{}
		}
		roots = append(roots, BindingRootEvidence{BindingID: binding.ID, ContentRoot: binding.ContentRoot, InstallRoot: binding.InstallRoot, Tree: tree})
	}

	if record, found, err := readImmutableSourceManifest(root); found || err != nil {
		if err != nil || record.DistributionID != distribution.ID || record.Revision != distribution.Revision || record.TreeDigest != distribution.TreeDigest {
			return attestationResult{}
		}
		return attestationResult{provenance: ProvenanceDistributionAttested, observedRevision: record.Revision, distributionTreeDigest: record.TreeDigest, bindingRoots: roots}
	}
	if version != "" && version == distribution.Revision {
		tree, err := integrity.DigestTree(root)
		if err == nil && tree.RootDigest == distribution.TreeDigest {
			return attestationResult{provenance: ProvenanceDistributionAttested, observedRevision: distribution.Revision, distributionTreeDigest: tree.RootDigest, bindingRoots: roots}
		}
		return attestationResult{}
	}
	return attestationResult{provenance: ProvenanceContentEquivalent, bindingRoots: roots}
}

func resolvePhysicalBindingRoot(root, relative string) (string, bool, error) {
	if err := validateProbePath(relative); err != nil {
		return "", false, err
	}
	current := root
	for _, segment := range splitRelativePath(relative) {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", false, errors.New("Binding root contains a symlink")
		}
	}
	if !pathContained(root, current) {
		return "", false, errors.New("Binding root escapes installation")
	}
	return filepath.Clean(current), true, nil
}

func splitRelativePath(value string) []string {
	return strings.Split(filepath.FromSlash(value), string(filepath.Separator))
}

func readImmutableSourceManifest(root string) (immutableSourceRecord, bool, error) {
	path := filepath.Join(root, immutableSourceManifest)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return immutableSourceRecord{}, false, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return immutableSourceRecord{}, true, errors.New("invalid immutable-source manifest file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return immutableSourceRecord{}, true, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var record immutableSourceRecord
	if err := decoder.Decode(&record); err != nil {
		return immutableSourceRecord{}, true, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return immutableSourceRecord{}, true, errors.New("immutable-source manifest has trailing data")
	}
	if record.DistributionID == "" || record.Revision == "" || record.TreeDigest == "" {
		return immutableSourceRecord{}, true, errors.New("immutable-source manifest is incomplete")
	}
	return record, true, nil
}
