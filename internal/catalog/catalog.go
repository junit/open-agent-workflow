package catalog

import (
	"errors"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

// Catalog is an immutable Provider and Binding identity inventory. It carries
// no Profile, lifecycle, classification, or execution semantics.
type Catalog struct {
	providers []ProviderDescriptorRecord
	digest    string
}

func New(providers []ProviderDescriptorRecord) (Catalog, error) {
	snapshot := Catalog{providers: cloneProviderList(providers)}
	if snapshot.providers == nil {
		snapshot.providers = []ProviderDescriptorRecord{}
	}
	seen := make(map[string]struct{}, len(snapshot.providers))
	for index := range snapshot.providers {
		provider := &snapshot.providers[index]
		if err := validateProviderRecord(provider); err != nil {
			return Catalog{}, err
		}
		normalizeProvider(provider)
		if _, duplicate := seen[provider.ID]; duplicate {
			return Catalog{}, errors.New("DUPLICATE_PROVIDER_ID: duplicate Provider ID")
		}
		seen[provider.ID] = struct{}{}
	}
	sort.Slice(snapshot.providers, func(left, right int) bool {
		return snapshot.providers[left].ID < snapshot.providers[right].ID
	})
	digest, _, err := canonicaljson.Digest(struct {
		Providers []ProviderDescriptorRecord `json:"providers"`
	}{Providers: snapshot.providers})
	if err != nil {
		return Catalog{}, err
	}
	snapshot.digest = digest
	return snapshot, nil
}

func (catalog Catalog) Providers() []ProviderDescriptorRecord {
	return cloneProviderList(catalog.providers)
}

func (catalog Catalog) Digest() string { return catalog.digest }

func cloneProviderList(values []ProviderDescriptorRecord) []ProviderDescriptorRecord {
	result := make([]ProviderDescriptorRecord, len(values))
	for index, value := range values {
		result[index] = cloneProvider(value)
	}
	return result
}
