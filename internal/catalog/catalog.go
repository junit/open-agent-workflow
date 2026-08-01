package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

type Catalog struct {
	providers []ProviderDescriptorRecord
	recipes   []ProfileRecipeRecord
	aliases   []ProfileAliasRecord
	digest    string
}

func cloneProviderList(values []ProviderDescriptorRecord) []ProviderDescriptorRecord {
	result := cloneSlice(values)
	for i, value := range values {
		result[i] = cloneProvider(value)
	}
	return result
}

func cloneRecipeList(values []ProfileRecipeRecord) []ProfileRecipeRecord {
	result := cloneSlice(values)
	for i, value := range values {
		result[i] = cloneRecipe(value)
	}
	return result
}

func New(providers []ProviderDescriptorRecord, recipes []ProfileRecipeRecord, aliases []ProfileAliasRecord) (Catalog, error) {
	snapshot := Catalog{
		providers: cloneProviderList(providers),
		recipes:   cloneRecipeList(recipes),
		aliases:   cloneSlice(aliases),
	}
	if err := validateCatalog(&snapshot); err != nil {
		return Catalog{}, err
	}
	sort.Slice(snapshot.providers, func(i, j int) bool { return snapshot.providers[i].ID < snapshot.providers[j].ID })
	sort.Slice(snapshot.recipes, func(i, j int) bool { return snapshot.recipes[i].ID < snapshot.recipes[j].ID })
	sort.Slice(snapshot.aliases, func(i, j int) bool { return snapshot.aliases[i].Alias < snapshot.aliases[j].Alias })
	payload := struct {
		Providers []ProviderDescriptorRecord `json:"providers"`
		Recipes   []ProfileRecipeRecord      `json:"recipes"`
		Aliases   []ProfileAliasRecord       `json:"aliases"`
	}{snapshot.providers, snapshot.recipes, snapshot.aliases}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Catalog{}, err
	}
	digest := sha256.Sum256(encoded)
	snapshot.digest = hex.EncodeToString(digest[:])
	return snapshot, nil
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

func cloneSlice[T any](values []T) []T {
	if values == nil {
		return nil
	}
	result := make([]T, len(values))
	copy(result, values)
	return result
}
