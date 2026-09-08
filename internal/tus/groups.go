package tus

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// groupOwnershipTTL is how long a group id stays bound to the user who first
// claimed it. It only has to outlive an upload's creation phase — every part's
// creation POST is sent up front, before any bytes move — so it is generous
// without pinning memory.
const groupOwnershipTTL = 24 * time.Hour

// maxGroupParts bounds the boundary list a client may declare. tus-js-client
// uses 3 or 6 parts depending on the negotiated protocol; the ceiling is only
// here so a hostile client cannot make the server parse an unbounded list.
const maxGroupParts = 64

// GroupRegistry binds a group id to the user who created its first part, so a
// second user cannot inject parts into someone else's upload.
//
// It is deliberately in memory rather than a table. A group's life is one
// upload session; the deployment is a single process (SQLite plus a local
// filestore); and after a restart a re-created part simply re-binds first-come
// to the same user, because the same client sends the same metadata. That
// avoids a schema change for state that is worthless the moment the upload
// finishes.
type GroupRegistry struct {
	mu     sync.Mutex
	owners map[string]groupOwner
	ttl    time.Duration
	now    func() time.Time
}

type groupOwner struct {
	userID  string
	expires time.Time
}

func NewGroupRegistry() *GroupRegistry {
	return &GroupRegistry{
		owners: map[string]groupOwner{},
		ttl:    groupOwnershipTTL,
		now:    time.Now,
	}
}

// Claim binds groupID to userID, or confirms an existing binding. It reports
// false when the group already belongs to somebody else.
func (r *GroupRegistry) Claim(groupID, userID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	// Opportunistic eviction: the map is only touched on upload creation, so
	// there is no separate sweep to run.
	for id, owner := range r.owners {
		if owner.expires.Before(now) {
			delete(r.owners, id)
		}
	}

	if owner, ok := r.owners[groupID]; ok && owner.userID != userID {
		return false
	}
	r.owners[groupID] = groupOwner{userID: userID, expires: now.Add(r.ttl)}
	return true
}

// Boundary is one part's byte range within the whole upload.
type Boundary struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// Size is the part's length, which is what the server matches Upload-Length
// against.
func (b Boundary) Size() int64 { return b.End - b.Start }

// ParseBoundaries reads and fully validates the client's boundary list.
//
// The checks are not input hygiene, they are the safety property the design
// rests on. filelocker locks per upload id, so the parts of one group write
// into the shared file with no mutual exclusion; only a list that is
// contiguous, exactly covers the file, and has distinct part lengths keeps two
// parts from overlapping. A list that fails any check is rejected before a
// single byte is accepted.
func ParseBoundaries(raw string, total int64) ([]Boundary, error) {
	if total <= 0 {
		return nil, fmt.Errorf("total must be positive, got %d", total)
	}

	var boundaries []Boundary
	if err := jsonUnmarshalStrict(raw, &boundaries); err != nil {
		return nil, fmt.Errorf("boundaries is not a JSON array of {start,end}: %w", err)
	}
	if len(boundaries) == 0 {
		return nil, fmt.Errorf("boundaries is empty")
	}
	if len(boundaries) > maxGroupParts {
		return nil, fmt.Errorf("boundaries has %d parts, at most %d allowed", len(boundaries), maxGroupParts)
	}

	sizes := make(map[int64]struct{}, len(boundaries))
	var next int64
	for i, b := range boundaries {
		if b.Start != next {
			return nil, fmt.Errorf("part %d starts at %d, expected %d: boundaries must be contiguous", i, b.Start, next)
		}
		if b.Size() <= 0 {
			return nil, fmt.Errorf("part %d is %d bytes: every part must be non-empty", i, b.Size())
		}
		if b.End > total {
			return nil, fmt.Errorf("part %d ends at %d, past the declared total of %d", i, b.End, total)
		}
		// Distinct lengths are what let the server tell the parts apart:
		// tus-js-client sends one shared metadata object and one shared header
		// set for every part, so Upload-Length is the only per-part signal.
		if _, clash := sizes[b.Size()]; clash {
			return nil, fmt.Errorf("part %d repeats the length %d: part lengths must be distinct", i, b.Size())
		}
		sizes[b.Size()] = struct{}{}
		next = b.End
	}
	if next != total {
		return nil, fmt.Errorf("boundaries cover %d bytes, expected %d", next, total)
	}
	return boundaries, nil
}

// MatchBoundary finds the part with exactly the given length. A length that
// matches none is a mismatch between what the client computed and what it
// sent, so the part is refused rather than guessed at.
func MatchBoundary(boundaries []Boundary, length int64) (Boundary, error) {
	for _, b := range boundaries {
		if b.Size() == length {
			return b, nil
		}
	}
	return Boundary{}, fmt.Errorf("no part of the declared boundaries is %d bytes long", length)
}

// FormatInt is the canonical rendering for the integer metadata values, which
// are strings on the wire.
func FormatInt(v int64) string { return strconv.FormatInt(v, 10) }

// jsonUnmarshalStrict rejects trailing content, so a truncated or doubled
// payload is an error rather than a partial parse.
func jsonUnmarshalStrict(raw string, out any) error {
	dec := json.NewDecoder(io.Reader(strings.NewReader(raw)))
	if err := dec.Decode(out); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("unexpected trailing content")
	}
	return nil
}
