package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/BurntSushi/toml"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

const maximumConfigBytes = 1 << 20

func DecodeUser(raw []byte, registry *schema.Registry) (Decoded[UserConfigRecord], error) {
	var record UserConfigRecord
	if err := strictTOML(raw, &record); err != nil {
		return Decoded[UserConfigRecord]{}, err
	}
	if record.SchemaVersion != UserConfigSchemaV3 {
		return Decoded[UserConfigRecord]{}, fmt.Errorf("CONFIG_SCHEMA_UNSUPPORTED: user configuration %q", record.SchemaVersion)
	}
	if err := normalizeUser(&record); err != nil {
		return Decoded[UserConfigRecord]{}, err
	}
	digest, encoded, err := canonicaljson.Digest(record)
	if err != nil {
		return Decoded[UserConfigRecord]{}, err
	}
	if err := registry.Validate(schema.UserConfigV3, encoded); err != nil {
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
	if err := strictDescriptorContent(raw, &record); err != nil {
		return Decoded[catalog.ProviderDescriptorRecord]{}, err
	}
	normalizeProvider(&record)
	_, encoded, err := canonicaljson.Digest(record)
	if err != nil {
		return Decoded[catalog.ProviderDescriptorRecord]{}, err
	}
	if err := registry.Validate(schema.ProviderDescriptorV3, encoded); err != nil {
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
	if err := strictDescriptorContent(raw, &record); err != nil {
		return Decoded[catalog.ProfileRecipeRecord]{}, err
	}
	normalizeRecipe(&record)
	_, encoded, err := canonicaljson.Digest(record)
	if err != nil {
		return Decoded[catalog.ProfileRecipeRecord]{}, err
	}
	if err := registry.Validate(schema.ProfileRecipeV2, encoded); err != nil {
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
	if len(raw) > maximumConfigBytes {
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

func strictDescriptorContent(raw []byte, destination any) error {
	if descriptorContentIsJSON(raw) {
		return strictJSON(raw, destination)
	}
	return strictTOML(raw, destination)
}

func descriptorContentIsJSON(raw []byte) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func strictJSON(raw []byte, destination any) error {
	if len(raw) > maximumConfigBytes {
		return fmt.Errorf("CONFIG_TOO_LARGE: %d bytes", len(raw))
	}
	if !utf8.Valid(raw) {
		return fmt.Errorf("CONFIG_INVALID_UTF8")
	}
	if err := rejectDuplicateJSONFields(raw); err != nil {
		return fmt.Errorf("CONFIG_JSON_INVALID: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return fmt.Errorf("CONFIG_UNKNOWN_FIELD: %w", err)
		}
		return fmt.Errorf("CONFIG_JSON_INVALID: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("trailing JSON value")
		}
		return fmt.Errorf("CONFIG_JSON_INVALID: %w", err)
	}
	return nil
}

func rejectDuplicateJSONFields(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := consumeUniqueJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON token")
		}
		return err
	}
	return nil
}

func consumeUniqueJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate object field %q", key)
			}
			seen[key] = struct{}{}
			if err := consumeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("object is not closed")
		}
	case '[':
		for decoder.More() {
			if err := consumeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("array is not closed")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
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
	if err := normalizeReferences(record.HostIntegrations, "DUPLICATE_HOST_INTEGRATION_REFERENCE"); err != nil {
		return err
	}
	for _, installation := range record.ProviderInstallations {
		if _, err := catalog.ParseQualifiedID(installation.ProviderID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_INSTALLATION: %w", err)
		}
		if _, err := catalog.ParseLocalID(installation.HostID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_INSTALLATION: %w", err)
		}
		if _, err := catalog.ParseLocalID(installation.SurfaceID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_INSTALLATION: %w", err)
		}
		if _, err := catalog.ParseLocalID(installation.DiscoveryProbeID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_INSTALLATION: %w", err)
		}
		if !cleanAbsolutePath(installation.Location) {
			return fmt.Errorf("INVALID_PROVIDER_INSTALLATION: unsafe location for %s/%s", installation.ProviderID, installation.HostID)
		}
	}
	sort.Slice(record.ProviderInstallations, func(i, j int) bool {
		return providerInstallationKey(record.ProviderInstallations[i]) < providerInstallationKey(record.ProviderInstallations[j])
	})
	if err := uniqueBy(len(record.ProviderInstallations), func(i int) string { return providerInstallationKey(record.ProviderInstallations[i]) }, "DUPLICATE_PROVIDER_INSTALLATION"); err != nil {
		return err
	}
	for _, pin := range record.ProviderPins {
		if _, err := catalog.ParseQualifiedID(pin.ProviderID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_PIN: %w", err)
		}
		if _, err := catalog.ParseLocalID(pin.HostID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_PIN: %w", err)
		}
		if pin.InstallationKey == "" || hasControl(pin.InstallationKey) || !validDigest(pin.EvidenceDigest) {
			return fmt.Errorf("INVALID_PROVIDER_PIN: invalid identity for %s/%s", pin.ProviderID, pin.HostID)
		}
		if pin.Location != "" && !cleanAbsolutePath(pin.Location) {
			return fmt.Errorf("INVALID_PROVIDER_PIN: unsafe location for %s/%s", pin.ProviderID, pin.HostID)
		}
		if pin.Version != "" && (strings.TrimSpace(pin.Version) != pin.Version || hasControl(pin.Version)) {
			return fmt.Errorf("INVALID_PROVIDER_PIN: invalid version for %s/%s", pin.ProviderID, pin.HostID)
		}
	}
	sort.Slice(record.ProviderPins, func(i, j int) bool {
		return providerPinKey(record.ProviderPins[i]) < providerPinKey(record.ProviderPins[j])
	})
	if err := uniqueBy(len(record.ProviderPins), func(i int) string { return providerPinKey(record.ProviderPins[i]) }, "DUPLICATE_PROVIDER_PIN"); err != nil {
		return err
	}
	for _, preference := range record.BindingPreferences {
		if _, err := catalog.ParseQualifiedID(preference.ProviderID); err != nil {
			return fmt.Errorf("INVALID_BINDING_PREFERENCE: %w", err)
		}
		if _, err := catalog.ParseLocalID(preference.CapabilityID); err != nil {
			return fmt.Errorf("INVALID_BINDING_PREFERENCE: %w", err)
		}
		if _, err := catalog.ParseLocalID(preference.HostID); err != nil || preference.Reference == "" || (preference.Kind != "skill" && preference.Kind != "agent" && preference.Kind != "tool") {
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
		normalizeStringSlice(&record.Discovery[i].Hosts)
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
	if record.HostIntegrations == nil {
		record.HostIntegrations = []ContentReference{}
	}
	if record.ProviderInstallations == nil {
		record.ProviderInstallations = []ProviderInstallation{}
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
	return value.ProviderID + "\x00" + value.HostID + "\x00" + value.CapabilityID
}

func providerPinKey(value ProviderPin) string {
	return value.ProviderID + "\x00" + value.HostID
}

func providerInstallationKey(value ProviderInstallation) string {
	return value.ProviderID + "\x00" + value.HostID + "\x00" + value.SurfaceID + "\x00" + value.Location
}

func cleanAbsolutePath(value string) bool {
	return value != "" && filepath.IsAbs(value) && filepath.Clean(value) == value && !hasControl(value)
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func hasControl(value string) bool {
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return true
		}
	}
	return false
}
