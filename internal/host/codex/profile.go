package codex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

// BindingResolver maps the immutable Runtime request to the physical Host
// evidence selected during dynamic Provider discovery.
type BindingResolver func(host.DispatchRequest) (host.BindingObservation, error)

type executionProfile struct {
	root        string
	home        string
	workspace   string
	projectRoot string
	codexHome   string
	bindingPath string
	stagedPath  string
}

func resolveCodexHome(value string) string {
	if value != "" {
		return filepath.Clean(value)
	}
	if value = os.Getenv("CODEX_HOME"); value != "" {
		return filepath.Clean(value)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".codex")
}

func (runner *Runner) prepareExecutionProfile(request host.DispatchRequest) (*executionProfile, error) {
	if runner.executionRoot == "" {
		return nil, nil
	}
	if runner.bindingResolver == nil {
		return nil, errors.New("CODEX_BINDING_EVIDENCE_REQUIRED: execution profile requires a verified Binding resolver")
	}
	if runner.codexHome == "" || !filepath.IsAbs(runner.codexHome) || filepath.Clean(runner.codexHome) != runner.codexHome {
		return nil, errors.New("CODEX_EXECUTION_PROFILE_INVALID: Codex home must be a clean absolute path")
	}
	if runner.projectRoot != "" && (!filepath.IsAbs(runner.projectRoot) || filepath.Clean(runner.projectRoot) != runner.projectRoot) {
		return nil, errors.New("CODEX_EXECUTION_PROFILE_INVALID: project root must be a clean absolute path")
	}
	root := filepath.Clean(runner.executionRoot)
	if !filepath.IsAbs(root) || root != runner.executionRoot {
		return nil, errors.New("CODEX_EXECUTION_PROFILE_INVALID: execution root must be a clean absolute path")
	}
	observation, err := runner.bindingResolver(request)
	if err != nil {
		return nil, err
	}
	if observation.Binding.Kind != "skill" {
		return nil, fmt.Errorf("CODEX_BINDING_KIND_UNSUPPORTED: isolated Codex execution cannot map %q Bindings", observation.Binding.Kind)
	}
	physical, evidence, err := verifyBindingEvidence(observation)
	if err != nil {
		return nil, err
	}
	executionRoot := filepath.Join(root, "codex-execution")
	profileRoot := filepath.Join(executionRoot, request.InvocationID)
	relativeProfile, err := filepath.Rel(executionRoot, profileRoot)
	if err != nil || relativeProfile == "." || filepath.Dir(relativeProfile) != "." {
		return nil, errors.New("CODEX_EXECUTION_PROFILE_INVALID: Invocation ID is not a safe path component")
	}
	if err := ensureExecutionRoot(root); err != nil {
		return nil, fmt.Errorf("CODEX_EXECUTION_PROFILE_INVALID: %w", err)
	}
	if err := ensurePrivateDirectory(executionRoot); err != nil {
		return nil, fmt.Errorf("CODEX_EXECUTION_PROFILE_INVALID: %w", err)
	}
	if err := ensurePrivateDirectory(profileRoot); err != nil {
		return nil, fmt.Errorf("CODEX_EXECUTION_PROFILE_INVALID: %w", err)
	}
	home := filepath.Join(profileRoot, "home")
	workspace := filepath.Join(profileRoot, "workspace")
	if err := ensurePrivateDirectory(home); err != nil {
		return nil, fmt.Errorf("CODEX_EXECUTION_PROFILE_INVALID: %w", err)
	}
	if err := ensurePrivateDirectory(workspace); err != nil {
		return nil, fmt.Errorf("CODEX_EXECUTION_PROFILE_INVALID: %w", err)
	}
	profile := &executionProfile{root: profileRoot, home: home, workspace: workspace, projectRoot: runner.projectRoot, codexHome: runner.codexHome, bindingPath: physical}
	stage := filepath.Join(home, ".agents", "skills", "bound-skill")
	if err := stageSkill(stage, evidence, observation.Digest); err != nil {
		return nil, fmt.Errorf("CODEX_EXECUTION_PROFILE_INVALID: %w", err)
	}
	profile.stagedPath = filepath.Join(stage, "SKILL.md")
	return profile, nil
}

func verifyBindingEvidence(observation host.BindingObservation) (string, []byte, error) {
	if observation.Source == "native-probe" || observation.EvidenceReference == "" || !filepath.IsAbs(observation.EvidenceReference) || filepath.Clean(observation.EvidenceReference) != observation.EvidenceReference {
		return "", nil, errors.New("CODEX_BINDING_EVIDENCE_REQUIRED: Binding evidence is not a physical Host file")
	}
	physical, err := filepath.EvalSymlinks(observation.EvidenceReference)
	if err != nil {
		return "", nil, fmt.Errorf("CODEX_BINDING_EVIDENCE_REQUIRED: resolve Binding evidence: %w", err)
	}
	data, err := os.ReadFile(physical)
	if err != nil {
		return "", nil, fmt.Errorf("CODEX_BINDING_EVIDENCE_REQUIRED: read Binding evidence: %w", err)
	}
	if canonicaljson.DigestBytes(data) != observation.Digest {
		return "", nil, errors.New("CODEX_BINDING_EVIDENCE_CHANGED: Binding evidence digest changed")
	}
	if !isRegularFile(physical) {
		return "", nil, errors.New("CODEX_BINDING_EVIDENCE_REQUIRED: Binding evidence is not a regular file")
	}
	return filepath.Clean(physical), data, nil
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func ensureExecutionRoot(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("execution root path is invalid: %q", path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.Mkdir(path, 0o700)
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("execution root is not a directory: %q", path)
	}
	return nil
}

func ensurePrivateDirectory(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("directory path is invalid: %q", path)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("directory is not a private directory: %q", path)
		}
		if info.Mode().Perm() != 0o700 {
			return fmt.Errorf("directory mode is %04o, not 0700: %q", info.Mode().Perm(), path)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		return err
	}
	return nil
}

func stageSkill(stage string, evidence []byte, digest string) error {
	if stage == "" || len(evidence) == 0 || canonicaljson.DigestBytes(evidence) != digest {
		return errors.New("verified skill evidence is invalid")
	}
	if err := ensurePrivateDirectoryTree(stage); err != nil {
		return err
	}
	return ensurePrivateFile(filepath.Join(stage, "SKILL.md"), evidence, digest)
}

func ensurePrivateFile(path string, data []byte, digest string) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			return errors.New("staged skill evidence is not a private regular file")
		}
		existing, readErr := os.ReadFile(path)
		if readErr != nil || canonicaljson.DigestBytes(existing) != digest {
			return errors.New("staged skill evidence does not match the verified digest")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return ensurePrivateFile(path, data, digest)
	}
	if err != nil {
		return err
	}
	written, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || written != len(data) || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
		return io.ErrShortWrite
	}
	return nil
}

func ensurePrivateDirectoryTree(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("directory path is invalid: %q", path)
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		parent := filepath.Dir(path)
		if parent == path {
			return fmt.Errorf("directory parent is missing: %q", path)
		}
		if err := ensurePrivateDirectoryTree(parent); err != nil {
			return err
		}
	}
	return ensurePrivateDirectory(path)
}

func bindingProfilePrompt(profile *executionProfile) string {
	if profile == nil {
		return ""
	}
	path := profile.bindingPath
	if profile.stagedPath != "" {
		path = profile.stagedPath
	}
	return "Binding evidence: " + path + "\n" +
		"Load and follow only this exact bound capability. Do not invoke another skill, plugin, hook, or MCP server.\n"
}

func runCommandProfile(ctx context.Context, command string, args []string, profile *executionProfile, maximum int64) ([]byte, []byte, error) {
	if profile == nil {
		return nil, nil, errors.New("execution profile is required")
	}
	stdout := &limitedBuffer{maximum: maximum}
	stderr := &limitedBuffer{maximum: maximum}
	process := exec.CommandContext(ctx, command, args...)
	process.Dir = profile.workspace
	process.Env = profileEnvironment(os.Environ(), profile.home, profile.codexHome)
	process.Stdout = stdout
	process.Stderr = stderr
	err := process.Run()
	if stdout.exceeded || stderr.exceeded {
		return stdout.Bytes(), stderr.Bytes(), errors.New("Codex process output exceeded limit")
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

func profileEnvironment(base []string, home, codexHome string) []string {
	result := make([]string, 0, len(base)+2)
	for _, entry := range base {
		key, _, found := strings.Cut(entry, "=")
		if found && (strings.EqualFold(key, "HOME") || strings.EqualFold(key, "CODEX_HOME")) {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "HOME="+home, "CODEX_HOME="+codexHome)
}
