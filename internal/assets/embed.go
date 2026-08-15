package assets

import (
	"embed"
	"io/fs"
)

// Only Provider and Binding identity evidence is embedded. Markdown Profiles
// remain the sole source of workflow semantics.
//go:embed providers/*.json audits/provider-sources-v5.json
var embedded embed.FS

func FS() fs.FS { return embedded }
