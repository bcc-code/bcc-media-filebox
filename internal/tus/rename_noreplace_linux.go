//go:build linux

package tus

import "golang.org/x/sys/unix"

// renameNoReplace atomically moves oldPath to newPath only when newPath does
// not exist. Linux has provided renameat2(RENAME_NOREPLACE) since 3.15; failing
// closed on an older kernel/filesystem is safer than silently replacing data.
func renameNoReplace(oldPath, newPath string) error {
	return unix.Renameat2(unix.AT_FDCWD, oldPath, unix.AT_FDCWD, newPath, unix.RENAME_NOREPLACE)
}
