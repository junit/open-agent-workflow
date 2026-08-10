package catalog

import (
	"errors"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

type Catalog struct {
	providers []ProviderDescriptorRecord
	recipes   []ProfileRecipeRecord
	aliases   []ProfileAliasRecord
	digest    string
}

func New(providers []ProviderDescriptorRecord, recipes []ProfileRecipeRecord, aliases []ProfileAliasRecord) (Catalog, error) {
	snapshot := Catalog{
		providers: cloneProviderList(providers),
		recipes:   cloneRecipeList(recipes),
		aliases:   cloneSlice(aliases),
	}
	if snapshot.providers == nil {
		snapshot.providers = []ProviderDescriptorRecord{}
	}
	if snapshot.recipes == nil {
		snapshot.recipes = []ProfileRecipeRecord{}
	}
	if snapshot.aliases == nil {
		snapshot.aliases = []ProfileAliasRecord{}
	}
	if err := validateCatalog(&snapshot); err != nil {
		return Catalog{}, err
	}
	sort.Slice(snapshot.providers, func(i, j int) bool { return snapshot.providers[i].ID < snapshot.providers[j].ID })
	sort.Slice(snapshot.recipes, func(i, j int) bool { return snapshot.recipes[i].ID < snapshot.recipes[j].ID })
	normalizeAliases(snapshot.aliases)
	payload := struct {
		Providers []ProviderDescriptorRecord `json:"providers"`
		Recipes   []ProfileRecipeRecord      `json:"recipes"`
		Aliases   []ProfileAliasRecord       `json:"aliases"`
	}{snapshot.providers, snapshot.recipes, snapshot.aliases}
	digest, _, err := canonicaljson.Digest(payload)
	if err != nil {
		return Catalog{}, err
	}
	snapshot.digest = digest
	return snapshot, nil
}

func NormalizeAndDigestRecipe(providers []ProviderDescriptorRecord, recipe ProfileRecipeRecord) (ProfileRecipeRecord, string, error) {
	providerCopies := cloneProviderList(providers)
	providerIndex := make(map[string]ProviderDescriptorRecord, len(providerCopies))
	for index := range providerCopies {
		provider := &providerCopies[index]
		if err := validateProviderRecord(provider); err != nil {
			return ProfileRecipeRecord{}, "", err
		}
		normalizeProvider(provider)
		if _, exists := providerIndex[provider.ID]; exists {
			return ProfileRecipeRecord{}, "", errors.New("DUPLICATE_PROVIDER_ID: duplicate provider id")
		}
		providerIndex[provider.ID] = cloneProvider(*provider)
	}
	return normalizeAndDigestRecipe(providerIndex, recipe)
}

func normalizeAndDigestRecipe(providers map[string]ProviderDescriptorRecord, recipe ProfileRecipeRecord) (ProfileRecipeRecord, string, error) {
	normalized := cloneRecipe(recipe)
	if err := validateRecipeRecord(&normalized); err != nil {
		return ProfileRecipeRecord{}, "", err
	}
	if err := normalizeRecipe(&normalized); err != nil {
		return ProfileRecipeRecord{}, "", err
	}
	if err := validateRecipeGraph(&normalized, providers); err != nil {
		return ProfileRecipeRecord{}, "", err
	}
	digest, _, err := canonicaljson.Digest(normalized)
	if err != nil {
		return ProfileRecipeRecord{}, "", err
	}
	return cloneRecipe(normalized), digest, nil
}

func (catalog Catalog) Providers() []ProviderDescriptorRecord {
	return cloneProviderList(catalog.providers)
}

func (catalog Catalog) Recipes() []ProfileRecipeRecord {
	return cloneRecipeList(catalog.recipes)
}

func (catalog Catalog) Aliases() []ProfileAliasRecord {
	return cloneSlice(catalog.aliases)
}

func (catalog Catalog) Digest() string { return catalog.digest }

func cloneProviderList(values []ProviderDescriptorRecord) []ProviderDescriptorRecord {
	result := cloneSlice(values)
	for index, value := range values {
		result[index] = cloneProvider(value)
	}
	return result
}

func cloneRecipeList(values []ProfileRecipeRecord) []ProfileRecipeRecord {
	result := cloneSlice(values)
	for index, value := range values {
		result[index] = cloneRecipe(value)
	}
	return result
}
