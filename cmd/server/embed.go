//go:build !dev

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend_dist
var frontend embed.FS

func embeddedFrontend() fs.FS {
	return frontend
}
