package embed

import (
	"embed"
	"io/fs"
)

//go:embed dist
var DistFS embed.FS

func GetDistFS() fs.FS {
	f, err := fs.Sub(DistFS, "dist")
	if err != nil {
		return nil
	}
	return f
}

func GetAssetsFS() fs.FS {
	f, err := fs.Sub(DistFS, "dist/assets")
	if err != nil {
		return nil
	}
	return f
}
