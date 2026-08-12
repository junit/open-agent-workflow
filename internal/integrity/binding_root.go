package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

type bindingRootSnapshot struct {
	components []bindingRootComponent
	directory  *os.Root
	info       os.FileInfo
	name       string
	parent     *os.Root
	roots      []*os.Root
}

type bindingRootComponent struct {
	info   os.FileInfo
	name   string
	parent *os.Root
}

type rootedDirectoryListing struct {
	names []string
}

func readRootedDirectory(root *os.Root) ([]os.DirEntry, rootedDirectoryListing, error) {
	directory, err := root.Open(".")
	if err != nil {
		return nil, rootedDirectoryListing{}, err
	}
	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil {
		return nil, rootedDirectoryListing{}, readErr
	}
	if closeErr != nil {
		return nil, rootedDirectoryListing{}, closeErr
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name() < entries[right].Name() })
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return entries, rootedDirectoryListing{names: names}, nil
}

func rootedDirectorySnapshot(root *os.Root) (rootedDirectoryListing, error) {
	_, snapshot, err := readRootedDirectory(root)
	return snapshot, err
}

func verifyRootedDirectorySnapshot(root *os.Root, snapshot rootedDirectoryListing) error {
	current, err := rootedDirectorySnapshot(root)
	if err != nil {
		return err
	}
	if !slices.Equal(current.names, snapshot.names) {
		return fmt.Errorf("directory entry set changed")
	}
	return nil
}

func verifyRootedBindingDirectorySnapshot(root *os.Root, snapshot rootedDirectoryListing) error {
	if err := verifyRootedDirectorySnapshot(root, snapshot); err != nil {
		return invalidBindingRoot("directory entry set changed", err)
	}
	return nil
}

func DigestBindingRoot(installationRoot, installRoot, contentRoot string) (TreeEvidence, error) {
	if !canonicalBindingRoot(installRoot) || !canonicalBindingRoot(contentRoot) {
		return TreeEvidence{}, invalidBindingRoot("root is not canonical", nil)
	}
	installationInfo, err := os.Lstat(installationRoot)
	if err != nil {
		return TreeEvidence{}, invalidBindingRoot("inspect installation root", err)
	}
	if installationInfo.Mode()&os.ModeSymlink != 0 || !installationInfo.IsDir() {
		return TreeEvidence{}, invalidBindingRoot("installation root must be a physical directory", nil)
	}
	root, err := os.OpenRoot(installationRoot)
	if err != nil {
		return TreeEvidence{}, invalidBindingRoot("open installation root", err)
	}
	defer root.Close()
	openedInstallation, err := root.Stat(".")
	if err != nil || !openedInstallation.IsDir() || !os.SameFile(installationInfo, openedInstallation) {
		return TreeEvidence{}, invalidBindingRoot("installation root identity changed", err)
	}
	snapshot, err := physicalBindingRoot(root, installRoot)
	if err != nil {
		return TreeEvidence{}, err
	}
	defer snapshot.close()
	rootInfo := snapshot.info

	var evidence TreeEvidence
	if rootInfo.IsDir() {
		evidence, err = digestRootedBindingTree(snapshot.directory)
		if err != nil {
			return TreeEvidence{}, err
		}
	} else {
		file, err := snapshot.parent.Open(snapshot.name)
		if err != nil {
			return TreeEvidence{}, invalidBindingRoot("open regular-file root", err)
		}
		entry, err := digestRootedBindingFile(file, rootInfo)
		closeErr := file.Close()
		if err != nil {
			return TreeEvidence{}, err
		}
		if closeErr != nil {
			return TreeEvidence{}, invalidBindingRoot("close regular-file root", closeErr)
		}
		entry.Path = contentRoot
		evidence, err = bindingFileEvidence(contentRoot, entry)
		if err != nil {
			return TreeEvidence{}, err
		}
	}
	if err := verifyPhysicalBindingRoot(root, snapshot); err != nil {
		return TreeEvidence{}, err
	}
	currentInstallation, err := os.Lstat(installationRoot)
	if err != nil || currentInstallation.Mode()&os.ModeSymlink != 0 || !currentInstallation.IsDir() || !os.SameFile(installationInfo, currentInstallation) {
		return TreeEvidence{}, invalidBindingRoot("installation root identity changed", err)
	}
	return evidence, nil
}

func physicalBindingRoot(root *os.Root, relative string) (bindingRootSnapshot, error) {
	snapshot := bindingRootSnapshot{roots: make([]*os.Root, 0, strings.Count(relative, "/")+1)}
	var info os.FileInfo
	currentRoot := root
	segments := strings.Split(relative, "/")
	for index, segment := range segments {
		value, err := currentRoot.Lstat(segment)
		if err != nil {
			snapshot.close()
			return bindingRootSnapshot{}, invalidBindingRoot("inspect root component", err)
		}
		if value.Mode()&os.ModeSymlink != 0 {
			snapshot.close()
			return bindingRootSnapshot{}, invalidBindingRoot("root contains a symlink", nil)
		}
		if index < len(segments)-1 && !value.IsDir() {
			snapshot.close()
			return bindingRootSnapshot{}, invalidBindingRoot("root ancestor is not a directory", nil)
		}
		component := bindingRootComponent{parent: currentRoot, name: segment, info: value}
		snapshot.components = append(snapshot.components, component)
		info = value
		if !value.IsDir() {
			continue
		}
		child, err := currentRoot.OpenRoot(segment)
		if err != nil {
			snapshot.close()
			return bindingRootSnapshot{}, invalidBindingRoot("open root component", err)
		}
		opened, err := child.Stat(".")
		if err != nil || !opened.IsDir() || !os.SameFile(value, opened) {
			child.Close()
			snapshot.close()
			return bindingRootSnapshot{}, invalidBindingRoot("root component identity changed", err)
		}
		current, err := currentRoot.Lstat(segment)
		if err != nil || current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(value, current) {
			child.Close()
			snapshot.close()
			return bindingRootSnapshot{}, invalidBindingRoot("root component identity changed", err)
		}
		snapshot.roots = append(snapshot.roots, child)
		currentRoot = child
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		snapshot.close()
		return bindingRootSnapshot{}, invalidBindingRoot("root must be a regular file or directory", nil)
	}
	last := snapshot.components[len(snapshot.components)-1]
	snapshot.info = info
	snapshot.parent = last.parent
	snapshot.name = last.name
	if info.IsDir() {
		snapshot.directory = snapshot.roots[len(snapshot.roots)-1]
	}
	return snapshot, nil
}

func verifyPhysicalBindingRoot(root *os.Root, snapshot bindingRootSnapshot) error {
	for index, component := range snapshot.components {
		current, err := component.parent.Lstat(component.name)
		if err != nil || current.Mode()&os.ModeSymlink != 0 || current.IsDir() != component.info.IsDir() || !os.SameFile(component.info, current) {
			return invalidBindingRoot("root identity changed", err)
		}
		if index == len(snapshot.components)-1 && component.info.Mode().IsRegular() && !sameFileSnapshot(component.info, current) {
			return invalidBindingRoot("root identity changed", nil)
		}
	}
	return nil
}

func (value *bindingRootSnapshot) close() {
	for index := len(value.roots) - 1; index >= 0; index-- {
		value.roots[index].Close()
	}
	value.roots = nil
}

func digestRootedBindingTree(root *os.Root) (TreeEvidence, error) {
	entries := make([]TreeEntry, 0)
	if err := appendRootedBindingEntries(root, "", &entries); err != nil {
		return TreeEvidence{}, err
	}
	if len(entries) == 0 {
		return TreeEvidence{}, invalidBindingRoot("tree contains no regular files", nil)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	payload := struct {
		SchemaVersion string      `json:"schema_version"`
		Entries       []TreeEntry `json:"entries"`
	}{SchemaVersion: "oaw.binding-tree/v1", Entries: entries}
	encoded, err := canonicaljson.Marshal(payload)
	if err != nil {
		return TreeEvidence{}, invalidBindingRoot("encode canonical tree", err)
	}
	return TreeEvidence{RootDigest: SHA256Digest(encoded), Entries: append([]TreeEntry{}, entries...)}, nil
}

func appendRootedBindingEntries(root *os.Root, prefix string, result *[]TreeEntry) error {
	entries, snapshot, err := readRootedDirectory(root)
	if err != nil {
		return invalidBindingRoot("read tree directory", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		info, err := root.Lstat(name)
		if err != nil {
			return invalidBindingRoot("inspect tree entry", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || entry.Type()&os.ModeSymlink != 0 {
			return invalidBindingRoot("symlink entry is forbidden", nil)
		}
		relative := path.Join(prefix, name)
		if relative == "" || strings.HasPrefix(relative, "/") || strings.Contains(relative, "\\") {
			return invalidBindingRoot("entry path is not canonical", nil)
		}
		if info.IsDir() {
			child, err := root.OpenRoot(name)
			if err != nil {
				return invalidBindingRoot("open tree directory", err)
			}
			opened, err := child.Stat(".")
			if err != nil || !opened.IsDir() || !os.SameFile(info, opened) {
				child.Close()
				return invalidBindingRoot("tree directory identity changed", err)
			}
			err = appendRootedBindingEntries(child, relative, result)
			closeErr := child.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return invalidBindingRoot("close tree directory", closeErr)
			}
			current, err := root.Lstat(name)
			if err != nil || current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(info, current) {
				return invalidBindingRoot("tree directory identity changed", err)
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return invalidBindingRoot("non-regular entry is forbidden", nil)
		}
		file, err := root.Open(name)
		if err != nil {
			return invalidBindingRoot("open regular file", err)
		}
		value, err := digestRootedBindingFile(file, info)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return invalidBindingRoot("close regular file", closeErr)
		}
		current, err := root.Lstat(name)
		if err != nil || current.Mode()&os.ModeSymlink != 0 || !sameFileSnapshot(info, current) {
			return invalidBindingRoot("regular file identity changed", err)
		}
		value.Path = relative
		*result = append(*result, value)
	}
	if err := verifyRootedBindingDirectorySnapshot(root, snapshot); err != nil {
		return err
	}
	return nil
}

func digestRootedBindingFile(file *os.File, expected os.FileInfo) (TreeEntry, error) {
	before, err := file.Stat()
	if err != nil {
		return TreeEntry{}, invalidBindingRoot("stat regular file before read", err)
	}
	if !sameFileSnapshot(expected, before) || !before.Mode().IsRegular() {
		return TreeEntry{}, invalidBindingRoot("regular file changed before read", nil)
	}
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return TreeEntry{}, invalidBindingRoot("read regular file", err)
	}
	after, err := file.Stat()
	if err != nil {
		return TreeEntry{}, invalidBindingRoot("stat regular file after read", err)
	}
	if !sameFileSnapshot(before, after) || size != after.Size() {
		return TreeEntry{}, invalidBindingRoot("regular file changed during read", nil)
	}
	return TreeEntry{Mode: uint32(after.Mode().Perm() & 0o111), Size: after.Size(), Digest: "sha256:" + hex.EncodeToString(hash.Sum(nil))}, nil
}

func bindingFileEvidence(contentRoot string, entry TreeEntry) (TreeEvidence, error) {
	payload := struct {
		SchemaVersion string `json:"schema_version"`
		Path          string `json:"path"`
		Executable    bool   `json:"executable"`
		Size          int64  `json:"size"`
		Digest        string `json:"digest"`
	}{"oaw.binding-file/v1", contentRoot, entry.Mode != 0, entry.Size, entry.Digest}
	encoded, err := canonicaljson.Marshal(payload)
	if err != nil {
		return TreeEvidence{}, invalidBindingRoot("encode canonical file", err)
	}
	return TreeEvidence{RootDigest: SHA256Digest(encoded), Entries: []TreeEntry{entry}}, nil
}

func canonicalBindingRoot(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.ContainsAny(value, "\\:") || strings.TrimSpace(value) != value {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	cleaned := path.Clean(value)
	return cleaned == value && cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

func invalidBindingRoot(detail string, cause error) error {
	if cause != nil {
		return fmt.Errorf("BINDING_ROOT_INVALID: %s: %w", detail, cause)
	}
	return fmt.Errorf("BINDING_ROOT_INVALID: %s", detail)
}
