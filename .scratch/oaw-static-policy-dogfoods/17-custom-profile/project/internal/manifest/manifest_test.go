package manifest

import "testing"

func TestValidateAcceptsReleaseManifest(t *testing.T) {
	got, err := Validate([]byte("version=1.2.3\ncommit=abc123\nowner=platform\n"))
	if err != nil || got != "valid release manifest: 1.2.3" {
		t.Fatalf("Validate() = %q, %v", got, err)
	}
}

func TestValidateRejectsMissingDefaultField(t *testing.T) {
	_, err := Validate([]byte("version=1.2.3\n"))
	if err == nil || err.Error() != `missing required field "commit"` {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsDuplicateField(t *testing.T) {
	_, err := Validate([]byte("version=1.2.3\nversion=1.2.4\ncommit=abc123\n"))
	if err == nil || err.Error() != `duplicate field "version"` {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsMalformedLine(t *testing.T) {
	_, err := Validate([]byte("version=1.2.3\nnot-a-field\ncommit=abc123\n"))
	if err == nil || err.Error() != `malformed line 2: expected key=value` {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRequiredAddsProjectField(t *testing.T) {
	data := []byte("version=1.2.3\ncommit=abc123\nowner=platform\n")
	got, err := ValidateRequired(data, []string{"version", "commit", "owner"})
	if err != nil || got != "valid release manifest: 1.2.3" {
		t.Fatalf("ValidateRequired() = %q, %v", got, err)
	}
}

func TestValidateRequiredRejectsMissingProjectField(t *testing.T) {
	_, err := ValidateRequired([]byte("version=1.2.3\ncommit=abc123\n"), []string{"owner"})
	if err == nil || err.Error() != `missing required field "owner"` {
		t.Fatalf("ValidateRequired() error = %v", err)
	}
}
