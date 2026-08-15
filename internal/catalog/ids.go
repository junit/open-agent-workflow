package catalog

import (
	"fmt"
	"regexp"
)

var (
	qualifiedIDPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*/[a-z0-9][a-z0-9._-]*$`)
	localIDPattern        = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)
	contentVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
)

type QualifiedID struct {
	value string
}

func ParseQualifiedID(value string) (QualifiedID, error) {
	if !qualifiedIDPattern.MatchString(value) {
		return QualifiedID{}, fmt.Errorf("INVALID_QUALIFIED_ID: %q", value)
	}
	return QualifiedID{value: value}, nil
}

func (id QualifiedID) String() string {
	return id.value
}

type LocalID struct {
	value string
}

func ParseLocalID(value string) (LocalID, error) {
	if !localIDPattern.MatchString(value) {
		return LocalID{}, fmt.Errorf("INVALID_LOCAL_ID: %q", value)
	}
	return LocalID{value: value}, nil
}

func (id LocalID) String() string {
	return id.value
}

type ContentVersion struct {
	value string
}

func ParseContentVersion(value string) (ContentVersion, error) {
	if !contentVersionPattern.MatchString(value) {
		return ContentVersion{}, fmt.Errorf("INVALID_CONTENT_VERSION: %q", value)
	}
	return ContentVersion{value: value}, nil
}

func (version ContentVersion) String() string {
	return version.value
}
