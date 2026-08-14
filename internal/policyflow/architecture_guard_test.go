package policyflow_test

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
)

func TestPolicyFlowProductionDoesNotImportMachineAuthorityPackages(t *testing.T) {
	t.Parallel()

	packageDir := policyFlowPackageDir(t)
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		t.Fatalf("read policyflow package: %v", err)
	}

	forbiddenRoots := []string{
		"github.com/wifibaby4u/open-agent-workflow/internal/catalog",
		"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge",
		"github.com/wifibaby4u/open-agent-workflow/internal/coordinator",
		"github.com/wifibaby4u/open-agent-workflow/internal/core",
		"github.com/wifibaby4u/open-agent-workflow/internal/discovery",
		"github.com/wifibaby4u/open-agent-workflow/internal/integrity",
		"github.com/wifibaby4u/open-agent-workflow/internal/registry",
	}

	fileSet := token.NewFileSet()
	var violations []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		filename := filepath.Join(packageDir, name)
		parsed, err := parser.ParseFile(fileSet, filename, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse production file %s: %v", name, err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			importSpec, ok := node.(*ast.ImportSpec)
			if !ok {
				return true
			}
			importPath, err := strconv.Unquote(importSpec.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", name, err)
			}
			if forbiddenImportRoot(importPath, forbiddenRoots) == "" {
				return false
			}
			position := fileSet.Position(importSpec.Pos())
			violations = append(violations, fmt.Sprintf("%s:%d imports %q", name, position.Line, importPath))
			return false
		})
	}

	sort.Strings(violations)
	if len(violations) != 0 {
		t.Fatalf("Policy projection imports machine-authority packages:\n%s", strings.Join(violations, "\n"))
	}
}

func TestPolicyFlowDependencyGraphExcludesMachineAuthorityPackages(t *testing.T) {
	t.Parallel()

	command := exec.Command("go", "list", "-deps", ".")
	command.Dir = policyFlowPackageDir(t)
	raw, err := command.Output()
	if err != nil {
		t.Fatalf("list policyflow dependencies: %v", err)
	}
	forbidden := []string{
		"/internal/catalog",
		"/internal/builtin",
		"/internal/codexbridge",
		"/internal/coordinator",
		"/internal/core",
		"/internal/discovery",
		"/internal/integrity",
		"/internal/provideraudit",
		"/internal/registry",
	}
	for _, dependency := range strings.Fields(string(raw)) {
		for _, fragment := range forbidden {
			if strings.Contains(dependency, fragment) {
				t.Errorf("Policy projection transitively depends on machine-authority package %q", dependency)
			}
		}
	}
}

func TestPolicyFlowSerializedOutputsContainNoMachineAuthority(t *testing.T) {
	profileIDs := []policyflow.ProfileID{
		policyflow.ProfileSPFull,
		policyflow.ProfileMattFull,
		policyflow.ProfileECCFull,
		policyflow.ProfileMattSPHybrid,
	}

	for _, profileID := range profileIDs {
		t.Run(string(profileID), func(t *testing.T) {
			module := policyflow.New()
			inventory := inventoryCoveringPolicyOffer(t, module)
			offer, err := module.Offer(inventory)
			if err != nil {
				t.Fatalf("offer: %v", err)
			}
			assertNoMachineAuthorityInJSON(t, "offer", offer)

			progress, err := module.Start(inventory, policyflow.Selection{
				OfferRef: offer.Ref,
				Profile:  profileID,
			})
			if err != nil {
				t.Fatalf("start %s: %v", profileID, err)
			}

			for step := 0; step < 128; step++ {
				assertNoMachineAuthorityInJSON(t, fmt.Sprintf("progress[%d]", step), progress)
				switch next := progress.Next.(type) {
				case policyflow.InvokeSkill:
					progress, err = module.Apply(inventory, successfulSkillEvent(next.WorkRef, next.Review))
				case policyflow.AwaitUserSkill:
					progress, err = module.Apply(inventory, successfulSkillEvent(next.WorkRef, next.Review))
				case policyflow.HostAction:
					progress, err = module.Apply(inventory, successfulHostActionEvent(next.WorkRef, next.Review))
				case policyflow.UserGate:
					progress, err = module.Apply(inventory, policyflow.UserGateApproved{GateRef: next.GateRef})
				case policyflow.HostGate:
					progress, err = module.Apply(inventory, policyflow.HostGateSatisfied{GateRef: next.GateRef})
				case policyflow.Done:
					return
				default:
					t.Fatalf("unexpected next work %T", progress.Next)
				}
				if err != nil {
					t.Fatalf("apply progress[%d]: %v", step, err)
				}
			}
			t.Fatal("Policy workflow did not terminate within 128 outputs")
		})
	}
}

func TestReservedMachineAuthorityTermUsesIdentifierBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  string
	}{
		{value: "ProviderID", want: "provider_id"},
		{value: "provider_id", want: "provider_id"},
		{value: "binding-id", want: "binding_id"},
		{value: "LifecycleBundle", want: "bundle"},
		{value: "capabilityGrant", want: "grant"},
		{value: "resource_lease", want: "lease"},
		{value: "HostReceipt", want: "receipt"},
		{value: "machineRevision", want: "machine_revision"},
		{value: "release", want: ""},
		{value: "leased", want: ""},
		{value: "receiptive", want: ""},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			if got := reservedMachineAuthorityTerm(test.value); got != test.want {
				t.Fatalf("reservedMachineAuthorityTerm(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestForbiddenImportRootMatchesPackageBoundaries(t *testing.T) {
	t.Parallel()

	root := "github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	tests := []struct {
		importPath string
		want       string
	}{
		{importPath: root, want: root},
		{importPath: root + "/appserver", want: root},
		{importPath: root + "test", want: ""},
		{importPath: "github.com/example/codexbridge", want: ""},
	}
	for _, test := range tests {
		t.Run(test.importPath, func(t *testing.T) {
			if got := forbiddenImportRoot(test.importPath, []string{root}); got != test.want {
				t.Fatalf("forbiddenImportRoot(%q) = %q, want %q", test.importPath, got, test.want)
			}
		})
	}
}

func inventoryCoveringPolicyOffer(t *testing.T, module *policyflow.Module) policyflow.RouteInventory {
	t.Helper()
	seed := completeInventory()
	offer, err := module.Offer(seed)
	if err != nil {
		t.Fatalf("seed offer: %v", err)
	}

	byName := make(map[string]policyflow.Route, len(seed))
	for _, route := range seed {
		byName[route.Name] = route
	}
	for _, profile := range offer.Profiles {
		for _, status := range profile.Routes {
			if _, exists := byName[status.Name]; exists {
				continue
			}
			mode := policyflow.HostVisible
			if status.Kind == policyflow.RouteHostAction {
				mode = policyflow.HostControlled
			}
			byName[status.Name] = policyflow.Route{Name: status.Name, Mode: mode}
		}
	}

	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	inventory := make(policyflow.RouteInventory, 0, len(names))
	for _, name := range names {
		inventory = append(inventory, byName[name])
	}
	return inventory
}

func policyFlowPackageDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate policyflow architecture guard")
	}
	return filepath.Dir(filename)
}

func forbiddenImportRoot(importPath string, roots []string) string {
	for _, root := range roots {
		if importPath == root || strings.HasPrefix(importPath, root+"/") {
			return root
		}
	}
	return ""
}

func assertNoMachineAuthorityInJSON(t *testing.T, label string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", label, err)
	}

	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("decode %s: %v", label, err)
	}
	var violations []string
	collectMachineAuthorityTerms(document, "$", &violations)
	if len(violations) != 0 {
		t.Errorf("%s exposes reserved machine-authority terms:\n%s\nJSON: %s", label, strings.Join(violations, "\n"), raw)
	}
}

func collectMachineAuthorityTerms(value any, path string, violations *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if reserved := reservedMachineAuthorityTerm(key); reserved != "" {
				*violations = append(*violations, fmt.Sprintf("%s key %q contains %q", path, key, reserved))
			}
			collectMachineAuthorityTerms(typed[key], path+"."+key, violations)
		}
	case []any:
		for index, item := range typed {
			collectMachineAuthorityTerms(item, fmt.Sprintf("%s[%d]", path, index), violations)
		}
	case string:
		if reserved := reservedMachineAuthorityTerm(typed); reserved != "" {
			*violations = append(*violations, fmt.Sprintf("%s value %q contains %q", path, typed, reserved))
		}
	}
}

func reservedMachineAuthorityTerm(value string) string {
	tokens := authorityTokens(value)
	for index, token := range tokens {
		switch token {
		case "bundle", "grant", "lease", "receipt":
			return token
		}
		if index+1 >= len(tokens) {
			continue
		}
		pair := token + "_" + tokens[index+1]
		switch pair {
		case "provider_id", "binding_id", "machine_revision":
			return pair
		}
	}
	return ""
}

func authorityTokens(value string) []string {
	runes := []rune(value)
	var tokens []string
	start := -1
	flush := func(end int) {
		if start >= 0 && start < end {
			tokens = append(tokens, strings.ToLower(string(runes[start:end])))
		}
		start = -1
	}
	for index, character := range runes {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			flush(index)
			continue
		}
		if start < 0 {
			start = index
			continue
		}
		previous := runes[index-1]
		nextIsLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
		if unicode.IsUpper(character) && (unicode.IsLower(previous) || unicode.IsDigit(previous) || unicode.IsUpper(previous) && nextIsLower) {
			flush(index)
			start = index
		}
	}
	flush(len(runes))
	return tokens
}
