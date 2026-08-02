package check

import (
	"path/filepath"
	"testing"
)

func TestChecksumBytesMatchesPOSIXVectors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "empty", data: nil, want: "4294967295:0"},
		{name: "version", data: []byte("0.1.0\n"), want: "917384547:6"},
		{name: "binary", data: []byte{0, 1, 2, 0xff}, want: "2107141715:4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checksumBytes(tt.data); got != tt.want {
				t.Fatalf("checksumBytes(%v) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

func TestChecksumFileReportsMissingInput(t *testing.T) {
	if _, err := checksumFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("checksumFile() accepted a missing file")
	}
}
