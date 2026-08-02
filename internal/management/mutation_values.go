package management

import (
	"bytes"
	"io/fs"
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
		return mutationAction{}, compatibilityError("invalid mutation effect")
	}
	if label == "" || !safeStateField(label) || destination == "" || !safeStateField(destination) || root == "" || !safeStateField(root) || suffix == "" || !safeStateField(suffix) {
		return mutationAction{}, compatibilityError("mutation action cannot be serialized")
	}
	rebuilt, err := validatedDestinationPath(root, suffix)
	if err != nil {
		return mutationAction{}, err
	}
	if rebuilt != destination {
		return mutationAction{}, compatibilityError("mutation action destination does not match registry: " + destination)
	}
	switch effect {
	case mutationReplace:
		if data == nil {
			return mutationAction{}, compatibilityError("replace action has no data")
		}
		if mode != 0o600 && mode != 0o644 {
			return mutationAction{}, compatibilityError("invalid prepared destination mode")
		}
	case mutationRemove:
		if data != nil {
			return mutationAction{}, compatibilityError("remove action has replacement data")
		}
		if mode != 0 {
			return mutationAction{}, compatibilityError("remove action has a destination mode")
		}
	case mutationRetain:
		if data != nil {
			return mutationAction{}, compatibilityError("retain action has replacement data")
		}
		if mode != 0 {
			return mutationAction{}, compatibilityError("retain action has a destination mode")
		}
	}
	return mutationAction{
		effect: effect, label: label, data: bytes.Clone(data), destination: destination,
		mode: mode, allowedRoot: root, relativeSuffix: suffix,
		before: cloneInstallPathSnapshot(before),
	}, nil
}

func addMutationAction(actions []mutationAction, action mutationAction) ([]mutationAction, error) {
	for _, existing := range actions {
		if existing.destination != action.destination {
			continue
		}
		if !equivalentMutationAction(existing, action) {
			return nil, compatibilityError("conflicting mutation actions for " + action.destination)
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
		return installAction{}, compatibilityError("install action must replace a destination")
	}
	return newInstallAction(
		action.label, action.data, action.destination, action.mode,
		action.allowedRoot, action.relativeSuffix, action.before,
	)
}
