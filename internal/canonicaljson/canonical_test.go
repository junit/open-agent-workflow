package canonicaljson

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestMarshalAndDigestAreDeterministic(t *testing.T) {
	value := struct {
		SchemaVersion string   `json:"schema_version"`
		Values        []string `json:"values"`
	}{"oaw.test/v1", []string{"a", "b"}}
	first, err := Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != `{"schema_version":"oaw.test/v1","values":["a","b"]}` || !bytes.Equal(first, second) {
		t.Fatalf("canonical outputs differ: %s / %s", first, second)
	}
	digest, encoded, err := Digest(value)
	if err != nil || !bytes.Equal(first, encoded) || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(digest) {
		t.Fatalf("Digest() = %q, %s, %v", digest, encoded, err)
	}
}

func TestMarshalRejectsOpenOrUnstableValues(t *testing.T) {
	for _, value := range []any{map[string]string{"a": "b"}, 1.5, func() {}} {
		if _, err := Marshal(value); err == nil || !strings.Contains(err.Error(), "CANONICAL_JSON_UNSUPPORTED") {
			t.Fatalf("Marshal(%T) error = %v", value, err)
		}
	}
}

func TestMarshalPreservesNonNilEmptyCollections(t *testing.T) {
	value := struct {
		Values []string `json:"values"`
	}{Values: []string{}}
	encoded, err := Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(encoded), `{"values":[]}`; got != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
}

func TestDigestBytesMatchesKnownSHA256(t *testing.T) {
	if got, want := DigestBytes([]byte("oaw")), "422068e530b93e9de2a70391bac7a5aab543e6e632a79756cecc685525c16f4d"; got != want {
		t.Fatalf("DigestBytes() = %q, want %q", got, want)
	}
}
