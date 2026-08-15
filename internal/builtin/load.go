package builtin

import (
	"fmt"
	"io/fs"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

var providerPaths = []string{
	"providers/oaw-ecc.json",
	"providers/oaw-matt.json",
	"providers/oaw-superpowers.json",
}

func Load() (catalog.Catalog, error) {
	return loadFromFS(assets.FS())
}

func loadFromFS(files fs.FS) (catalog.Catalog, error) {
	return loadCatalogRecords(files)
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
