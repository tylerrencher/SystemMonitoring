package ui

import (
	"embed"
	"io/fs"
)

//go:embed dist
var dist embed.FS

// FS returns the embedded frontend build, rooted at dist/.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
