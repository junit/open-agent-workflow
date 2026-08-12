package provideraudit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

func Build(checkouts []Checkout) (Manifest, error) {
	if len(checkouts) != len(lockedProviderSpecs) {
		return Manifest{}, invalidAudit("exactly four source checkouts are required", nil)
	}
	bySource := make(map[sourceKey]Checkout, len(checkouts))
	for _, checkout := range checkouts {
		key := sourceKey{ProviderID: checkout.ProviderID, DistributionID: checkout.DistributionID}
		if _, duplicate := bySource[key]; duplicate {
			return Manifest{}, invalidAudit("duplicate source checkout", nil)
		}
		bySource[key] = checkout
	}
	providers := make([]ProviderSource, len(lockedProviderSpecs))
	for index, spec := range lockedProviderSpecs {
		key := sourceKey{ProviderID: spec.ID, DistributionID: spec.DistributionID}
		checkout, found := bySource[key]
		if !found || checkout.SourceURI != spec.SourceURI || checkout.Revision != spec.Revision || checkout.DistributionRoot != spec.DistributionRoot || checkout.Root == "" || len(checkout.BindingRoots) != len(spec.Bindings) {
			return Manifest{}, invalidAudit("Provider checkout pin mismatch", nil)
		}
		distributionRoot, err := physicalDistributionRoot(checkout.Root, checkout.DistributionRoot)
		if err != nil {
			return Manifest{}, invalidAudit("Provider Distribution root is unavailable", err)
		}
		for _, evidenceRoot := range spec.EvidenceRoots {
			if _, _, err := physicalSourceRoot(distributionRoot, evidenceRoot, "evidence"); err != nil {
				return Manifest{}, invalidAudit("Provider evidence root is unavailable", err)
			}
		}
		distributionDigest, err := digestDistributionRoot(distributionRoot)
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
			digest, err := digestBindingRoot(distributionRoot, binding.Root)
			if err != nil {
				return Manifest{}, invalidAudit("digest Binding root", err)
			}
			bindings[bindingIndex] = BindingSource{ID: expected.ID, ContentRoot: expected.ContentRoot, InstallRoot: expected.InstallRoot, TreeDigest: digest, Kind: expected.Kind, References: append([]string{}, expected.References...)}
		}
		providers[index] = ProviderSource{ProviderID: spec.ID, SourceURI: spec.SourceURI, Revision: spec.Revision, DistributionID: spec.DistributionID, DistributionRoot: spec.DistributionRoot, DistributionTreeDigest: distributionDigest, Bindings: bindings, EvidenceRoots: append([]string{}, spec.EvidenceRoots...)}
	}
	manifest := Manifest{SchemaVersion: ProviderSourceAuditSchemaV1, CanonicalMatrixDigest: CanonicalMatrixDigest, Providers: providers}
	manifest.Digest = manifest.ContentDigest()
	if err := Validate(manifest); err != nil {
		return Manifest{}, err
	}
	return cloneManifest(manifest), nil
}

func physicalDistributionRoot(checkoutRoot, relative string) (string, error) {
	if !cleanRelative(relative, true) {
		return "", fmt.Errorf("Distribution root is not canonical")
	}
	absolute, err := filepath.Abs(checkoutRoot)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("checkout root must be a physical directory")
	}
	current := absolute
	if relative == "." {
		return current, nil
	}
	for _, component := range strings.Split(relative, "/") {
		current = filepath.Join(current, filepath.FromSlash(component))
		info, err = os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("Distribution root must contain only physical directories")
		}
	}
	return current, nil
}

func digestDistributionRoot(root string) (string, error) {
	evidence, err := integrity.DigestDistributionTree(root)
	if err != nil {
		return "", err
	}
	return evidence.RootDigest, nil
}

func digestBindingRoot(checkoutRoot, relative string) (string, error) {
	root, info, err := physicalSourceRoot(checkoutRoot, relative, "Binding")
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

func physicalSourceRoot(distributionRoot, relative, label string) (string, os.FileInfo, error) {
	if !cleanRelative(relative, false) {
		return "", nil, fmt.Errorf("%s root is not canonical", label)
	}
	absolute, err := filepath.Abs(distributionRoot)
	if err != nil {
		return "", nil, err
	}
	absolute = filepath.Clean(absolute)
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", nil, fmt.Errorf("Distribution root must be a physical directory")
	}
	current := absolute
	components := strings.Split(relative, "/")
	for index, component := range components {
		current = filepath.Join(current, filepath.FromSlash(component))
		info, err = os.Lstat(current)
		if err != nil {
			return "", nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", nil, fmt.Errorf("%s root must not traverse symlinks", label)
		}
		if index < len(components)-1 && !info.IsDir() {
			return "", nil, fmt.Errorf("%s root ancestors must be physical directories", label)
		}
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("%s root is not a regular file or directory", label)
	}
	return current, info, nil
}
