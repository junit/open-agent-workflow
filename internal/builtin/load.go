package builtin

import (
	"fmt"
	"io/fs"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/provideraudit"
)

var providerPaths = []string{
	"providers/oaw-ecc.json",
	"providers/oaw-matt.json",
	"providers/oaw-superpowers.json",
}

func Load() (catalog.Catalog, error) {
	return loadFromFS(assets.FS())
}

func LoadSourceAudit() (provideraudit.Manifest, error) {
	return loadSourceAuditFromFS(assets.FS())
}

func loadSourceAuditFromFS(files fs.FS) (provideraudit.Manifest, error) {
	raw, err := fs.ReadFile(files, "audits/provider-sources-v5.json")
	if err != nil {
		return provideraudit.Manifest{}, fmt.Errorf("BUILTIN_SOURCE_AUDIT_INVALID: read audit: %w", err)
	}
	value, err := provideraudit.Decode(raw)
	if err != nil {
		return provideraudit.Manifest{}, fmt.Errorf("BUILTIN_SOURCE_AUDIT_INVALID: %w", err)
	}
	return value, nil
}

func loadFromFS(files fs.FS) (catalog.Catalog, error) {
	result, _, err := loadCatalogAndAuditFromFS(files)
	return result, err
}

func loadCatalogAndAuditFromFS(files fs.FS) (catalog.Catalog, provideraudit.Manifest, error) {
	audit, err := loadSourceAuditFromFS(files)
	if err != nil {
		return catalog.Catalog{}, provideraudit.Manifest{}, err
	}
	result, err := loadCatalogRecords(files)
	if err != nil {
		return catalog.Catalog{}, provideraudit.Manifest{}, err
	}
	if err := validateProviderAudit(result.Providers(), audit); err != nil {
		return catalog.Catalog{}, provideraudit.Manifest{}, fmt.Errorf("BUILTIN_SOURCE_AUDIT_INVALID: %w", err)
	}
	return result, audit, nil
}

func loadCatalogRecords(files fs.FS) (catalog.Catalog, error) {
	providers := make([]catalog.ProviderDescriptorRecord, 0, len(providerPaths))
	for _, path := range providerPaths {
		raw, readErr := fs.ReadFile(files, path)
		if readErr != nil {
			return catalog.Catalog{}, fmt.Errorf("BUILTIN_PROVIDER_READ_FAILED: %s: %w", path, readErr)
		}
		provider, decodeErr := catalog.DecodeProvider(raw)
		if decodeErr != nil {
			return catalog.Catalog{}, fmt.Errorf("BUILTIN_PROVIDER_INVALID: %s: %w", path, decodeErr)
		}
		providers = append(providers, provider)
	}
	result, err := catalog.New(providers)
	if err != nil {
		return catalog.Catalog{}, fmt.Errorf("BUILTIN_CATALOG_INVALID: %w", err)
	}
	return result, nil
}

func validateProviderAudit(providers []catalog.ProviderDescriptorRecord, audit provideraudit.Manifest) error {
	byID := make(map[string]catalog.ProviderDescriptorRecord, len(providers))
	for _, provider := range providers {
		byID[provider.ID] = provider
	}
	sourcesByProvider := make(map[string][]provideraudit.ProviderSource, len(providers))
	for _, source := range audit.Providers {
		sourcesByProvider[source.ProviderID] = append(sourcesByProvider[source.ProviderID], source)
	}
	if len(byID) != len(sourcesByProvider) {
		return fmt.Errorf("Provider count mismatch")
	}
	for providerID, provider := range byID {
		sources, found := sourcesByProvider[providerID]
		if !found || len(provider.Distributions) != len(sources) {
			return fmt.Errorf("Provider %s inventory mismatch", providerID)
		}
		distributions := make(map[string]catalog.DistributionRecord, len(provider.Distributions))
		for _, distribution := range provider.Distributions {
			distributions[distribution.ID] = distribution
		}
		bindings := make(map[string]catalog.BindingRecord, len(provider.Bindings))
		for _, binding := range provider.Bindings {
			bindings[binding.ID] = binding
		}
		auditedBindingCount := 0
		for _, source := range sources {
			distribution, found := distributions[source.DistributionID]
			if !found || distribution.SourceURI != source.SourceURI || distribution.Revision != source.Revision || distribution.TreeDigest != source.DistributionTreeDigest {
				return fmt.Errorf("Provider %s Distribution mismatch", providerID)
			}
			auditedBindingCount += len(source.Bindings)
			for _, audited := range source.Bindings {
				binding, found := bindings[audited.ID]
				if !found || binding.DistributionID != source.DistributionID || binding.ContentRoot != audited.ContentRoot || binding.InstallRoot != audited.InstallRoot || binding.TreeDigest != audited.TreeDigest || string(binding.Kind) != audited.Kind || !containsReference(audited.References, binding.Reference) {
					return fmt.Errorf("Provider %s Binding %s mismatch", providerID, audited.ID)
				}
			}
		}
		if len(provider.Bindings) != auditedBindingCount {
			return fmt.Errorf("Provider %s inventory mismatch", providerID)
		}
	}
	return nil
}

func containsReference(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
