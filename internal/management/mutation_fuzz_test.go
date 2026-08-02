package management

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func FuzzMutationAction(f *testing.F) {
	f.Add(uint8(mutationReplace), "artifact", "nested/artifact", []byte("replacement\n"), []byte("before\n"), uint32(0o644))
	f.Add(uint8(mutationRemove), "artifact", "artifact", []byte(nil), []byte("before\n"), uint32(0))
	root := f.TempDir()
	f.Fuzz(func(t *testing.T, rawEffect uint8, label, suffix string, data, beforeData []byte, rawMode uint32) {
		if len(label) > 4096 || len(suffix) > 4096 || len(data) > 8192 || len(beforeData) > 8192 {
			t.Skip()
		}
		destination := root + string(filepath.Separator) + filepath.FromSlash(suffix)
		before := installPathSnapshot{kind: installPathMissing, data: bytes.Clone(beforeData)}
		effect := mutationEffect(rawEffect)
		mode := fs.FileMode(rawMode & 0o777)
		first, firstErr := newMutationAction(effect, label, data, destination, mode, root, suffix, before)
		second, secondErr := newMutationAction(effect, label, data, destination, mode, root, suffix, before)
		if fuzzErrorText(firstErr) != fuzzErrorText(secondErr) {
			t.Fatalf("constructor is nondeterministic: %v / %v", firstErr, secondErr)
		}
		if firstErr != nil {
			return
		}
		if first.effect != second.effect || first.label != second.label || first.destination != second.destination ||
			first.mode != second.mode || first.allowedRoot != second.allowedRoot || first.relativeSuffix != second.relativeSuffix ||
			!bytes.Equal(first.data, second.data) || !bytes.Equal(first.before.data, second.before.data) {
			t.Fatalf("constructor output is nondeterministic: %#v / %#v", first, second)
		}
		if err := compareMutationPathIdentity(first.identity, second.identity, destination); err != nil {
			t.Fatal(err)
		}
		if !containedStrictly(root, first.destination) {
			t.Fatalf("accepted destination escapes root: %q", first.destination)
		}
		wantData := bytes.Clone(data)
		wantBefore := bytes.Clone(beforeData)
		if len(data) > 0 {
			data[0] ^= 0xff
		}
		if len(beforeData) > 0 {
			beforeData[0] ^= 0xff
		}
		if !bytes.Equal(first.data, wantData) || !bytes.Equal(first.before.data, wantBefore) {
			t.Fatal("mutation action aliases fuzz input")
		}
	})
}

func FuzzManagedRemoval(f *testing.F) {
	f.Add([]byte("personal prefix\n"), []byte("managed body\n"), []byte("personal suffix\n"))
	f.Add([]byte{0xff, '\n'}, []byte{0x00, 0xfe}, []byte{0x80})
	f.Fuzz(func(t *testing.T, prefix, body, suffix []byte) {
		if len(prefix)+len(body)+len(suffix) > 64<<10 {
			t.Skip()
		}
		data := bytes.Clone(prefix)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
		blockStart := len(data)
		data = append(data, beginMarker...)
		data = append(data, '\n')
		data = append(data, body...)
		if len(body) > 0 && body[len(body)-1] != '\n' {
			data = append(data, '\n')
		}
		data = append(data, endMarker...)
		data = append(data, '\n')
		blockEnd := len(data)
		data = append(data, suffix...)

		first, firstErr := renderManagedFileWithoutBlock(data)
		second, secondErr := renderManagedFileWithoutBlock(data)
		if fuzzErrorText(firstErr) != fuzzErrorText(secondErr) || !bytes.Equal(first, second) {
			t.Fatalf("managed removal is nondeterministic: %v / %v", firstErr, secondErr)
		}
		for _, value := range [][]byte{prefix, body, suffix} {
			if bytes.Contains(value, []byte(beginMarker)) || bytes.Contains(value, []byte(endMarker)) {
				return
			}
		}
		if firstErr != nil {
			t.Fatal(firstErr)
		}
		want := append(bytes.Clone(data[:blockStart]), data[blockEnd:]...)
		if !bytes.Equal(first, want) {
			t.Fatalf("foreign bytes changed: got %q want %q", first, want)
		}
		block := bytes.Clone(data[blockStart:blockEnd])
		withoutBegin := block[len(beginMarker)+1:]
		repairedBegin, ok := repairManagedMarkerStructure(withoutBegin, block)
		if !ok || !bytes.Equal(repairedBegin, block) {
			t.Fatalf("begin repair = %q, %t; want %q", repairedBegin, ok, block)
		}
		withoutEnd := block[:len(block)-len(endMarker)-1]
		repairedEnd, ok := repairManagedMarkerStructure(withoutEnd, block)
		if !ok || !bytes.Equal(repairedEnd, block) {
			t.Fatalf("end repair = %q, %t; want %q", repairedEnd, ok, block)
		}
	})
}

func FuzzBackupManifest(f *testing.F) {
	f.Add("update", "user", "target", "001-target", "1:1")
	f.Add("uninstall", "project", "state.tsv", "002-state.tsv", "123:456")
	root := f.TempDir()
	operationPath := filepath.Join(root, "backups", "operation")
	f.Fuzz(func(t *testing.T, operation, scope, originalPart, artifactPart, checksum string) {
		if len(operation)+len(scope)+len(originalPart)+len(artifactPart)+len(checksum) > 16<<10 {
			t.Skip()
		}
		candidate := backupCandidate{
			original: root + string(filepath.Separator) + filepath.FromSlash(originalPart),
			backup:   operationPath + string(filepath.Separator) + filepath.FromSlash(artifactPart),
			checksum: checksum,
		}
		plan := backupPlan{required: true, operation: operation, scope: scope, path: operationPath, candidates: []backupCandidate{candidate}}
		first, firstErr := renderBackupManifest(plan)
		second, secondErr := renderBackupManifest(plan)
		if fuzzErrorText(firstErr) != fuzzErrorText(secondErr) || !bytes.Equal(first, second) {
			t.Fatalf("manifest rendering is nondeterministic: %v / %v", firstErr, secondErr)
		}
		if firstErr != nil {
			return
		}
		if filepath.Dir(filepath.Clean(candidate.backup)) != filepath.Clean(operationPath) {
			t.Fatalf("accepted artifact escapes operation path: %q", candidate.backup)
		}
		if bytes.Count(first, []byte{'\n'}) != 4 || bytes.ContainsAny(first, "\x00\r") {
			t.Fatalf("manifest framing is unsafe: %q", first)
		}
		first[0] ^= 0xff
		again, err := renderBackupManifest(plan)
		if err != nil || !bytes.Equal(again, second) {
			t.Fatal("manifest output aliases a prior render")
		}
	})
}

func FuzzMutationRollbackOrder(f *testing.F) {
	f.Add([]byte("first before\n"), []byte("first after\n"), []byte("second before\n"), []byte("second after\n"))
	f.Fuzz(func(t *testing.T, firstBefore, firstAfter, secondBefore, secondAfter []byte) {
		if len(firstBefore)+len(firstAfter)+len(secondBefore)+len(secondAfter) > 32<<10 {
			t.Skip()
		}
		root := t.TempDir()
		paths := []string{filepath.Join(root, "first"), filepath.Join(root, "second")}
		beforeData := [][]byte{bytes.Clone(firstBefore), bytes.Clone(secondBefore)}
		afterData := [][]byte{append([]byte{}, firstAfter...), append([]byte{}, secondAfter...)}
		actions := make([]mutationAction, 0, 2)
		for index, destination := range paths {
			writePrepareFile(t, destination, beforeData[index], 0o640)
			before, err := inspectInstallPath(destination)
			if err != nil {
				t.Fatal(err)
			}
			action, err := newMutationAction(mutationReplace, filepath.Base(destination), afterData[index], destination, 0o600, root, filepath.Base(destination), before)
			if err != nil {
				t.Fatal(err)
			}
			if err := scopedAtomicReplaceMutation(action); err != nil {
				t.Fatal(err)
			}
			actions = append(actions, action)
		}
		journal := []mutationInverse{{action: &actions[0]}, {action: &actions[1]}}
		if err := rollbackMutationJournal(journal); err != nil {
			t.Fatal(err)
		}
		for index, destination := range paths {
			data, err := os.ReadFile(destination)
			if err != nil || !bytes.Equal(data, beforeData[index]) {
				t.Fatalf("rollback %s = %q, %v; want %q", destination, data, err, beforeData[index])
			}
			info, err := os.Stat(destination)
			if err != nil || info.Mode().Perm() != 0o640 {
				t.Fatalf("rollback mode %s = %v, %v", destination, info, err)
			}
		}
	})
}

func FuzzMutationPathSuffix(f *testing.F) {
	f.Add("open-agent-workflow/ENGINEERING.md")
	f.Add("../escape")
	root := f.TempDir()
	f.Fuzz(func(t *testing.T, suffix string) {
		if len(suffix) > 4096 {
			t.Skip()
		}
		first, firstErr := validatedDestinationPath(root, suffix)
		second, secondErr := validatedDestinationPath(root, suffix)
		if fuzzErrorText(firstErr) != fuzzErrorText(secondErr) || first != second {
			t.Fatalf("path validation is nondeterministic: %q/%v %q/%v", first, firstErr, second, secondErr)
		}
		if firstErr == nil && !containedStrictly(root, first) {
			t.Fatalf("accepted suffix escaped root: %q => %q", suffix, first)
		}
	})
}

func FuzzMutationStateFields(f *testing.F) {
	valid := "format\t1\nversion\t0.1.0\nscope\tuser\npolicy\t/config/policy\t1:1\n" +
		"target\tclaude\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file\n"
	f.Add([]byte(valid))
	f.Add([]byte("format\t1\nversion\tbad\x00value\n"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 64<<10 {
			t.Skip()
		}
		first, firstErr := parseInstallationState(raw)
		second, secondErr := parseInstallationState(raw)
		if fuzzErrorText(firstErr) != fuzzErrorText(secondErr) || !reflect.DeepEqual(first, second) {
			t.Fatalf("state parsing is nondeterministic: %#v/%v %#v/%v", first, firstErr, second, secondErr)
		}
		if firstErr != nil {
			return
		}
		encoded, err := serializeInstallState(first)
		if err != nil {
			t.Fatalf("parsed state cannot be serialized: %v", err)
		}
		roundTrip, err := parseInstallationState(encoded)
		if err != nil || !reflect.DeepEqual(roundTrip, first) {
			t.Fatalf("state round trip = %#v, %v; want %#v", roundTrip, err, first)
		}
	})
}

func fuzzErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
