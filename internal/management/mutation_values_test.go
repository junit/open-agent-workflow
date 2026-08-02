package management

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMutationActionContracts(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "nested", "target")
	before := installPathSnapshot{kind: installPathRegular, mode: 0o640, data: []byte("before\n")}

	tests := []struct {
		name   string
		effect mutationEffect
		data   []byte
		mode   fs.FileMode
		want   mutationEffect
	}{
		{name: "replace", effect: mutationReplace, data: []byte("after\n"), mode: 0o644, want: mutationReplace},
		{name: "remove", effect: mutationRemove, want: mutationRemove},
		{name: "retain", effect: mutationRetain, want: mutationRetain},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := bytes.Clone(tt.data)
			action, err := newMutationAction(tt.effect, "target", input, destination, tt.mode, root, "nested/target", before)
			if err != nil {
				t.Fatal(err)
			}
			if action.effect != tt.want || action.destination != destination || action.allowedRoot != root || action.relativeSuffix != "nested/target" {
				t.Fatalf("action = %#v", action)
			}
			if len(input) > 0 {
				input[0] ^= 0xff
				if bytes.Equal(action.data, input) {
					t.Fatal("action aliases caller data")
				}
			}
			before.data[0] ^= 0xff
			if bytes.Equal(action.before.data, before.data) {
				t.Fatal("action aliases caller snapshot")
			}
			before.data[0] ^= 0xff
		})
	}
}

func TestMutationActionRejectsUnsafeValues(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "target")
	valid := func() (mutationAction, error) {
		return newMutationAction(mutationReplace, "target", []byte("data"), destination, 0o644, root, "target", installPathSnapshot{})
	}
	if _, err := valid(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		call func() (mutationAction, error)
		want string
	}{
		{name: "unknown effect", call: func() (mutationAction, error) {
			return newMutationAction(mutationEffect(99), "target", nil, destination, 0, root, "target", installPathSnapshot{})
		}, want: "invalid mutation effect"},
		{name: "empty label", call: func() (mutationAction, error) {
			return newMutationAction(mutationReplace, "", []byte("data"), destination, 0o644, root, "target", installPathSnapshot{})
		}, want: "mutation action cannot be serialized"},
		{name: "control label", call: func() (mutationAction, error) {
			return newMutationAction(mutationReplace, "bad\nlabel", []byte("data"), destination, 0o644, root, "target", installPathSnapshot{})
		}, want: "mutation action cannot be serialized"},
		{name: "traversing suffix", call: func() (mutationAction, error) {
			return newMutationAction(mutationReplace, "target", []byte("data"), destination, 0o644, root, "../target", installPathSnapshot{})
		}, want: "unsafe component"},
		{name: "destination mismatch", call: func() (mutationAction, error) {
			return newMutationAction(mutationReplace, "target", []byte("data"), destination+"-other", 0o644, root, "target", installPathSnapshot{})
		}, want: "destination does not match"},
		{name: "replace without data", call: func() (mutationAction, error) {
			return newMutationAction(mutationReplace, "target", nil, destination, 0o644, root, "target", installPathSnapshot{})
		}, want: "replace action has no data"},
		{name: "replace invalid mode", call: func() (mutationAction, error) {
			return newMutationAction(mutationReplace, "target", []byte("data"), destination, 0o666, root, "target", installPathSnapshot{})
		}, want: "invalid prepared destination mode"},
		{name: "remove with data", call: func() (mutationAction, error) {
			return newMutationAction(mutationRemove, "target", []byte("data"), destination, 0, root, "target", installPathSnapshot{})
		}, want: "remove action has replacement data"},
		{name: "remove with mode", call: func() (mutationAction, error) {
			return newMutationAction(mutationRemove, "target", nil, destination, 0o600, root, "target", installPathSnapshot{})
		}, want: "remove action has a destination mode"},
		{name: "retain with data", call: func() (mutationAction, error) {
			return newMutationAction(mutationRetain, "target", []byte("data"), destination, 0, root, "target", installPathSnapshot{})
		}, want: "retain action has replacement data"},
		{name: "retain with mode", call: func() (mutationAction, error) {
			return newMutationAction(mutationRetain, "target", nil, destination, 0o600, root, "target", installPathSnapshot{})
		}, want: "retain action has a destination mode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.call()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestMutationActionDeduplicationAndConflict(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "target")
	first, err := newMutationAction(mutationReplace, "one", []byte("data"), destination, 0o644, root, "target", installPathSnapshot{})
	if err != nil {
		t.Fatal(err)
	}
	equivalent, err := newMutationAction(mutationReplace, "two", []byte("data"), destination, 0o644, root, "target", installPathSnapshot{})
	if err != nil {
		t.Fatal(err)
	}
	remove, err := newMutationAction(mutationRemove, "remove", nil, destination, 0, root, "target", installPathSnapshot{})
	if err != nil {
		t.Fatal(err)
	}

	actions, err := addMutationAction(nil, first)
	if err != nil {
		t.Fatal(err)
	}
	actions, err = addMutationAction(actions, equivalent)
	if err != nil || len(actions) != 1 {
		t.Fatalf("equivalent add = %#v, %v", actions, err)
	}
	if _, err := addMutationAction(actions, remove); err == nil || !strings.Contains(err.Error(), "conflicting mutation actions") {
		t.Fatalf("conflict error = %v", err)
	}
	differentData, err := newMutationAction(mutationReplace, "different", []byte("other"), destination, 0o644, root, "target", installPathSnapshot{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := addMutationAction(actions, differentData); err == nil || !strings.Contains(err.Error(), "conflicting mutation actions") {
		t.Fatalf("data conflict error = %v", err)
	}

	copyOfActions := cloneMutationActions(actions)
	copyOfActions[0].data[0] = 'X'
	if bytes.Equal(copyOfActions[0].data, actions[0].data) {
		t.Fatal("cloned actions alias source")
	}
}

func TestInstallActionRegressionRoundTrip(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "target")
	before := installPathSnapshot{kind: installPathRegular, mode: 0o644, data: []byte("before")}
	original, err := newInstallAction("target", []byte("after"), destination, 0o644, root, "target", before)
	if err != nil {
		t.Fatal(err)
	}
	converted, err := mutationActionFromInstall(original)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip, err := installActionFromMutation(converted)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundTrip, original) {
		t.Fatalf("round trip = %#v, want %#v", roundTrip, original)
	}
	converted.data[0] = 'X'
	if bytes.Equal(converted.data, original.data) {
		t.Fatal("conversion aliases install data")
	}
	remove, err := newMutationAction(mutationRemove, "target", nil, destination, 0, root, "target", before)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installActionFromMutation(remove); err == nil || !strings.Contains(err.Error(), "must replace") {
		t.Fatalf("remove conversion error = %v", err)
	}
}

func TestPredictMutationResultCoversRemoveRetainAndCreate(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	existing := filepath.Join(root, "existing")
	writePrepareFile(t, existing, []byte("existing"), 0o644)
	existingView, err := inspectInstallPath(existing)
	if err != nil {
		t.Fatal(err)
	}
	missingView := installPathSnapshot{kind: installPathMissing}
	create, err := newMutationAction(mutationReplace, "create", []byte("new"), missing, 0o644, root, "missing", missingView)
	if err != nil {
		t.Fatal(err)
	}
	remove, err := newMutationAction(mutationRemove, "remove", nil, existing, 0, root, "existing", existingView)
	if err != nil {
		t.Fatal(err)
	}
	retain, err := newMutationAction(mutationRetain, "retain", nil, existing, 0, root, "existing", existingView)
	if err != nil {
		t.Fatal(err)
	}
	missingRemove, err := newMutationAction(mutationRemove, "missing-remove", nil, missing, 0, root, "missing", missingView)
	if err != nil {
		t.Fatal(err)
	}
	plan := mutationPlan{targetActions: []mutationAction{create, remove, missingRemove}, policyAction: retain}
	if got, want := predictMutationResult(plan).Lines, []string{"oaw: would-create: " + missing, "oaw: would-remove: " + existing}; !reflect.DeepEqual(got, want) {
		t.Fatalf("prediction = %v, want %v", got, want)
	}
}

func TestNewStateMutationActionRejectsDestinationOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.state")
	if _, err := newStateMutationAction("state", []byte("state"), outside, root); err == nil || !strings.Contains(err.Error(), "escapes its allowed root") {
		t.Fatalf("error = %v", err)
	}
}

func TestDirectoryActionContracts(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "nested")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	action, err := newDirectoryAction(directory, root, "nested", false)
	if err != nil {
		t.Fatal(err)
	}
	if action.destination != directory || action.before.kind != installPathDirectory || action.namespace {
		t.Fatalf("action = %#v", action)
	}
	missing, err := newDirectoryAction(filepath.Join(root, "missing"), root, "missing", true)
	if err != nil || missing.before.kind != installPathMissing || !missing.namespace {
		t.Fatalf("missing action = %#v, %v", missing, err)
	}

	tests := []struct {
		name        string
		destination string
		root        string
		suffix      string
		want        string
	}{
		{name: "empty", root: root, suffix: "nested", want: "cannot be serialized"},
		{name: "unsafe suffix", destination: directory, root: root, suffix: "../nested", want: "unsafe component"},
		{name: "mismatch", destination: directory + "-other", root: root, suffix: "nested", want: "does not match registry"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newDirectoryAction(tt.destination, tt.root, tt.suffix, false)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
	regular := filepath.Join(root, "regular")
	writePrepareFile(t, regular, []byte("not a directory"), 0o644)
	if _, err := newDirectoryAction(regular, root, "regular", false); err == nil || !strings.Contains(err.Error(), "changed before removal") {
		t.Fatalf("regular error = %v", err)
	}

	left, err := newDirectoryAction(filepath.Join(root, "aa"), root, "aa", false)
	if err != nil {
		t.Fatal(err)
	}
	right, err := newDirectoryAction(filepath.Join(root, "bb"), root, "bb", false)
	if err != nil {
		t.Fatal(err)
	}
	actions := []directoryAction{right, left, action}
	sortDirectoryActions(actions)
	if actions[0].destination != directory || actions[1].destination != left.destination || actions[2].destination != right.destination {
		t.Fatalf("sorted = %#v", actions)
	}
	clone := cloneDirectoryActions(actions)
	clone[0].before.data = []byte("changed")
	if len(actions[0].before.data) != 0 {
		t.Fatal("directory clone aliases source")
	}
}

func TestPredictMutationActionIgnoresZeroAndUnknownEffects(t *testing.T) {
	if got := predictMutationAction(mutationAction{}); got != nil {
		t.Fatalf("zero prediction = %v", got)
	}
	if got := predictMutationAction(mutationAction{effect: mutationEffect(99)}); got != nil {
		t.Fatalf("unknown prediction = %v", got)
	}
}
