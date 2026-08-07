package assets

import (
	"embed"
	"io/fs"
)

//go:embed schemas/v1/*.json schemas/v2/*.json schemas/v3/*.json providers/*.json recipes/*.json audits/*.json conformance/*.json profile-aliases.json host-integrations.json
var embedded embed.FS

func FS() fs.FS { return embedded }
