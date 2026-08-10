package provideraudit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

func Build(checkouts []Checkout) (Manifest, error) {
	if len(checkouts) != len(lockedProviderSpecs) {
		return Manifest{}, invalidAudit("exactly three Provider checkouts are required", nil)
	}
	byProvider := make(map[string]Checkout, len(checkouts))
	for _, checkout := range checkouts {
		if _, duplicate := byProvider[checkout.ProviderID]; duplicate {
			return Manifest{}, invalidAudit("duplicate Provider checkout", nil)
		}
		byProvider[checkout.ProviderID] = checkout
	}
	providers := make([]ProviderSource, len(lockedProviderSpecs))
	for index, spec := range lockedProviderSpecs {
		checkout, found := byProvider[spec.ID]
		if !found || checkout.SourceURI != spec.SourceURI || checkout.Revision != spec.Revision || checkout.DistributionRoot != "." || checkout.Root == "" || len(checkout.BindingRoots) != len(spec.Bindings) {
			return Manifest{}, invalidAudit("Provider checkout pin mismatch", nil)
		}
		rootInfo, err := os.Stat(checkout.Root)
		if err != nil || !rootInfo.IsDir() {
			return Manifest{}, invalidAudit("Provider checkout root is unavailable", err)
		}
		distributionDigest, err := digestDistributionRoot(filepath.Join(checkout.Root, filepath.FromSlash(checkout.DistributionRoot)))
		if err != nil {
			return Manifest{}, invalidAudit("digest Distribution tree", err)
		}
		byBinding := make(map[string]BindingCheckout, len(checkout.BindingRoots))
		for _, binding := range checkout.BindingRoots {
			if _, duplicate := byBinding[binding.ID]; duplicate {
				return Manifest{}, invalidAudit("duplicate Binding checkout", nil)
			}
			byBinding[binding.ID] = binding
		}
		bindings := make([]BindingSource, len(spec.Bindings))
		for bindingIndex, expected := range spec.Bindings {
			binding, found := byBinding[expected.ID]
			if !found || binding.ContentRoot != expected.ContentRoot || binding.InstallRoot != expected.InstallRoot || binding.Root != expected.ContentRoot || !cleanRelative(binding.Root, false) {
				return Manifest{}, invalidAudit("Binding checkout mapping mismatch", nil)
			}
			digest, err := digestBindingRoot(checkout.Root, binding.Root)
			if err != nil {
				return Manifest{}, invalidAudit("digest Binding root", err)
			}
			bindings[bindingIndex] = BindingSource{ID: expected.ID, ContentRoot: expected.ContentRoot, InstallRoot: expected.InstallRoot, TreeDigest: digest, Kind: expected.Kind, References: append([]string{}, expected.References...)}
		}
		providers[index] = ProviderSource{ProviderID: spec.ID, SourceURI: spec.SourceURI, Revision: spec.Revision, DistributionID: spec.DistributionID, DistributionRoot: ".", DistributionTreeDigest: distributionDigest, Bindings: bindings, EvidenceRoots: append([]string{}, spec.EvidenceRoots...)}
	}
	manifest := Manifest{SchemaVersion: ProviderSourceAuditSchemaV1, CanonicalMatrixDigest: CanonicalMatrixDigest, Providers: providers}
	manifest.Digest = manifest.ContentDigest()
	if err := Validate(manifest); err != nil {
		return Manifest{}, err
	}
	return cloneManifest(manifest), nil
}

func digestDistributionRoot(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	rootInfo, err := os.Lstat(absolute)
	if err != nil {
		return "", err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return "", fmt.Errorf("Distribution root must be a physical directory")
	}

	type distributionEntry struct {
		Path   string `json:"path"`
		Kind   string `json:"kind"`
		Mode   uint32 `json:"mode"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
	}
	entries := make([]distributionEntry, 0)
	err = filepath.WalkDir(absolute, func(entryPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(entryPath)
		if err != nil {
			return err
		}
		if entryPath == absolute {
			if !info.IsDir() || !os.SameFile(rootInfo, info) {
				return fmt.Errorf("Distribution root identity changed")
			}
			return nil
		}
		relative, err := filepath.Rel(absolute, entryPath)
		if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("Distribution entry escapes root")
		}
		canonicalPath := filepath.ToSlash(relative)
		if canonicalPath == "" || strings.HasPrefix(canonicalPath, "/") || strings.Contains(canonicalPath, "\\") {
			return fmt.Errorf("Distribution entry path is not canonical")
		}
		if info.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 || entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(entryPath)
			if err != nil {
				return err
			}
			if !containedSymlinkTarget(absolute, entryPath, target) {
				return fmt.Errorf("Distribution symlink escapes root")
			}
			entries = append(entries, distributionEntry{Path: canonicalPath, Kind: "symlink", Size: int64(len(target)), Digest: integrity.SHA256Digest([]byte(target))})
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Distribution entry is not a regular file or contained symlink")
		}
		content, err := os.ReadFile(entryPath)
		if err != nil {
			return err
		}
		current, err := os.Lstat(entryPath)
		if err != nil {
			return err
		}
		if !os.SameFile(info, current) || info.Size() != current.Size() || info.Mode() != current.Mode() || info.ModTime() != current.ModTime() || int64(len(content)) != current.Size() {
			return fmt.Errorf("Distribution file changed during read")
		}
		entries = append(entries, distributionEntry{Path: canonicalPath, Kind: "file", Mode: uint32(current.Mode().Perm() & 0o111), Size: current.Size(), Digest: integrity.SHA256Digest(content)})
		return nil
	})
	if err != nil {
		return "", err
	}
	currentRoot, err := os.Lstat(absolute)
	if err != nil || !currentRoot.IsDir() || !os.SameFile(rootInfo, currentRoot) {
		return "", fmt.Errorf("Distribution root identity changed")
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("Distribution tree is empty")
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Path < entries[right].Path })
	payload := struct {
		SchemaVersion string              `json:"schema_version"`
		Entries       []distributionEntry `json:"entries"`
	}{SchemaVersion: "oaw.distribution-tree/v1", Entries: entries}
	encoded, err := canonicaljson.Marshal(payload)
	if err != nil {
		return "", err
	}
	return integrity.SHA256Digest(encoded), nil
}

func containedSymlinkTarget(root, linkPath, target string) bool {
	if target == "" || filepath.IsAbs(target) || strings.Contains(target, "\\") || filepath.Clean(target) != target {
		return false
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(linkPath), target))
	relative, err := filepath.Rel(root, resolved)
	return err == nil && relative != ".." && !filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func digestBindingRoot(checkoutRoot, relative string) (string, error) {
	root := filepath.Join(checkoutRoot, filepath.FromSlash(relative))
	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		evidence, err := integrity.DigestTree(root)
		if err != nil {
			return "", err
		}
		return evidence.RootDigest, nil
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("Binding root is not a regular file or directory")
	}
	content, err := os.ReadFile(root)
	if err != nil {
		return "", err
	}
	payload := struct {
		SchemaVersion string `json:"schema_version"`
		Path          string `json:"path"`
		Executable    bool   `json:"executable"`
		Size          int64  `json:"size"`
		Digest        string `json:"digest"`
	}{"oaw.binding-file/v1", relative, info.Mode().Perm()&0o111 != 0, info.Size(), integrity.SHA256Digest(content)}
	encoded, err := canonicaljson.Marshal(payload)
	if err != nil {
		return "", err
	}
	return integrity.SHA256Digest(encoded), nil
}
