package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

func strictDecode(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func DecodeProvider(data []byte) (ProviderDescriptorRecord, error) {
	var record ProviderDescriptorRecord
	if err := strictDecode(data, &record); err != nil {
		return ProviderDescriptorRecord{}, fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
	}
	if record.SchemaVersion != ProviderDescriptorSchemaV1 {
		return ProviderDescriptorRecord{}, fmt.Errorf("UNSUPPORTED_PROVIDER_SCHEMA: %q", record.SchemaVersion)
	}
	if record.DescriptorVersion == "" || record.ID == "" || record.DisplayName == "" || record.Discovery == nil || record.Capabilities == nil {
		return ProviderDescriptorRecord{}, errors.New("INVALID_PROVIDER_DESCRIPTOR: required field missing")
	}
	if _, err := ParseContentVersion(record.DescriptorVersion); err != nil {
		return ProviderDescriptorRecord{}, fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
	}
	if _, err := ParseQualifiedID(record.ID); err != nil {
		return ProviderDescriptorRecord{}, fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
	}
	if err := validateProviderMembers(&record); err != nil {
		return ProviderDescriptorRecord{}, err
	}
	return cloneProvider(record), nil
}

func DecodeRecipe(data []byte) (ProfileRecipeRecord, error) {
	var record ProfileRecipeRecord
	if err := strictDecode(data, &record); err != nil {
		return ProfileRecipeRecord{}, fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
	}
	if record.SchemaVersion != ProfileRecipeSchemaV1 {
		return ProfileRecipeRecord{}, fmt.Errorf("UNSUPPORTED_RECIPE_SCHEMA: %q", record.SchemaVersion)
	}
	if record.RecipeVersion == "" || record.ID == "" || record.DisplayName == "" || record.RequiredResponsibilities == nil || record.Nodes == nil || record.IncidentRoutes == nil || record.TerminalGates == nil || record.StableBoundaries == nil || record.Entry == "" {
		return ProfileRecipeRecord{}, errors.New("INVALID_PROFILE_RECIPE: required field missing")
	}
	if _, err := ParseContentVersion(record.RecipeVersion); err != nil {
		return ProfileRecipeRecord{}, fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
	}
	if _, err := ParseQualifiedID(record.ID); err != nil {
		return ProfileRecipeRecord{}, fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
	}
	if err := validateRecipeMembers(&record); err != nil {
		return ProfileRecipeRecord{}, err
	}
	return cloneRecipe(record), nil
}

func DecodeAliasSet(data []byte) (ProfileAliasSetRecord, error) {
	var record ProfileAliasSetRecord
	if err := strictDecode(data, &record); err != nil {
		return ProfileAliasSetRecord{}, fmt.Errorf("INVALID_PROFILE_ALIAS_SET: %w", err)
	}
	if record.SchemaVersion != ProfileAliasSetSchemaV1 {
		return ProfileAliasSetRecord{}, fmt.Errorf("UNSUPPORTED_ALIAS_SET_SCHEMA: %q", record.SchemaVersion)
	}
	if record.Aliases == nil || len(record.Aliases) == 0 {
		return ProfileAliasSetRecord{}, errors.New("INVALID_PROFILE_ALIAS_SET: aliases must not be empty")
	}
	seen := make(map[string]struct{}, len(record.Aliases))
	for _, alias := range record.Aliases {
		if _, err := ParseAlias(alias.Alias); err != nil {
			return ProfileAliasSetRecord{}, fmt.Errorf("INVALID_PROFILE_ALIAS_SET: %w", err)
		}
		if _, err := ParseQualifiedID(alias.RecipeID); err != nil {
			return ProfileAliasSetRecord{}, fmt.Errorf("INVALID_PROFILE_ALIAS_SET: %w", err)
		}
		if _, exists := seen[alias.Alias]; exists {
			return ProfileAliasSetRecord{}, errors.New("DUPLICATE_PROFILE_ALIAS: duplicate alias")
		}
		seen[alias.Alias] = struct{}{}
	}
	return cloneAliases(record), nil
}

func validateProviderMembers(record *ProviderDescriptorRecord) error {
	probeIDs := map[string]struct{}{}
	for i := range record.Discovery {
		probe := &record.Discovery[i]
		if _, err := ParseLocalID(probe.ID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if _, exists := probeIDs[probe.ID]; exists {
			return errors.New("DUPLICATE_DISCOVERY_PROBE_ID: duplicate probe id")
		}
		probeIDs[probe.ID] = struct{}{}
		if err := uniqueStrings(probe.Paths, "DUPLICATE_DISCOVERY_PATH"); err != nil {
			return err
		}
		if err := validateProbe(probe); err != nil {
			return err
		}
	}
	capIDs := map[string]struct{}{}
	for i := range record.Capabilities {
		capability := &record.Capabilities[i]
		if _, err := ParseLocalID(capability.ID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if _, exists := capIDs[capability.ID]; exists {
			return errors.New("DUPLICATE_CAPABILITY_ID: duplicate capability id")
		}
		capIDs[capability.ID] = struct{}{}
		for _, values := range []struct {
			items []string
			code  string
		}{
			{capability.MaximumEffects, "DUPLICATE_CAPABILITY_EFFECT"},
			{capability.Resources, "DUPLICATE_CAPABILITY_RESOURCE"},
			{capability.Responsibilities, "DUPLICATE_CAPABILITY_RESPONSIBILITY"},
			{capability.DelegationAllowList, "DUPLICATE_DELEGATION_TARGET"},
		} {
			if err := uniqueStrings(values.items, values.code); err != nil {
				return err
			}
		}
		seenModes := make(map[RequestMode]struct{}, len(capability.RequestModes))
		for _, mode := range capability.RequestModes {
			if mode != RequestModeBounded && mode != RequestModeWorkflow {
				return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: invalid request mode %q", mode)
			}
			if _, exists := seenModes[mode]; exists {
				return errors.New("DUPLICATE_REQUEST_MODE: duplicate request mode")
			}
			seenModes[mode] = struct{}{}
		}
		if capability.InputSchema == "" || capability.OutcomeSchema == "" || len(capability.RequestModes) == 0 || len(capability.HostBindings) == 0 {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: capability contract is incomplete")
		}
		for _, effect := range capability.MaximumEffects {
			if effect != "read-project" && effect != "write-project" && effect != "run-process" && effect != "git-local" && effect != "network-read" {
				return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: invalid effect %q", effect)
			}
		}
		for _, resource := range capability.Resources {
			if resource != "project" && resource != "project-worktree" && resource != "git-repository" {
				return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: invalid resource %q", resource)
			}
		}
		if capability.ExecutorTopology != MainAgentAllowed && capability.ExecutorTopology != IsolatedRequired {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: invalid executor topology %q", capability.ExecutorTopology)
		}
		bindingKeys := make(map[string]struct{}, len(capability.HostBindings))
		for _, binding := range capability.HostBindings {
			if binding.Host == "" || binding.Reference == "" || (binding.Kind != "skill" && binding.Kind != "agent" && binding.Kind != "tool") {
				return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: invalid host binding")
			}
			key := binding.Host + "\x00" + binding.Kind + "\x00" + binding.Reference
			if _, exists := bindingKeys[key]; exists {
				return errors.New("DUPLICATE_HOST_BINDING: duplicate host binding")
			}
			bindingKeys[key] = struct{}{}
		}
	}
	return nil
}

func validateRecipeMembers(record *ProfileRecipeRecord) error {
	if err := uniqueStrings(record.RequiredResponsibilities, "DUPLICATE_RECIPE_RESPONSIBILITY"); err != nil {
		return err
	}
	if err := uniqueStrings(record.TerminalGates, "DUPLICATE_TERMINAL_GATE"); err != nil {
		return err
	}
	if err := uniqueStrings(record.StableBoundaries, "DUPLICATE_STABLE_BOUNDARY"); err != nil {
		return err
	}
	nodeIDs := make(map[string]struct{}, len(record.Nodes))
	for i := range record.Nodes {
		node := &record.Nodes[i]
		if _, err := ParseLocalID(node.ID); err != nil {
			return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
		}
		if _, exists := nodeIDs[node.ID]; exists {
			return errors.New("DUPLICATE_RECIPE_NODE_ID: duplicate node id")
		}
		nodeIDs[node.ID] = struct{}{}
		if node.Kind != PhaseNode && node.Kind != ProcedureNode && node.Kind != IncidentHandlerNode && node.Kind != CheckpointNode && node.Kind != GateNode {
			return fmt.Errorf("INVALID_PROFILE_RECIPE: invalid node kind %q", node.Kind)
		}
		if node.Kind != CheckpointNode && node.Responsibility == "" {
			return errors.New("INVALID_PROFILE_RECIPE: node responsibility is required")
		}
		if node.Kind == ProcedureNode && (node.Phase == "" || len(node.Transitions) != 0) {
			return errors.New("INVALID_PROFILE_RECIPE: procedure phase and empty transitions are required")
		}
		if node.Kind != ProcedureNode && node.Phase != "" {
			return errors.New("INVALID_PROFILE_RECIPE: non-procedure phase is forbidden")
		}
		if _, err := ParseQualifiedID(node.Selector.ProviderID); err != nil {
			return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
		}
		if _, err := ParseLocalID(node.Selector.CapabilityID); err != nil {
			return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
		}
		seenSignals := make(map[string]struct{}, len(node.Transitions))
		for _, transition := range node.Transitions {
			if transition.Signal != "succeeded" && transition.Signal != "finding" && transition.Signal != "remediated" {
				return fmt.Errorf("INVALID_PROFILE_RECIPE: invalid transition signal %q", transition.Signal)
			}
			if _, exists := seenSignals[transition.Signal]; exists {
				return errors.New("DUPLICATE_TRANSITION_SIGNAL: duplicate transition signal")
			}
			seenSignals[transition.Signal] = struct{}{}
		}
	}
	routeKeys := make(map[string]struct{}, len(record.IncidentRoutes))
	for _, route := range record.IncidentRoutes {
		if route.Incident != "functional-failure" && route.Incident != "build-failure" && route.Incident != "dependency-failure" && route.Incident != "type-failure" && route.Incident != "security-finding" {
			return fmt.Errorf("INVALID_PROFILE_RECIPE: invalid incident %q", route.Incident)
		}
		if _, exists := routeKeys[route.Incident]; exists {
			return errors.New("DUPLICATE_INCIDENT_ROUTE: duplicate incident")
		}
		routeKeys[route.Incident] = struct{}{}
	}
	return nil
}

func validateProbe(probe *DiscoveryProbe) error {
	if probe.Kind != "path-exists" && probe.Kind != "all-paths-exist" && probe.Kind != "one-level-version-path-exists" {
		return fmt.Errorf("DISCOVERY_PROBE_SHAPE_INVALID: invalid discovery kind %q", probe.Kind)
	}
	if probe.Root != "user-home" && probe.Root != "xdg-config-home" && probe.Root != "project-root" {
		return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: invalid discovery root %q", probe.Root)
	}
	paths := make([]string, 0, len(probe.Paths)+2)
	switch probe.Kind {
	case "path-exists":
		if probe.Path == "" || probe.Prefix != "" || probe.Suffix != "" || len(probe.Paths) != 0 {
			return errors.New("DISCOVERY_PROBE_SHAPE_INVALID: path-exists payload mismatch")
		}
		paths = append(paths, probe.Path)
	case "all-paths-exist":
		if len(probe.Paths) == 0 || probe.Path != "" || probe.Prefix != "" || probe.Suffix != "" {
			return errors.New("DISCOVERY_PROBE_SHAPE_INVALID: all-paths-exist payload mismatch")
		}
		paths = append(paths, probe.Paths...)
	case "one-level-version-path-exists":
		if probe.Prefix == "" || probe.Suffix == "" || probe.Path != "" || len(probe.Paths) != 0 {
			return errors.New("DISCOVERY_PROBE_SHAPE_INVALID: version probe payload mismatch")
		}
		paths = append(paths, probe.Prefix, probe.Suffix)
	}
	for _, value := range paths {
		if !safeRelativePath(value) {
			return fmt.Errorf("DISCOVERY_PATH_INVALID: %q", value)
		}
	}
	return nil
}

func safeRelativePath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." || strings.ContainsAny(component, "*?[]{}()") {
			return false
		}
	}
	return true
}

func uniqueStrings(values []string, code string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			return errors.New(code + ": duplicate value")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func cloneProvider(record ProviderDescriptorRecord) ProviderDescriptorRecord {
	record.Discovery = append([]DiscoveryProbe(nil), record.Discovery...)
	for i := range record.Discovery {
		record.Discovery[i].Paths = append([]string(nil), record.Discovery[i].Paths...)
	}
	record.Capabilities = append([]CapabilityRecord(nil), record.Capabilities...)
	for i := range record.Capabilities {
		record.Capabilities[i].MaximumEffects = append([]string(nil), record.Capabilities[i].MaximumEffects...)
		record.Capabilities[i].Resources = append([]string(nil), record.Capabilities[i].Resources...)
		record.Capabilities[i].RequestModes = append([]RequestMode(nil), record.Capabilities[i].RequestModes...)
		record.Capabilities[i].Responsibilities = append([]string(nil), record.Capabilities[i].Responsibilities...)
		record.Capabilities[i].DelegationAllowList = append([]string(nil), record.Capabilities[i].DelegationAllowList...)
		record.Capabilities[i].HostBindings = append([]HostBinding(nil), record.Capabilities[i].HostBindings...)
	}
	return record
}

func cloneRecipe(record ProfileRecipeRecord) ProfileRecipeRecord {
	record.RequiredResponsibilities = append([]string(nil), record.RequiredResponsibilities...)
	record.Nodes = append([]RecipeNode(nil), record.Nodes...)
	record.IncidentRoutes = append([]IncidentRoute(nil), record.IncidentRoutes...)
	record.TerminalGates = append([]string(nil), record.TerminalGates...)
	record.StableBoundaries = append([]string(nil), record.StableBoundaries...)
	for i := range record.Nodes {
		record.Nodes[i].Transitions = append([]RecipeTransition(nil), record.Nodes[i].Transitions...)
	}
	return record
}

func cloneAliases(record ProfileAliasSetRecord) ProfileAliasSetRecord {
	record.Aliases = append([]ProfileAliasRecord(nil), record.Aliases...)
	return record
}
