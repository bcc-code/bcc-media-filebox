//go:build !linux && !darwin

package tus

// renameNoReplace falls back to an atomic no-overwrite hard-link claim on
// platforms without a native exclusive rename. This requires oldPath and
// newPath to be on a filesystem that supports hard links; claimPath falls
// through to its placeholder claim when they are not. The Linux and Darwin
// builds use native atomic rename operations and do not have this limitation.
func renameNoReplace(oldPath, newPath string) error {
	return linkClaim(oldPath, newPath)
}
