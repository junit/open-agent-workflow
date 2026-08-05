package catalog

import (
	"errors"
	"fmt"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func validateCatalog(catalog *Catalog) error {
	providerIndex := make(map[string]ProviderDescriptorRecord, len(catalog.providers))
	for i := range catalog.providers {
		provider := &catalog.providers[i]
		if provider.SchemaVersion != ProviderDescriptorSchemaV3 {
			return fmt.Errorf("UNSUPPORTED_PROVIDER_SCHEMA: %q", provider.SchemaVersion)
		}
		if _, err := ParseQualifiedID(provider.ID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if _, exists := providerIndex[provider.ID]; exists {
			return errors.New("DUPLICATE_PROVIDER_ID: duplicate provider id")
		}
		providerIndex[provider.ID] = *provider
		if err := validateProviderMembers(provider); err != nil {
			return normalizeCatalogError(err)
		}
		capabilityIndex := make(map[string]CapabilityRecord, len(provider.Capabilities))
		for _, capability := range provider.Capabilities {
			capabilityIndex[capability.ID] = capability
			for _, target := range capability.DelegationAllowList {
				if _, exists := providerCapability(provider, target); !exists {
					return errors.New("DELEGATION_CAPABILITY_NOT_FOUND: capability is not declared")
				}
			}
		}
	}
	recipeIndex := make(map[string]ProfileRecipeRecord, len(catalog.recipes))
	for i := range catalog.recipes {
		recipe := &catalog.recipes[i]
		if recipe.SchemaVersion != ProfileRecipeSchemaV2 {
			return fmt.Errorf("UNSUPPORTED_RECIPE_SCHEMA: %q", recipe.SchemaVersion)
		}
		if recipe.EnvironmentRequirements == nil {
			return errors.New("INVALID_PROFILE_RECIPE: environment requirements are required")
		}
		requirements, err := execution.NormalizeRequirements(recipe.EnvironmentRequirements)
		if err != nil {
			return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
		}
		recipe.EnvironmentRequirements = requirements
		if _, exists := recipeIndex[recipe.ID]; exists {
			return errors.New("DUPLICATE_RECIPE_ID: duplicate recipe id")
		}
		recipeIndex[recipe.ID] = *recipe
		if err := validateRecipeGraph(recipe, providerIndex); err != nil {
			return err
		}
	}
	seenAliases := make(map[string]struct{}, len(catalog.aliases))
	for _, alias := range catalog.aliases {
		if _, err := ParseAlias(alias.Alias); err != nil {
			return fmt.Errorf("INVALID_PROFILE_ALIAS_SET: %w", err)
		}
		if _, exists := seenAliases[alias.Alias]; exists {
			return errors.New("DUPLICATE_PROFILE_ALIAS: duplicate alias")
		}
		seenAliases[alias.Alias] = struct{}{}
		if _, exists := recipeIndex[alias.RecipeID]; !exists {
			return errors.New("ALIAS_RECIPE_NOT_FOUND: alias target is not declared")
		}
	}
	return nil
}

func providerCapability(provider *ProviderDescriptorRecord, id string) (CapabilityRecord, bool) {
	for _, capability := range provider.Capabilities {
		if capability.ID == id {
			return capability, true
		}
	}
	return CapabilityRecord{}, false
}

func validateRecipeGraph(recipe *ProfileRecipeRecord, providers map[string]ProviderDescriptorRecord) error {
	nodes := make(map[string]RecipeNode, len(recipe.Nodes))
	for _, node := range recipe.Nodes {
		if _, exists := nodes[node.ID]; exists {
			return errors.New("DUPLICATE_RECIPE_NODE_ID: duplicate node id")
		}
		nodes[node.ID] = node
		provider, exists := providers[node.Selector.ProviderID]
		if !exists {
			return errors.New("RECIPE_PROVIDER_NOT_FOUND: selector provider is missing")
		}
		capability, exists := providerCapability(&provider, node.Selector.CapabilityID)
		if !exists {
			return errors.New("RECIPE_CAPABILITY_NOT_FOUND: selector capability is missing")
		}
		if node.Responsibility != "" && !contains(capability.Responsibilities, node.Responsibility) {
			return errors.New("CAPABILITY_RESPONSIBILITY_MISMATCH: capability does not declare node responsibility")
		}
		if node.Kind == ProcedureNode {
			phase, ok := nodes[node.Phase]
			if !ok || phase.Kind != PhaseNode {
				return errors.New("PROCEDURE_PHASE_INVALID: procedure phase is missing or not a phase")
			}
			if len(node.Transitions) != 0 {
				return errors.New("PROCEDURE_TRANSITION_FORBIDDEN: procedure has graph transitions")
			}
		}
		if node.Kind != ProcedureNode && node.Phase != "" {
			return errors.New("PROCEDURE_PHASE_INVALID: non-procedure has phase")
		}
	}
	owners := make(map[string]int)
	for _, node := range recipe.Nodes {
		if node.Responsibility != "" {
			owners[node.Responsibility]++
		}
	}
	for _, responsibility := range recipe.RequiredResponsibilities {
		if owners[responsibility] == 0 {
			return errors.New("RESPONSIBILITY_OWNER_MISSING: required responsibility has no owner")
		}
		if owners[responsibility] > 1 {
			return errors.New("RESPONSIBILITY_OWNER_DUPLICATE: required responsibility has multiple owners")
		}
	}
	if _, exists := nodes[recipe.Entry]; !exists {
		return errors.New("RECIPE_NODE_NOT_FOUND: entry node is missing")
	}
	for _, node := range recipe.Nodes {
		seenSignals := make(map[string]struct{})
		for _, transition := range node.Transitions {
			if _, exists := nodes[transition.Target]; !exists {
				return errors.New("RECIPE_NODE_NOT_FOUND: transition target is missing")
			}
			if _, exists := seenSignals[transition.Signal]; exists {
				return errors.New("DUPLICATE_TRANSITION_SIGNAL: transition signal is duplicated")
			}
			seenSignals[transition.Signal] = struct{}{}
		}
	}
	for _, route := range recipe.IncidentRoutes {
		handler, exists := nodes[route.Handler]
		if !exists || handler.Kind != IncidentHandlerNode {
			return errors.New("INCIDENT_HANDLER_INVALID: route target is not an incident handler")
		}
	}
	for _, gate := range recipe.TerminalGates {
		node, exists := nodes[gate]
		if !exists || node.Kind != GateNode {
			return errors.New("TERMINAL_GATE_INVALID: terminal gate is not a gate node")
		}
	}
	return nil
}

func normalizeCatalogError(err error) error {
	if err == nil {
		return nil
	}
	return err
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
