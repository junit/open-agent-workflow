package management

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	oaw "github.com/wifibaby4u/open-agent-workflow"
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
	policySetActions   []installAction
	stateActions       []installAction
	plannedDirectories []string
	predicted          Result
}

func PrepareInstall(source Source, environment Environment, request InstallRequest) (PreparedInstall, error) {
	source, err := validateSource(source)
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
	policyContent, err := sourcePolicyContent(source, coords)
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
	if !stateExists {
		if err := requireUnclaimedPolicySetDirectory(coords); err != nil {
			return PreparedInstall{}, err
		}
	}
	if stateExists {
		if err := validateCurrentInstallState(existingState, coords, resolved, policyView); err != nil {
			return PreparedInstall{}, err
		}
		if !bytes.Equal(policyContent, policyView.data) || checksumBytes(policyContent) != existingState.policyChecksum || source.version != existingState.version {
			return PreparedInstall{}, integrityError("installed content differs from this checkout; run update")
		}
		if !policySetMatchesSource(existingState, source, coords, resolved) {
			return PreparedInstall{}, integrityError("installed content differs from this checkout; run update")
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
			return PreparedInstall{}, integrityError("unknown target '" + id + "'")
		}

		origin := "existing-file"
		recorded, wasInstalled := findTargetRecord(existingRecords, id)
		sharedChecksum := ""
		joinsShared := false
		if wasInstalled {
			if recorded.path != destination {
				return PreparedInstall{}, integrityError("installed target path does not match")
			}
			origin = recorded.origin
		} else {
			switch candidate.Ownership {
			case "managed-block":
				status, _ := managedInstallStatus(current)
				if destinationReferenced(existingRecords, destination) {
					if status != "present" {
						return PreparedInstall{}, integrityError("managed target block has drifted")
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
						return PreparedInstall{}, integrityError("untracked OAW markers already exist: " + destination)
					}
					if current.kind == installPathMissing {
						origin = "created-file"
					}
				}
			case "owned-file":
				if current.kind != installPathMissing {
					return PreparedInstall{}, integrityError("owned target already exists: " + destination)
				}
				origin = "created-file"
			default:
				return PreparedInstall{}, integrityError("unknown target ownership mode: " + candidate.Ownership)
			}
		}

		var rendered []byte
		var renderedChecksum string
		switch candidate.Ownership {
		case "managed-block":
			block, err := renderManagedBlock(targetID(id), scope(resolved.scope), policyRouterReference(coords))
			if err != nil {
				return PreparedInstall{}, err
			}
			rendered, err = renderManagedFile(current.data, block)
			if err != nil {
				return PreparedInstall{}, err
			}
			renderedChecksum = checksumBytes(block)
		case "owned-file":
			rendered, err = renderTarget(targetID(id), scope(resolved.scope), policyRouterReference(coords))
			if err != nil {
				return PreparedInstall{}, err
			}
			renderedChecksum = checksumBytes(rendered)
		}
		if joinsShared && renderedChecksum != sharedChecksum {
			return PreparedInstall{}, integrityError("conflicting target renders for " + destination)
		}
		if wasInstalled && renderedChecksum != recorded.checksum {
			return PreparedInstall{}, integrityError("installed content differs from this checkout; run update")
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
	policyAction, policySetActions, err := prepareInstallPolicyActions(source, coords, resolved)
	if err != nil {
		return PreparedInstall{}, err
	}
	directoryActions := append(cloneInstallActions(targetActions), cloneInstallAction(policyAction))
	directoryActions = append(directoryActions, cloneInstallActions(policySetActions)...)
	finalDirectories, plannedDirectories, err := prepareInstallDirectories(coords, resolved, existingDirectories, directoryActions, finalRecords)
	if err != nil {
		return PreparedInstall{}, err
	}

	currentState := installationState{
		version: source.version, scope: resolved.scope, project: resolved.projectRoot,
		policyPath: coords.policyPath, policyChecksum: checksumBytes(policyContent),
		policyFiles: policyFileRecordsFromInstallActions(policyAction, policySetActions),
		backupPath:  backupPath, directories: finalDirectories, targets: finalRecords,
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

	prepared := PreparedInstall{
		source: source, environment: environment, request: request, resolved: cloneResolvedRequest(resolved), coordinates: coords,
		targetActions: cloneInstallActions(targetActions), policyAction: cloneInstallAction(policyAction),
		policySetActions: cloneInstallActions(policySetActions),
		stateActions:     cloneInstallActions(stateActions), plannedDirectories: append([]string(nil), plannedDirectories...),
	}
	prepared.predicted = predictInstallResult(prepared)
	return prepared, nil
}

func requireUnclaimedPolicySetDirectory(coords coordinates) error {
	current, err := inspectInstallPath(coords.policyDir)
	if err != nil {
		return err
	}
	switch current.kind {
	case installPathMissing:
		return nil
	case installPathDirectory:
		entries, err := os.ReadDir(coords.policyDir)
		if err != nil {
			return integrityError("Policy Set directory could not be inspected")
		}
		if len(entries) == 0 {
			return nil
		}
		if coords.currentScope == "user" {
			return requireUserPolicySetDirectoryContainsOnlyCustomProfiles(coords, entries)
		}
	}
	return integrityError("untracked managed Policy Set content exists: " + coords.policyDir)
}

func requireUserPolicySetDirectoryContainsOnlyCustomProfiles(coords coordinates, entries []os.DirEntry) error {
	for _, entry := range entries {
		if entry.Name() != "profiles" || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return integrityError("untracked managed Policy Set content exists: " + filepath.Join(coords.policyDir, entry.Name()))
		}
	}
	profiles, err := os.ReadDir(coords.customProfilesDir)
	if err != nil {
		return integrityError("user Custom Profile directory could not be inspected")
	}
	for _, entry := range profiles {
		if entry.Name() != "builtin" {
			continue
		}
		builtin := filepath.Join(coords.customProfilesDir, "builtin")
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return integrityError("untracked managed Policy Set content exists: " + builtin)
		}
		children, err := os.ReadDir(builtin)
		if err != nil || len(children) != 0 {
			return integrityError("untracked managed Policy Set content exists: " + builtin)
		}
	}
	return nil
}

func sourcePolicyContent(source Source, coords coordinates) ([]byte, error) {
	for _, file := range source.policySet {
		if file.Path == "POLICY.md" {
			return policySetFileContent(coords, file), nil
		}
	}
	return nil, integrityError("managed Policy Set is missing POLICY.md")
}

func policySetFileContent(coords coordinates, file oaw.PolicyFile) []byte {
	content := bytes.Clone(file.Content)
	if coords.currentScope == "user" {
		content = bytes.ReplaceAll(content, []byte("](profiles/"), []byte("](profiles/builtin/"))
	}
	return content
}

func prepareInstallPolicyActions(source Source, coords coordinates, resolved resolvedRequest) (installAction, []installAction, error) {
	var primary installAction
	extras := make([]installAction, 0, len(source.policySet)-1)
	for _, file := range source.policySet {
		destination, root, suffix, err := policySetDestination(coords, resolved, file.Path)
		if err != nil {
			return installAction{}, nil, err
		}
		current, err := inspectInstallPath(destination)
		if err != nil {
			return installAction{}, nil, err
		}
		label := "policy/" + file.Path
		if file.Path == "POLICY.md" {
			label = "policy"
		}
		action, err := newInstallAction(label, policySetFileContent(coords, file), destination, 0o644, root, suffix, current)
		if err != nil {
			return installAction{}, nil, err
		}
		if file.Path == "POLICY.md" {
			primary = action
		} else {
			extras = append(extras, action)
		}
	}
	if primary.destination == "" {
		return installAction{}, nil, integrityError("managed Policy Set is missing POLICY.md")
	}
	return primary, extras, nil
}

func policyFileRecordsFromInstallActions(primary installAction, extras []installAction) []policyFileRecord {
	if len(extras) == 0 {
		return nil
	}
	actions := append([]installAction{primary}, extras...)
	records := make([]policyFileRecord, 0, len(actions))
	for _, action := range actions {
		records = append(records, policyFileRecord{path: action.destination, checksum: checksumBytes(action.data)})
	}
	return records
}

func policySetMatchesSource(state installationState, source Source, coords coordinates, resolved resolvedRequest) bool {
	if len(source.policySet) == 0 || len(state.policyFiles) != len(source.policySet) {
		return false
	}
	want := make(map[string]string, len(source.policySet))
	for _, file := range source.policySet {
		destination, _, _, err := policySetDestination(coords, resolved, file.Path)
		if err != nil {
			return false
		}
		want[filepath.Clean(destination)] = checksumBytes(policySetFileContent(coords, file))
	}
	for _, record := range state.policyFiles {
		if want[filepath.Clean(record.path)] != record.checksum {
			return false
		}
		delete(want, filepath.Clean(record.path))
	}
	return len(want) == 0
}

func validateCurrentInstallState(state installationState, coords coordinates, resolved resolvedRequest, policy installPathSnapshot) error {
	if state.scope != resolved.scope {
		return integrityError("installed scope does not match")
	}
	if resolved.scope == "user" {
		if state.project != "" {
			return integrityError("installed project root does not match")
		}
	} else if state.project != resolved.projectRoot {
		return integrityError("installed project root does not match")
	}
	if state.policyPath != coords.policyPath {
		return integrityError("installed policy path does not match")
	}
	if policy.kind != installPathRegular {
		return integrityError("managed policy is missing")
	}
	if checksumBytes(policy.data) != state.policyChecksum {
		return integrityError("managed policy has drifted")
	}
	if err := validatePolicySetFiles(state, coords); err != nil {
		return err
	}
	if err := validateLiveTargetRecords(state.targets, coords, state.scope, state.project); err != nil {
		return err
	}
	if err := validateOwnedDirectories(state, coords); err != nil {
		return integrityError(err.Error())
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
		return integrityError(fmt.Sprintf("installed target path does not match: %s at %s", record.id, record.path))
	}
	candidate, _ := findTarget(record.id)
	if record.mode != candidate.Ownership {
		return integrityError(fmt.Sprintf("installed target ownership does not match: %s at %s", record.id, record.path))
	}
	current, err := inspectInstallPath(record.path)
	if err != nil {
		return err
	}
	switch record.mode {
	case "managed-block":
		status, checksum := managedInstallStatus(current)
		if status != "present" || checksum != record.checksum {
			return integrityError(fmt.Sprintf("managed target block has drifted: %s at %s", record.id, record.path))
		}
	case "owned-file":
		if current.kind != installPathRegular || checksumBytes(current.data) != record.checksum {
			return integrityError(fmt.Sprintf("owned target file has drifted: %s at %s", record.id, record.path))
		}
	default:
		return integrityError("unknown target ownership mode: " + record.mode)
	}
	return nil
}

func prepareInstallDirectories(
	coords coordinates,
	resolved resolvedRequest,
	existing []string,
	actions []installAction,
	records []targetRecord,
) ([]string, []string, error) {
	directories := append([]string(nil), existing...)
	planned := make([]string, 0)
	namespaces := []string{coords.policyDir, coords.stateDir, coords.installations}
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
		return nil, nil, integrityError(err.Error())
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
			return nil, integrityError("selected targets render conflicting checksums for " + record.path)
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
		return nil, integrityError(err.Error())
	}
	return result, nil
}

func targetInstallCoordinates(coords coordinates, resolved resolvedRequest, id string) (string, string, error) {
	candidate, found := findTarget(id)
	if !found {
		return "", "", integrityError("unknown target '" + id + "'")
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
		return installAction{}, integrityError("target action cannot be serialized")
	}
	if mode != 0o600 && mode != 0o644 {
		return installAction{}, integrityError("invalid prepared destination mode")
	}
	rebuilt, err := validatedDestinationPath(root, suffix)
	if err != nil {
		return installAction{}, err
	}
	if rebuilt != destination {
		return installAction{}, integrityError("target action destination does not match registry: " + destination)
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
			return nil, integrityError("conflicting target renders for " + action.destination)
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
		return installPathSnapshot{}, integrityError("destination path could not be inspected: " + path)
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
		return installPathSnapshot{}, integrityError("destination path could not be read: " + path)
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
	actions := make([]installAction, 0, len(prepared.targetActions)+1+len(prepared.policySetActions)+len(prepared.stateActions))
	actions = append(actions, prepared.policyAction)
	actions = append(actions, prepared.policySetActions...)
	actions = append(actions, prepared.targetActions...)
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
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(destination))
	if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", integrityError("destination escapes its allowed root: " + destination)
	}
	relative = filepath.ToSlash(relative)
	rebuilt, err := validatedDestinationPath(root, relative)
	if err != nil {
		return "", err
	}
	if !matchesValidatedDestination(rebuilt, destination) {
		return "", integrityError("destination does not match its allowed root: " + destination)
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
	return false, integrityError("destination path could not be inspected: " + path)
}

func installNamespaceDirectory(coords coordinates, path string) bool {
	if isPolicySetDirectory(coords, coords.currentProject, path) {
		return true
	}
	return path == coords.configDir || path == coords.stateDir || path == coords.installations || path == coords.projects
}

func isPolicySetDirectory(coords coordinates, projectRoot, candidate string) bool {
	root := projectRoot
	if coords.currentScope == "user" {
		root = coords.environment.ConfigHome
	}
	if root == "" || !containedStrictly(root, candidate) {
		return false
	}
	withinPolicySet := candidate == coords.policyDir || containedStrictly(candidate, coords.policyDir) ||
		containedStrictly(coords.policyDir, candidate)
	if !withinPolicySet || coords.currentScope != "user" {
		return withinPolicySet
	}
	return candidate == coords.customProfilesDir || !userCustomProfilePath(coords, candidate)
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
			return "", integrityError("conflicting target origins for " + destination)
		}
		value = record.origin
	}
	if value == "" {
		return "", integrityError("target destination is not referenced: " + destination)
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
			return "", integrityError("conflicting target checksums for " + destination)
		}
		value = record.checksum
	}
	if value == "" {
		return "", integrityError("target destination is not referenced: " + destination)
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
