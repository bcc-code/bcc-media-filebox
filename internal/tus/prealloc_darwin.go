//go:build darwin

package tus

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// preallocate reserves size bytes for f on disk. F_PREALLOCATE allocates
// blocks past the current end of file (F_PEOFPOSMODE) but deliberately leaves
// the logical size alone, so the Truncate afterwards is what makes the file
// addressable at every offset. F_ALLOCATEALL asks for all-or-nothing rather
// than a partial reservation.
//
// APFS may refuse the reservation; the file is still made full length, just
// sparse, which is correct if unreserved.
func preallocate(f *os.File, size int64) error {
	store := &unix.Fstore_t{
		Flags:   unix.F_ALLOCATEALL,
		Posmode: unix.F_PEOFPOSMODE,
		Offset:  0,
		Length:  size,
	}
	if err := unix.FcntlFstore(f.Fd(), unix.F_PREALLOCATE, store); err != nil {
		if !errors.Is(err, unix.ENOTSUP) && !errors.Is(err, unix.EINVAL) {
			return err
		}
	}
	return f.Truncate(size)
}
