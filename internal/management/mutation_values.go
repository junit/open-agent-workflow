package management

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type mutationEffect uint8

const (
	mutationReplace mutationEffect = iota + 1
	mutationRemove
	mutationRetain
)

type mutationAction struct {
	effect         mutationEffect
	label          string
	data           []byte
	destination    string
	mode           fs.FileMode
	allowedRoot    string
	relativeSuffix string
	before         installPathSnapshot
	identity       mutationPathIdentity
}

type mutationPathIdentity struct {
	captured    bool
	root        fs.FileInfo
	parent      fs.FileInfo
	destination fs.FileInfo
}

func newMutationAction(
	effect mutationEffect,
	label string,
	data []byte,
	destination string,
	mode fs.FileMode,
	root string,
	suffix string,
	before installPathSnapshot,
) (mutationAction, error) {
	if effect != mutationReplace && effect != mutationRemove && effect != mutationRetain {
		return mutationAction{}, integrityError("invalid mutation effect")
	}
	if label == "" || !safeStateField(label) || destination == "" || !safeStateField(destination) || root == "" || !safeStateField(root) || suffix == "" || !safeStateField(suffix) {
		return mutationAction{}, integrityError("mutation action cannot be serialized")
	}
	rebuilt, err := validatedDestinationPath(root, suffix)
	if err != nil {
		return mutationAction{}, err
	}
	if rebuilt != destination {
		return mutationAction{}, integrityError("mutation action destination does not match registry: " + destination)
	}
	if err := validateMutationEffect(effect, data, mode); err != nil {
		return mutationAction{}, err
	}
	identity, err := captureMutationPathIdentity(root, destination)
	if err != nil {
		return mutationAction{}, err
	}
	return mutationAction{
		effect: effect, label: label, data: bytes.Clone(data), destination: destination,
		mode: mode, allowedRoot: root, relativeSuffix: suffix,
		before: cloneInstallPathSnapshot(before), identity: identity,
	}, nil
}

func validateMutationEffect(effect mutationEffect, data []byte, mode fs.FileMode) error {
	switch effect {
	case mutationReplace:
		if data == nil {
			return integrityError("replace action has no data")
		}
		if mode != 0o600 && mode != 0o644 {
			return integrityError("invalid prepared destination mode")
		}
	case mutationRemove:
		if data != nil {
			return integrityError("remove action has replacement data")
		}
		if mode != 0 {
			return integrityError("remove action has a destination mode")
		}
	case mutationRetain:
		if data != nil {
			return integrityError("retain action has replacement data")
		}
		if mode != 0 {
			return integrityError("retain action has a destination mode")
		}
	}
	return nil
}

func captureMutationPathIdentity(root, destination string) (mutationPathIdentity, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return mutationPathIdentity{}, integrityError("mutation root identity could not be captured: " + root)
	}
	inspect := func(path string, directory bool) (fs.FileInfo, error) {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			return nil, nil
		}
		if err != nil {
			return nil, integrityError("mutation path identity could not be captured: " + path)
		}
		if info.Mode()&os.ModeSymlink != 0 || (directory && !info.IsDir()) {
			return nil, integrityError("mutation path identity is unsafe: " + path)
		}
		return info, nil
	}
	parent, err := inspect(filepath.Dir(destination), true)
	if err != nil {
		return mutationPathIdentity{}, err
	}
	final, err := inspect(destination, false)
	if err != nil {
		return mutationPathIdentity{}, err
	}
	return mutationPathIdentity{captured: true, root: rootInfo, parent: parent, destination: final}, nil
}

func revalidateMutationPathIdentity(expected mutationPathIdentity, root, destination string) error {
	if !expected.captured {
		return nil
	}
	current, err := captureMutationPathIdentity(root, destination)
	if err != nil {
		return err
	}
	if !sameMutationFileIdentity(expected.root, current.root) ||
		!sameMutationFileIdentity(expected.parent, current.parent) ||
		!sameMutationFileIdentity(expected.destination, current.destination) {
		return integrityError("destination identity changed after preparation: " + destination)
	}
	return nil
}

func sameMutationFileIdentity(left, right fs.FileInfo) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return os.SameFile(left, right)
}

func addMutationAction(actions []mutationAction, action mutationAction) ([]mutationAction, error) {
	for _, existing := range actions {
		if filepath.Clean(existing.destination) != filepath.Clean(action.destination) {
			continue
		}
		if !equivalentMutationAction(existing, action) {
			return nil, integrityError("conflicting mutation actions for " + action.destination)
		}
		return actions, nil
	}
	return append(actions, cloneMutationAction(action)), nil
}

func equivalentMutationAction(left, right mutationAction) bool {
	if left.effect != right.effect || left.destination != right.destination || left.mode != right.mode || left.allowedRoot != right.allowedRoot || left.relativeSuffix != right.relativeSuffix {
		return false
	}
	if left.effect == mutationReplace && !bytes.Equal(left.data, right.data) {
		return false
	}
	return true
}

func cloneMutationAction(action mutationAction) mutationAction {
	action.data = bytes.Clone(action.data)
	action.before = cloneInstallPathSnapshot(action.before)
	return action
}

func cloneMutationActions(actions []mutationAction) []mutationAction {
	result := make([]mutationAction, len(actions))
	for index, action := range actions {
		result[index] = cloneMutationAction(action)
	}
	return result
}

func mutationActionFromInstall(action installAction) (mutationAction, error) {
	return newMutationAction(
		mutationReplace, action.label, action.data, action.destination, action.mode,
		action.allowedRoot, action.relativeSuffix, action.before,
	)
}

func installActionFromMutation(action mutationAction) (installAction, error) {
	if action.effect != mutationReplace {
		return installAction{}, integrityError("install action must replace a destination")
	}
	return newInstallAction(
		action.label, action.data, action.destination, action.mode,
		action.allowedRoot, action.relativeSuffix, action.before,
	)
}

func newDirectoryAction(destination, root, suffix string, namespace bool) (directoryAction, error) {
	if destination == "" || !safeStateField(destination) || root == "" || !safeStateField(root) || suffix == "" || !safeStateField(suffix) {
		return directoryAction{}, integrityError("directory action cannot be serialized")
	}
	rebuilt, err := validatedDestinationPath(root, suffix)
	if err != nil {
		return directoryAction{}, err
	}
	if rebuilt != destination {
		return directoryAction{}, integrityError("directory action destination does not match registry: " + destination)
	}
	before, err := inspectInstallPath(destination)
	if err != nil {
		return directoryAction{}, err
	}
	if before.kind != installPathMissing && before.kind != installPathDirectory {
		return directoryAction{}, integrityError("owned directory changed before removal: " + destination)
	}
	identity, err := captureMutationPathIdentity(root, destination)
	if err != nil {
		return directoryAction{}, err
	}
	return directoryAction{
		destination: destination, allowedRoot: root, relativeSuffix: suffix,
		before: cloneInstallPathSnapshot(before), namespace: namespace, identity: identity,
	}, nil
}

func cloneDirectoryActions(actions []directoryAction) []directoryAction {
	result := make([]directoryAction, len(actions))
	for index, action := range actions {
		action.before = cloneInstallPathSnapshot(action.before)
		result[index] = action
	}
	return result
}

func sortDirectoryActions(actions []directoryAction) {
	sort.Slice(actions, func(left, right int) bool {
		leftPath, rightPath := actions[left].destination, actions[right].destination
		if len(leftPath) != len(rightPath) {
			return len(leftPath) > len(rightPath)
		}
		return leftPath < rightPath
	})
}
