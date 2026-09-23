package tus

import (
	"errors"
	"log"
	"os"
	"sync"
	"syscall"
)

// claimPath moves src onto dst only when dst is unoccupied, reporting
// os.ErrExist when the name is already taken. Three publication primitives are
// tried in order; each claims the name atomically, and none of them can
// overwrite a file belonging to anybody else:
//
//  1. the host's exclusive rename (renameNoReplace);
//  2. a hard-link claim, for filesystems that reject rename flags outright —
//     NFS answers EINVAL to any renameat2 flag, as do some FUSE mounts;
//  3. an O_EXCL placeholder that our own plain rename then replaces, for
//     filesystems that also lack usable hard links.
//
// EXDEV is passed through untranslated so callers can fall back to a copy.
func claimPath(src, dst string) error {
	err := renameNoReplace(src, dst)
	if !renameFlagsUnsupported(err) {
		return err
	}
	logDegradedPublication(err)

	err = linkClaim(src, dst)
	if !hardLinkUnsupported(err) {
		return err
	}
	return placeholderClaim(src, dst)
}

// linkClaim publishes src at dst by hard-linking it into place. link fails with
// EEXIST when the name is taken, so the claim is as exclusive as an atomic
// rename, and it needs no rename flags.
func linkClaim(src, dst string) error {
	if err := os.Link(src, dst); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		// Preserve move semantics on a reported failure. Dropping the link we
		// just created cannot lose data because src still names the same inode.
		_ = os.Remove(dst)
		return err
	}
	return nil
}

// placeholderClaim claims dst with an exclusively created empty file and then
// renames src over it. The only file a plain rename can replace here is the
// placeholder this call owns, and the payload still arrives by rename rather
// than being written under the final name, so no consumer of the target
// directory can observe a partially written file. A crash between the two
// steps leaves an empty file behind at dst; that costs the next attempt a
// collision suffix but never corrupts or replaces real data.
func placeholderClaim(src, dst string) error {
	placeholder, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	if err := placeholder.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}

// renameFlagsUnsupported reports whether err means the filesystem cannot honour
// the exclusive-rename flag at all, as opposed to the rename being rejected on
// its merits. Linux filesystems that do not implement renameat2 flags return
// EINVAL for any flag (NFS is the common case), Darwin returns ENOTSUP, and a
// pre-3.15 kernel has no renameat2 syscall at all.
func renameFlagsUnsupported(err error) bool {
	return errors.Is(err, syscall.EINVAL) ||
		errors.Is(err, syscall.ENOSYS) ||
		errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.EOPNOTSUPP)
}

// hardLinkUnsupported reports whether err means the filesystem has no usable
// hard links. POSIX reports that as EPERM from link; a genuine permission
// failure is EACCES, and a cross-device link is EXDEV, so neither is swallowed.
func hardLinkUnsupported(err error) bool {
	return renameFlagsUnsupported(err) || errors.Is(err, syscall.EPERM)
}

var degradedPublication sync.Once

// logDegradedPublication records once per process that the destination
// filesystem forced us off the atomic exclusive rename, so the weaker
// publication path is visible in the logs instead of being silent.
func logDegradedPublication(err error) {
	degradedPublication.Do(func() {
		log.Printf("exclusive rename unsupported on destination filesystem (%v): publishing via hard-link claim", err)
	})
}
