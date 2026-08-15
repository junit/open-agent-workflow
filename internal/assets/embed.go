package assets

import (
	"embed"
	"io/fs"
)

// Provider and Binding identity evidence is embedded. Markdown Profiles remain
// the sole source of workflow semantics.
//
//go:embed providers/*.json
var embedded embed.FS

func FS() fs.FS { return embedded }
