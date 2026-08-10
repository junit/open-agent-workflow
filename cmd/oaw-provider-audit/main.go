package main

import (
	"archive/tar"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/provideraudit"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "oaw-provider-audit: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("oaw-provider-audit", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	validateMode := flags.Bool("validate", false, "validate a committed manifest")
	writeMode := flags.Bool("write", false, "write a manifest from pinned checkouts")
	checkMode := flags.Bool("check", false, "compare pinned checkouts with a committed manifest")
	exportedMode := flags.Bool("exported", false, "use roots exported from an exact pinned archive")
	manifestPath := flags.String("manifest", "", "committed manifest path")
	outputPath := flags.String("output", "", "output manifest path")
	mattRoot := flags.String("matt-root", "", "Matt source checkout")
	superpowersRoot := flags.String("superpowers-root", "", "Superpowers source checkout")
	eccRoot := flags.String("ecc-root", "", "ECC source checkout")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("CLI_ARGUMENT_INVALID: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("CLI_ARGUMENT_INVALID: positional arguments are not supported")
	}
	modes := 0
	for _, selected := range []bool{*validateMode, *writeMode, *checkMode} {
		if selected {
			modes++
		}
	}
	if modes != 1 {
		return fmt.Errorf("CLI_ARGUMENT_INVALID: select exactly one of --validate, --write, or --check")
	}
	if *validateMode {
		if *manifestPath == "" || *outputPath != "" || *mattRoot != "" || *superpowersRoot != "" || *eccRoot != "" || *exportedMode {
			return fmt.Errorf("CLI_ARGUMENT_INVALID: --validate requires only an explicit --manifest")
		}
		raw, err := os.ReadFile(*manifestPath)
		if err != nil {
			return err
		}
		manifest, err := provideraudit.Decode(raw)
		if err != nil {
			return err
		}
		fmt.Printf("%s %s\n", manifest.SchemaVersion, manifest.Digest)
		return nil
	}
	if *writeMode && (*outputPath == "" || *manifestPath != "") {
		return fmt.Errorf("CLI_ARGUMENT_INVALID: --write requires only an explicit --output")
	}
	if *checkMode && (*manifestPath == "" || *outputPath != "") {
		return fmt.Errorf("CLI_ARGUMENT_INVALID: --check requires only an explicit --manifest")
	}
	if *mattRoot == "" || *superpowersRoot == "" || *eccRoot == "" {
		return fmt.Errorf("CLI_ARGUMENT_INVALID: all three explicit Provider roots are required")
	}
	var manifest provideraudit.Manifest
	var err error
	if *exportedMode {
		manifest, err = provideraudit.Build(provideraudit.LockedCheckouts(*mattRoot, *superpowersRoot, *eccRoot))
	} else {
		manifest, err = buildFromPinnedGitCheckouts(*mattRoot, *superpowersRoot, *eccRoot)
	}
	if err != nil {
		return err
	}
	encoded, err := canonicalManifest(manifest)
	if err != nil {
		return err
	}
	if *writeMode {
		return writeAtomic(*outputPath, encoded)
	}
	committed, err := os.ReadFile(*manifestPath)
	if err != nil {
		return err
	}
	if _, err := provideraudit.Decode(committed); err != nil {
		return err
	}
	if !bytes.Equal(committed, encoded) {
		return fmt.Errorf("PROVIDER_AUDIT_DRIFT: committed manifest differs from pinned source trees")
	}
	return nil
}

func buildFromPinnedGitCheckouts(mattRoot, superpowersRoot, eccRoot string) (provideraudit.Manifest, error) {
	checkouts := provideraudit.LockedCheckouts(mattRoot, superpowersRoot, eccRoot)
	exportRoot, err := os.MkdirTemp("", "oaw-provider-audit-")
	if err != nil {
		return provideraudit.Manifest{}, err
	}
	defer os.RemoveAll(exportRoot)
	for index := range checkouts {
		checkout := &checkouts[index]
		if err := verifyGitRevision(checkout.Root, checkout.Revision); err != nil {
			return provideraudit.Manifest{}, err
		}
		destination := filepath.Join(exportRoot, strings.ReplaceAll(checkout.ProviderID, "/", "-"))
		if err := exportGitTree(checkout.Root, checkout.Revision, destination); err != nil {
			return provideraudit.Manifest{}, err
		}
		checkout.Root = destination
	}
	return provideraudit.Build(checkouts)
}

func verifyGitRevision(root, expected string) error {
	command := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: verify %s: %w", root, err)
	}
	if strings.TrimSpace(string(output)) != expected {
		return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: checkout %s is not pinned to %s", root, expected)
	}
	return nil
}

func exportGitTree(root, revision, destination string) error {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	command := exec.Command("git", "-C", root, "archive", "--format=tar", revision)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return err
	}
	archiveErr := extractTrackedArchive(tar.NewReader(stdout), destination)
	if archiveErr != nil {
		_ = stdout.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
		return archiveErr
	}
	waitErr := command.Wait()
	if waitErr != nil {
		return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: export tracked tree: %w", waitErr)
	}
	return nil
}

func extractTrackedArchive(reader *tar.Reader, destination string) error {
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		archiveName := header.Name
		if header.Typeflag == tar.TypeDir {
			archiveName = strings.TrimSuffix(archiveName, "/")
		}
		cleaned := path.Clean(archiveName)
		if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") || cleaned != archiveName {
			return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: unsafe archive path %q", header.Name)
		}
		target := filepath.Join(destination, filepath.FromSlash(cleaned))
		switch header.Typeflag {
		case tar.TypeXHeader, tar.TypeXGlobalHeader:
			continue
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			mode := os.FileMode(0o600) | os.FileMode(header.Mode&0o111)
			file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if !containedArchiveSymlink(destination, target, header.Linkname) {
				return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: unsafe symlink target %q", header.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: unsupported tracked entry type %d", header.Typeflag)
		}
	}
}

func containedArchiveSymlink(destination, linkPath, target string) bool {
	if target == "" || path.IsAbs(target) || path.Clean(target) != target || strings.Contains(target, "\\") {
		return false
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(linkPath), filepath.FromSlash(target)))
	relative, err := filepath.Rel(destination, resolved)
	return err == nil && relative != ".." && !filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func canonicalManifest(manifest provideraudit.Manifest) ([]byte, error) {
	encoded, err := canonicaljson.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func writeAtomic(target string, content []byte) error {
	directory := filepath.Dir(target)
	temporary, err := os.CreateTemp(directory, ".provider-sources-v4-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return err
	}
	remove = false
	return nil
}
