package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

type DistributionTreeEntry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Mode   uint32 `json:"mode"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}

type DistributionTreeEvidence struct {
	RootDigest string                  `json:"root_digest"`
	Entries    []DistributionTreeEntry `json:"entries"`
}

func DigestDistributionTree(root string) (DistributionTreeEvidence, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return DistributionTreeEvidence{}, invalidDistributionTree("resolve root", err)
	}
	absolute = filepath.Clean(absolute)
	rootInfo, err := os.Lstat(absolute)
	if err != nil {
		return DistributionTreeEvidence{}, invalidDistributionTree("inspect root", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return DistributionTreeEvidence{}, invalidDistributionTree("root must be a physical directory", nil)
	}
	rooted, err := os.OpenRoot(absolute)
	if err != nil {
		return DistributionTreeEvidence{}, invalidDistributionTree("open root", err)
	}
	defer rooted.Close()
	openedRoot, err := rooted.Stat(".")
	if err != nil || !openedRoot.IsDir() || !os.SameFile(rootInfo, openedRoot) {
		return DistributionTreeEvidence{}, invalidDistributionTree("root identity changed", err)
	}
	entries := make([]DistributionTreeEntry, 0)
	if err := appendRootedDistributionEntries(rooted, "", &entries); err != nil {
		return DistributionTreeEvidence{}, err
	}
	if err := ensureSameRoot(absolute, rootInfo); err != nil {
		return DistributionTreeEvidence{}, invalidDistributionTree("root identity changed", err)
	}
	if len(entries) == 0 {
		return DistributionTreeEvidence{}, invalidDistributionTree("tree contains no files", nil)
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Path < entries[right].Path })
	payload := struct {
		SchemaVersion string                  `json:"schema_version"`
		Entries       []DistributionTreeEntry `json:"entries"`
	}{SchemaVersion: "oaw.distribution-tree/v1", Entries: entries}
	encoded, err := canonicaljson.Marshal(payload)
	if err != nil {
		return DistributionTreeEvidence{}, invalidDistributionTree("encode canonical tree", err)
	}
	return DistributionTreeEvidence{RootDigest: SHA256Digest(encoded), Entries: append([]DistributionTreeEntry{}, entries...)}, nil
}

func appendRootedDistributionEntries(root *os.Root, prefix string, result *[]DistributionTreeEntry) error {
	entries, snapshot, err := readRootedDirectory(root)
	if err != nil {
		return invalidDistributionTree("read tree directory", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		info, err := root.Lstat(name)
		if err != nil {
			return invalidDistributionTree("inspect tree entry", err)
		}
		relative := path.Join(prefix, name)
		if relative == "" || strings.HasPrefix(relative, "/") || strings.Contains(relative, "\\") {
			return invalidDistributionTree("entry path is not canonical", nil)
		}
		if info.Mode()&os.ModeSymlink != 0 || entry.Type()&os.ModeSymlink != 0 {
			if info.Mode()&os.ModeSymlink == 0 || entry.Type()&os.ModeSymlink == 0 {
				return invalidDistributionTree("symlink identity changed", nil)
			}
			target, err := root.Readlink(name)
			if err != nil || !containedDistributionSymlink(relative, target) {
				return invalidDistributionTree("symlink target escapes root", err)
			}
			current, err := root.Lstat(name)
			if err != nil || current.Mode()&os.ModeSymlink == 0 || !sameFileSnapshot(info, current) {
				return invalidDistributionTree("symlink identity changed", err)
			}
			*result = append(*result, DistributionTreeEntry{
				Path: relative, Kind: "symlink", Size: int64(len(target)), Digest: SHA256Digest([]byte(target)),
			})
			continue
		}
		if info.IsDir() {
			child, err := root.OpenRoot(name)
			if err != nil {
				return invalidDistributionTree("open tree directory", err)
			}
			opened, err := child.Stat(".")
			if err != nil || !opened.IsDir() || !os.SameFile(info, opened) {
				child.Close()
				return invalidDistributionTree("tree directory identity changed", err)
			}
			err = appendRootedDistributionEntries(child, relative, result)
			closeErr := child.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return invalidDistributionTree("close tree directory", closeErr)
			}
			if err := verifyRootedDistributionDirectory(root, name, info); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return invalidDistributionTree("entry is not a regular file", nil)
		}
		file, err := root.Open(name)
		if err != nil {
			return invalidDistributionTree("open regular file", err)
		}
		value, err := digestRootedDistributionFile(file, info)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return invalidDistributionTree("close regular file", closeErr)
		}
		current, err := root.Lstat(name)
		if err != nil || current.Mode()&os.ModeSymlink != 0 || !sameFileSnapshot(info, current) {
			return invalidDistributionTree("regular file identity changed", err)
		}
		*result = append(*result, DistributionTreeEntry{
			Path: relative, Kind: "file", Mode: value.Mode, Size: value.Size, Digest: value.Digest,
		})
	}
	if err := verifyRootedDistributionDirectorySnapshot(root, snapshot); err != nil {
		return err
	}
	return nil
}

func containedDistributionSymlink(linkPath, target string) bool {
	if target == "" || !utf8.ValidString(target) || strings.ContainsRune(target, 0) || path.IsAbs(target) ||
		path.Clean(target) != target || strings.Contains(target, "\\") {
		return false
	}
	resolved := path.Clean(path.Join(path.Dir(linkPath), target))
	return resolved != "." && resolved != ".." && !strings.HasPrefix(resolved, "../") && !path.IsAbs(resolved)
}

func verifyRootedDistributionDirectorySnapshot(root *os.Root, snapshot rootedDirectoryListing) error {
	if err := verifyRootedDirectorySnapshot(root, snapshot); err != nil {
		return invalidDistributionTree("directory entry set changed", err)
	}
	return nil
}

func verifyRootedDistributionDirectory(root *os.Root, name string, expected os.FileInfo) error {
	current, err := root.Lstat(name)
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(expected, current) {
		return invalidDistributionTree("tree directory identity changed", err)
	}
	return nil
}

func digestRootedDistributionFile(file *os.File, expected os.FileInfo) (TreeEntry, error) {
	before, err := file.Stat()
	if err != nil {
		return TreeEntry{}, invalidDistributionTree("stat regular file before read", err)
	}
	if !sameFileSnapshot(expected, before) || !before.Mode().IsRegular() {
		return TreeEntry{}, invalidDistributionTree("regular file changed before read", nil)
	}
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return TreeEntry{}, invalidDistributionTree("read regular file", err)
	}
	after, err := file.Stat()
	if err != nil {
		return TreeEntry{}, invalidDistributionTree("stat regular file after read", err)
	}
	if !sameFileSnapshot(before, after) || size != after.Size() {
		return TreeEntry{}, invalidDistributionTree("regular file changed during read", nil)
	}
	return TreeEntry{
		Mode: uint32(after.Mode().Perm() & 0o111), Size: after.Size(),
		Digest: "sha256:" + hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

func invalidDistributionTree(detail string, cause error) error {
	if cause != nil {
		return fmt.Errorf("DISTRIBUTION_TREE_INVALID: %s: %w", detail, cause)
	}
	return fmt.Errorf("DISTRIBUTION_TREE_INVALID: %s", detail)
}
