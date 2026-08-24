//go:build !linux && !darwin

package tus

import "os"

// renameNoReplace falls back to an atomic no-overwrite hard-link claim on
// platforms without a native exclusive rename. This requires oldPath and
// newPath to be on a filesystem that supports hard links. The Linux and Darwin
// builds use native atomic rename operations and do not have this limitation.
func renameNoReplace(oldPath, newPath string) error {
	if err := os.Link(oldPath, newPath); err != nil {
		return err
	}
	if err := os.Remove(oldPath); err != nil {
		// Preserve move semantics on a reported failure. Removing the link cannot
		// lose data because oldPath still names the same inode.
		_ = os.Remove(newPath)
		return err
	}
	return nil
}
