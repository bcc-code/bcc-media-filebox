//go:build darwin

package tus

import "golang.org/x/sys/unix"

// renameNoReplace is Darwin's atomic exclusive rename. A filesystem that does
// not implement RENAME_EXCL is rejected rather than risking an overwrite.
func renameNoReplace(oldPath, newPath string) error {
	return unix.RenamexNp(oldPath, newPath, unix.RENAME_EXCL)
}
