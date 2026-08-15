package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var (
	contentDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	revisionPattern      = regexp.MustCompile(`^(?:[0-9a-f]{40}|sha256:[0-9a-f]{64})$`)
)

func DecodeProvider(data []byte) (ProviderDescriptorRecord, error) {
	var record ProviderDescriptorRecord
	if err := strictDecode(data, &record); err != nil {
		return ProviderDescriptorRecord{}, fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
	}
	if err := validateProviderRecord(&record); err != nil {
		return ProviderDescriptorRecord{}, err
	}
	normalizeProvider(&record)
	return cloneProvider(record), nil
}

func strictDecode(data []byte, destination any) error {
	if err := rejectDuplicateJSONFields(data); err != nil {
		return err
	}
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
			if _, duplicate := seen[key]; duplicate {
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

func validateProviderRecord(record *ProviderDescriptorRecord) error {
	if record.SchemaVersion != ProviderDescriptorSchemaV5 {
		return fmt.Errorf("UNSUPPORTED_PROVIDER_SCHEMA: %q", record.SchemaVersion)
	}
	if _, err := ParseContentVersion(record.DescriptorVersion); err != nil {
		return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
	}
	if _, err := ParseQualifiedID(record.ID); err != nil {
		return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
	}
	if strings.TrimSpace(record.DisplayName) == "" || strings.TrimSpace(record.DisplayName) != record.DisplayName ||
		len(record.Distributions) == 0 || len(record.Discovery) == 0 || len(record.Bindings) == 0 {
		return errors.New("INVALID_PROVIDER_DESCRIPTOR: required field is missing")
	}
	if err := validateDistributions(record.Distributions); err != nil {
		return err
	}
	if err := validateDiscovery(record.Discovery); err != nil {
		return err
	}
	if err := validateBindings(record.Bindings); err != nil {
		return err
	}
	return validateProviderReferences(record)
}

func validateDistributions(values []DistributionRecord) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, err := ParseLocalID(value.ID); err != nil || strings.TrimSpace(value.SourceURI) == "" || strings.TrimSpace(value.SourceURI) != value.SourceURI {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: invalid Distribution")
		}
		if _, duplicate := seen[value.ID]; duplicate {
			return errors.New("DUPLICATE_DISTRIBUTION_ID: duplicate Distribution ID")
		}
		seen[value.ID] = struct{}{}
		if !revisionPattern.MatchString(value.Revision) || !contentDigestPattern.MatchString(value.TreeDigest) {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: invalid Distribution identity")
		}
	}
	return nil
}

func validateDiscovery(values []DiscoveryProbe) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, err := ParseLocalID(value.ID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if _, duplicate := seen[value.ID]; duplicate {
			return errors.New("DUPLICATE_DISCOVERY_PROBE_ID: duplicate discovery probe ID")
		}
		seen[value.ID] = struct{}{}
		if len(value.Hosts) == 0 || !uniqueStrings(value.Hosts) {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: invalid discovery Hosts")
		}
		for _, host := range value.Hosts {
			if _, err := ParseLocalID(host); err != nil {
				return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
			}
		}
		if _, err := ParseLocalID(value.Surface); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if _, err := ParseLocalID(value.DistributionID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if err := validateProbe(value); err != nil {
			return err
		}
	}
	return nil
}

func validateProbe(value DiscoveryProbe) error {
	if value.Root != "user-home" {
		return errors.New("INVALID_PROVIDER_DESCRIPTOR: unsupported discovery root")
	}
	switch value.Kind {
	case "path-exists":
		if !safeRelativePath(value.CandidatePath) || !safeRelativePath(value.EvidencePath) || value.Prefix != "" {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: invalid direct discovery probe")
		}
	case "one-level-version-path-exists":
		if !safeRelativePath(value.Prefix) || !safeRelativePath(value.EvidencePath) || value.CandidatePath != "" {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: invalid versioned discovery probe")
		}
	default:
		return errors.New("INVALID_PROVIDER_DESCRIPTOR: unsupported discovery probe")
	}
	return nil
}

func validateBindings(values []BindingRecord) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, err := ParseLocalID(value.ID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if _, duplicate := seen[value.ID]; duplicate {
			return errors.New("DUPLICATE_BINDING_ID: duplicate Binding ID")
		}
		seen[value.ID] = struct{}{}
		if _, err := ParseLocalID(value.DistributionID); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if !safeRelativePath(value.ContentRoot) || !safeRelativePath(value.InstallRoot) || !contentDigestPattern.MatchString(value.TreeDigest) {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: invalid Binding content identity")
		}
		if _, err := ParseLocalID(value.Host); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if _, err := ParseLocalID(value.Surface); err != nil {
			return fmt.Errorf("INVALID_PROVIDER_DESCRIPTOR: %w", err)
		}
		if !validBindingKind(value.Kind) || strings.TrimSpace(value.Reference) == "" || strings.TrimSpace(value.Reference) != value.Reference || !validInvocation(value.Invocation) {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: invalid Binding surface")
		}
	}
	return nil
}

func validateProviderReferences(record *ProviderDescriptorRecord) error {
	distributions := make(map[string]struct{}, len(record.Distributions))
	for _, distribution := range record.Distributions {
		distributions[distribution.ID] = struct{}{}
	}
	surfaces := make(map[string]struct{})
	for _, probe := range record.Discovery {
		if _, found := distributions[probe.DistributionID]; !found {
			return errors.New("PROVIDER_DISTRIBUTION_NOT_FOUND: discovery Distribution is missing")
		}
		for _, host := range probe.Hosts {
			surfaces[host+"\x00"+probe.Surface+"\x00"+probe.DistributionID] = struct{}{}
		}
	}
	for _, binding := range record.Bindings {
		if _, found := distributions[binding.DistributionID]; !found {
			return errors.New("PROVIDER_DISTRIBUTION_NOT_FOUND: Binding Distribution is missing")
		}
		if _, found := surfaces[binding.Host+"\x00"+binding.Surface+"\x00"+binding.DistributionID]; !found {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: Binding has no matching discovery surface")
		}
	}
	return nil
}

func validBindingKind(value BindingKind) bool {
	switch value {
	case BindingSkill, BindingAgent, BindingRole, BindingInstruction, BindingTool:
		return true
	default:
		return false
	}
}

func validInvocation(value InvocationDisposition) bool {
	switch value {
	case InvocationHumanExplicit, InvocationModel, InvocationHost, InvocationInternal:
		return true
	default:
		return false
	}
}

func safeRelativePath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.ContainsAny(value, `\\:`) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
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

func uniqueStrings(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func normalizeProvider(provider *ProviderDescriptorRecord) {
	sort.Slice(provider.Distributions, func(left, right int) bool {
		return provider.Distributions[left].ID < provider.Distributions[right].ID
	})
	sort.Slice(provider.Discovery, func(left, right int) bool {
		return provider.Discovery[left].ID < provider.Discovery[right].ID
	})
	for index := range provider.Discovery {
		sort.Strings(provider.Discovery[index].Hosts)
	}
	sort.Slice(provider.Bindings, func(left, right int) bool {
		return provider.Bindings[left].ID < provider.Bindings[right].ID
	})
}

func cloneProvider(record ProviderDescriptorRecord) ProviderDescriptorRecord {
	record.Distributions = append([]DistributionRecord(nil), record.Distributions...)
	record.Discovery = append([]DiscoveryProbe(nil), record.Discovery...)
	for index := range record.Discovery {
		record.Discovery[index].Hosts = append([]string(nil), record.Discovery[index].Hosts...)
	}
	record.Bindings = append([]BindingRecord(nil), record.Bindings...)
	return record
}
