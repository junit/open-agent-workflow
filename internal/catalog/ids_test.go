package catalog

import (
	"strings"
	"testing"
)

func TestParseQualifiedID(t *testing.T) {
	valid := []string{
		"oaw/superpowers",
		"oaw/reliable-feature",
		"acme/engineering-suite",
	}
	for _, input := range valid {
		t.Run("valid_"+input, func(t *testing.T) {
			got, err := ParseQualifiedID(input)
			if err != nil {
				t.Fatalf("ParseQualifiedID(%q) returned error: %v", input, err)
			}
			if got.String() != input {
				t.Errorf("ParseQualifiedID(%q).String() = %q, want %q", input, got.String(), input)
			}
		})
	}

	invalid := []string{
		"",
		"oaw",
		"OAW/ecc",
		"oaw/ecc/full",
		"oaw/../ecc",
		"oaw/ecc ",
		"oaw/ecc\n",
		"/ecc",
		"oaw/",
	}
	for _, input := range invalid {
		t.Run("invalid_"+input, func(t *testing.T) {
			_, err := ParseQualifiedID(input)
			assertParseErrorCode(t, err, "INVALID_QUALIFIED_ID")
		})
	}
}

func TestParseLocalID(t *testing.T) {
	valid := []string{
		"implementation",
		"functional-debugging",
		"security.review",
	}
	for _, input := range valid {
		t.Run("valid_"+input, func(t *testing.T) {
			got, err := ParseLocalID(input)
			if err != nil {
				t.Fatalf("ParseLocalID(%q) returned error: %v", input, err)
			}
			if got.String() != input {
				t.Errorf("ParseLocalID(%q).String() = %q, want %q", input, got.String(), input)
			}
		})
	}

	invalid := []string{
		"",
		"Functional-debugging",
		"-implementation",
		"security_review",
		"implementation ",
		"implementation\n",
		"implementation/debugging",
	}
	for _, input := range invalid {
		t.Run("invalid_"+input, func(t *testing.T) {
			_, err := ParseLocalID(input)
			assertParseErrorCode(t, err, "INVALID_LOCAL_ID")
		})
	}
}

func TestParseAlias(t *testing.T) {
	valid := []string{
		"SP-FULL",
		"MATT-FULL",
		"ECC-FULL",
		"MATT-SP-HYBRID",
	}
	for _, input := range valid {
		t.Run("valid_"+input, func(t *testing.T) {
			got, err := ParseAlias(input)
			if err != nil {
				t.Fatalf("ParseAlias(%q) returned error: %v", input, err)
			}
			if got.String() != input {
				t.Errorf("ParseAlias(%q).String() = %q, want %q", input, got.String(), input)
			}
		})
	}

	invalid := []string{
		"",
		"SP FULL",
		" SP-FULL",
		"SP-FULL ",
		"SP--FULL",
		"-SP-FULL",
		"SP-FULL-",
		"sp-full",
		"SP_FULL",
	}
	for _, input := range invalid {
		t.Run("invalid_"+input, func(t *testing.T) {
			_, err := ParseAlias(input)
			assertParseErrorCode(t, err, "INVALID_PROFILE_ALIAS")
		})
	}
}

func TestParseContentVersion(t *testing.T) {
	valid := []string{"1.0.0"}
	for _, input := range valid {
		t.Run("valid_"+input, func(t *testing.T) {
			got, err := ParseContentVersion(input)
			if err != nil {
				t.Fatalf("ParseContentVersion(%q) returned error: %v", input, err)
			}
			if got.String() != input {
				t.Errorf("ParseContentVersion(%q).String() = %q, want %q", input, got.String(), input)
			}
		})
	}

	invalid := []string{
		"",
		"1",
		"1.0",
		"v1.0.0",
		"1.0.0-beta",
		"01.0.0",
	}
	for _, input := range invalid {
		t.Run("invalid_"+input, func(t *testing.T) {
			_, err := ParseContentVersion(input)
			assertParseErrorCode(t, err, "INVALID_CONTENT_VERSION")
		})
	}
}

func assertParseErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", code)
	}
	if !strings.Contains(err.Error(), code) {
		t.Errorf("error %q does not contain %q", err, code)
	}
}
