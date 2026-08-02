package assets

import (
	"encoding/json"
	"io/fs"
	"testing"
)

func TestEmbeddedSchemasHaveStableMetadata(t *testing.T) {
	want := map[string]string{
		"schemas/v1/provider-descriptor.schema.json":  "https://open-agent-workflow.dev/schemas/v1/provider-descriptor.schema.json",
		"schemas/v1/profile-recipe.schema.json":       "https://open-agent-workflow.dev/schemas/v1/profile-recipe.schema.json",
		"schemas/v1/profile-alias-set.schema.json":    "https://open-agent-workflow.dev/schemas/v1/profile-alias-set.schema.json",
		"schemas/v1/host-manifest.schema.json":        "https://open-agent-workflow.dev/schemas/v1/host-manifest.schema.json",
		"schemas/v1/host-integration.schema.json":     "https://open-agent-workflow.dev/schemas/v1/host-integration.schema.json",
		"schemas/v1/host-integration-set.schema.json": "https://open-agent-workflow.dev/schemas/v1/host-integration-set.schema.json",
	}
	for path, id := range want {
		data, err := fs.ReadFile(FS(), path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var document map[string]any
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		if document["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Errorf("%s $schema = %v", path, document["$schema"])
		}
		if document["$id"] != id {
			t.Errorf("%s $id = %v, want %s", path, document["$id"], id)
		}
		if document["type"] != "object" || document["additionalProperties"] != false {
			t.Errorf("%s root metadata = %#v", path, document)
		}
		assertClosedObjects(t, path, document)
	}
}

func assertClosedObjects(t *testing.T, path string, value any) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		if value["type"] == "object" && value["additionalProperties"] != false {
			t.Errorf("%s contains an open object: %#v", path, value)
		}
		for _, child := range value {
			assertClosedObjects(t, path, child)
		}
	case []any:
		for _, child := range value {
			assertClosedObjects(t, path, child)
		}
	}
}
