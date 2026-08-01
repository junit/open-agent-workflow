package assets

import (
	"embed"
	"io/fs"
)

//go:embed schemas/v1/*.json providers/*.json recipes/*.json profile-aliases.json
var embedded embed.FS

func FS() fs.FS { return embedded }
