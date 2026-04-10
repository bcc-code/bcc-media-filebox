//go:build dev

package main

import "io/fs"

func embeddedFrontend() fs.FS {
	return nil
}
