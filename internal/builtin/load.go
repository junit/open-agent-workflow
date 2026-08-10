package builtin

import (
	"fmt"
	"io/fs"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

func Load() (catalog.Catalog, error) {
	return loadFromFS(assets.FS())
}

func loadFromFS(files fs.FS) (catalog.Catalog, error) {
	registry, err := schema.New(files)
	if err != nil {
		return catalog.Catalog{}, err
	}
	providerPaths, err := fs.Glob(files, "providers/*.json")
	if err != nil {
		return catalog.Catalog{}, fmt.Errorf("BUILTIN_PROVIDER_LIST_FAILED: %w", err)
	}
	sort.Strings(providerPaths)
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
	recipePaths, err := fs.Glob(files, "recipes/*.json")
	if err != nil {
		return catalog.Catalog{}, fmt.Errorf("BUILTIN_RECIPE_LIST_FAILED: %w", err)
	}
	sort.Strings(recipePaths)
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
