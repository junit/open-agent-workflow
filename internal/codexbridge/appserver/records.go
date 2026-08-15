package appserver

type MetadataError struct {
	Message string `json:"message"`
	Path    string `json:"path"`
}

type SkillMetadata struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
	Scope   string `json:"scope"`
}

type SkillsEntry struct {
	CWD    string          `json:"cwd"`
	Errors []MetadataError `json:"errors"`
	Skills []SkillMetadata `json:"skills"`
}

type MetadataObservation struct {
	Skills       SkillsEntry `json:"skills"`
	CodexVersion string      `json:"codex_version"`
}

// Wire records use pointers for required projected fields. This preserves the
// distinction between a missing field and the field's valid zero value.
type metadataErrorWire struct {
	Message *string `json:"message"`
	Path    *string `json:"path"`
}

type skillMetadataWire struct {
	Name    *string `json:"name"`
	Enabled *bool   `json:"enabled"`
	Path    *string `json:"path"`
	Scope   *string `json:"scope"`
}

type skillsEntryWire struct {
	CWD    *string              `json:"cwd"`
	Errors *[]metadataErrorWire `json:"errors"`
	Skills *[]skillMetadataWire `json:"skills"`
}

type skillsResultWire struct {
	Data *[]skillsEntryWire `json:"data"`
}
