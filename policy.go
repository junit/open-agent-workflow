package oaw

import (
	"bytes"
	_ "embed"
)

//go:embed policy/ENGINEERING.md
var canonicalPolicy []byte

func CanonicalPolicy() []byte {
	return bytes.Clone(canonicalPolicy)
}
