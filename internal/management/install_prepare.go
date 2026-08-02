package management

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const maximumInstallArtifactBytes = 4 << 20

type installPathKind uint8

const (
	installPathMissing installPathKind = iota
	installPathRegular
	installPathDirectory
	installPathSymlink
	installPathOther
)

type installPathSnapshot struct {
	kind installPathKind
	mode fs.FileMode
	data []byte
	link string
}

type installAction struct {
	label          string
	data           []byte
	destination    string
	mode           fs.FileMode
	allowedRoot    string
	relativeSuffix string
	before         installPathSnapshot
}

type PreparedInstall struct {
	source             Source
	environment        Environment
	request            InstallRequest
	resolved           resolvedRequest
	coordinates        coordinates
	targetActions      []installAction
	policyAction       installAction
	stateActions       []installAction
	plannedDirectories []string
	predicted          Result
}

type policyStateReference struct {
	index int
	path  string
	state installationState
}

func PrepareInstall(source Source, environment Environment, request InstallRequest) (PreparedInstall, error) {
	source, err := NewSource(source.version, source.policy)
	if err != nil {
		return PreparedInstall{}, err
	}
	resolved, err := resolve(CheckRequest{Project: request.Project, Targets: request.Targets})
	if err != nil {
		return PreparedInstall{}, err
	}
	coords, err := initializeCoordinates(environment, resolved)
	if err != nil {
		return PreparedInstall{}, err
	}

	policyView, err := inspectInstallPath(coords.policyPath)
	if err != nil {
		return PreparedInstall{}, err
	}
	existingState, stateExists, err := readInstallationState(coords.stateFile)
	if err != nil {
		return PreparedInstall{}, installError(err)
	}
	if stateExists {
		if err := validateCurrentInstallState(existingState, coords, resolved, policyView); err != nil {
			return PreparedInstall{}, err
		}
		if !bytes.Equal(source.policy, policyView.data) || checksumBytes(source.policy) != existingState.policyChecksum || source.version != existingState.version {
			return PreparedInstall{}, compatibilityError("installed content differs from this checkout; run update")
		}
	}

	existingRecords := []targetRecord(nil)
	existingDirectories := []string(nil)
	backupPath := ""
	if stateExists {
		existingRecords = cloneTargetRecords(existingState.targets)
		existingDirectories = append([]string(nil), existingState.directories...)
		backupPath = existingState.backupPath
	}

	selectedRecords := make([]targetRecord, 0, len(resolved.targets))
	targetActions := make([]installAction, 0, len(resolved.targets))
	for _, id := range resolved.targets {
		destination, err := targetDestination(coords, resolved.scope, resolved.projectRoot, id)
		if err != nil {
			return PreparedInstall{}, err
		}
		allowedRoot, relativeSuffix, err := targetInstallCoordinates(coords, resolved, id)
		if err != nil {
			return PreparedInstall{}, err
		}
		current, err := inspectInstallPath(destination)
		if err != nil {
			return PreparedInstall{}, err
		}
		candidate, found := findTarget(id)
		if !found {
			return PreparedInstall{}, compatibilityError("unknown target '" + id + "'")
		}

		origin := "existing-file"
		recorded, wasInstalled := findTargetRecord(existingRecords, id)
		sharedChecksum := ""
		joinsShared := false
		if wasInstalled {
			if recorded.path != destination {
				return PreparedInstall{}, compatibilityError("installed target path does not match")
			}
			origin = recorded.origin
		} else {
			switch candidate.Ownership {
			case "managed-block":
				status, _ := managedInstallStatus(current)
				if destinationReferenced(existingRecords, destination) {
					if status != "present" {
						return PreparedInstall{}, compatibilityError("managed target block has drifted")
					}
					origin, err = sharedDestinationOrigin(existingRecords, destination)
					if err != nil {
						return PreparedInstall{}, err
					}
					sharedChecksum, err = sharedDestinationChecksum(existingRecords, destination)
					if err != nil {
						return PreparedInstall{}, err
					}
					joinsShared = true
				} else {
					if status != "absent" {
						return PreparedInstall{}, compatibilityError("untracked OAW markers already exist: " + destination)
					}
					if current.kind == installPathMissing {
						origin = "created-file"
					}
				}
			case "owned-file":
				if current.kind != installPathMissing {
					return PreparedInstall{}, compatibilityError("owned target already exists: " + destination)
				}
				origin = "created-file"
			default:
				return PreparedInstall{}, compatibilityError("unknown target ownership mode: " + candidate.Ownership)
			}
		}

		var rendered []byte
		var renderedChecksum string
		switch candidate.Ownership {
		case "managed-block":
			block, err := renderManagedBlock(targetID(id), scope(resolved.scope), coords.policyPath)
			if err != nil {
				return PreparedInstall{}, err
			}
			rendered, err = renderManagedFile(current.data, block)
			if err != nil {
				return PreparedInstall{}, err
			}
			renderedChecksum = checksumBytes(block)
		case "owned-file":
			rendered, err = renderTarget(targetID(id), scope(resolved.scope), coords.policyPath)
			if err != nil {
				return PreparedInstall{}, err
			}
			renderedChecksum = checksumBytes(rendered)
		}
		if joinsShared && renderedChecksum != sharedChecksum {
			return PreparedInstall{}, compatibilityError("conflicting target renders for " + destination)
		}
		if wasInstalled && renderedChecksum != recorded.checksum {
			return PreparedInstall{}, compatibilityError("installed content differs from this checkout; run update")
		}

		action, err := newInstallAction(id, rendered, destination, 0o644, allowedRoot, relativeSuffix, current)
		if err != nil {
			return PreparedInstall{}, err
		}
		targetActions, err = addInstallAction(targetActions, action)
		if err != nil {
			return PreparedInstall{}, err
		}
		selectedRecords = append(selectedRecords, targetRecord{
			id: id, path: destination, mode: candidate.Ownership,
			checksum: renderedChecksum, origin: origin,
		})
	}

	finalRecords, err := mergeInstallRecords(existingRecords, selectedRecords, resolved.scope)
	if err != nil {
		return PreparedInstall{}, err
	}
	references, err := collectPolicyStateReferences(coords, coords.stateFile, policyView)
	if err != nil {
		return PreparedInstall{}, err
	}
	finalDirectories, plannedDirectories, err := prepareInstallDirectories(coords, resolved, existingDirectories, references, targetActions, finalRecords)
	if err != nil {
		return PreparedInstall{}, err
	}

	currentState := installationState{
		version: source.version, scope: resolved.scope, project: resolved.projectRoot,
		policyPath: coords.policyPath, policyChecksum: checksumBytes(source.policy),
		backupPath: backupPath, directories: finalDirectories, targets: finalRecords,
	}
	stateBytes, err := serializeInstallState(currentState)
	if err != nil {
		return PreparedInstall{}, err
	}
	currentStateAction, err := newStateInstallAction("state", stateBytes, coords.stateFile, environment.StateHome)
	if err != nil {
		return PreparedInstall{}, err
	}
	stateActions := []installAction{currentStateAction}
	for _, reference := range references {
		updated := cloneInstallationStateValue(reference.state)
		updated.version = source.version
		updated.policyChecksum = checksumBytes(source.policy)
		rendered, err := serializeInstallState(updated)
		if err != nil {
			return PreparedInstall{}, err
		}
		action, err := newStateInstallAction(fmt.Sprintf("state-reference-%d", reference.index), rendered, reference.path, environment.StateHome)
		if err != nil {
			return PreparedInstall{}, err
		}
		stateActions, err = addInstallAction(stateActions, action)
		if err != nil {
			return PreparedInstall{}, err
		}
	}

	policyAction, err := newInstallAction(
		"policy", source.policy, coords.policyPath, 0o600, environment.ConfigHome,
		"open-agent-workflow/ENGINEERING.md", policyView,
	)
	if err != nil {
		return PreparedInstall{}, err
	}
	prepared := PreparedInstall{
		source: source, environment: environment, request: request, resolved: cloneResolvedRequest(resolved), coordinates: coords,
		targetActions: cloneInstallActions(targetActions), policyAction: cloneInstallAction(policyAction),
		stateActions: cloneInstallActions(stateActions), plannedDirectories: append([]string(nil), plannedDirectories...),
	}
	prepared.predicted = predictInstallResult(prepared)
	return prepared, nil
}

func validateCurrentInstallState(state installationState, coords coordinates, resolved resolvedRequest, policy installPathSnapshot) error {
	if state.scope != resolved.scope {
		return compatibilityError("installed scope does not match")
	}
	if resolved.scope == "user" {
		if state.project != "" {
			return compatibilityError("installed project root does not match")
		}
	} else if state.project != resolved.projectRoot {
		return compatibilityError("installed project root does not match")
	}
	if state.policyPath != coords.policyPath {
		return compatibilityError("installed policy path does not match")
	}
	if policy.kind != installPathRegular {
		return compatibilityError("managed policy is missing")
	}
	if checksumBytes(policy.data) != state.policyChecksum {
		return compatibilityError("managed policy has drifted")
	}
	if err := validateLiveTargetRecords(state.targets, coords, state.scope, state.project); err != nil {
		return err
	}
	if err := validateOwnedDirectories(state, coords); err != nil {
		return compatibilityError(err.Error())
	}
	return nil
}

func validateLiveTargetRecords(records []targetRecord, coords coordinates, recordScope, project string) error {
	for _, record := range records {
		if err := validateLiveTargetRecord(record, coords, recordScope, project); err != nil {
			return err
		}
	}
	return nil
}

func validateLiveTargetRecord(record targetRecord, coords coordinates, recordScope, project string) error {
	expected, err := targetDestination(coords, recordScope, project, record.id)
	if err != nil {
		return err
	}
	if record.path != expected {
		return compatibilityError(fmt.Sprintf("installed target path does not match: %s at %s", record.id, record.path))
	}
	candidate, _ := findTarget(record.id)
	if record.mode != candidate.Ownership {
		return compatibilityError(fmt.Sprintf("installed target ownership does not match: %s at %s", record.id, record.path))
	}
	current, err := inspectInstallPath(record.path)
	if err != nil {
		return err
	}
	switch record.mode {
	case "managed-block":
		status, checksum := managedInstallStatus(current)
		if status != "present" || checksum != record.checksum {
			return compatibilityError(fmt.Sprintf("managed target block has drifted: %s at %s", record.id, record.path))
		}
	case "owned-file":
		if current.kind != installPathRegular || checksumBytes(current.data) != record.checksum {
			return compatibilityError(fmt.Sprintf("owned target file has drifted: %s at %s", record.id, record.path))
		}
	default:
		return compatibilityError("unknown target ownership mode: " + record.mode)
	}
	return nil
}

func collectPolicyStateReferences(coords coordinates, excluded string, policy installPathSnapshot) ([]policyStateReference, error) {
	return collectPolicyStateReferencesWithBaseline(coords, excluded, policy, "")
}

func collectPolicyStateReferencesWithBaseline(coords coordinates, excluded string, policy installPathSnapshot, baseline string) ([]policyStateReference, error) {
	locations := []struct {
		directory string
		pattern   string
	}{
		{directory: coords.installations, pattern: coords.installations + string(filepath.Separator) + "*.state"},
		{directory: coords.projects, pattern: coords.projects + string(filepath.Separator) + "*.state"},
	}
	result := make([]policyStateReference, 0)
	index := 0
	for _, location := range locations {
		paths, err := filepath.Glob(location.pattern)
		if err != nil {
			return nil, compatibilityError("invalid installation state pattern")
		}
		for _, discovered := range paths {
			// Glob cleans repeated path separators. Rebuild from the validated
			// lexical root so state actions retain the caller's allowed root.
			path := location.directory + string(filepath.Separator) + filepath.Base(discovered)
			if path == excluded {
				continue
			}
			index++
			if err := validateStateActionPath(coords.environment.StateHome, path); err != nil {
				return nil, err
			}
			state, exists, err := readInstallationState(path)
			if err != nil {
				return nil, installError(err)
			}
			if !exists {
				return nil, compatibilityError("installation state is not a regular file: " + path)
			}
			if state.policyPath != coords.policyPath {
				continue
			}
			if err := validatePolicyStateReference(path, state, coords, policy, baseline); err != nil {
				return nil, err
			}
			result = append(result, policyStateReference{index: index, path: path, state: cloneInstallationStateValue(state)})
		}
	}
	return result, nil
}

func validatePolicyStateReference(path string, state installationState, coords coordinates, policy installPathSnapshot, baseline string) error {
	var expected string
	switch state.scope {
	case "user":
		expected = coords.installations + string(filepath.Separator) + "user.state"
	case "project":
		identity := strings.Replace(checksumBytes([]byte(state.project)), ":", "-", 1)
		expected = coords.projects + string(filepath.Separator) + identity + ".state"
	default:
		return compatibilityError("installed scope does not match")
	}
	if path != expected {
		if state.scope == "user" {
			return compatibilityError("installed user state path does not match")
		}
		return compatibilityError("installed project root does not match")
	}
	if err := validateLiveTargetRecords(state.targets, coords, state.scope, state.project); err != nil {
		return err
	}
	if err := validateOwnedDirectories(state, coords); err != nil {
		return compatibilityError(err.Error())
	}
	installedChecksum := ""
	if policy.kind == installPathRegular {
		installedChecksum = checksumBytes(policy.data)
	}
	if baseline != "" {
		installedChecksum = baseline
	}
	if installedChecksum == "" || installedChecksum != state.policyChecksum {
		return compatibilityError("managed policy has drifted")
	}
	return nil
}

func prepareInstallDirectories(
	coords coordinates,
	resolved resolvedRequest,
	existing []string,
	references []policyStateReference,
	actions []installAction,
	records []targetRecord,
) ([]string, []string, error) {
	directories := append([]string(nil), existing...)
	planned := make([]string, 0)
	for _, reference := range references {
		for _, directory := range reference.state.directories {
			if installNamespaceDirectory(coords, directory) {
				directories = appendUniqueString(directories, directory)
			}
		}
	}
	namespaces := []string{coords.configDir, coords.stateDir, coords.installations}
	if resolved.scope == "project" {
		namespaces = append(namespaces, coords.projects)
	}
	for _, directory := range namespaces {
		missing, err := installPathIsMissing(directory)
		if err != nil {
			return nil, nil, err
		}
		if missing {
			directories = appendUniqueString(directories, directory)
			planned = appendUniqueString(planned, directory)
		}
	}
	for _, action := range actions {
		parentSuffix := filepath.ToSlash(filepath.Dir(filepath.FromSlash(action.relativeSuffix)))
		if parentSuffix == "." {
			continue
		}
		consumed := ""
		for _, component := range strings.Split(parentSuffix, "/") {
			if consumed == "" {
				consumed = component
			} else {
				consumed += "/" + component
			}
			directory, err := validatedDestinationPath(action.allowedRoot, consumed)
			if err != nil {
				return nil, nil, err
			}
			missing, err := installPathIsMissing(directory)
			if err != nil {
				return nil, nil, err
			}
			if missing {
				directories = appendUniqueString(directories, directory)
				planned = appendUniqueString(planned, directory)
			}
		}
	}
	state := installationState{scope: resolved.scope, project: resolved.projectRoot, directories: directories, targets: records}
	if err := validateOwnedDirectories(state, coords); err != nil {
		return nil, nil, compatibilityError(err.Error())
	}
	return directories, planned, nil
}

func mergeInstallRecords(existing, selected []targetRecord, recordScope string) ([]targetRecord, error) {
	existingByID := make(map[string]targetRecord, len(existing))
	selectedByID := make(map[string]targetRecord, len(selected))
	changedChecksums := make(map[string]string)
	for _, record := range existing {
		existingByID[record.id] = record
	}
	for _, record := range selected {
		selectedByID[record.id] = record
		if previous, exists := changedChecksums[record.path]; exists && previous != record.checksum {
			return nil, compatibilityError("selected targets render conflicting checksums for " + record.path)
		}
		changedChecksums[record.path] = record.checksum
	}
	result := make([]targetRecord, 0, len(existing)+len(selected))
	for _, candidate := range targetRegistry {
		record, exists := selectedByID[candidate.ID]
		if !exists {
			record, exists = existingByID[candidate.ID]
		}
		if !exists {
			continue
		}
		if checksum, changed := changedChecksums[record.path]; changed {
			record.checksum = checksum
		}
		result = append(result, record)
	}
	state := installationState{scope: recordScope, targets: result}
	if err := validateTargetRecords(state); err != nil {
		return nil, compatibilityError(err.Error())
	}
	return result, nil
}

func targetInstallCoordinates(coords coordinates, resolved resolvedRequest, id string) (string, string, error) {
	candidate, found := findTarget(id)
	if !found {
		return "", "", compatibilityError("unknown target '" + id + "'")
	}
	if resolved.scope == "project" {
		return resolved.projectRoot, candidate.ProjectSuffix, nil
	}
	if !candidate.User {
		return "", "", &Error{Status: 69, Message: fmt.Sprintf("target '%s' is not implemented for user scope", id)}
	}
	if id == "opencode" {
		return coords.environment.ConfigHome, candidate.UserSuffix, nil
	}
	return coords.environment.Home, candidate.UserSuffix, nil
}

func newStateInstallAction(label string, data []byte, destination, root string) (installAction, error) {
	relative, err := stateActionRelativeSuffix(root, destination)
	if err != nil {
		return installAction{}, err
	}
	before, err := inspectInstallPath(destination)
	if err != nil {
		return installAction{}, err
	}
	return newInstallAction(label, data, destination, 0o600, root, relative, before)
}

func newInstallAction(label string, data []byte, destination string, mode fs.FileMode, root, suffix string, before installPathSnapshot) (installAction, error) {
	if !safeStateField(label) || label == "" || !safeStateField(destination) || !safeStateField(root) || !safeStateField(suffix) {
		return installAction{}, compatibilityError("target action cannot be serialized")
	}
	if mode != 0o600 && mode != 0o644 {
		return installAction{}, compatibilityError("invalid prepared destination mode")
	}
	rebuilt, err := validatedDestinationPath(root, suffix)
	if err != nil {
		return installAction{}, err
	}
	if rebuilt != destination {
		return installAction{}, compatibilityError("target action destination does not match registry: " + destination)
	}
	return installAction{
		label: label, data: bytes.Clone(data), destination: destination, mode: mode,
		allowedRoot: root, relativeSuffix: suffix, before: cloneInstallPathSnapshot(before),
	}, nil
}

func addInstallAction(actions []installAction, action installAction) ([]installAction, error) {
	for _, existing := range actions {
		if existing.destination != action.destination {
			continue
		}
		if existing.mode != action.mode || existing.allowedRoot != action.allowedRoot || existing.relativeSuffix != action.relativeSuffix || !bytes.Equal(existing.data, action.data) {
			return nil, compatibilityError("conflicting target renders for " + action.destination)
		}
		return actions, nil
	}
	return append(actions, cloneInstallAction(action)), nil
}

func inspectInstallPath(path string) (installPathSnapshot, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return installPathSnapshot{kind: installPathMissing}, nil
		}
		return installPathSnapshot{}, compatibilityError("destination path could not be inspected: " + path)
	}
	result := installPathSnapshot{mode: info.Mode()}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		result.kind = installPathSymlink
		result.link, err = os.Readlink(path)
	case info.Mode().IsRegular():
		result.kind = installPathRegular
		result.data, err = readBoundedInstallFile(path, info)
	case info.IsDir():
		result.kind = installPathDirectory
	default:
		result.kind = installPathOther
	}
	if err != nil {
		return installPathSnapshot{}, compatibilityError("destination path could not be read: " + path)
	}
	return result, nil
}

func readBoundedInstallFile(path string, info fs.FileInfo) ([]byte, error) {
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("file is not regular")
	}
	if info.Size() > maximumInstallArtifactBytes {
		return nil, fmt.Errorf("file exceeds read limit")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("file changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumInstallArtifactBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maximumInstallArtifactBytes {
		return nil, fmt.Errorf("file exceeds read limit")
	}
	current, err := os.Lstat(path)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
		return nil, fmt.Errorf("file changed while reading")
	}
	return data, nil
}

func managedInstallStatus(current installPathSnapshot) (string, string) {
	if current.kind == installPathMissing {
		return "absent", ""
	}
	if current.kind != installPathRegular {
		return "drift", ""
	}
	lines := managedLineSpans(current.data)
	beginCount, endCount, beginIndex, endIndex := 0, 0, -1, -1
	for index, line := range lines {
		switch string(current.data[line.start:line.contentEnd]) {
		case beginMarker:
			beginCount++
			if beginIndex == -1 {
				beginIndex = index
			}
		case endMarker:
			endCount++
			if endIndex == -1 {
				endIndex = index
			}
		}
	}
	if beginCount == 0 && endCount == 0 {
		return "absent", ""
	}
	if beginCount != 1 || endCount != 1 || beginIndex >= endIndex {
		return "drift", ""
	}
	block := append([]byte(nil), current.data[lines[beginIndex].start:lines[endIndex].end]...)
	if len(block) == 0 || block[len(block)-1] != '\n' {
		block = append(block, '\n')
	}
	return "present", checksumBytes(block)
}

func predictInstallResult(prepared PreparedInstall) Result {
	actions := make([]installAction, 0, len(prepared.targetActions)+1+len(prepared.stateActions))
	actions = append(actions, prepared.targetActions...)
	actions = append(actions, prepared.policyAction)
	actions = append(actions, prepared.stateActions...)
	lines := make([]string, 0, len(actions))
	for _, action := range actions {
		if action.before.kind == installPathRegular && bytes.Equal(action.before.data, action.data) {
			lines = append(lines, "oaw: unchanged: "+action.label)
			continue
		}
		verb := "would-update"
		if action.before.kind == installPathMissing {
			verb = "would-create"
		}
		lines = append(lines, "oaw: "+verb+": "+action.destination)
	}
	return Result{Lines: lines}
}

func stateActionRelativeSuffix(root, destination string) (string, error) {
	prefix := root + string(filepath.Separator)
	if !strings.HasPrefix(destination, prefix) {
		return "", compatibilityError("destination escapes its allowed root: " + destination)
	}
	relative := filepath.ToSlash(strings.TrimPrefix(destination, prefix))
	rebuilt, err := validatedDestinationPath(root, relative)
	if err != nil {
		return "", err
	}
	if rebuilt != destination {
		return "", compatibilityError("destination does not match its allowed root: " + destination)
	}
	return relative, nil
}

func validateStateActionPath(root, destination string) error {
	_, err := stateActionRelativeSuffix(root, destination)
	return err
}

func installPathIsMissing(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return false, nil
	}
	if os.IsNotExist(err) {
		return true, nil
	}
	return false, compatibilityError("destination path could not be inspected: " + path)
}

func installNamespaceDirectory(coords coordinates, path string) bool {
	return path == coords.configDir || path == coords.stateDir || path == coords.installations || path == coords.projects
}

func destinationReferenced(records []targetRecord, destination string) bool {
	for _, record := range records {
		if record.path == destination {
			return true
		}
	}
	return false
}

func sharedDestinationOrigin(records []targetRecord, destination string) (string, error) {
	value := ""
	for _, record := range records {
		if record.path != destination {
			continue
		}
		if value != "" && value != record.origin {
			return "", compatibilityError("conflicting target origins for " + destination)
		}
		value = record.origin
	}
	if value == "" {
		return "", compatibilityError("target destination is not referenced: " + destination)
	}
	return value, nil
}

func sharedDestinationChecksum(records []targetRecord, destination string) (string, error) {
	value := ""
	for _, record := range records {
		if record.path != destination {
			continue
		}
		if value != "" && value != record.checksum {
			return "", compatibilityError("conflicting target checksums for " + destination)
		}
		value = record.checksum
	}
	if value == "" {
		return "", compatibilityError("target destination is not referenced: " + destination)
	}
	return value, nil
}

func findTargetRecord(records []targetRecord, id string) (targetRecord, bool) {
	for _, record := range records {
		if record.id == id {
			return record, true
		}
	}
	return targetRecord{}, false
}
