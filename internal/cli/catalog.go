package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

const catalogOutputSchemaV1 = "oaw.catalog-output/v1"

type catalogListResponse struct {
	SchemaVersion string `json:"schema_version"`
	Kind          string `json:"kind"`
	Digest        string `json:"digest"`
	Items         any    `json:"items"`
}

type catalogValidationResponse struct {
	SchemaVersion string `json:"schema_version"`
	Valid         bool   `json:"valid"`
	Digest        string `json:"digest"`
	Providers     int    `json:"providers"`
	Recipes       int    `json:"recipes"`
	Aliases       int    `json:"aliases"`
}

func renderList(catalogValue catalog.Catalog, kind, format string, stdout, stderr io.Writer) int {
	if format == "text" {
		switch kind {
		case "providers":
			for _, provider := range catalogValue.Providers() {
				fmt.Fprintf(stdout, "provider %s version=%s capabilities=%d\n", provider.ID, provider.DescriptorVersion, len(provider.Capabilities))
			}
		case "recipes":
			for _, recipe := range catalogValue.Recipes() {
				fmt.Fprintf(stdout, "recipe %s version=%s\n", recipe.ID, recipe.RecipeVersion)
			}
		case "aliases":
			for _, alias := range catalogValue.Aliases() {
				fmt.Fprintf(stdout, "alias %s recipe=%s\n", alias.Alias, alias.RecipeID)
			}
		}
		return 0
	}
	var items any
	switch kind {
	case "providers":
		items = catalogValue.Providers()
	case "recipes":
		items = catalogValue.Recipes()
	case "aliases":
		items = catalogValue.Aliases()
	}
	return writeJSON(stdout, catalogListResponse{SchemaVersion: catalogOutputSchemaV1, Kind: kind, Digest: catalogValue.Digest(), Items: items}, stderr)
}

func renderValidation(catalogValue catalog.Catalog, format string, stdout, stderr io.Writer) int {
	if format == "text" {
		fmt.Fprintf(stdout, "catalog valid providers=%d recipes=%d aliases=%d\n", len(catalogValue.Providers()), len(catalogValue.Recipes()), len(catalogValue.Aliases()))
		return 0
	}
	return writeJSON(stdout, catalogValidationResponse{SchemaVersion: catalogOutputSchemaV1, Valid: true, Digest: catalogValue.Digest(), Providers: len(catalogValue.Providers()), Recipes: len(catalogValue.Recipes()), Aliases: len(catalogValue.Aliases())}, stderr)
}

func writeJSON(stdout io.Writer, value any, stderr io.Writer) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintf(stderr, "oaw: OUTPUT_ERROR: %v\n", err)
		return 1
	}
	return 0
}
