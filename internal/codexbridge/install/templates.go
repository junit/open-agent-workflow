package install

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MarketplaceName = "oaw-local"
	PluginName      = "oaw-codex-assurance"
)

var (
	semverPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?(?:\+[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$`)
	renderedFiles = []struct {
		source string
		target string
	}{
		{source: ".agents/plugins/marketplace.json", target: ".agents/plugins/marketplace.json"},
		{source: ".codex-plugin/plugin.json", target: "plugins/oaw-codex-assurance/.codex-plugin/plugin.json"},
		{source: ".mcp.json", target: "plugins/oaw-codex-assurance/.mcp.json"},
		{source: "hooks/hooks.json", target: "plugins/oaw-codex-assurance/hooks/hooks.json"},
		{source: "skills/oaw-codex-bridge/SKILL.md", target: "plugins/oaw-codex-assurance/skills/oaw-codex-bridge/SKILL.md"},
	}
	expectedHookMatchers = []string{
		"mcp__oaw_codex_bridge__observe_profile",
	}
)

//go:embed assets/.agents/plugins/marketplace.json assets/.codex-plugin/plugin.json assets/.mcp.json assets/hooks/hooks.json assets/skills/oaw-codex-bridge/SKILL.md
var templateFS embed.FS

type RenderOptions struct {
	Binary      string
	Version     string
	Marketplace string
	Plugin      string
}

type Error struct {
	code    string
	message string
	cause   error
}

func (e *Error) Error() string {
	if e.cause == nil {
		return e.message
	}
	return fmt.Sprintf("%s: %v", e.message, e.cause)
}

func (e *Error) Unwrap() error {
	return e.cause
}

func Code(err error) string {
	var installErr *Error
	if errors.As(err, &installErr) {
		return installErr.code
	}
	return ""
}

func installError(code, message string, cause error) error {
	return &Error{code: code, message: message, cause: cause}
}

func Render(options RenderOptions) (map[string][]byte, error) {
	if err := validateRenderOptions(options); err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" {
		return nil, installError("BRIDGE_INSTALL_UNSUPPORTED", "Codex Hook command rendering is not verified on Windows", nil)
	}

	hookCommand := quotePOSIX(options.Binary) + " hook codex"
	replacements := map[string]string{
		"{{OAW_BINARY}}":       options.Binary,
		"{{OAW_HOOK_COMMAND}}": hookCommand,
		"{{BRIDGE_VERSION}}":   options.Version,
		"{{MARKETPLACE_NAME}}": options.Marketplace,
		"{{PLUGIN_NAME}}":      options.Plugin,
	}
	result := make(map[string][]byte, len(renderedFiles))
	for _, file := range renderedFiles {
		raw, err := templateFS.ReadFile("assets/" + file.source)
		if err != nil {
			return nil, installError("BRIDGE_INSTALL_TEMPLATE_INVALID", "read embedded Bridge template", err)
		}
		rendered, err := replaceTemplateValues(raw, replacements)
		if err != nil {
			return nil, err
		}
		result[file.target] = rendered
	}
	if err := validateRenderedFiles(result, options, hookCommand); err != nil {
		return nil, err
	}
	return result, nil
}

func validateRenderOptions(options RenderOptions) error {
	values := []string{options.Binary, options.Version, options.Marketplace, options.Plugin}
	for _, value := range values {
		if !utf8.ValidString(value) || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return installError("BRIDGE_INSTALL_INPUT_INVALID", "Bridge render input contains invalid text", nil)
		}
	}
	if !filepath.IsAbs(options.Binary) || filepath.Clean(options.Binary) != options.Binary {
		return installError("BRIDGE_INSTALL_INPUT_INVALID", "Bridge binary path must be absolute and clean", nil)
	}
	if !semverPattern.MatchString(options.Version) {
		return installError("BRIDGE_INSTALL_INPUT_INVALID", "Bridge version must be semver-like", nil)
	}
	if options.Marketplace != MarketplaceName || options.Plugin != PluginName {
		return installError("BRIDGE_INSTALL_INPUT_INVALID", "Bridge marketplace and Plugin identities are fixed in v1", nil)
	}
	return nil
}

func replaceTemplateValues(raw []byte, replacements map[string]string) ([]byte, error) {
	rendered := slices.Clone(raw)
	for token, value := range replacements {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, installError("BRIDGE_INSTALL_INPUT_INVALID", "encode Bridge template value", err)
		}
		rendered = bytes.ReplaceAll(rendered, []byte(token), encoded[1:len(encoded)-1])
	}
	if bytes.Contains(rendered, []byte("{{")) || bytes.Contains(rendered, []byte("}}")) {
		return nil, installError("BRIDGE_INSTALL_TEMPLATE_INVALID", "unresolved Bridge template placeholder", nil)
	}
	return rendered, nil
}

func quotePOSIX(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

type pluginInterface struct {
	DisplayName      string `json:"displayName"`
	ShortDescription string `json:"shortDescription"`
	Category         string `json:"category"`
}

type pluginManifest struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Skills      string          `json:"skills"`
	MCPServers  string          `json:"mcpServers"`
	Hooks       string          `json:"hooks"`
	Interface   pluginInterface `json:"interface"`
}

type mcpServer struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	CWD     string   `json:"cwd"`
	EnvVars []string `json:"env_vars"`
}

type hookHandler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type hookMatcher struct {
	Matcher string        `json:"matcher"`
	Hooks   []hookHandler `json:"hooks"`
}

type hookEvents struct {
	PreToolUse []hookMatcher `json:"PreToolUse"`
}

type hookDocument struct {
	Hooks hookEvents `json:"hooks"`
}

type marketplacePlugin struct {
	Name    string            `json:"name"`
	Source  marketplaceSource `json:"source"`
	Version string            `json:"version"`
}

type marketplaceSource struct {
	Source string `json:"source"`
	Path   string `json:"path"`
}

type marketplaceManifest struct {
	Name    string              `json:"name"`
	Plugins []marketplacePlugin `json:"plugins"`
}

func validateRenderedFiles(files map[string][]byte, options RenderOptions, hookCommand string) error {
	manifest := pluginManifest{}
	if err := decodeClosed(files["plugins/oaw-codex-assurance/.codex-plugin/plugin.json"], &manifest); err != nil {
		return invalidTemplate("decode Plugin manifest", err)
	}
	if manifest.Name != options.Plugin || manifest.Version != options.Version || manifest.Description == "" ||
		manifest.Skills != "./skills/" || manifest.MCPServers != "./.mcp.json" || manifest.Hooks != "./hooks/hooks.json" ||
		manifest.Interface.DisplayName == "" || manifest.Interface.ShortDescription == "" || manifest.Interface.Category != "Developer Tools" {
		return invalidTemplate("Plugin manifest does not match the locked Bridge surface", nil)
	}

	servers := map[string]mcpServer{}
	if err := decodeClosed(files["plugins/oaw-codex-assurance/.mcp.json"], &servers); err != nil {
		return invalidTemplate("decode MCP server map", err)
	}
	server, ok := servers["oaw_codex_bridge"]
	if !ok || len(servers) != 1 || server.Command != options.Binary || !slices.Equal(server.Args, []string{"serve", "codex"}) || server.CWD != "." || len(server.EnvVars) != 0 {
		return invalidTemplate("MCP server map does not match the locked Bridge surface", nil)
	}

	hooks := hookDocument{}
	if err := decodeClosed(files["plugins/oaw-codex-assurance/hooks/hooks.json"], &hooks); err != nil {
		return invalidTemplate("decode Hook configuration", err)
	}
	if len(hooks.Hooks.PreToolUse) != len(expectedHookMatchers) {
		return invalidTemplate("Hook configuration must contain exactly one PreToolUse matcher", nil)
	}
	for index, matcher := range hooks.Hooks.PreToolUse {
		if matcher.Matcher != expectedHookMatchers[index] || len(matcher.Hooks) != 1 || matcher.Hooks[0].Type != "command" || matcher.Hooks[0].Command != hookCommand {
			return invalidTemplate("Hook matcher does not match the locked Bridge surface", nil)
		}
	}
	marketplace := marketplaceManifest{}
	if err := decodeClosed(files[".agents/plugins/marketplace.json"], &marketplace); err != nil {
		return invalidTemplate("decode marketplace manifest", err)
	}
	if marketplace.Name != options.Marketplace || len(marketplace.Plugins) != 1 {
		return invalidTemplate("marketplace identity does not match the locked Bridge surface", nil)
	}
	entry := marketplace.Plugins[0]
	if entry.Name != options.Plugin || entry.Source.Source != "local" || entry.Source.Path != "./plugins/oaw-codex-assurance" || entry.Version != options.Version {
		return invalidTemplate("marketplace Plugin does not match the locked Bridge surface", nil)
	}
	if len(files["plugins/oaw-codex-assurance/skills/oaw-codex-bridge/SKILL.md"]) == 0 {
		return invalidTemplate("Bridge Skill must not be empty", nil)
	}
	return nil
}

func decodeClosed(content []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func invalidTemplate(message string, cause error) error {
	return installError("BRIDGE_INSTALL_TEMPLATE_INVALID", message, cause)
}
