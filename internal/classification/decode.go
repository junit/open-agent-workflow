package classification

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

const maximumProposalBytes = 1 << 20

func DecodeProposal(raw []byte) (ClassificationProposal, error) {
	if len(raw) > maximumProposalBytes {
		return ClassificationProposal{}, fmt.Errorf("CLASSIFICATION_TOO_LARGE: %d bytes", len(raw))
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var proposal ClassificationProposal
	if err := decoder.Decode(&proposal); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return ClassificationProposal{}, fmt.Errorf("CLASSIFICATION_UNKNOWN_FIELD: %w", err)
		}
		return ClassificationProposal{}, fmt.Errorf("CLASSIFICATION_JSON_INVALID: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return ClassificationProposal{}, errors.New("CLASSIFICATION_TRAILING_JSON: trailing JSON value")
	} else if !errors.Is(err, io.EOF) {
		return ClassificationProposal{}, fmt.Errorf("CLASSIFICATION_JSON_INVALID: %w", err)
	}
	if err := normalizeProposal(&proposal); err != nil {
		return ClassificationProposal{}, err
	}
	_, encoded, err := canonicaljson.Digest(proposal)
	if err != nil {
		return ClassificationProposal{}, err
	}
	registry, err := schema.New(assets.FS())
	if err != nil {
		return ClassificationProposal{}, err
	}
	if err := registry.Validate(schema.ClassificationProposalV1, encoded); err != nil {
		return ClassificationProposal{}, fmt.Errorf("INVALID_CLASSIFICATION_PROPOSAL: %w", err)
	}
	return cloneProposal(proposal), nil
}

func normalizeProposal(value *ClassificationProposal) error {
	if value.SchemaVersion != ProposalSchemaV1 {
		return fmt.Errorf("UNSUPPORTED_CLASSIFICATION_SCHEMA: %q", value.SchemaVersion)
	}
	if value.Traits == nil {
		value.Traits = []TraitObservation{}
	}
	if value.Resources == nil {
		value.Resources = []Resource{}
	}
	if value.Evidence == nil {
		value.Evidence = []ProposalEvidence{}
	}
	for _, observation := range value.Traits {
		if !knownTrait(observation.Trait) {
			return fmt.Errorf("CLASSIFICATION_TRAIT_UNKNOWN: %q", observation.Trait)
		}
		if observation.Value != TraitTrue && observation.Value != TraitFalse && observation.Value != TraitUnknown {
			return fmt.Errorf("CLASSIFICATION_TRAIT_VALUE_INVALID: %q", observation.Value)
		}
	}
	sortProposalCollections(value)
	for i := 1; i < len(value.Traits); i++ {
		if value.Traits[i-1].Trait == value.Traits[i].Trait {
			return fmt.Errorf("CLASSIFICATION_DUPLICATE_TRAIT: %s", value.Traits[i].Trait)
		}
	}
	for i, resource := range value.Resources {
		if !knownResource(resource) {
			return fmt.Errorf("CLASSIFICATION_RESOURCE_UNKNOWN: %q", resource)
		}
		if i > 0 && value.Resources[i-1] == resource {
			return fmt.Errorf("CLASSIFICATION_DUPLICATE_RESOURCE: %s", resource)
		}
	}
	for i, evidence := range value.Evidence {
		if !knownEvidence(evidence.Kind) || evidence.Reference == "" {
			return fmt.Errorf("CLASSIFICATION_EVIDENCE_INVALID: %s", evidence.Kind)
		}
		if len(evidence.Digest) != 64 || strings.ToLower(evidence.Digest) != evidence.Digest {
			return fmt.Errorf("CLASSIFICATION_EVIDENCE_DIGEST_INVALID: %s", evidence.Kind)
		}
		if _, err := hex.DecodeString(evidence.Digest); err != nil {
			return fmt.Errorf("CLASSIFICATION_EVIDENCE_DIGEST_INVALID: %s", evidence.Kind)
		}
		if i > 0 && evidenceIdentity(value.Evidence[i-1]) == evidenceIdentity(evidence) {
			return fmt.Errorf("CLASSIFICATION_DUPLICATE_EVIDENCE: %s", evidence.Kind)
		}
	}
	if value.CapabilitySelector != nil {
		if _, err := catalog.ParseQualifiedID(value.CapabilitySelector.ProviderID); err != nil {
			return fmt.Errorf("CLASSIFICATION_SELECTOR_INVALID: %w", err)
		}
		if _, err := catalog.ParseLocalID(value.CapabilitySelector.CapabilityID); err != nil {
			return fmt.Errorf("CLASSIFICATION_SELECTOR_INVALID: %w", err)
		}
		if value.CapabilitySelector.Source != SelectorUserIntent && value.CapabilitySelector.Source != SelectorTrustedRule {
			return fmt.Errorf("CLASSIFICATION_SELECTOR_INVALID: %q", value.CapabilitySelector.Source)
		}
	}
	return nil
}

func knownTrait(value Trait) bool {
	for _, candidate := range allCriticalTraits() {
		if candidate == value {
			return true
		}
	}
	return false
}

func knownResource(value Resource) bool {
	switch value {
	case ResourceProject, ResourceWorktree, ResourceGitRepository, ResourcePublicAPI, ResourceSchema, ResourceDependency, ResourceSecurity, ResourceData, ResourceDeployment, ResourceCredentials, ResourceNetwork, ResourceDestructive:
		return true
	default:
		return false
	}
}

func knownEvidence(value EvidenceKind) bool {
	switch value {
	case EvidenceScope, EvidenceChangePoint, EvidenceVerification, EvidenceCapabilitySelector, EvidenceSecurityAcceptance, EvidenceNegativeTest, EvidenceArchitecture, EvidenceAuthorization, EvidenceRecovery:
		return true
	default:
		return false
	}
}

func evidenceIdentity(value ProposalEvidence) string {
	return string(value.Kind) + "\x00" + value.Reference + "\x00" + value.Digest
}
