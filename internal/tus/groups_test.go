package tus

import (
	"testing"
	"time"
)

func TestParseBoundariesRejectsUnsafeLists(t *testing.T) {
	const total = 30
	cases := []struct {
		name string
		raw  string
	}{
		{"not contiguous", `[{"start":0,"end":10},{"start":11,"end":30}]`},
		{"overlapping", `[{"start":0,"end":20},{"start":10,"end":30}]`},
		{"sum too small", `[{"start":0,"end":10},{"start":10,"end":25}]`},
		{"sum too large", `[{"start":0,"end":10},{"start":10,"end":40}]`},
		{"duplicate lengths", `[{"start":0,"end":15},{"start":15,"end":30}]`},
		{"empty part", `[{"start":0,"end":0},{"start":0,"end":30}]`},
		{"negative part", `[{"start":0,"end":-5},{"start":-5,"end":30}]`},
		{"does not start at zero", `[{"start":5,"end":15},{"start":15,"end":30}]`},
		{"empty list", `[]`},
		{"not an array", `{"start":0,"end":30}`},
		{"not json", `boundaries`},
		{"missing", ``},
		{"trailing content", `[{"start":0,"end":30}][{"start":0,"end":30}]`},
	}
	for _, c := range cases {
		if _, err := ParseBoundaries(c.raw, total); err == nil {
			t.Errorf("%s: expected rejection, got none (raw=%s)", c.name, c.raw)
		}
	}
}

func TestParseBoundariesAcceptsAValidList(t *testing.T) {
	got, err := ParseBoundaries(`[{"start":0,"end":10},{"start":10,"end":30}]`, 30)
	if err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d parts, want 2", len(got))
	}
	if got[0].Size() != 10 || got[1].Size() != 20 {
		t.Fatalf("sizes = %d,%d want 10,20", got[0].Size(), got[1].Size())
	}
}

func TestParseBoundariesRejectsTooManyParts(t *testing.T) {
	// A list long enough to be a denial-of-service on its own.
	raw := "["
	total := int64(0)
	for i := 0; i < maxGroupParts+1; i++ {
		if i > 0 {
			raw += ","
		}
		size := int64(i + 1)
		raw += `{"start":` + FormatInt(total) + `,"end":` + FormatInt(total+size) + `}`
		total += size
	}
	raw += "]"
	if _, err := ParseBoundaries(raw, total); err == nil {
		t.Fatal("a list past the part ceiling must be refused")
	}
}

func TestMatchBoundaryRefusesAnUnknownLength(t *testing.T) {
	bs, err := ParseBoundaries(`[{"start":0,"end":10},{"start":10,"end":30}]`, 30)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := MatchBoundary(bs, 20); err != nil || got.Start != 10 {
		t.Fatalf("MatchBoundary(20) = %+v, %v; want start 10", got, err)
	}
	if _, err := MatchBoundary(bs, 11); err == nil {
		t.Fatal("a length matching no part must be refused, never placed by guesswork")
	}
}

func TestGroupRegistryBindsGroupToItsFirstClaimant(t *testing.T) {
	r := NewGroupRegistry()

	if !r.Claim("g1", "bcc:alice") {
		t.Fatal("first claim should succeed")
	}
	// The same user's remaining parts must all be admitted.
	for i := 0; i < 5; i++ {
		if !r.Claim("g1", "bcc:alice") {
			t.Fatalf("re-claim %d by the owner should succeed", i)
		}
	}
	if r.Claim("g1", "bcc:bob") {
		t.Fatal("a second user must not be able to inject parts into someone else's group")
	}
	// Rejecting bob must not have disturbed alice's ownership.
	if !r.Claim("g1", "bcc:alice") {
		t.Fatal("owner lost the group after a rejected claim")
	}
	// A different group is unaffected.
	if !r.Claim("g2", "bcc:bob") {
		t.Fatal("bob should own an unclaimed group")
	}
}

func TestGroupRegistryExpiresBindings(t *testing.T) {
	r := NewGroupRegistry()
	now := time.Now()
	r.now = func() time.Time { return now }
	r.ttl = time.Hour

	if !r.Claim("g1", "bcc:alice") {
		t.Fatal("first claim should succeed")
	}
	now = now.Add(2 * time.Hour)
	// Once the binding has lapsed the id is free again, which is what lets a
	// restarted client re-create its parts.
	if !r.Claim("g1", "bcc:bob") {
		t.Fatal("an expired binding should be reclaimable")
	}
	if len(r.owners) != 1 {
		t.Fatalf("expired entries should be evicted, have %d", len(r.owners))
	}
}
