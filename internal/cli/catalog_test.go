package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

func TestRunCatalogTextAndJSON(t *testing.T) {
	for _, args := range [][]string{
		{"catalog", "list", "providers"},
		{"catalog", "list", "recipes", "--format", "text"},
		{"catalog", "list", "aliases", "--format=text"},
		{"catalog", "validate", "--format", "json"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("Run(%v) = %d, stderr=%q", args, code, stderr.String())
		}
		if stderr.Len() != 0 || stdout.Len() == 0 {
			t.Fatalf("Run(%v) stdout=%q stderr=%q", args, stdout.String(), stderr.String())
		}
	}
}

func TestRunCatalogTextOutputMatchesContract(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "providers",
			args: []string{"catalog", "list", "providers"},
			want: "provider oaw/ecc version=1.0.0 capabilities=11\n" +
				"provider oaw/matt version=1.0.0 capabilities=9\n" +
				"provider oaw/superpowers version=1.0.0 capabilities=10\n",
		},
		{
			name: "recipes",
			args: []string{"catalog", "list", "recipes"},
			want: "recipe oaw/delivery version=1.0.0\n" +
				"recipe oaw/domain-engineering version=1.0.0\n" +
				"recipe oaw/ecc-engineering version=1.0.0\n" +
				"recipe oaw/hardening version=1.0.0\n" +
				"recipe oaw/reliable-feature version=1.0.0\n",
		},
		{
			name: "aliases",
			args: []string{"catalog", "list", "aliases"},
			want: "alias ECC-FULL recipe=oaw/ecc-engineering\n" +
				"alias MATT-FULL recipe=oaw/domain-engineering\n" +
				"alias MATT-SP-HYBRID recipe=oaw/reliable-feature\n" +
				"alias SP-FULL recipe=oaw/delivery\n",
		},
		{
			name: "validation",
			args: []string{"catalog", "validate"},
			want: "catalog valid providers=3 recipes=5 aliases=4\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(tt.args, &stdout, &stderr); code != 0 {
				t.Fatalf("Run(%v) = %d, stderr=%q", tt.args, code, stderr.String())
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("stdout = %q, want %q", got, tt.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestRunHelpDoesNotLoadCatalog(t *testing.T) {
	loaderCalled := false
	loader := func() (catalog.Catalog, error) {
		loaderCalled = true
		return catalog.Catalog{}, errors.New("loader must not be called")
	}
	for _, args := range [][]string{
		{"help"},
		{"--help"},
		{"catalog", "--help"},
		{"catalog", "list", "--help"},
		{"catalog", "validate", "--help"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr, loader); code != 0 {
			t.Errorf("run(%v) = %d, want 0; stderr=%q", args, code, stderr.String())
		}
		if stdout.Len() == 0 || stderr.Len() != 0 {
			t.Errorf("run(%v) stdout=%q stderr=%q", args, stdout.String(), stderr.String())
		}
	}
	if loaderCalled {
		t.Fatal("help loaded the catalog")
	}
}

func TestRunCatalogJSONShape(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"catalog", "list", "providers", "--format", "json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("Run() = %d, stderr=%q", code, stderr.String())
	}
	var response struct {
		SchemaVersion string `json:"schema_version"`
		Kind          string `json:"kind"`
		Digest        string `json:"digest"`
		Items         []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("JSON decode: %v", err)
	}
	if response.SchemaVersion != "oaw.catalog-output/v1" || response.Kind != "providers" || response.Digest == "" || len(response.Items) != 3 {
		t.Fatalf("response = %#v", response)
	}
	if response.Items[0].ID != "oaw/ecc" {
		t.Fatalf("first provider = %q", response.Items[0].ID)
	}
}

func TestRunCatalogJSONListsHaveExpectedKinds(t *testing.T) {
	for _, kind := range []string{"providers", "recipes", "aliases"} {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"catalog", "list", kind, "--format=json"}, &stdout, &stderr); code != 0 {
			t.Fatalf("Run(%s) = %d, stderr=%q", kind, code, stderr.String())
		}
		var response struct {
			SchemaVersion string          `json:"schema_version"`
			Kind          string          `json:"kind"`
			Digest        string          `json:"digest"`
			Items         json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
			t.Fatalf("%s JSON decode: %v", kind, err)
		}
		if response.SchemaVersion != "oaw.catalog-output/v1" || response.Kind != kind || response.Digest == "" {
			t.Fatalf("%s response = %#v", kind, response)
		}
		if len(response.Items) == 0 || string(response.Items) == "null" {
			t.Fatalf("%s items = %s, want array", kind, response.Items)
		}
	}
}

func TestRunRejectsInvalidCatalogArguments(t *testing.T) {
	for _, args := range [][]string{
		{"catalog"},
		{"catalog", "list"},
		{"catalog", "list", "unknown"},
		{"catalog", "list", "providers", "--format", "yaml"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 64 {
			t.Errorf("Run(%v) = %d, want 64", args, code)
		}
		if !strings.Contains(stderr.String(), "oaw: INVALID_ARGUMENT:") {
			t.Errorf("Run(%v) stderr = %q", args, stderr.String())
		}
	}
}

func TestRunRejectsInvalidArgumentsBeforeLoadingCatalog(t *testing.T) {
	loaderCalled := false
	loader := func() (catalog.Catalog, error) {
		loaderCalled = true
		return catalog.Catalog{}, errors.New("loader must not be called")
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"catalog", "list", "providers", "--format", "yaml"}, &stdout, &stderr, loader); code != 64 {
		t.Fatalf("run() = %d, want 64", code)
	}
	if loaderCalled {
		t.Fatal("invalid arguments loaded the catalog")
	}
}

func TestRunCatalogValidationFailure(t *testing.T) {
	loader := func() (catalog.Catalog, error) {
		return catalog.Catalog{}, errors.New("DUPLICATE_PROVIDER_ID: oaw/ecc")
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"catalog", "validate"}, &stdout, &stderr, loader); code != 65 {
		t.Fatalf("run() = %d, want 65", code)
	}
	if !strings.Contains(stderr.String(), "oaw: CATALOG_INVALID:") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunCatalogOutputIsDeterministic(t *testing.T) {
	var first, second bytes.Buffer
	if Run([]string{"catalog", "list", "recipes", "--format", "json"}, &first, new(bytes.Buffer)) != 0 || Run([]string{"catalog", "list", "recipes", "--format", "json"}, &second, new(bytes.Buffer)) != 0 {
		t.Fatal("catalog list failed")
	}
	if first.String() != second.String() {
		t.Fatal("catalog output is not deterministic")
	}
	if _, err := builtin.Load(); err != nil {
		t.Fatalf("builtin.Load() error = %v", err)
	}
}
