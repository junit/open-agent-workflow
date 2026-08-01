package config

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/BurntSushi/toml"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

const maximumTOMLBytes = 1 << 20

func DecodeUser(raw []byte, registry *schema.Registry) (Decoded[UserConfigRecord], error) {
	var record UserConfigRecord
	if err := strictTOML(raw, &record); err != nil {
		return Decoded[UserConfigRecord]{}, err
	}
	if record.SchemaVersion != UserConfigSchemaV1 {
		return Decoded[UserConfigRecord]{}, fmt.Errorf("UNSUPPORTED_USER_CONFIG_SCHEMA: %q", record.SchemaVersion)
	}
	if err := normalizeUser(&record); err != nil {
		return Decoded[UserConfigRecord]{}, err
	}
	digest, encoded, err := canonicaljson.Digest(record)
	if err != nil {
		return Decoded[UserConfigRecord]{}, err
	}
	if err := registry.Validate(schema.UserConfigV1, encoded); err != nil {
		return Decoded[UserConfigRecord]{}, fmt.Errorf("INVALID_USER_CONFIG: %w", err)
	}
	return Decoded[UserConfigRecord]{Record: record, CanonicalJSON: encoded, Digest: digest}, nil
}

func DecodeProject(raw []byte, registry *schema.Registry) (Decoded[ProjectConfigRecord], error) {
	var record ProjectConfigRecord
	if err := strictTOML(raw, &record); err != nil {
		return Decoded[ProjectConfigRecord]{}, err
	}
	if record.SchemaVersion != ProjectConfigSchemaV1 {
		return Decoded[ProjectConfigRecord]{}, fmt.Errorf("UNSUPPORTED_PROJECT_CONFIG_SCHEMA: %q", record.SchemaVersion)
	}
	if err := normalizeProject(&record); err != nil {
		return Decoded[ProjectConfigRecord]{}, err
	}
	digest, encoded, err := canonicaljson.Digest(record)
	if err != nil {
		return Decoded[ProjectConfigRecord]{}, err
	}
	if err := registry.Validate(schema.ProjectConfigV1, encoded); err != nil {
		return Decoded[ProjectConfigRecord]{}, fmt.Errorf("INVALID_PROJECT_CONFIG: %w", err)
	}
	return Decoded[ProjectConfigRecord]{Record: record, CanonicalJSON: encoded, Digest: digest}, nil
}

func DecodeProvider(raw []byte, registry *schema.Registry) (Decoded[catalog.ProviderDescriptorRecord], error) {
	var record catalog.ProviderDescriptorRecord
	if err := strictTOML(raw, &record); err != nil {
		return Decoded[catalog.ProviderDescriptorRecord]{}, err
	}
	normalizeProvider(&record)
	_, encoded, err := canonicaljson.Digest(record)
	if err != nil {
		return Decoded[catalog.ProviderDescriptorRecord]{}, err
	}
	if err := registry.Validate(schema.ProviderDescriptorV1, encoded); err != nil {
		return Decoded[catalog.ProviderDescriptorRecord]{}, fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
	}
	validated, err := catalog.DecodeProvider(encoded)
	if err != nil {
		return Decoded[catalog.ProviderDescriptorRecord]{}, err
	}
	digest, canonical, err := canonicaljson.Digest(validated)
	return Decoded[catalog.ProviderDescriptorRecord]{Record: validated, CanonicalJSON: canonical, Digest: digest}, err
}

func DecodeRecipe(raw []byte, registry *schema.Registry) (Decoded[catalog.ProfileRecipeRecord], error) {
	var record catalog.ProfileRecipeRecord
	if err := strictTOML(raw, &record); err != nil {
		return Decoded[catalog.ProfileRecipeRecord]{}, err
	}
	normalizeRecipe(&record)
	_, encoded, err := canonicaljson.Digest(record)
	if err != nil {
		return Decoded[catalog.ProfileRecipeRecord]{}, err
	}
	if err := registry.Validate(schema.ProfileRecipeV1, encoded); err != nil {
		return Decoded[catalog.ProfileRecipeRecord]{}, fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
	}
	validated, err := catalog.DecodeRecipe(encoded)
	if err != nil {
		return Decoded[catalog.ProfileRecipeRecord]{}, err
	}
	digest, canonical, err := canonicaljson.Digest(validated)
	return Decoded[catalog.ProfileRecipeRecord]{Record: validated, CanonicalJSON: canonical, Digest: digest}, err
}

func strictTOML(raw []byte, destination any) error {
	if len(raw) > maximumTOMLBytes {
		return fmt.Errorf("CONFIG_TOO_LARGE: %d bytes", len(raw))
	}
	if !utf8.Valid(raw) {
		return fmt.Errorf("CONFIG_INVALID_UTF8")
	}
	metadata, err := toml.Decode(string(raw), destination)
	if err != nil {
		return fmt.Errorf("CONFIG_TOML_INVALID: %w", err)
	}
	if unknown := metadata.Undecoded(); len(unknown) != 0 {
		return fmt.Errorf("CONFIG_UNKNOWN_FIELD: %s", unknown[0].String())
	}
	return nil
}

func normalizeUser(record *UserConfigRecord) error {
	normalizeUserCollections(record)
	for _, id := range record.DeniedProviders {
		if _, err := catalog.ParseQualifiedID(id); err != nil {
			return fmt.Errorf("INVALID_USER_CONFIG: %w", err)
		}
	}
	if err := uniqueSorted(record.DeniedProviders, "DUPLICATE_DENIED_PROVIDER"); err != nil {
		return err
	}
	if err := normalizeReferences(record.ProviderDescriptors, "DUPLICATE_PROVIDER_REFERENCE"); err != nil {
		return err
	}
	if err := normalizeReferences(record.ProfileRecipes, "DUPLICATE_RECIPE_REFERENCE"); err != nil {
		return err
	}
	for _, pin := range record.ProviderPins {
		if _, err := catalog.ParseQualifiedID(pin.ID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_PIN: %w", err)
		}
		if pin.Location == "" && pin.Version == "" {
			return fmt.Errorf("INVALID_PROVIDER_PIN: %s has no location or version", pin.ID)
		}
	}
	sort.Slice(record.ProviderPins, func(i, j int) bool { return record.ProviderPins[i].ID < record.ProviderPins[j].ID })
	if err := uniqueBy(len(record.ProviderPins), func(i int) string { return record.ProviderPins[i].ID }, "DUPLICATE_PROVIDER_PIN"); err != nil {
		return err
	}
	for _, preference := range record.BindingPreferences {
		if _, err := catalog.ParseQualifiedID(preference.ProviderID); err != nil {
			return fmt.Errorf("INVALID_BINDING_PREFERENCE: %w", err)
		}
		if _, err := catalog.ParseLocalID(preference.CapabilityID); err != nil {
			return fmt.Errorf("INVALID_BINDING_PREFERENCE: %w", err)
		}
		if preference.Host == "" || preference.Reference == "" || (preference.Kind != "skill" && preference.Kind != "agent" && preference.Kind != "tool") {
			return fmt.Errorf("INVALID_BINDING_PREFERENCE: %s/%s", preference.ProviderID, preference.CapabilityID)
		}
	}
	sort.Slice(record.BindingPreferences, func(i, j int) bool {
		return bindingPreferenceKey(record.BindingPreferences[i]) < bindingPreferenceKey(record.BindingPreferences[j])
	})
	if err := uniqueBy(len(record.BindingPreferences), func(i int) string { return bindingPreferenceKey(record.BindingPreferences[i]) }, "DUPLICATE_BINDING_PREFERENCE"); err != nil {
		return err
	}
	for _, boundedDefault := range record.BoundedCapabilityDefaults {
		if _, err := catalog.ParseLocalID(boundedDefault.ID); err != nil {
			return fmt.Errorf("INVALID_BOUNDED_CAPABILITY_DEFAULT: %w", err)
		}
		if _, err := catalog.ParseQualifiedID(boundedDefault.ProviderID); err != nil {
			return fmt.Errorf("INVALID_BOUNDED_CAPABILITY_DEFAULT: %w", err)
		}
		if _, err := catalog.ParseLocalID(boundedDefault.CapabilityID); err != nil {
			return fmt.Errorf("INVALID_BOUNDED_CAPABILITY_DEFAULT: %w", err)
		}
	}
	sort.Slice(record.BoundedCapabilityDefaults, func(i, j int) bool {
		return record.BoundedCapabilityDefaults[i].ID < record.BoundedCapabilityDefaults[j].ID
	})
	if err := uniqueBy(len(record.BoundedCapabilityDefaults), func(i int) string {
		return record.BoundedCapabilityDefaults[i].ID
	}, "DUPLICATE_BOUNDED_CAPABILITY_DEFAULT"); err != nil {
		return err
	}
	for i := range record.ProjectTrust {
		normalizeStringSlice(&record.ProjectTrust[i].DescriptorDigests)
		normalizeStringSlice(&record.ProjectTrust[i].RecipeDigests)
	}
	sort.Slice(record.ProjectTrust, func(i, j int) bool { return record.ProjectTrust[i].Root < record.ProjectTrust[j].Root })
	return uniqueBy(len(record.ProjectTrust), func(i int) string { return record.ProjectTrust[i].Root }, "DUPLICATE_PROJECT_TRUST")
}

func normalizeProject(record *ProjectConfigRecord) error {
	normalizeStringSlice(&record.RequiredProviders)
	normalizeStringSlice(&record.RecommendedProviders)
	if record.ProviderDescriptors == nil {
		record.ProviderDescriptors = []ContentReference{}
	}
	if record.ProfileRecipes == nil {
		record.ProfileRecipes = []ContentReference{}
	}
	if record.CapabilityLimits == nil {
		record.CapabilityLimits = []CapabilityLimit{}
	}
	for _, ids := range [][]string{record.RequiredProviders, record.RecommendedProviders} {
		for _, id := range ids {
			if _, err := catalog.ParseQualifiedID(id); err != nil {
				return fmt.Errorf("INVALID_PROJECT_CONFIG: %w", err)
			}
		}
		if err := uniqueSorted(ids, "DUPLICATE_PROJECT_PROVIDER"); err != nil {
			return err
		}
	}
	if err := normalizeReferences(record.ProviderDescriptors, "DUPLICATE_PROVIDER_REFERENCE"); err != nil {
		return err
	}
	if err := normalizeReferences(record.ProfileRecipes, "DUPLICATE_RECIPE_REFERENCE"); err != nil {
		return err
	}
	for i := range record.CapabilityLimits {
		limit := &record.CapabilityLimits[i]
		if _, err := catalog.ParseQualifiedID(limit.ProviderID); err != nil {
			return fmt.Errorf("INVALID_CAPABILITY_LIMIT: %w", err)
		}
		normalizeStringSlice(&limit.CapabilityIDs)
		if len(limit.CapabilityIDs) == 0 {
			return fmt.Errorf("INVALID_CAPABILITY_LIMIT: %s has no capabilities", limit.ProviderID)
		}
		for _, id := range limit.CapabilityIDs {
			if _, err := catalog.ParseLocalID(id); err != nil {
				return fmt.Errorf("INVALID_CAPABILITY_LIMIT: %w", err)
			}
		}
		if err := uniqueSorted(limit.CapabilityIDs, "DUPLICATE_CAPABILITY_LIMIT_ID"); err != nil {
			return err
		}
	}
	sort.Slice(record.CapabilityLimits, func(i, j int) bool {
		return record.CapabilityLimits[i].ProviderID < record.CapabilityLimits[j].ProviderID
	})
	return uniqueBy(len(record.CapabilityLimits), func(i int) string { return record.CapabilityLimits[i].ProviderID }, "DUPLICATE_CAPABILITY_LIMIT")
}

func normalizeProvider(record *catalog.ProviderDescriptorRecord) {
	if record.Discovery == nil {
		record.Discovery = []catalog.DiscoveryProbe{}
	}
	if record.Capabilities == nil {
		record.Capabilities = []catalog.CapabilityRecord{}
	}
	for i := range record.Discovery {
		normalizeStringSlice(&record.Discovery[i].Paths)
	}
	sort.Slice(record.Discovery, func(i, j int) bool { return record.Discovery[i].ID < record.Discovery[j].ID })
	for i := range record.Capabilities {
		capability := &record.Capabilities[i]
		normalizeStringSlice(&capability.MaximumEffects)
		normalizeStringSlice(&capability.Resources)
		if capability.RequestModes == nil {
			capability.RequestModes = []catalog.RequestMode{}
		}
		sort.Slice(capability.RequestModes, func(i, j int) bool { return capability.RequestModes[i] < capability.RequestModes[j] })
		normalizeStringSlice(&capability.Responsibilities)
		normalizeStringSlice(&capability.DelegationAllowList)
		if capability.HostBindings == nil {
			capability.HostBindings = []catalog.HostBinding{}
		}
		sort.Slice(capability.HostBindings, func(i, j int) bool {
			return hostBindingKey(capability.HostBindings[i]) < hostBindingKey(capability.HostBindings[j])
		})
	}
	sort.Slice(record.Capabilities, func(i, j int) bool { return record.Capabilities[i].ID < record.Capabilities[j].ID })
}

func normalizeRecipe(record *catalog.ProfileRecipeRecord) {
	normalizeStringSlice(&record.RequiredResponsibilities)
	if record.Nodes == nil {
		record.Nodes = []catalog.RecipeNode{}
	}
	for i := range record.Nodes {
		if record.Nodes[i].Transitions == nil {
			record.Nodes[i].Transitions = []catalog.RecipeTransition{}
		}
		sort.Slice(record.Nodes[i].Transitions, func(left, right int) bool {
			first := record.Nodes[i].Transitions[left]
			second := record.Nodes[i].Transitions[right]
			return first.Signal+"\x00"+first.Target < second.Signal+"\x00"+second.Target
		})
	}
	sort.Slice(record.Nodes, func(i, j int) bool { return record.Nodes[i].ID < record.Nodes[j].ID })
	if record.IncidentRoutes == nil {
		record.IncidentRoutes = []catalog.IncidentRoute{}
	}
	sort.Slice(record.IncidentRoutes, func(i, j int) bool {
		return record.IncidentRoutes[i].Incident+"\x00"+record.IncidentRoutes[i].Handler < record.IncidentRoutes[j].Incident+"\x00"+record.IncidentRoutes[j].Handler
	})
	normalizeStringSlice(&record.TerminalGates)
	normalizeStringSlice(&record.StableBoundaries)
}

func hostBindingKey(value catalog.HostBinding) string {
	return value.Host + "\x00" + value.Kind + "\x00" + value.Reference
}

func normalizeUserCollections(record *UserConfigRecord) {
	normalizeStringSlice(&record.DeniedProviders)
	if record.ProviderDescriptors == nil {
		record.ProviderDescriptors = []ContentReference{}
	}
	if record.ProfileRecipes == nil {
		record.ProfileRecipes = []ContentReference{}
	}
	if record.ProviderPins == nil {
		record.ProviderPins = []ProviderPin{}
	}
	if record.BindingPreferences == nil {
		record.BindingPreferences = []BindingPreference{}
	}
	if record.BoundedCapabilityDefaults == nil {
		record.BoundedCapabilityDefaults = []BoundedCapabilityDefault{}
	}
	if record.ProjectTrust == nil {
		record.ProjectTrust = []ProjectTrust{}
	}
}

func normalizeReferences(values []ContentReference, duplicateCode string) error {
	for _, reference := range values {
		if _, err := catalog.ParseQualifiedID(reference.ID); err != nil {
			return fmt.Errorf("INVALID_CONTENT_REFERENCE: %w", err)
		}
		if strings.TrimSpace(reference.Path) == "" {
			return fmt.Errorf("INVALID_CONTENT_REFERENCE: empty path for %s", reference.ID)
		}
		if err := validateReferencePath(reference.Path); err != nil {
			return err
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return uniqueBy(len(values), func(i int) string { return values[i].ID }, duplicateCode)
}

func normalizeStringSlice(values *[]string) {
	if *values == nil {
		*values = []string{}
	}
	sort.Strings(*values)
}

func uniqueSorted(values []string, code string) error {
	return uniqueBy(len(values), func(i int) string { return values[i] }, code)
}

func uniqueBy(length int, key func(int) string, code string) error {
	for i := 1; i < length; i++ {
		if key(i-1) == key(i) {
			return fmt.Errorf("%s: %s", code, key(i))
		}
	}
	return nil
}

func bindingPreferenceKey(value BindingPreference) string {
	return value.ProviderID + "\x00" + value.CapabilityID
}
