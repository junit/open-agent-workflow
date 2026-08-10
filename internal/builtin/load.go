package builtin

import (
	"fmt"
	"io/fs"
	"slices"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/provideraudit"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

var providerPaths = []string{
	"providers/oaw-ecc.json",
	"providers/oaw-matt.json",
	"providers/oaw-superpowers.json",
}

var recipePaths = []string{
	"recipes/oaw-delivery.json",
	"recipes/oaw-domain-engineering.json",
	"recipes/oaw-ecc-engineering.json",
	"recipes/oaw-reliable-feature.json",
}

func Load() (catalog.Catalog, error) {
	return loadFromFS(assets.FS())
}

func LoadSourceAudit() (provideraudit.Manifest, error) {
	return loadSourceAuditFromFS(assets.FS())
}

func loadSourceAuditFromFS(files fs.FS) (provideraudit.Manifest, error) {
	raw, err := fs.ReadFile(files, "audits/provider-sources-v4.json")
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
	registry, err := schema.New(files)
	if err != nil {
		return catalog.Catalog{}, err
	}
	audit, err := loadSourceAuditFromFS(files)
	if err != nil {
		return catalog.Catalog{}, err
	}
	providers := make([]catalog.ProviderDescriptorRecord, 0, len(providerPaths))
	for _, path := range providerPaths {
		raw, readErr := fs.ReadFile(files, path)
		if readErr != nil {
			return catalog.Catalog{}, fmt.Errorf("BUILTIN_PROVIDER_READ_FAILED: %s: %w", path, readErr)
		}
		if validationErr := registry.Validate(schema.ProviderDescriptorV4, raw); validationErr != nil {
			return catalog.Catalog{}, fmt.Errorf("BUILTIN_PROVIDER_INVALID: %s: %w", path, validationErr)
		}
		provider, decodeErr := catalog.DecodeProvider(raw)
		if decodeErr != nil {
			return catalog.Catalog{}, fmt.Errorf("BUILTIN_PROVIDER_INVALID: %s: %w", path, decodeErr)
		}
		providers = append(providers, provider)
	}
	if err := validateProviderAudit(providers, audit); err != nil {
		return catalog.Catalog{}, fmt.Errorf("BUILTIN_SOURCE_AUDIT_INVALID: %w", err)
	}
	recipes := make([]catalog.ProfileRecipeRecord, 0, len(recipePaths))
	for _, path := range recipePaths {
		raw, readErr := fs.ReadFile(files, path)
		if readErr != nil {
			return catalog.Catalog{}, fmt.Errorf("BUILTIN_RECIPE_READ_FAILED: %s: %w", path, readErr)
		}
		if validationErr := registry.Validate(schema.ProfileRecipeV3, raw); validationErr != nil {
			return catalog.Catalog{}, fmt.Errorf("BUILTIN_RECIPE_INVALID: %s: %w", path, validationErr)
		}
		recipe, decodeErr := catalog.DecodeRecipe(raw)
		if decodeErr != nil {
			return catalog.Catalog{}, fmt.Errorf("BUILTIN_RECIPE_INVALID: %s: %w", path, decodeErr)
		}
		recipes = append(recipes, recipe)
	}
	aliasRaw, err := fs.ReadFile(files, "profile-aliases.json")
	if err != nil {
		return catalog.Catalog{}, fmt.Errorf("BUILTIN_ALIAS_READ_FAILED: %w", err)
	}
	if err := registry.Validate(schema.ProfileAliasSetV1, aliasRaw); err != nil {
		return catalog.Catalog{}, fmt.Errorf("BUILTIN_ALIAS_INVALID: profile-aliases.json: %w", err)
	}
	aliasSet, err := catalog.DecodeAliasSet(aliasRaw)
	if err != nil {
		return catalog.Catalog{}, fmt.Errorf("BUILTIN_ALIAS_INVALID: profile-aliases.json: %w", err)
	}
	aliases := make([]catalog.ProfileAliasRecord, len(aliasSet.Aliases))
	copy(aliases, aliasSet.Aliases)
	result, err := catalog.New(providers, recipes, aliases)
	if err != nil {
		return catalog.Catalog{}, fmt.Errorf("BUILTIN_CATALOG_INVALID: %w", err)
	}
	return result, nil
}

func validateProviderAudit(providers []catalog.ProviderDescriptorRecord, audit provideraudit.Manifest) error {
	if len(providers) != len(audit.Providers) {
		return fmt.Errorf("Provider count mismatch")
	}
	byID := make(map[string]catalog.ProviderDescriptorRecord, len(providers))
	for _, provider := range providers {
		byID[provider.ID] = provider
	}
	for _, source := range audit.Providers {
		provider, found := byID[source.ProviderID]
		if !found || len(provider.Distributions) != 1 || len(provider.Bindings) != len(source.Bindings) {
			return fmt.Errorf("Provider %s inventory mismatch", source.ProviderID)
		}
		distribution := provider.Distributions[0]
		if distribution.ID != source.DistributionID || distribution.SourceURI != source.SourceURI || distribution.Revision != source.Revision || distribution.TreeDigest != source.DistributionTreeDigest {
			return fmt.Errorf("Provider %s Distribution mismatch", source.ProviderID)
		}
		bindings := make(map[string]catalog.BindingRecord, len(provider.Bindings))
		for _, binding := range provider.Bindings {
			bindings[binding.ID] = binding
		}
		for _, audited := range source.Bindings {
			binding, found := bindings[audited.ID]
			if !found || binding.DistributionID != source.DistributionID || binding.ContentRoot != audited.ContentRoot || binding.InstallRoot != audited.InstallRoot || binding.TreeDigest != audited.TreeDigest || string(binding.Kind) != audited.Kind || !slices.Contains(audited.References, binding.Reference) {
				return fmt.Errorf("Provider %s Binding %s mismatch", source.ProviderID, audited.ID)
			}
		}
	}
	return nil
}
