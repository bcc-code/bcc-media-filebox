//go:build !linux && !darwin

package tus

import "os"

// preallocate makes f size bytes long without reserving blocks — there is no
// portable reservation syscall. The file is addressable at every offset, which
// is what positioned writes require, but a later write can still fail with
// ENOSPC. The Linux and Darwin builds reserve properly.
func preallocate(f *os.File, size int64) error {
	return f.Truncate(size)
}
