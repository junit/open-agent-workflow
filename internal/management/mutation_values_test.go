package management

import (
	"bytes"
	"io/fs"
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
}
