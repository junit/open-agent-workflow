package discovery

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"sort"

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
		tree, err := integrity.DigestBindingRoot(root, binding.InstallRoot, binding.ContentRoot)
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
		tree, err := integrity.DigestDistributionTree(root)
		if err == nil && tree.RootDigest == distribution.TreeDigest {
			return attestationResult{provenance: ProvenanceDistributionAttested, observedRevision: distribution.Revision, distributionTreeDigest: tree.RootDigest, bindingRoots: roots}
		}
		return attestationResult{}
	}
	return attestationResult{provenance: ProvenanceContentEquivalent, bindingRoots: roots}
}

func readImmutableSourceManifest(root string) (immutableSourceRecord, bool, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return immutableSourceRecord{}, true, errors.New("invalid immutable-source manifest root")
	}
	rooted, err := os.OpenRoot(root)
	if err != nil {
		return immutableSourceRecord{}, true, err
	}
	defer rooted.Close()
	openedRoot, err := rooted.Stat(".")
	if err != nil || !os.SameFile(rootInfo, openedRoot) {
		return immutableSourceRecord{}, true, errors.New("immutable-source manifest root changed while opening")
	}

	info, err := rooted.Lstat(immutableSourceManifest)
	if errors.Is(err, os.ErrNotExist) {
		return immutableSourceRecord{}, false, nil
	}
	if err != nil {
		return immutableSourceRecord{}, true, err
	}
	record, found, err := readRootedImmutableSourceManifest(rooted, info)
	currentRoot, rootErr := os.Lstat(root)
	if rootErr != nil || currentRoot.Mode()&os.ModeSymlink != 0 || !os.SameFile(rootInfo, currentRoot) {
		return immutableSourceRecord{}, true, errors.New("immutable-source manifest root changed while reading")
	}
	return record, found, err
}

func readRootedImmutableSourceManifest(rooted *os.Root, info fs.FileInfo) (immutableSourceRecord, bool, error) {
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return immutableSourceRecord{}, true, errors.New("invalid immutable-source manifest file")
	}
	file, err := rooted.Open(immutableSourceManifest)
	if err != nil {
		return immutableSourceRecord{}, true, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !sameImmutableManifestSnapshot(info, opened) || !opened.Mode().IsRegular() {
		return immutableSourceRecord{}, true, errors.New("immutable-source manifest changed while opening")
	}
	raw, err := readImmutableManifestBytes(file)
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
	after, statErr := file.Stat()
	current, inspectErr := rooted.Lstat(immutableSourceManifest)
	if statErr != nil || inspectErr != nil || after.Size() > 64<<10 ||
		!sameImmutableManifestSnapshot(opened, after) || !sameImmutableManifestSnapshot(info, current) ||
		current.Mode()&os.ModeSymlink != 0 || !immutableManifestContentUnchanged(file, raw) {
		return immutableSourceRecord{}, true, errors.New("immutable-source manifest changed while reading")
	}
	return record, true, nil
}

func readImmutableManifestBytes(file *os.File) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(file, 64<<10+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 64<<10 {
		return nil, errors.New("immutable-source manifest is too large")
	}
	return raw, nil
}

func immutableManifestContentUnchanged(file *os.File, expected []byte) bool {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return false
	}
	current, err := readImmutableManifestBytes(file)
	return err == nil && bytes.Equal(current, expected)
}

func sameImmutableManifestSnapshot(left, right fs.FileInfo) bool {
	return os.SameFile(left, right) && left.Size() == right.Size() && left.Mode() == right.Mode() && left.ModTime().Equal(right.ModTime())
}
