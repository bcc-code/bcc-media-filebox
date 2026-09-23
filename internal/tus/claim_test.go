package tus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
)

// Every publication tier must be exclusive on its own, because a filesystem
// that rejects rename flags leaves us relying on the weaker ones.
func TestClaimTiersNeverReplace(t *testing.T) {
	tiers := map[string]func(src, dst string) error{
		"claimPath":        claimPath,
		"linkClaim":        linkClaim,
		"placeholderClaim": placeholderClaim,
	}

	for name, claim := range tiers {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			dst := filepath.Join(dir, "occupied.bin")
			if err := os.WriteFile(dst, []byte("existing"), 0644); err != nil {
				t.Fatalf("write existing destination: %v", err)
			}
			src := filepath.Join(dir, "incoming")
			if err := os.WriteFile(src, []byte("incoming"), 0644); err != nil {
				t.Fatalf("write source: %v", err)
			}

			if err := claim(src, dst); !errors.Is(err, os.ErrExist) {
				t.Fatalf("claim of an occupied name returned %v, want os.ErrExist", err)
			}
			contents, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("read destination: %v", err)
			}
			if string(contents) != "existing" {
				t.Fatalf("destination contains %q, want it untouched", contents)
			}
			if _, err := os.Stat(src); err != nil {
				t.Fatalf("source must survive a refused claim: %v", err)
			}
		})
	}
}

func TestClaimTiersPublishExactlyOneWinner(t *testing.T) {
	tiers := map[string]func(src, dst string) error{
		"claimPath":        claimPath,
		"linkClaim":        linkClaim,
		"placeholderClaim": placeholderClaim,
	}

	for name, claim := range tiers {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			dst := filepath.Join(dir, "contended.bin")

			const claimants = 16
			sources := make([]string, claimants)
			for i := range sources {
				sources[i] = filepath.Join(dir, fmt.Sprintf("source-%02d", i))
				if err := os.WriteFile(sources[i], []byte(fmt.Sprintf("payload %02d", i)), 0644); err != nil {
					t.Fatalf("write source %d: %v", i, err)
				}
			}

			errs := make([]error, claimants)
			start := make(chan struct{})
			var wg sync.WaitGroup
			for i := range sources {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					errs[i] = claim(sources[i], dst)
				}()
			}
			close(start)
			wg.Wait()

			winner := -1
			for i, err := range errs {
				switch {
				case err == nil && winner >= 0:
					t.Fatalf("sources %d and %d both claimed %q", winner, i, dst)
				case err == nil:
					winner = i
				case !errors.Is(err, os.ErrExist):
					t.Fatalf("source %d failed with %v, want os.ErrExist", i, err)
				}
			}
			if winner < 0 {
				t.Fatal("no claimant published the destination")
			}

			contents, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("read destination: %v", err)
			}
			if want := fmt.Sprintf("payload %02d", winner); string(contents) != want {
				t.Fatalf("destination contains %q, want %q", contents, want)
			}
			if _, err := os.Stat(sources[winner]); !os.IsNotExist(err) {
				t.Fatalf("winning source still present: %v", err)
			}
		})
	}
}

// A filesystem without rename flags reports EINVAL, which must route to the
// fallback tiers; EXDEV must not, so the caller can still switch to a copy.
func TestUnsupportedClassification(t *testing.T) {
	cases := []struct {
		err          error
		renameFlags  bool
		hardLinkable bool
	}{
		{err: nil, renameFlags: false, hardLinkable: false},
		{err: syscall.EINVAL, renameFlags: true, hardLinkable: true},
		{err: syscall.ENOSYS, renameFlags: true, hardLinkable: true},
		{err: syscall.EPERM, renameFlags: false, hardLinkable: true},
		{err: syscall.EXDEV, renameFlags: false, hardLinkable: false},
		{err: os.ErrExist, renameFlags: false, hardLinkable: false},
		{err: syscall.EACCES, renameFlags: false, hardLinkable: false},
		{err: &os.LinkError{Err: syscall.EINVAL}, renameFlags: true, hardLinkable: true},
	}
	for _, tc := range cases {
		if got := renameFlagsUnsupported(tc.err); got != tc.renameFlags {
			t.Errorf("renameFlagsUnsupported(%v) = %v, want %v", tc.err, got, tc.renameFlags)
		}
		if got := hardLinkUnsupported(tc.err); got != tc.hardLinkable {
			t.Errorf("hardLinkUnsupported(%v) = %v, want %v", tc.err, got, tc.hardLinkable)
		}
	}
}
