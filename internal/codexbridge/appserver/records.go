package appserver

import "encoding/json"

type MetadataError struct {
	Message string `json:"message"`
	Path    string `json:"path"`
}

type ObservationDiagnostic struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

func observationDiagnostic(code, detail string) ObservationDiagnostic {
	return ObservationDiagnostic{Code: code, Detail: detail}
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

type HookMetadata struct {
	CurrentHash string `json:"currentHash"`
	Enabled     bool   `json:"enabled"`
	EventName   string `json:"eventName"`
	PluginID    string `json:"pluginId"`
	Source      string `json:"source"`
	TrustStatus string `json:"trustStatus"`
}

type HooksEntry struct {
	CWD      string          `json:"cwd"`
	Errors   []MetadataError `json:"errors"`
	Warnings []string        `json:"warnings"`
	Hooks    []HookMetadata  `json:"hooks"`
}

type ConfigProjection struct {
	CWDObserved         bool   `json:"cwd_observed"`
	SandboxDisposition  string `json:"sandbox_disposition"`
	MCPDisposition      string `json:"mcp_disposition"`
	HookDisposition     string `json:"hook_disposition"`
	ApprovalDisposition string `json:"approval_disposition"`
}

type MetadataObservation struct {
	Skills       SkillsEntry             `json:"skills"`
	Hooks        HooksEntry              `json:"hooks"`
	Config       ConfigProjection        `json:"config"`
	Methods      []string                `json:"methods"`
	Diagnostics  []ObservationDiagnostic `json:"diagnostics"`
	CodexVersion string                  `json:"codex_version"`
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

type hookMetadataWire struct {
	CurrentHash *string `json:"currentHash"`
	Enabled     *bool   `json:"enabled"`
	EventName   *string `json:"eventName"`
	PluginID    *string `json:"pluginId"`
	Source      *string `json:"source"`
	TrustStatus *string `json:"trustStatus"`
}

type hooksEntryWire struct {
	CWD      *string              `json:"cwd"`
	Errors   *[]metadataErrorWire `json:"errors"`
	Warnings *[]string            `json:"warnings"`
	Hooks    *[]hookMetadataWire  `json:"hooks"`
}

type hooksResultWire struct {
	Data *[]hooksEntryWire `json:"data"`
}

type configResultWire struct {
	Config  *json.RawMessage `json:"config"`
	Origins *json.RawMessage `json:"origins"`
}
