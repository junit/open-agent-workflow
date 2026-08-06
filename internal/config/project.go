package config

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

type ProjectTrustStatus string

const (
	ProjectAbsent    ProjectTrustStatus = "absent"
	ProjectTrusted   ProjectTrustStatus = "trusted"
	ProjectUntrusted ProjectTrustStatus = "untrusted"
)

type ProjectFingerprint struct {
	Root              string
	Config            ProjectConfigRecord
	ConfigDigest      string
	DescriptorDigests []string
	RecipeDigests     []string
	ProviderIDs       []string
	RecipeIDs         []string
}

type projectInspection struct {
	fingerprint ProjectFingerprint
	providers   []Decoded[catalog.ProviderDescriptorRecord]
	recipes     []Decoded[catalog.ProfileRecipeRecord]
}

func InspectProject(projectRoot string, registry *schema.Registry) (ProjectFingerprint, error) {
	inspection, err := inspectProject(projectRoot, registry)
	if err != nil {
		return ProjectFingerprint{}, err
	}
	return cloneProjectFingerprint(inspection.fingerprint), nil
}

func inspectProject(projectRoot string, registry *schema.Registry) (projectInspection, error) {
	root, err := physicalRoot(projectRoot)
	if err != nil {
		return projectInspection{}, err
	}
	raw, _, err := readContained(root, ".oaw/config.toml", maximumConfigBytes)
	if err != nil {
		return projectInspection{}, err
	}
	decodedConfig, err := DecodeProject(raw, registry)
	if err != nil {
		return projectInspection{}, err
	}
	contentRoot, err := physicalRoot(filepath.Join(root, ".oaw"))
	if err != nil {
		return projectInspection{}, err
	}
	inspection := projectInspection{
		fingerprint: ProjectFingerprint{
			Root:              root,
			Config:            decodedConfig.Record,
			ConfigDigest:      decodedConfig.Digest,
			DescriptorDigests: []string{},
			RecipeDigests:     []string{},
			ProviderIDs:       []string{},
			RecipeIDs:         []string{},
		},
		providers: []Decoded[catalog.ProviderDescriptorRecord]{},
		recipes:   []Decoded[catalog.ProfileRecipeRecord]{},
	}
	for _, reference := range decodedConfig.Record.ProviderDescriptors {
		content, _, readErr := readContained(contentRoot, reference.Path, maximumConfigBytes)
		if readErr != nil {
			return projectInspection{}, readErr
		}
		provider, decodeErr := DecodeProvider(content, registry)
		if decodeErr != nil {
			return projectInspection{}, fmt.Errorf("PROJECT_PROVIDER_INVALID: %s: %w", reference.ID, decodeErr)
		}
		if provider.Record.ID != reference.ID {
			return projectInspection{}, fmt.Errorf("CONTENT_REFERENCE_ID_MISMATCH: %s != %s", reference.ID, provider.Record.ID)
		}
		inspection.providers = append(inspection.providers, provider)
		inspection.fingerprint.DescriptorDigests = append(inspection.fingerprint.DescriptorDigests, provider.Digest)
		inspection.fingerprint.ProviderIDs = append(inspection.fingerprint.ProviderIDs, provider.Record.ID)
	}
	for _, reference := range decodedConfig.Record.ProfileRecipes {
		content, _, readErr := readContained(contentRoot, reference.Path, maximumConfigBytes)
		if readErr != nil {
			return projectInspection{}, readErr
		}
		recipe, decodeErr := DecodeRecipe(content, registry)
		if decodeErr != nil {
			return projectInspection{}, fmt.Errorf("PROJECT_RECIPE_INVALID: %s: %w", reference.ID, decodeErr)
		}
		if recipe.Record.ID != reference.ID {
			return projectInspection{}, fmt.Errorf("CONTENT_REFERENCE_ID_MISMATCH: %s != %s", reference.ID, recipe.Record.ID)
		}
		inspection.recipes = append(inspection.recipes, recipe)
		inspection.fingerprint.RecipeDigests = append(inspection.fingerprint.RecipeDigests, recipe.Digest)
		inspection.fingerprint.RecipeIDs = append(inspection.fingerprint.RecipeIDs, recipe.Record.ID)
	}
	sort.Strings(inspection.fingerprint.DescriptorDigests)
	sort.Strings(inspection.fingerprint.RecipeDigests)
	sort.Strings(inspection.fingerprint.ProviderIDs)
	sort.Strings(inspection.fingerprint.RecipeIDs)
	return inspection, nil
}

func EvaluateProjectTrust(records []ProjectTrust, fingerprint ProjectFingerprint) (ProjectTrustStatus, string) {
	if len(records) == 0 {
		return ProjectUntrusted, "PROJECT_TRUST_MISSING"
	}
	for _, record := range records {
		if record.Root != fingerprint.Root {
			continue
		}
		if record.ConfigDigest != fingerprint.ConfigDigest {
			return ProjectUntrusted, "PROJECT_CONFIG_DIGEST_MISMATCH"
		}
		if !slices.Equal(record.DescriptorDigests, fingerprint.DescriptorDigests) {
			return ProjectUntrusted, "PROJECT_DESCRIPTOR_DIGEST_MISMATCH"
		}
		if !slices.Equal(record.RecipeDigests, fingerprint.RecipeDigests) {
			return ProjectUntrusted, "PROJECT_RECIPE_DIGEST_MISMATCH"
		}
		return ProjectTrusted, "PROJECT_TRUST_VERIFIED"
	}
	return ProjectUntrusted, "PROJECT_ROOT_MISMATCH"
}

func cloneProjectFingerprint(value ProjectFingerprint) ProjectFingerprint {
	value.Config = cloneProjectConfig(value.Config)
	value.DescriptorDigests = append([]string{}, value.DescriptorDigests...)
	value.RecipeDigests = append([]string{}, value.RecipeDigests...)
	value.ProviderIDs = append([]string{}, value.ProviderIDs...)
	value.RecipeIDs = append([]string{}, value.RecipeIDs...)
	return value
}

func cloneProjectConfig(value ProjectConfigRecord) ProjectConfigRecord {
	value.RequiredProviders = append([]string{}, value.RequiredProviders...)
	value.RecommendedProviders = append([]string{}, value.RecommendedProviders...)
	value.ProviderDescriptors = append([]ContentReference{}, value.ProviderDescriptors...)
	value.ProfileRecipes = append([]ContentReference{}, value.ProfileRecipes...)
	value.CapabilityLimits = append([]CapabilityLimit{}, value.CapabilityLimits...)
	for i := range value.CapabilityLimits {
		value.CapabilityLimits[i].CapabilityIDs = append([]string{}, value.CapabilityLimits[i].CapabilityIDs...)
	}
	return value
}
