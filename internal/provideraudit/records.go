package provideraudit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"regexp"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

const (
	ProviderSourceAuditSchemaV2 = "oaw.provider-source-audit/v2"
)

type BindingSource struct {
	ID          string   `json:"id"`
	ContentRoot string   `json:"content_root"`
	InstallRoot string   `json:"install_root"`
	TreeDigest  string   `json:"tree_digest"`
	Kind        string   `json:"kind"`
	References  []string `json:"references"`
}

type BindingCheckout struct {
	ID          string `json:"id"`
	ContentRoot string `json:"content_root"`
	InstallRoot string `json:"install_root"`
	Root        string `json:"root"`
}

type Checkout struct {
	ProviderID       string            `json:"provider_id"`
	DistributionID   string            `json:"distribution_id"`
	SourceURI        string            `json:"source_uri"`
	Revision         string            `json:"revision"`
	Root             string            `json:"root"`
	DistributionRoot string            `json:"distribution_root"`
	BindingRoots     []BindingCheckout `json:"binding_roots"`
}

type ProviderSource struct {
	ProviderID             string          `json:"provider_id"`
	SourceURI              string          `json:"source_uri"`
	Revision               string          `json:"revision"`
	DistributionID         string          `json:"distribution_id"`
	DistributionRoot       string          `json:"distribution_root"`
	DistributionTreeDigest string          `json:"distribution_tree_digest"`
	Bindings               []BindingSource `json:"bindings"`
	EvidenceRoots          []string        `json:"evidence_roots"`
}

type Manifest struct {
	SchemaVersion string           `json:"schema_version"`
	Providers     []ProviderSource `json:"providers"`
	Digest        string           `json:"digest"`
}

type bindingSpec struct {
	ID          string
	ContentRoot string
	InstallRoot string
	Kind        string
	References  []string
}

type providerSpec struct {
	ID               string
	SourceURI        string
	Revision         string
	DistributionID   string
	DistributionRoot string
	Bindings         []bindingSpec
	EvidenceRoots    []string
}

type sourceKey struct {
	ProviderID     string
	DistributionID string
}

var (
	recordDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	treeDigestPattern   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	revisionPattern     = regexp.MustCompile(`^[0-9a-f]{40}$`)
	localIDPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)
	qualifiedIDPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*/[a-z0-9][a-z0-9.-]*$`)
)

var lockedProviderSpecs = []providerSpec{
	{
		ID: "oaw/matt", SourceURI: "https://github.com/mattpocock/skills", Revision: "84fdeffd12f2ee307994d1eb6feb48173b6e0502", DistributionID: "matt-skills", DistributionRoot: ".",
		Bindings: mattSkillBindings([]skillRoot{
			{"grill-with-docs", "skills/engineering/grill-with-docs", "grill-with-docs"},
			{"grilling", "skills/productivity/grilling", "grilling"},
			{"domain-modeling", "skills/engineering/domain-modeling", "domain-modeling"},
			{"to-spec", "skills/engineering/to-spec", "to-spec"},
			{"to-tickets", "skills/engineering/to-tickets", "to-tickets"},
			{"implement", "skills/engineering/implement", "implement"},
			{"tdd", "skills/engineering/tdd", "tdd"},
			{"diagnosing-bugs", "skills/engineering/diagnosing-bugs", "diagnosing-bugs"},
			{"code-review", "skills/engineering/code-review", "code-review"},
		}),
		EvidenceRoots: []string{".agents/invocation.md", ".claude-plugin/plugin.json", "README.md"},
	},
	{
		ID: "oaw/superpowers", SourceURI: "https://github.com/obra/superpowers", Revision: "44c9b2d6e889982ac18c27d05a19fefe335194e1", DistributionID: "superpowers", DistributionRoot: ".",
		Bindings:      qualifiedSkillBindings(superpowersSkillRoots(), []string{"codex-upstream", "claude"}, true),
		EvidenceRoots: []string{"README.md", "RELEASE-NOTES.md", "skills/using-superpowers"},
	},
	{
		ID: "oaw/superpowers", SourceURI: "https://github.com/openai/plugins", Revision: "11c74d6ba24d3a6d48f54a194cd00ef3beea18f9", DistributionID: "superpowers-codex", DistributionRoot: "plugins/superpowers",
		Bindings:      qualifiedSkillBindings(superpowersSkillRoots(), []string{"codex"}, true),
		EvidenceRoots: []string{".codex-plugin/plugin.json", "README.md", "skills/using-superpowers"},
	},
	{
		ID: "oaw/ecc", SourceURI: "https://github.com/affaan-m/ECC", Revision: "2d46e80e0925c7be0907f18c1812311ac212a6c5", DistributionID: "ecc", DistributionRoot: ".",
		Bindings:      eccBindings(),
		EvidenceRoots: []string{".codex-plugin/plugin.json", "AGENTS.md", "hooks"},
	},
}

type skillRoot struct {
	Name        string
	ContentRoot string
	InstallRoot string
}

func mattSkillBindings(values []skillRoot) []bindingSpec {
	result := make([]bindingSpec, 0, len(values)*2)
	for _, host := range []string{"codex", "claude"} {
		for _, value := range values {
			installRoot := value.ContentRoot
			if host == "codex" {
				installRoot = "skills/" + value.InstallRoot
			}
			result = append(result, bindingSpec{
				ID: host + "-" + value.Name, ContentRoot: value.ContentRoot,
				InstallRoot: installRoot, Kind: "skill", References: []string{value.Name},
			})
		}
	}
	return result
}

func qualifiedSkillBindings(values []skillRoot, prefixes []string, superpowersNamespace bool) []bindingSpec {
	result := make([]bindingSpec, 0, len(values)*len(prefixes))
	for _, prefix := range prefixes {
		for _, value := range values {
			reference := value.Name
			if superpowersNamespace {
				reference = "superpowers:" + value.Name
			}
			result = append(result, bindingSpec{ID: prefix + "-" + value.Name, ContentRoot: value.ContentRoot, InstallRoot: value.InstallRoot, Kind: "skill", References: []string{reference}})
		}
	}
	return result
}

func superpowersSkillRoots() []skillRoot {
	return []skillRoot{
		{"brainstorming", "skills/brainstorming", "skills/brainstorming"},
		{"writing-plans", "skills/writing-plans", "skills/writing-plans"},
		{"using-git-worktrees", "skills/using-git-worktrees", "skills/using-git-worktrees"},
		{"subagent-driven-development", "skills/subagent-driven-development", "skills/subagent-driven-development"},
		{"executing-plans", "skills/executing-plans", "skills/executing-plans"},
		{"test-driven-development", "skills/test-driven-development", "skills/test-driven-development"},
		{"systematic-debugging", "skills/systematic-debugging", "skills/systematic-debugging"},
		{"requesting-code-review", "skills/requesting-code-review", "skills/requesting-code-review"},
		{"receiving-code-review", "skills/receiving-code-review", "skills/receiving-code-review"},
		{"verification-before-completion", "skills/verification-before-completion", "skills/verification-before-completion"},
		{"finishing-a-development-branch", "skills/finishing-a-development-branch", "skills/finishing-a-development-branch"},
	}
}

func eccBindings() []bindingSpec {
	skills := []skillRoot{
		{"intent-driven-development", "skills/intent-driven-development", "skills/intent-driven-development"},
		{"product-capability", "skills/product-capability", "skills/product-capability"},
		{"contract-first", "skills/contract-first", "skills/contract-first"},
		{"blueprint", "skills/blueprint", "skills/blueprint"},
		{"git-workflow", "skills/git-workflow", "skills/git-workflow"},
		{"tdd-workflow", "skills/tdd-workflow", "skills/tdd-workflow"},
		{"verification-loop", "skills/verification-loop", "skills/verification-loop"},
		{"security-review", "skills/security-review", "skills/security-review"},
		{"e2e-testing", "skills/e2e-testing", "skills/e2e-testing"},
	}
	result := make([]bindingSpec, 0, len(skills)*2+12)
	for _, prefix := range []string{"codex", "claude"} {
		for _, skill := range skills {
			result = append(result, bindingSpec{
				ID: prefix + "-" + skill.Name, ContentRoot: skill.ContentRoot, InstallRoot: skill.InstallRoot,
				Kind: "skill", References: []string{"ecc:" + skill.Name},
			})
		}
	}
	for _, name := range []string{"architect", "planner", "tdd-guide", "build-error-resolver", "code-reviewer", "security-reviewer", "e2e-runner"} {
		root := "agents/" + name + ".md"
		result = append(result, bindingSpec{ID: "claude-" + name, ContentRoot: root, InstallRoot: root, Kind: "agent", References: []string{name}})
	}
	for _, name := range []string{"explorer", "reviewer", "docs-researcher"} {
		root := ".codex/agents/" + name + ".toml"
		result = append(result, bindingSpec{ID: "codex-" + name, ContentRoot: root, InstallRoot: root, Kind: "role", References: []string{name}})
	}
	for _, value := range []struct{ name, reference string }{{"plan", "/plan"}, {"feature-dev", "/feature-dev"}} {
		root := "commands/" + value.name + ".md"
		result = append(result, bindingSpec{ID: "codex-" + value.name, ContentRoot: root, InstallRoot: root, Kind: "instruction", References: []string{value.reference}})
	}
	return result
}

func Decode(raw []byte) (Manifest, error) {
	var value Manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return Manifest{}, invalidAudit("decode manifest", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Manifest{}, invalidAudit("trailing JSON value", nil)
		}
		return Manifest{}, invalidAudit("decode trailing JSON", err)
	}
	if err := Validate(value); err != nil {
		return Manifest{}, err
	}
	return cloneManifest(value), nil
}

func Validate(value Manifest) error {
	if value.SchemaVersion != ProviderSourceAuditSchemaV2 || len(value.Providers) != len(lockedProviderSpecs) || !recordDigestPattern.MatchString(value.Digest) || value.ContentDigest() != value.Digest {
		return invalidAudit("manifest header or digest mismatch", nil)
	}
	for index, spec := range lockedProviderSpecs {
		provider := value.Providers[index]
		if provider.ProviderID != spec.ID || provider.SourceURI != spec.SourceURI || provider.Revision != spec.Revision || provider.DistributionID != spec.DistributionID || provider.DistributionRoot != spec.DistributionRoot || !revisionPattern.MatchString(provider.Revision) || !treeDigestPattern.MatchString(provider.DistributionTreeDigest) || len(provider.Bindings) != len(spec.Bindings) || !reflect.DeepEqual(provider.EvidenceRoots, spec.EvidenceRoots) {
			return invalidAudit("Provider source pin mismatch", nil)
		}
		seen := make(map[string]struct{}, len(provider.Bindings))
		for bindingIndex, bindingSpec := range spec.Bindings {
			binding := provider.Bindings[bindingIndex]
			if _, duplicate := seen[binding.ID]; duplicate {
				return invalidAudit("duplicate Binding ID", nil)
			}
			seen[binding.ID] = struct{}{}
			if binding.ID != bindingSpec.ID || binding.ContentRoot != bindingSpec.ContentRoot || binding.InstallRoot != bindingSpec.InstallRoot || binding.Kind != bindingSpec.Kind || !reflect.DeepEqual(binding.References, bindingSpec.References) || !treeDigestPattern.MatchString(binding.TreeDigest) || !validLocalID(binding.ID) || !cleanRelative(binding.ContentRoot, false) || !cleanRelative(binding.InstallRoot, false) {
				return invalidAudit("Binding source mapping mismatch", nil)
			}
		}
		if !qualifiedIDPattern.MatchString(provider.ProviderID) || !validLocalID(provider.DistributionID) || !cleanRelative(provider.DistributionRoot, true) {
			return invalidAudit("invalid Provider identity", nil)
		}
	}
	return nil
}

func (value Manifest) ContentDigest() string {
	value = cloneManifest(value)
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		return ""
	}
	return digest
}

func (value Manifest) Binding(providerID, bindingID string) (BindingSource, bool) {
	for _, provider := range value.Providers {
		if provider.ProviderID != providerID {
			continue
		}
		for _, binding := range provider.Bindings {
			if binding.ID == bindingID {
				binding.References = append([]string{}, binding.References...)
				return binding, true
			}
		}
	}
	return BindingSource{}, false
}

func LockedCheckouts(mattRoot, superpowersRoot, openaiPluginsRoot, eccRoot string) []Checkout {
	roots := map[sourceKey]string{
		{ProviderID: "oaw/matt", DistributionID: "matt-skills"}:              mattRoot,
		{ProviderID: "oaw/superpowers", DistributionID: "superpowers"}:       superpowersRoot,
		{ProviderID: "oaw/superpowers", DistributionID: "superpowers-codex"}: openaiPluginsRoot,
		{ProviderID: "oaw/ecc", DistributionID: "ecc"}:                       eccRoot,
	}
	result := make([]Checkout, len(lockedProviderSpecs))
	for index, spec := range lockedProviderSpecs {
		bindings := make([]BindingCheckout, len(spec.Bindings))
		for bindingIndex, binding := range spec.Bindings {
			bindings[bindingIndex] = BindingCheckout{ID: binding.ID, ContentRoot: binding.ContentRoot, InstallRoot: binding.InstallRoot, Root: binding.ContentRoot}
		}
		key := sourceKey{ProviderID: spec.ID, DistributionID: spec.DistributionID}
		result[index] = Checkout{ProviderID: spec.ID, DistributionID: spec.DistributionID, SourceURI: spec.SourceURI, Revision: spec.Revision, Root: roots[key], DistributionRoot: spec.DistributionRoot, BindingRoots: bindings}
	}
	return result
}

func LockedRevision(providerID, distributionID string) (string, bool) {
	for _, spec := range lockedProviderSpecs {
		if spec.ID == providerID && spec.DistributionID == distributionID {
			return spec.Revision, true
		}
	}
	return "", false
}

func cloneManifest(value Manifest) Manifest {
	providers := value.Providers
	value.Providers = make([]ProviderSource, len(providers))
	for index, provider := range providers {
		value.Providers[index] = provider
		value.Providers[index].EvidenceRoots = append([]string{}, provider.EvidenceRoots...)
		value.Providers[index].Bindings = make([]BindingSource, len(provider.Bindings))
		for bindingIndex, binding := range provider.Bindings {
			value.Providers[index].Bindings[bindingIndex] = binding
			value.Providers[index].Bindings[bindingIndex].References = append([]string{}, binding.References...)
		}
	}
	return value
}

func cleanRelative(value string, allowDot bool) bool {
	if value == "" || strings.Contains(value, "\\") || strings.TrimSpace(value) != value || strings.HasPrefix(value, "/") {
		return false
	}
	cleaned := path.Clean(value)
	if cleaned != value || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return false
	}
	return allowDot || cleaned != "."
}

func validLocalID(value string) bool { return localIDPattern.MatchString(value) }

func invalidAudit(detail string, err error) error {
	if err == nil {
		return fmt.Errorf("PROVIDER_AUDIT_INVALID: %s", detail)
	}
	return fmt.Errorf("PROVIDER_AUDIT_INVALID: %s: %w", detail, err)
}
