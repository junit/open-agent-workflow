package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
)

const (
	testMarketplace = "oaw-local"
	testPlugin      = "oaw-codex-assurance"
	testVersion     = "1.0.0"
)

var expectedRenderedPaths = []string{
	".agents/plugins/marketplace.json",
	"plugins/oaw-codex-assurance/.codex-plugin/plugin.json",
	"plugins/oaw-codex-assurance/.mcp.json",
	"plugins/oaw-codex-assurance/hooks/hooks.json",
	"plugins/oaw-codex-assurance/skills/oaw-codex-bridge/SKILL.md",
}

func TestRenderPluginHasExactSurface(t *testing.T) {
	files, err := Render(validRenderOptions("/state/bin/oaw-bridge"))
	if err != nil {
		t.Fatal(err)
	}

	paths := slices.Sorted(maps.Keys(files))
	expected := slices.Sorted(slices.Values(expectedRenderedPaths))
	if !slices.Equal(paths, expected) {
		t.Fatalf("rendered paths = %q, want %q", paths, expected)
	}
	for path, content := range files {
		if bytes.Contains(content, []byte("{{")) {
			t.Fatalf("%s contains an unresolved placeholder", path)
		}
	}

	assertPluginManifest(t, files["plugins/oaw-codex-assurance/.codex-plugin/plugin.json"])
	assertDirectMCPMap(t, files["plugins/oaw-codex-assurance/.mcp.json"], "/state/bin/oaw-bridge")
	assertExactMatchers(t, files["plugins/oaw-codex-assurance/hooks/hooks.json"], quotePOSIX("/state/bin/oaw-bridge")+" hook codex")
	skill := string(files["plugins/oaw-codex-assurance/skills/oaw-codex-bridge/SKILL.md"])
	for _, required := range []string{
		"optional current Codex Binding assurance",
		"Assurance Overlay",
		"does not select or run",
		"hand-authored Hook JSON",
		"machine claim",
	} {
		if !strings.Contains(skill, required) {
			t.Fatalf("Bridge Skill is missing trust-boundary text %q", required)
		}
	}
	if strings.Contains(skill, "core_compile") || strings.Contains(skill, "workflow_exchange") {
		t.Fatal("Bridge Skill retains machine workflow operations")
	}
	assertMarketplaceSource(t, files[".agents/plugins/marketplace.json"])
}

func TestRenderRejectsUnsafeAbsoluteBinary(t *testing.T) {
	invalid := []string{
		"relative/oaw",
		"/state/bin/../oaw",
		"/state/bin/oaw\nnext",
		"",
	}
	for _, binary := range invalid {
		t.Run(strings.ReplaceAll(binary, "/", "_"), func(t *testing.T) {
			options := validRenderOptions(binary)
			if _, err := Render(options); Code(err) != "BRIDGE_INSTALL_INPUT_INVALID" {
				t.Fatalf("Render(%q) error = %v", binary, err)
			}
		})
	}
}

func TestRenderRejectsAlternateBridgeIdentities(t *testing.T) {
	for _, options := range []RenderOptions{
		{Binary: "/state/bin/oaw", Version: testVersion, Marketplace: "other", Plugin: testPlugin},
		{Binary: "/state/bin/oaw", Version: testVersion, Marketplace: testMarketplace, Plugin: "other"},
	} {
		if _, err := Render(options); Code(err) != "BRIDGE_INSTALL_INPUT_INVALID" {
			t.Fatalf("Render(%#v) error = %v", options, err)
		}
	}
}

func TestRenderRejectsInvalidVersion(t *testing.T) {
	for _, version := range []string{"", "v1", "1", "1.0", "1.0.0/latest", "1.0.0\nnext"} {
		options := validRenderOptions("/state/bin/oaw")
		options.Version = version
		if _, err := Render(options); Code(err) != "BRIDGE_INSTALL_INPUT_INVALID" {
			t.Fatalf("Render version %q error = %v", version, err)
		}
	}
}

func TestRenderEscapesBinaryPathForJSONAndHookShell(t *testing.T) {
	binary := "/state/space $dir/it's/oaw"
	files, err := Render(validRenderOptions(binary))
	if err != nil {
		t.Fatal(err)
	}

	assertDirectMCPMap(t, files["plugins/oaw-codex-assurance/.mcp.json"], binary)
	assertExactMatchers(t, files["plugins/oaw-codex-assurance/hooks/hooks.json"], quotePOSIX(binary)+" hook codex")
}

func TestRenderedProjectionRejectsEverySurfaceDrift(t *testing.T) {
	options := validRenderOptions("/state/bin/oaw-bridge")
	hookCommand := quotePOSIX(options.Binary) + " hook codex"
	tests := []struct {
		name   string
		mutate func(map[string][]byte)
	}{
		{name: "manifest", mutate: func(files map[string][]byte) {
			path := "plugins/oaw-codex-assurance/.codex-plugin/plugin.json"
			files[path] = bytes.Replace(files[path], []byte(`"./skills/"`), []byte(`"./other/"`), 1)
		}},
		{name: "mcp", mutate: func(files map[string][]byte) {
			path := "plugins/oaw-codex-assurance/.mcp.json"
			files[path] = bytes.Replace(files[path], []byte(`"serve", "codex"`), []byte(`"serve", "other"`), 1)
		}},
		{name: "hooks", mutate: func(files map[string][]byte) {
			path := "plugins/oaw-codex-assurance/hooks/hooks.json"
			var document hookDocument
			if err := json.Unmarshal(files[path], &document); err != nil {
				t.Fatal(err)
			}
			document.Hooks.PreToolUse = nil
			files[path], _ = json.Marshal(document)
		}},
		{name: "marketplace", mutate: func(files map[string][]byte) {
			path := ".agents/plugins/marketplace.json"
			files[path] = bytes.Replace(files[path], []byte(`"./plugins/oaw-codex-assurance"`), []byte(`"./plugins/other"`), 1)
		}},
		{name: "skill", mutate: func(files map[string][]byte) {
			files["plugins/oaw-codex-assurance/skills/oaw-codex-bridge/SKILL.md"] = nil
		}},
		{name: "unknown manifest field", mutate: func(files map[string][]byte) {
			path := "plugins/oaw-codex-assurance/.codex-plugin/plugin.json"
			files[path] = bytes.Replace(files[path], []byte("{"), []byte(`{"unknown":true,`), 1)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files, err := Render(options)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(files)
			if err := validateRenderedFiles(files, options, hookCommand); Code(err) != "BRIDGE_INSTALL_TEMPLATE_INVALID" {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestTemplateHelpersRejectUnknownAndTrailingValues(t *testing.T) {
	if _, err := replaceTemplateValues([]byte(`{"value":"{{UNKNOWN}}"}`), map[string]string{}); Code(err) != "BRIDGE_INSTALL_TEMPLATE_INVALID" {
		t.Fatalf("placeholder error = %v", err)
	}
	if err := decodeClosed([]byte(`{"name":"x"} {}`), &struct {
		Name string `json:"name"`
	}{}); err == nil {
		t.Fatal("trailing JSON accepted")
	}
	cause := errors.New("cause")
	err := installError("TEST", "message", cause)
	if !errors.Is(err, cause) || err.Error() != "message: cause" {
		t.Fatalf("wrapped error = %v", err)
	}
}

func validRenderOptions(binary string) RenderOptions {
	return RenderOptions{
		Binary:      binary,
		Version:     testVersion,
		Marketplace: testMarketplace,
		Plugin:      testPlugin,
	}
}

func assertPluginManifest(t *testing.T, content []byte) {
	t.Helper()
	type interfaceProjection struct {
		DisplayName      string `json:"displayName"`
		ShortDescription string `json:"shortDescription"`
		Category         string `json:"category"`
	}
	type manifestProjection struct {
		Name        string              `json:"name"`
		Version     string              `json:"version"`
		Description string              `json:"description"`
		Skills      string              `json:"skills"`
		MCPServers  string              `json:"mcpServers"`
		Hooks       string              `json:"hooks"`
		Interface   interfaceProjection `json:"interface"`
	}
	var manifest manifestProjection
	decodeClosedJSON(t, content, &manifest)
	if manifest.Name != testPlugin || manifest.Version != testVersion {
		t.Fatalf("manifest identity = %q@%q", manifest.Name, manifest.Version)
	}
	if manifest.Skills != "./skills/" || manifest.MCPServers != "./.mcp.json" || manifest.Hooks != "./hooks/hooks.json" {
		t.Fatalf("manifest component paths = skills:%q mcp:%q hooks:%q", manifest.Skills, manifest.MCPServers, manifest.Hooks)
	}
	if manifest.Interface.DisplayName == "" || manifest.Interface.ShortDescription == "" || manifest.Interface.Category != "Developer Tools" {
		t.Fatalf("manifest interface = %#v", manifest.Interface)
	}
}

func assertDirectMCPMap(t *testing.T, content []byte, binary string) {
	t.Helper()
	type serverProjection struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
		CWD     string   `json:"cwd"`
		EnvVars []string `json:"env_vars"`
	}
	servers := map[string]serverProjection{}
	decodeClosedJSON(t, content, &servers)
	if _, wrapped := servers["mcpServers"]; wrapped {
		t.Fatal("MCP configuration uses the forbidden mcpServers wrapper")
	}
	if len(servers) != 1 {
		t.Fatalf("MCP servers = %#v", servers)
	}
	server, ok := servers["oaw_codex_bridge"]
	if !ok {
		t.Fatalf("MCP server oaw_codex_bridge missing: %#v", servers)
	}
	if server.Command != binary || !slices.Equal(server.Args, []string{"serve", "codex"}) || server.CWD != "." || len(server.EnvVars) != 0 {
		t.Fatalf("MCP server = %#v", server)
	}
}

func assertExactMatchers(t *testing.T, content []byte, command string) {
	t.Helper()
	type handlerProjection struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	type matcherProjection struct {
		Matcher string              `json:"matcher"`
		Hooks   []handlerProjection `json:"hooks"`
	}
	type hooksProjection struct {
		PreToolUse []matcherProjection `json:"PreToolUse"`
	}
	type documentProjection struct {
		Hooks hooksProjection `json:"hooks"`
	}
	var document documentProjection
	decodeClosedJSON(t, content, &document)

	want := []string{"mcp__oaw_codex_bridge__observe_profile"}
	got := make([]string, 0, len(document.Hooks.PreToolUse))
	for _, matcher := range document.Hooks.PreToolUse {
		got = append(got, matcher.Matcher)
		if len(matcher.Hooks) != 1 || matcher.Hooks[0].Type != "command" || matcher.Hooks[0].Command != command {
			t.Fatalf("matcher %q handlers = %#v", matcher.Matcher, matcher.Hooks)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("PreToolUse matchers = %q, want %q", got, want)
	}
}

func assertMarketplaceSource(t *testing.T, content []byte) {
	t.Helper()
	type sourceProjection struct {
		Source string `json:"source"`
		Path   string `json:"path"`
	}
	type pluginProjection struct {
		Name    string           `json:"name"`
		Source  sourceProjection `json:"source"`
		Version string           `json:"version"`
	}
	type marketplaceProjection struct {
		Name    string             `json:"name"`
		Plugins []pluginProjection `json:"plugins"`
	}
	var marketplace marketplaceProjection
	decodeClosedJSON(t, content, &marketplace)
	if marketplace.Name != testMarketplace || len(marketplace.Plugins) != 1 {
		t.Fatalf("marketplace = %#v", marketplace)
	}
	plugin := marketplace.Plugins[0]
	if plugin.Name != testPlugin || plugin.Source.Source != "local" || plugin.Source.Path != "./plugins/oaw-codex-assurance" || plugin.Version != testVersion {
		t.Fatalf("marketplace plugin = %#v", plugin)
	}
}

func decodeClosedJSON(t *testing.T, content []byte, target any) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, content)
	}
	if decoder.More() {
		t.Fatalf("multiple JSON values: %s", content)
	}
}
