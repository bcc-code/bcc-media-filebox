//go:build linux

package tus

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// preallocate reserves size bytes for f on disk. fallocate(2) with mode 0 both
// reserves the blocks and extends the file, so ENOSPC surfaces here — at the
// first PATCH of an upload — rather than hours later during assembly. It also
// keeps the extent contiguous, which matters when N writers interleave into
// one file.
//
// Not every filesystem implements it (tmpfs and some network mounts do not);
// those report EOPNOTSUPP or ENOSYS, and a sparse file via Truncate is a
// correct if unreserved fallback.
func preallocate(f *os.File, size int64) error {
	err := unix.Fallocate(int(f.Fd()), 0, 0, size)
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) {
		return f.Truncate(size)
	}
	return err
}
