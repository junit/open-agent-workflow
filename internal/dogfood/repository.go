package dogfood

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

func inspectRepository(root string) (repositoryFingerprint, error) {
	canonicalRoot, err := canonicalExistingDirectory(root)
	if err != nil {
		return repositoryFingerprint{}, fmt.Errorf("repository root: %w", err)
	}
	gitRoot, err := runGit(canonicalRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return repositoryFingerprint{}, fmt.Errorf("inspect Git root: %w", err)
	}
	if filepath.Clean(strings.TrimSpace(string(gitRoot))) != canonicalRoot {
		return repositoryFingerprint{}, errors.New("requested path is not the Git repository root")
	}
	commitBytes, err := runGit(canonicalRoot, "rev-parse", "HEAD")
	if err != nil {
		return repositoryFingerprint{}, fmt.Errorf("inspect Git commit: %w", err)
	}
	commit := strings.TrimSpace(string(commitBytes))
	if !validCommit(commit) {
		return repositoryFingerprint{}, errors.New("Git HEAD is not a valid commit")
	}
	status, err := runGit(canonicalRoot, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return repositoryFingerprint{}, fmt.Errorf("inspect Git status: %w", err)
	}
	if strings.TrimSpace(string(status)) != "" {
		return repositoryFingerprint{}, errors.New("target repository is not clean")
	}
	skillPath := filepath.Join(canonicalRoot, "skills", "open-code-review", "SKILL.md")
	entry, err := os.Lstat(skillPath)
	if err != nil {
		return repositoryFingerprint{}, fmt.Errorf("required Skill: %w", err)
	}
	if entry.Mode()&os.ModeSymlink != 0 || !entry.Mode().IsRegular() {
		return repositoryFingerprint{}, errors.New("required Skill must be a regular non-symlinked file")
	}
	physicalSkill, err := filepath.EvalSymlinks(skillPath)
	if err != nil || filepath.Clean(physicalSkill) != skillPath {
		return repositoryFingerprint{}, errors.New("required Skill path is not canonical")
	}
	skill, err := os.ReadFile(skillPath)
	if err != nil {
		return repositoryFingerprint{}, fmt.Errorf("read required Skill: %w", err)
	}
	if len(skill) == 0 || len(skill) > maximumRecordBytes || !validUTF8(skill) {
		return repositoryFingerprint{}, errors.New("required Skill content is invalid")
	}
	skillTree, err := integrity.DigestTree(filepath.Dir(skillPath))
	if err != nil {
		return repositoryFingerprint{}, fmt.Errorf("digest required Skill tree: %w", err)
	}
	return repositoryFingerprint{
		Root: canonicalRoot, Commit: commit, StatusDigest: canonicaljson.DigestBytes(status),
		SkillPath: skillPath, SkillDigest: canonicaljson.DigestBytes(skill), SkillTreeDigest: skillTree.RootDigest,
	}, nil
}

func verifyRepository(expected repositoryFingerprint) (repositoryFingerprint, error) {
	if expected.Root == "" || expected.Commit == "" || !validDigest(expected.StatusDigest) || expected.SkillPath == "" ||
		!validDigest(expected.SkillDigest) || !validTreeDigest(expected.SkillTreeDigest) {
		return repositoryFingerprint{}, errors.New("repository fingerprint is invalid")
	}
	if _, err := os.Lstat(filepath.Join(expected.Root, ".oaw-production")); err == nil {
		return repositoryFingerprint{}, errors.New("production-marked repositories are not eligible for dogfood")
	} else if !errors.Is(err, os.ErrNotExist) {
		return repositoryFingerprint{}, fmt.Errorf("inspect production marker: %w", err)
	}
	actual, err := inspectRepository(expected.Root)
	if err != nil {
		return repositoryFingerprint{}, err
	}
	if actual != expected {
		return repositoryFingerprint{}, fmt.Errorf("repository fingerprint changed since pilot start")
	}
	return actual, nil
}

func runGit(root string, arguments ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	command.Env = os.Environ()
	output, err := command.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			message := strings.TrimSpace(string(exitErr.Stderr))
			if message != "" {
				return nil, fmt.Errorf("git %s: %s", strings.Join(arguments, " "), message)
			}
		}
		return nil, err
	}
	return output, nil
}

func validCommit(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validUTF8(value []byte) bool {
	if len(value) == 0 || !utf8.Valid(value) {
		return false
	}
	for _, character := range string(value) {
		if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return false
		}
	}
	return strings.TrimSpace(string(value)) != ""
}
