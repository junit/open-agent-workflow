package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

const configurationSnapshotSchemaV1 = "oaw.configuration-snapshot/v1"

type LoadOptions struct {
	UserConfigRoot string
	ProjectRoot    string
}

type ProviderSettings struct {
	ProviderID      string              `json:"provider_id"`
	Disabled        bool                `json:"disabled"`
	Pin             *ProviderPin        `json:"pin"`
	Preferences     []BindingPreference `json:"preferences"`
	CapabilityLimit []string            `json:"capability_limit"`
	Digest          string              `json:"digest"`
}

type Snapshot struct {
	digest               string
	catalog              catalog.Catalog
	projectStatus        ProjectTrustStatus
	projectReason        string
	settings             []ProviderSettings
	requiredProviders    []string
	recommendedProviders []string
	untrustedProviderIDs []string
	userConfigDigest     string
	projectRoot          string
	projectConfigDigest  string
}

func Load(options LoadOptions) (Snapshot, error) {
	builtIn, err := builtin.Load()
	if err != nil {
		return Snapshot{}, err
	}
	registry, err := schema.New(assets.FS())
	if err != nil {
		return Snapshot{}, err
	}
	user, err := inspectUser(options.UserConfigRoot, registry)
	if err != nil {
		return Snapshot{}, err
	}
	providers := builtIn.Providers()
	recipes := builtIn.Recipes()
	if err := mergeProviders(&providers, user.providers, user.config.Record.ProviderDescriptors); err != nil {
		return Snapshot{}, err
	}
	if err := mergeRecipes(&recipes, user.recipes, user.config.Record.ProfileRecipes); err != nil {
		return Snapshot{}, err
	}
	projectStatus := ProjectAbsent
	projectReason := "PROJECT_CONFIG_ABSENT"
	projectRoot := ""
	projectDigest := ""
	projectConfig := emptyProjectConfig()
	requiredProviders := []string{}
	recommendedProviders := []string{}
	untrustedProviderIDs := []string{}
	if options.ProjectRoot != "" {
		projectRoot, err = physicalRoot(options.ProjectRoot)
		if err != nil {
			return Snapshot{}, err
		}
		configPath := filepath.Join(projectRoot, ".oaw", "config.toml")
		if _, statErr := os.Stat(configPath); statErr == nil {
			inspection, inspectErr := inspectProject(projectRoot, registry)
			if inspectErr != nil {
				return Snapshot{}, inspectErr
			}
			projectRoot = inspection.fingerprint.Root
			projectDigest = inspection.fingerprint.ConfigDigest
			projectStatus, projectReason = EvaluateProjectTrust(user.config.Record.ProjectTrust, inspection.fingerprint)
			if projectStatus == ProjectTrusted {
				if err := mergeProviders(&providers, inspection.providers, inspection.fingerprint.Config.ProviderDescriptors); err != nil {
					return Snapshot{}, err
				}
				if err := mergeRecipes(&recipes, inspection.recipes, inspection.fingerprint.Config.ProfileRecipes); err != nil {
					return Snapshot{}, err
				}
				projectConfig = inspection.fingerprint.Config
				requiredProviders = append([]string{}, projectConfig.RequiredProviders...)
				recommendedProviders = append([]string{}, projectConfig.RecommendedProviders...)
			} else {
				untrustedProviderIDs = projectOnlyProviderIDs(providers, inspection.fingerprint.ProviderIDs)
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return Snapshot{}, fmt.Errorf("CONFIG_FILE_READ_FAILED: .oaw/config.toml: %w", statErr)
		}
	}
	effectiveCatalog, err := catalog.New(providers, recipes, builtIn.Aliases())
	if err != nil {
		return Snapshot{}, fmt.Errorf("EFFECTIVE_CATALOG_INVALID: %w", err)
	}
	settings, err := buildProviderSettings(effectiveCatalog, user.config.Record, projectConfig)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{
		catalog:              effectiveCatalog,
		projectStatus:        projectStatus,
		projectReason:        projectReason,
		settings:             settings,
		requiredProviders:    requiredProviders,
		recommendedProviders: recommendedProviders,
		untrustedProviderIDs: untrustedProviderIDs,
		userConfigDigest:     user.config.Digest,
		projectRoot:          projectRoot,
		projectConfigDigest:  projectDigest,
	}
	if err := snapshot.setDigest(); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func projectOnlyProviderIDs(trusted []catalog.ProviderDescriptorRecord, projectIDs []string) []string {
	known := make(map[string]struct{}, len(trusted))
	for _, provider := range trusted {
		known[provider.ID] = struct{}{}
	}
	result := make([]string, 0, len(projectIDs))
	for _, id := range projectIDs {
		if _, found := known[id]; !found {
			result = append(result, id)
		}
	}
	return result
}

type userInspection struct {
	config    Decoded[UserConfigRecord]
	providers []Decoded[catalog.ProviderDescriptorRecord]
	recipes   []Decoded[catalog.ProfileRecipeRecord]
}

func inspectUser(root string, registry *schema.Registry) (userInspection, error) {
	if root == "" {
		empty := emptyUserConfig()
		digest, encoded, err := canonicaljson.Digest(empty)
		return userInspection{config: Decoded[UserConfigRecord]{Record: empty, CanonicalJSON: encoded, Digest: digest}, providers: []Decoded[catalog.ProviderDescriptorRecord]{}, recipes: []Decoded[catalog.ProfileRecipeRecord]{}}, err
	}
	physical, err := physicalRoot(root)
	if err != nil {
		return userInspection{}, err
	}
	if _, err := os.Stat(filepath.Join(physical, "config.toml")); errors.Is(err, os.ErrNotExist) {
		empty := emptyUserConfig()
		digest, encoded, digestErr := canonicaljson.Digest(empty)
		return userInspection{config: Decoded[UserConfigRecord]{Record: empty, CanonicalJSON: encoded, Digest: digest}, providers: []Decoded[catalog.ProviderDescriptorRecord]{}, recipes: []Decoded[catalog.ProfileRecipeRecord]{}}, digestErr
	} else if err != nil {
		return userInspection{}, fmt.Errorf("CONFIG_FILE_READ_FAILED: config.toml: %w", err)
	}
	raw, _, err := readContained(physical, "config.toml", maximumTOMLBytes)
	if err != nil {
		return userInspection{}, err
	}
	decoded, err := DecodeUser(raw, registry)
	if err != nil {
		return userInspection{}, err
	}
	result := userInspection{config: decoded, providers: []Decoded[catalog.ProviderDescriptorRecord]{}, recipes: []Decoded[catalog.ProfileRecipeRecord]{}}
	for _, reference := range decoded.Record.ProviderDescriptors {
		content, _, readErr := readContained(physical, reference.Path, maximumTOMLBytes)
		if readErr != nil {
			return userInspection{}, readErr
		}
		provider, decodeErr := DecodeProvider(content, registry)
		if decodeErr != nil {
			return userInspection{}, fmt.Errorf("USER_PROVIDER_INVALID: %s: %w", reference.ID, decodeErr)
		}
		if provider.Record.ID != reference.ID {
			return userInspection{}, fmt.Errorf("CONTENT_REFERENCE_ID_MISMATCH: %s != %s", reference.ID, provider.Record.ID)
		}
		result.providers = append(result.providers, provider)
	}
	for _, reference := range decoded.Record.ProfileRecipes {
		content, _, readErr := readContained(physical, reference.Path, maximumTOMLBytes)
		if readErr != nil {
			return userInspection{}, readErr
		}
		recipe, decodeErr := DecodeRecipe(content, registry)
		if decodeErr != nil {
			return userInspection{}, fmt.Errorf("USER_RECIPE_INVALID: %s: %w", reference.ID, decodeErr)
		}
		if recipe.Record.ID != reference.ID {
			return userInspection{}, fmt.Errorf("CONTENT_REFERENCE_ID_MISMATCH: %s != %s", reference.ID, recipe.Record.ID)
		}
		result.recipes = append(result.recipes, recipe)
	}
	return result, nil
}

func mergeProviders(destination *[]catalog.ProviderDescriptorRecord, sources []Decoded[catalog.ProviderDescriptorRecord], references []ContentReference) error {
	replacements := referenceIndex(references)
	index := make(map[string]int, len(*destination))
	for i, record := range *destination {
		index[record.ID] = i
	}
	for _, source := range sources {
		id := source.Record.ID
		if strings.HasPrefix(id, "oaw/") {
			return fmt.Errorf("RESERVED_PROVIDER_NAMESPACE: %s", id)
		}
		if existing, found := index[id]; found {
			if !replacements[id].Replace {
				return fmt.Errorf("DUPLICATE_PROVIDER_REPLACEMENT_REQUIRED: %s", id)
			}
			(*destination)[existing] = source.Record
			continue
		}
		index[id] = len(*destination)
		*destination = append(*destination, source.Record)
	}
	return nil
}

func mergeRecipes(destination *[]catalog.ProfileRecipeRecord, sources []Decoded[catalog.ProfileRecipeRecord], references []ContentReference) error {
	replacements := referenceIndex(references)
	index := make(map[string]int, len(*destination))
	for i, record := range *destination {
		index[record.ID] = i
	}
	for _, source := range sources {
		id := source.Record.ID
		if strings.HasPrefix(id, "oaw/") {
			return fmt.Errorf("RESERVED_RECIPE_NAMESPACE: %s", id)
		}
		if existing, found := index[id]; found {
			if !replacements[id].Replace {
				return fmt.Errorf("DUPLICATE_RECIPE_REPLACEMENT_REQUIRED: %s", id)
			}
			(*destination)[existing] = source.Record
			continue
		}
		index[id] = len(*destination)
		*destination = append(*destination, source.Record)
	}
	return nil
}

func referenceIndex(values []ContentReference) map[string]ContentReference {
	result := make(map[string]ContentReference, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result
}

func buildProviderSettings(value catalog.Catalog, user UserConfigRecord, project ProjectConfigRecord) ([]ProviderSettings, error) {
	settings := make(map[string]*ProviderSettings)
	for _, provider := range value.Providers() {
		settings[provider.ID] = &ProviderSettings{ProviderID: provider.ID, Preferences: []BindingPreference{}, CapabilityLimit: []string{}}
	}
	for _, id := range user.DeniedProviders {
		setting := ensureProviderSettings(settings, id)
		setting.Disabled = true
	}
	for i := range user.ProviderPins {
		pin := user.ProviderPins[i]
		setting := ensureProviderSettings(settings, pin.ID)
		copyValue := pin
		setting.Pin = &copyValue
	}
	for _, preference := range user.BindingPreferences {
		setting := ensureProviderSettings(settings, preference.ProviderID)
		setting.Preferences = append(setting.Preferences, preference)
	}
	for _, limit := range project.CapabilityLimits {
		setting := ensureProviderSettings(settings, limit.ProviderID)
		setting.CapabilityLimit = append([]string{}, limit.CapabilityIDs...)
	}
	result := make([]ProviderSettings, 0, len(settings))
	for _, setting := range settings {
		sort.Slice(setting.Preferences, func(i, j int) bool {
			return bindingPreferenceKey(setting.Preferences[i]) < bindingPreferenceKey(setting.Preferences[j])
		})
		if err := validateSettingsAgainstCatalog(*setting, value); err != nil {
			return nil, err
		}
		if err := setProviderSettingsDigest(setting); err != nil {
			return nil, err
		}
		result = append(result, cloneProviderSettings(*setting))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ProviderID < result[j].ProviderID })
	return result, nil
}

func ensureProviderSettings(values map[string]*ProviderSettings, id string) *ProviderSettings {
	if value, found := values[id]; found {
		return value
	}
	value := &ProviderSettings{ProviderID: id, Preferences: []BindingPreference{}, CapabilityLimit: []string{}}
	values[id] = value
	return value
}

func validateSettingsAgainstCatalog(settings ProviderSettings, value catalog.Catalog) error {
	var provider *catalog.ProviderDescriptorRecord
	for _, record := range value.Providers() {
		if record.ID == settings.ProviderID {
			copyValue := record
			provider = &copyValue
			break
		}
	}
	if provider == nil {
		if len(settings.Preferences) != 0 || len(settings.CapabilityLimit) != 0 {
			return fmt.Errorf("PROVIDER_SETTINGS_UNKNOWN: %s", settings.ProviderID)
		}
		return nil
	}
	capabilities := make(map[string]catalog.CapabilityRecord, len(provider.Capabilities))
	for _, capability := range provider.Capabilities {
		capabilities[capability.ID] = capability
	}
	for _, id := range settings.CapabilityLimit {
		if _, found := capabilities[id]; !found {
			return fmt.Errorf("CAPABILITY_LIMIT_UNKNOWN: %s/%s", settings.ProviderID, id)
		}
	}
	for _, preference := range settings.Preferences {
		capability, found := capabilities[preference.CapabilityID]
		if !found || !containsBinding(capability.HostBindings, preference) {
			return fmt.Errorf("BINDING_PREFERENCE_UNDECLARED: %s/%s", settings.ProviderID, preference.CapabilityID)
		}
	}
	return nil
}

func containsBinding(values []catalog.HostBinding, preference BindingPreference) bool {
	for _, value := range values {
		if value.Host == preference.Host && value.Kind == preference.Kind && value.Reference == preference.Reference {
			return true
		}
	}
	return false
}

func (snapshot Snapshot) Digest() string { return snapshot.digest }

func (snapshot Snapshot) Catalog() catalog.Catalog { return snapshot.catalog }

func (snapshot Snapshot) ProjectStatus() ProjectTrustStatus { return snapshot.projectStatus }

func (snapshot Snapshot) ProjectReason() string { return snapshot.projectReason }

func (snapshot Snapshot) ProviderSettings(id string) ProviderSettings {
	index := sort.Search(len(snapshot.settings), func(i int) bool { return snapshot.settings[i].ProviderID >= id })
	if index == len(snapshot.settings) || snapshot.settings[index].ProviderID != id {
		return ProviderSettings{ProviderID: id, Preferences: []BindingPreference{}, CapabilityLimit: []string{}}
	}
	return cloneProviderSettings(snapshot.settings[index])
}

func (snapshot Snapshot) RequiredProviders() []string {
	return append([]string{}, snapshot.requiredProviders...)
}

func (snapshot Snapshot) RecommendedProviders() []string {
	return append([]string{}, snapshot.recommendedProviders...)
}

func (snapshot Snapshot) UntrustedProviderIDs() []string {
	return append([]string{}, snapshot.untrustedProviderIDs...)
}

func (snapshot *Snapshot) setDigest() error {
	record := struct {
		SchemaVersion        string             `json:"schema_version"`
		CatalogDigest        string             `json:"catalog_digest"`
		UserConfigDigest     string             `json:"user_config_digest"`
		ProjectRoot          string             `json:"project_root"`
		ProjectConfigDigest  string             `json:"project_config_digest"`
		ProjectStatus        ProjectTrustStatus `json:"project_status"`
		ProjectReason        string             `json:"project_reason"`
		Settings             []ProviderSettings `json:"settings"`
		RequiredProviders    []string           `json:"required_providers"`
		RecommendedProviders []string           `json:"recommended_providers"`
		UntrustedProviderIDs []string           `json:"untrusted_provider_ids"`
	}{
		configurationSnapshotSchemaV1,
		snapshot.catalog.Digest(),
		snapshot.userConfigDigest,
		snapshot.projectRoot,
		snapshot.projectConfigDigest,
		snapshot.projectStatus,
		snapshot.projectReason,
		snapshot.settings,
		snapshot.requiredProviders,
		snapshot.recommendedProviders,
		snapshot.untrustedProviderIDs,
	}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return err
	}
	snapshot.digest = digest
	return nil
}

func setProviderSettingsDigest(value *ProviderSettings) error {
	record := struct {
		ProviderID      string              `json:"provider_id"`
		Disabled        bool                `json:"disabled"`
		Pin             *ProviderPin        `json:"pin"`
		Preferences     []BindingPreference `json:"preferences"`
		CapabilityLimit []string            `json:"capability_limit"`
	}{value.ProviderID, value.Disabled, value.Pin, value.Preferences, value.CapabilityLimit}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return err
	}
	value.Digest = digest
	return nil
}

func cloneProviderSettings(value ProviderSettings) ProviderSettings {
	if value.Pin != nil {
		pin := *value.Pin
		value.Pin = &pin
	}
	value.Preferences = append([]BindingPreference{}, value.Preferences...)
	value.CapabilityLimit = append([]string{}, value.CapabilityLimit...)
	return value
}

func emptyUserConfig() UserConfigRecord {
	return UserConfigRecord{
		SchemaVersion:       UserConfigSchemaV1,
		DeniedProviders:     []string{},
		ProviderDescriptors: []ContentReference{},
		ProfileRecipes:      []ContentReference{},
		ProviderPins:        []ProviderPin{},
		BindingPreferences:  []BindingPreference{},
		ProjectTrust:        []ProjectTrust{},
	}
}

func emptyProjectConfig() ProjectConfigRecord {
	return ProjectConfigRecord{
		SchemaVersion:        ProjectConfigSchemaV1,
		RequiredProviders:    []string{},
		RecommendedProviders: []string{},
		ProviderDescriptors:  []ContentReference{},
		ProfileRecipes:       []ContentReference{},
		CapabilityLimits:     []CapabilityLimit{},
	}
}
