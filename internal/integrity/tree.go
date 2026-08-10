package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

type TreeEntry struct {
	Path   string `json:"path"`
	Mode   uint32 `json:"mode"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}

type TreeEvidence struct {
	RootDigest string      `json:"root_digest"`
	Entries    []TreeEntry `json:"entries"`
}

func SHA256Digest(value []byte) string {
	digest := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func DigestTree(root string) (TreeEvidence, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return TreeEvidence{}, invalidTree("resolve root", err)
	}
	absolute = filepath.Clean(absolute)
	rootInfo, err := os.Lstat(absolute)
	if err != nil {
		return TreeEvidence{}, invalidTree("inspect root", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return TreeEvidence{}, invalidTree("root must be a physical directory", nil)
	}

	entries := make([]TreeEntry, 0)
	err = filepath.WalkDir(absolute, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return invalidTree("walk tree", walkErr)
		}
		if err := ensureSameRoot(absolute, rootInfo); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return invalidTree("inspect entry", err)
		}
		if path == absolute {
			if !os.SameFile(rootInfo, info) || !info.IsDir() {
				return invalidTree("root identity changed", nil)
			}
			return nil
		}
		relative, err := filepath.Rel(absolute, path)
		if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return invalidTree("entry escapes root", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || entry.Type()&os.ModeSymlink != 0 {
			return invalidTree("symlink entry is forbidden", nil)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return invalidTree("non-regular entry is forbidden", nil)
		}
		value, err := digestRegularFile(path, info)
		if err != nil {
			return err
		}
		value.Path = filepath.ToSlash(relative)
		if value.Path == "" || strings.HasPrefix(value.Path, "/") || strings.Contains(value.Path, "\\") {
			return invalidTree("entry path is not canonical", nil)
		}
		entries = append(entries, value)
		return nil
	})
	if err != nil {
		return TreeEvidence{}, err
	}
	if err := ensureSameRoot(absolute, rootInfo); err != nil {
		return TreeEvidence{}, err
	}
	if len(entries) == 0 {
		return TreeEvidence{}, invalidTree("tree contains no regular files", nil)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	payload := struct {
		SchemaVersion string      `json:"schema_version"`
		Entries       []TreeEntry `json:"entries"`
	}{SchemaVersion: "oaw.binding-tree/v1", Entries: entries}
	encoded, err := canonicaljson.Marshal(payload)
	if err != nil {
		return TreeEvidence{}, invalidTree("encode canonical tree", err)
	}
	return TreeEvidence{RootDigest: SHA256Digest(encoded), Entries: append([]TreeEntry{}, entries...)}, nil
}

func digestRegularFile(path string, walked os.FileInfo) (TreeEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return TreeEntry{}, invalidTree("open regular file", err)
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil {
		return TreeEntry{}, invalidTree("stat regular file before read", err)
	}
	if !sameFileSnapshot(walked, before) || !before.Mode().IsRegular() {
		return TreeEntry{}, invalidTree("file changed before read", nil)
	}
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return TreeEntry{}, invalidTree("read regular file", err)
	}
	after, err := file.Stat()
	if err != nil {
		return TreeEntry{}, invalidTree("stat regular file after read", err)
	}
	current, err := os.Lstat(path)
	if err != nil {
		return TreeEntry{}, invalidTree("inspect regular file after read", err)
	}
	if !sameFileSnapshot(before, after) || !sameFileSnapshot(after, current) || size != after.Size() {
		return TreeEntry{}, invalidTree("file changed during read", nil)
	}
	return TreeEntry{
		Mode:   uint32(after.Mode().Perm() & 0o111),
		Size:   after.Size(),
		Digest: "sha256:" + hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

func ensureSameRoot(path string, expected os.FileInfo) error {
	current, err := os.Lstat(path)
	if err != nil {
		return invalidTree("inspect root identity", err)
	}
	if !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(expected, current) {
		return invalidTree("root identity changed", nil)
	}
	return nil
}

func sameFileSnapshot(left, right os.FileInfo) bool {
	return os.SameFile(left, right) && left.Size() == right.Size() && left.Mode() == right.Mode() && left.ModTime().Equal(right.ModTime())
}

func invalidTree(detail string, err error) error {
	if err == nil {
		return fmt.Errorf("BINDING_TREE_INVALID: %s", detail)
	}
	return fmt.Errorf("BINDING_TREE_INVALID: %s: %w", detail, err)
}
