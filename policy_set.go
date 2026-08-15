package oaw

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// PolicyFile is one file relative to the root of an installed Canonical Policy Set.
type PolicyFile struct {
	Path    string
	Content []byte
}

// Responsibility is one stable engineering outcome named by the Policy.
type Responsibility string

const (
	ProblemFraming       Responsibility = "Problem framing"
	Specification        Responsibility = "Specification"
	DeliveryPlanning     Responsibility = "Delivery planning"
	WorkspacePreparation Responsibility = "Workspace preparation"
	ImplementationAndTDD Responsibility = "Implementation and TDD"
	ReviewAndRemediation Responsibility = "Review and remediation"
	FreshVerification    Responsibility = "Fresh verification"
	Closeout             Responsibility = "Closeout"
)

var policyResponsibilities = []Responsibility{
	ProblemFraming,
	Specification,
	DeliveryPlanning,
	WorkspacePreparation,
	ImplementationAndTDD,
	ReviewAndRemediation,
	FreshVerification,
	Closeout,
}

// PolicyProfile is the machine-readable minimum of a model-readable Profile.
// The original Markdown body remains normative.
type PolicyProfile struct {
	ID               string
	Name             string
	Responsibilities map[Responsibility]string
}

//go:embed policy/POLICY.md policy/cooperative-protocol.md policy/profiles/*.md policy/adapters/*.md
var canonicalPolicySetFS embed.FS

var embeddedCanonicalPolicySet = mustLoadCanonicalPolicySet()

var markdownLinkPattern = regexp.MustCompile(`\[[^\]\n]*\]\(([^)[:space:]]+)\)`)

// CanonicalPolicySet returns an independent snapshot of the embedded Policy Set.
func CanonicalPolicySet() []PolicyFile {
	return clonePolicyFiles(embeddedCanonicalPolicySet)
}

// PolicyResponsibilities returns the stable Responsibility vocabulary in order.
func PolicyResponsibilities() []Responsibility {
	return append([]Responsibility(nil), policyResponsibilities...)
}

// ParsePolicyProfile reads the minimal Profile metadata and Responsibility table.
// Partial Profiles are valid; Policy defaults cover omitted Responsibilities.
func ParsePolicyProfile(profilePath string, content []byte) (PolicyProfile, error) {
	profile, warnings, err := InspectPolicyProfile(profilePath, content)
	if err != nil {
		return PolicyProfile{}, err
	}
	if len(warnings) != 0 {
		return PolicyProfile{}, fmt.Errorf("Profile %q: %s", profilePath, warnings[0])
	}
	return profile, nil
}

// InspectPolicyProfile reads required identity metadata while keeping body
// diagnostics advisory. The Markdown body remains the normative Profile.
func InspectPolicyProfile(profilePath string, content []byte) (PolicyProfile, []string, error) {
	metadata, body, err := splitProfileDocument(content)
	if err != nil {
		return PolicyProfile{}, nil, fmt.Errorf("Profile %q: %w", profilePath, err)
	}
	metadata.ID = strings.TrimSpace(metadata.ID)
	metadata.Name = strings.TrimSpace(metadata.Name)
	if metadata.ID == "" {
		return PolicyProfile{}, nil, fmt.Errorf("Profile %q: missing required frontmatter field id", profilePath)
	}
	if metadata.Name == "" {
		return PolicyProfile{}, nil, fmt.Errorf("Profile %q: missing required frontmatter field name", profilePath)
	}
	if strings.IndexFunc(metadata.ID, unicode.IsControl) >= 0 {
		return PolicyProfile{}, nil, fmt.Errorf("Profile %q: frontmatter field id contains control characters", profilePath)
	}
	if strings.IndexFunc(metadata.Name, unicode.IsControl) >= 0 {
		return PolicyProfile{}, nil, fmt.Errorf("Profile %q: frontmatter field name contains control characters", profilePath)
	}

	responsibilities, warnings := inspectResponsibilities(body)
	return PolicyProfile{
		ID:               metadata.ID,
		Name:             metadata.Name,
		Responsibilities: responsibilities,
	}, warnings, nil
}

// ValidatePolicySet checks the static product invariants without treating Skill
// discovery or Host observations as Profile eligibility evidence.
func ValidatePolicySet(files []PolicyFile) error {
	byPath := make(map[string][]byte, len(files))
	for _, file := range files {
		clean := path.Clean(file.Path)
		if file.Path == "" || path.IsAbs(file.Path) || clean != file.Path || clean == "." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("Policy Set contains invalid file path %q", file.Path)
		}
		if len(bytes.TrimSpace(file.Content)) == 0 {
			return fmt.Errorf("Policy Set file %q is empty", file.Path)
		}
		if _, exists := byPath[file.Path]; exists {
			return fmt.Errorf("Policy Set contains duplicate file path %q", file.Path)
		}
		byPath[file.Path] = file.Content
	}

	requiredFiles := []string{
		"POLICY.md",
		"cooperative-protocol.md",
		"adapters/codex-policy.md",
		"profiles/SP-FULL.md",
		"profiles/MATT-FULL.md",
		"profiles/ECC-FULL.md",
		"profiles/MATT-SP-HYBRID.md",
	}
	for _, required := range requiredFiles {
		if _, exists := byPath[required]; !exists {
			return fmt.Errorf("Policy Set is missing required file %q", required)
		}
	}

	builtIns := map[string]string{
		"profiles/SP-FULL.md":        "SP-FULL",
		"profiles/MATT-FULL.md":      "MATT-FULL",
		"profiles/ECC-FULL.md":       "ECC-FULL",
		"profiles/MATT-SP-HYBRID.md": "MATT-SP-HYBRID",
	}
	profileIDs := make(map[string]string)
	for filePath, content := range byPath {
		if !strings.HasPrefix(filePath, "profiles/") || path.Ext(filePath) != ".md" {
			continue
		}
		profile, err := ParsePolicyProfile(filePath, content)
		if err != nil {
			return err
		}
		if previous, exists := profileIDs[profile.ID]; exists {
			return fmt.Errorf("Policy Set contains duplicate Profile ID %q in %q and %q", profile.ID, previous, filePath)
		}
		profileIDs[profile.ID] = filePath

		wantID, builtIn := builtIns[filePath]
		if !builtIn {
			continue
		}
		if profile.ID != wantID {
			return fmt.Errorf("built-in Profile %q has ID %q, want %q", filePath, profile.ID, wantID)
		}
		for _, responsibility := range policyResponsibilities {
			if strings.TrimSpace(profile.Responsibilities[responsibility]) == "" {
				return fmt.Errorf("built-in Profile %q is missing Responsibility %q", profile.ID, responsibility)
			}
		}
	}

	for filePath, content := range byPath {
		for _, match := range markdownLinkPattern.FindAllSubmatch(content, -1) {
			target := string(match[1])
			if strings.HasPrefix(target, "#") || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			target = strings.SplitN(target, "#", 2)[0]
			if target == "" {
				continue
			}
			resolved := path.Clean(path.Join(path.Dir(filePath), target))
			if resolved == "." || strings.HasPrefix(resolved, "../") {
				return fmt.Errorf("Policy Set file %q has escaping local reference %q", filePath, target)
			}
			if _, exists := byPath[resolved]; !exists {
				return fmt.Errorf("Policy Set file %q has missing local reference %q", filePath, target)
			}
		}
	}
	return nil
}

type profileMetadata struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

func splitProfileDocument(content []byte) (profileMetadata, string, error) {
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return profileMetadata{}, "", fmt.Errorf("missing YAML frontmatter")
	}
	closing := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			closing = index
			break
		}
	}
	if closing == -1 {
		return profileMetadata{}, "", fmt.Errorf("unterminated YAML frontmatter")
	}
	var metadata profileMetadata
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:closing], "\n")), &metadata); err != nil {
		return profileMetadata{}, "", fmt.Errorf("invalid YAML frontmatter: %w", err)
	}
	return metadata, strings.Join(lines[closing+1:], "\n"), nil
}

func parseResponsibilities(body string) (map[Responsibility]string, error) {
	result, warnings := inspectResponsibilities(body)
	if len(warnings) != 0 {
		return nil, fmt.Errorf("%s", warnings[0])
	}
	return result, nil
}

func inspectResponsibilities(body string) (map[Responsibility]string, []string) {
	result := make(map[Responsibility]string)
	var warnings []string
	inSection := false
	for _, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.EqualFold(line, "## Responsibilities") {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "## ") {
			break
		}
		if !inSection || !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 2 {
			continue
		}
		label := strings.TrimSpace(cells[0])
		if strings.EqualFold(label, "Responsibility") || isMarkdownTableSeparator(label) {
			continue
		}
		responsibility, ok := canonicalResponsibility(label)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("unknown Responsibility %q", label))
			continue
		}
		if _, duplicate := result[responsibility]; duplicate {
			warnings = append(warnings, fmt.Sprintf("duplicate Responsibility %q", responsibility))
			continue
		}
		action := strings.TrimSpace(cells[1])
		if action == "" {
			warnings = append(warnings, fmt.Sprintf("Responsibility %q has no Skill or action", responsibility))
			continue
		}
		result[responsibility] = action
	}
	return result, warnings
}

func canonicalResponsibility(value string) (Responsibility, bool) {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(value, "-", " ")), " "))
	for _, responsibility := range policyResponsibilities {
		candidate := strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(string(responsibility), "-", " ")), " "))
		if normalized == candidate {
			return responsibility, true
		}
	}
	return "", false
}

func isMarkdownTableSeparator(value string) bool {
	trimmed := strings.Trim(value, " :-")
	return trimmed == "" && strings.Contains(value, "-")
}

func mustLoadCanonicalPolicySet() []PolicyFile {
	var files []PolicyFile
	err := fs.WalkDir(canonicalPolicySetFS, "policy", func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := canonicalPolicySetFS.ReadFile(filePath)
		if err != nil {
			return err
		}
		files = append(files, PolicyFile{
			Path:    strings.TrimPrefix(filePath, "policy/"),
			Content: content,
		})
		return nil
	})
	if err != nil {
		panic(fmt.Sprintf("load embedded Canonical Policy Set: %v", err))
	}
	sort.Slice(files, func(left, right int) bool { return files[left].Path < files[right].Path })
	if err := ValidatePolicySet(files); err != nil {
		panic(fmt.Sprintf("validate embedded Canonical Policy Set: %v", err))
	}
	return files
}

func clonePolicyFiles(files []PolicyFile) []PolicyFile {
	result := make([]PolicyFile, len(files))
	for index, file := range files {
		result[index] = PolicyFile{Path: file.Path, Content: bytes.Clone(file.Content)}
	}
	return result
}
