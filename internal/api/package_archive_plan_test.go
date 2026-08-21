package api

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
)

const testGiB = int64(1) << 30

func archiveSource(id, filename string, size int64) PackageArchiveSource {
	return PackageArchiveSource{ID: id, Filename: filename, Size: size}
}

// smallArchiveSources returns n distinct sources of the given size each.
func smallArchiveSources(n int, size int64) []PackageArchiveSource {
	sources := make([]PackageArchiveSource, n)
	for i := range sources {
		sources[i] = archiveSource(fmt.Sprintf("u%02d", i), fmt.Sprintf("f%02d.bin", i), size)
	}
	return sources
}

// memberNames flattens an artifact's members for order assertions.
func memberNames(artifact PackageArchiveArtifact) []string {
	names := make([]string, len(artifact.Members))
	for i, m := range artifact.Members {
		names[i] = m.ArchiveName
	}
	return names
}

func TestPlanPackageArchivesEmptyInput(t *testing.T) {
	plan, err := planPackageArchives(nil)
	if err != nil {
		t.Fatalf("planPackageArchives(nil) error: %v", err)
	}
	if len(plan.Artifacts) != 0 {
		t.Fatalf("got %d artifacts, want 0", len(plan.Artifacts))
	}
}

// At or below the direct-file limit every source stays a direct download in
// input order, with exact sizes and the original (unsanitised) filename.
func TestPlanPackageArchivesAllDirectAtLimit(t *testing.T) {
	sources := smallArchiveSources(packageArchiveDirectFileLimit, 5)
	// Direct artifacts must not rewrite names; that only matters inside a ZIP.
	sources[3].Filename = "weird:name/with\\separators"

	plan, err := planPackageArchives(sources)
	if err != nil {
		t.Fatalf("planPackageArchives: %v", err)
	}
	if len(plan.Artifacts) != len(sources) {
		t.Fatalf("got %d artifacts, want %d", len(plan.Artifacts), len(sources))
	}
	for i, artifact := range plan.Artifacts {
		if artifact.Kind != PackageArchiveArtifactDirect {
			t.Errorf("artifact %d kind = %q, want %q", i, artifact.Kind, PackageArchiveArtifactDirect)
		}
		if len(artifact.Members) != 1 {
			t.Fatalf("artifact %d has %d members, want 1", i, len(artifact.Members))
		}
		member := artifact.Members[0]
		if member.Source != sources[i] {
			t.Errorf("artifact %d source = %+v, want %+v", i, member.Source, sources[i])
		}
		if member.ArchiveName != sources[i].Filename {
			t.Errorf("artifact %d name = %q, want original %q", i, member.ArchiveName, sources[i].Filename)
		}
		if artifact.PayloadSize != sources[i].Size || artifact.EstimatedSize != sources[i].Size {
			t.Errorf("artifact %d sizes = (%d, %d), want exactly %d",
				i, artifact.PayloadSize, artifact.EstimatedSize, sources[i].Size)
		}
	}
}

// One file over the limit flips the whole package into ZIP mode: a single
// archive holding every source, in input order, with exact size accounting.
func TestPlanPackageArchivesSingleZIPJustOverLimit(t *testing.T) {
	n := packageArchiveDirectFileLimit + 1
	sources := make([]PackageArchiveSource, n)
	var payload int64
	for i := range sources {
		sources[i] = archiveSource(fmt.Sprintf("u%02d", i), fmt.Sprintf("f%02d.bin", i), int64(i+1))
		payload += int64(i + 1)
	}

	plan, err := planPackageArchives(sources)
	if err != nil {
		t.Fatalf("planPackageArchives: %v", err)
	}
	if len(plan.Artifacts) != 1 {
		t.Fatalf("got %d artifacts, want 1 ZIP", len(plan.Artifacts))
	}
	artifact := plan.Artifacts[0]
	if artifact.Kind != PackageArchiveArtifactZIP {
		t.Fatalf("kind = %q, want %q", artifact.Kind, PackageArchiveArtifactZIP)
	}
	if len(artifact.Members) != n {
		t.Fatalf("got %d members, want %d", len(artifact.Members), n)
	}
	for i, member := range artifact.Members {
		if member.Source.ID != sources[i].ID {
			t.Errorf("member %d is source %q, want %q (input order)", i, member.Source.ID, sources[i].ID)
		}
	}
	if artifact.PayloadSize != payload {
		t.Errorf("PayloadSize = %d, want %d", artifact.PayloadSize, payload)
	}
	// All names are "fNN.bin" (7 bytes), so the estimate is fully predictable.
	wantEstimated := payload + packageArchiveZIPEndOverhead +
		int64(n)*(packageArchiveZIPEntryFixedOverhead+2*7)
	if artifact.EstimatedSize != wantEstimated {
		t.Errorf("EstimatedSize = %d, want %d", artifact.EstimatedSize, wantEstimated)
	}
}

// When the total fits under the archive cap, even a file above the small-file
// limit is zipped along with everything else.
func TestPlanPackageArchivesZipsEverythingUnderCap(t *testing.T) {
	sources := smallArchiveSources(packageArchiveDirectFileLimit+1, 1)
	sources[5].Size = 20 * testGiB // above packageArchiveSmallFileLimit

	plan, err := planPackageArchives(sources)
	if err != nil {
		t.Fatalf("planPackageArchives: %v", err)
	}
	if len(plan.Artifacts) != 1 || plan.Artifacts[0].Kind != PackageArchiveArtifactZIP {
		t.Fatalf("plan = %+v, want a single ZIP containing all sources", plan.Artifacts)
	}
	if got := len(plan.Artifacts[0].Members); got != len(sources) {
		t.Fatalf("ZIP has %d members, want %d", got, len(sources))
	}
}

// Over the total cap, large files go direct and only small files are zipped.
// Artifact order must follow the first source index of each artifact, so the
// interleaved layout here pins the stable-sort behavior too.
func TestPlanPackageArchivesLargeFilesDirectOverCap(t *testing.T) {
	sources := smallArchiveSources(packageArchiveDirectFileLimit+1, testGiB)
	sources[2].Size = 60 * testGiB
	sources[7].Size = 60 * testGiB // total 129 GiB > 100 GiB cap

	plan, err := planPackageArchives(sources)
	if err != nil {
		t.Fatalf("planPackageArchives: %v", err)
	}
	if len(plan.Artifacts) != 3 {
		t.Fatalf("got %d artifacts, want ZIP + 2 direct", len(plan.Artifacts))
	}

	// First index order: ZIP starts at source 0, directs start at 2 and 7.
	if plan.Artifacts[0].Kind != PackageArchiveArtifactZIP {
		t.Errorf("artifact 0 kind = %q, want ZIP first (its first member is source 0)", plan.Artifacts[0].Kind)
	}
	for i, wantID := range map[int]string{1: "u02", 2: "u07"} {
		artifact := plan.Artifacts[i]
		if artifact.Kind != PackageArchiveArtifactDirect || artifact.Members[0].Source.ID != wantID {
			t.Errorf("artifact %d = kind %q source %q, want direct %q",
				i, artifact.Kind, artifact.Members[0].Source.ID, wantID)
		}
	}
	if got := len(plan.Artifacts[0].Members); got != len(sources)-2 {
		t.Errorf("ZIP has %d members, want %d", got, len(sources)-2)
	}
}

// Small files whose combined size exceeds the cap must split across several
// ZIPs, each under the cap, with every source appearing exactly once.
func TestPlanPackageArchivesSplitsAcrossZIPs(t *testing.T) {
	sources := smallArchiveSources(12, 9*testGiB) // 108 GiB total, each under 10 GiB

	plan, err := planPackageArchives(sources)
	if err != nil {
		t.Fatalf("planPackageArchives: %v", err)
	}
	if len(plan.Artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2 ZIPs", len(plan.Artifacts))
	}

	var seen []string
	for i, artifact := range plan.Artifacts {
		if artifact.Kind != PackageArchiveArtifactZIP {
			t.Errorf("artifact %d kind = %q, want ZIP", i, artifact.Kind)
		}
		if artifact.EstimatedSize > packageArchiveMaxSize {
			t.Errorf("artifact %d EstimatedSize %d exceeds cap %d", i, artifact.EstimatedSize, packageArchiveMaxSize)
		}
		for _, member := range artifact.Members {
			seen = append(seen, member.Source.ID)
		}
	}
	if len(seen) != len(sources) {
		t.Fatalf("plan covers %d sources, want %d", len(seen), len(sources))
	}
	for i, id := range seen {
		if id != sources[i].ID {
			t.Errorf("member order: position %d is %q, want %q", i, id, sources[i].ID)
		}
	}
}

// A zip-eligible source can be so large that even alone it leaves no room for
// the ZIP envelope under the cap. The planner must fall back to a direct
// artifact for it instead of failing or splitting the file.
func TestPlanPackageArchivesDirectFallbackAtCapBoundary(t *testing.T) {
	sources := smallArchiveSources(packageArchiveDirectFileLimit+1, 1)
	sources[0].Size = packageArchiveMaxSize - 1 // total ≤ cap, so everything is zip-eligible

	plan, err := planPackageArchives(sources)
	if err != nil {
		t.Fatalf("planPackageArchives: %v", err)
	}
	if len(plan.Artifacts) != 2 {
		t.Fatalf("got %d artifacts, want direct fallback + ZIP", len(plan.Artifacts))
	}
	fallback, rest := plan.Artifacts[0], plan.Artifacts[1]
	if fallback.Kind != PackageArchiveArtifactDirect || fallback.Members[0].Source.ID != sources[0].ID {
		t.Errorf("artifact 0 = kind %q source %q, want the oversized source as direct",
			fallback.Kind, fallback.Members[0].Source.ID)
	}
	if rest.Kind != PackageArchiveArtifactZIP || len(rest.Members) != len(sources)-1 {
		t.Errorf("artifact 1 = kind %q with %d members, want ZIP of the remaining %d",
			rest.Kind, len(rest.Members), len(sources)-1)
	}
}

func TestPlanPackageArchivesValidationErrors(t *testing.T) {
	cases := []struct {
		name    string
		sources []PackageArchiveSource
		wantErr error
	}{
		{
			name: "duplicate ID",
			sources: []PackageArchiveSource{
				archiveSource("dup", "a.txt", 1),
				archiveSource("dup", "b.txt", 1),
			},
			wantErr: errPackageArchiveDuplicateSource,
		},
		{
			name:    "negative size",
			sources: []PackageArchiveSource{archiveSource("neg", "a.txt", -1)},
			wantErr: errPackageArchiveNegativeSize,
		},
		{
			name: "total overflows int64",
			sources: []PackageArchiveSource{
				archiveSource("big1", "a.txt", math.MaxInt64),
				archiveSource("big2", "b.txt", 1),
			},
			wantErr: errPackageArchiveSizeOverflow,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := planPackageArchives(tc.sources)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// A source whose sanitised name is intrinsically too long can never be placed
// in any ZIP; the plan as a whole must fail.
func TestPlanPackageArchivesIntrinsicallyTooLongName(t *testing.T) {
	sources := smallArchiveSources(packageArchiveDirectFileLimit+1, 1)
	sources[0].Filename = strings.Repeat("a", packageArchiveZIPMaxNameBytes+1)

	_, err := planPackageArchives(sources)
	if !errors.Is(err, errPackageArchiveNameTooLong) {
		t.Fatalf("error = %v, want %v", err, errPackageArchiveNameTooLong)
	}
}

// A maximum-length name is valid once, but its duplicate cannot take a dedup
// suffix. The planner must start a fresh ZIP where the original name is free
// again, instead of erroring out.
func TestPlanPackageArchivesRestartsZIPOnDedupOverflow(t *testing.T) {
	maxName := strings.Repeat("a", packageArchiveZIPMaxNameBytes)
	sources := smallArchiveSources(packageArchiveDirectFileLimit+1, 1)
	sources[0].Filename = maxName
	sources[1].Filename = maxName

	plan, err := planPackageArchives(sources)
	if err != nil {
		t.Fatalf("planPackageArchives: %v", err)
	}
	if len(plan.Artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2 ZIPs", len(plan.Artifacts))
	}
	first, second := plan.Artifacts[0], plan.Artifacts[1]
	if first.Kind != PackageArchiveArtifactZIP || len(first.Members) != 1 || first.Members[0].ArchiveName != maxName {
		t.Errorf("artifact 0 = kind %q, names %v, want a ZIP holding just the first max-length name",
			first.Kind, len(first.Members))
	}
	if second.Kind != PackageArchiveArtifactZIP || len(second.Members) != len(sources)-1 {
		t.Fatalf("artifact 1 = kind %q with %d members, want ZIP with the rest", second.Kind, len(second.Members))
	}
	if second.Members[0].ArchiveName != maxName {
		t.Errorf("duplicate kept name of %d bytes, want the untouched original in the fresh ZIP",
			len(second.Members[0].ArchiveName))
	}
}

func TestPackageArchiveNameAllocator(t *testing.T) {
	t.Run("dedup suffixes preserve extension", func(t *testing.T) {
		a := newPackageArchiveNameAllocator()
		for i, want := range []string{"report.txt", "report (2).txt", "report (3).txt"} {
			got, err := a.allocate("report.txt")
			if err != nil {
				t.Fatalf("allocate #%d: %v", i+1, err)
			}
			if got != want {
				t.Errorf("allocate #%d = %q, want %q", i+1, got, want)
			}
		}
	})

	t.Run("dotfile is a stem not an extension", func(t *testing.T) {
		a := newPackageArchiveNameAllocator()
		a.allocate(".gitignore")
		got, err := a.allocate(".gitignore")
		if err != nil {
			t.Fatalf("allocate: %v", err)
		}
		if got != ".gitignore (2)" {
			t.Errorf("got %q, want %q", got, ".gitignore (2)")
		}
	})

	t.Run("no extension", func(t *testing.T) {
		a := newPackageArchiveNameAllocator()
		a.allocate("README")
		got, err := a.allocate("README")
		if err != nil {
			t.Fatalf("allocate: %v", err)
		}
		if got != "README (2)" {
			t.Errorf("got %q, want %q", got, "README (2)")
		}
	})

	// A literal name that matches an already-generated dedup name must itself
	// be deduplicated, never silently merged.
	t.Run("literal name collides with generated name", func(t *testing.T) {
		a := newPackageArchiveNameAllocator()
		a.allocate("a.txt")
		a.allocate("a.txt") // takes "a (2).txt"
		got, err := a.allocate("a (2).txt")
		if err != nil {
			t.Fatalf("allocate: %v", err)
		}
		if got != "a (2) (2).txt" {
			t.Errorf("got %q, want %q", got, "a (2) (2).txt")
		}
	})

	// The reverse order: a generated candidate that is already occupied by a
	// literal source must be skipped, not reused.
	t.Run("generated name skips occupied literal", func(t *testing.T) {
		a := newPackageArchiveNameAllocator()
		a.allocate("a (2).txt")
		a.allocate("a.txt")
		got, err := a.allocate("a.txt")
		if err != nil {
			t.Fatalf("allocate: %v", err)
		}
		if got != "a (3).txt" {
			t.Errorf("got %q, want %q", got, "a (3).txt")
		}
	})

	t.Run("max-length name is allowed once", func(t *testing.T) {
		a := newPackageArchiveNameAllocator()
		maxName := strings.Repeat("a", packageArchiveZIPMaxNameBytes)
		if _, err := a.allocate(maxName); err != nil {
			t.Fatalf("allocate max-length name: %v", err)
		}
		if _, err := a.allocate(maxName); !errors.Is(err, errPackageArchiveNameTooLong) {
			t.Fatalf("duplicate of max-length name: error = %v, want %v", err, errPackageArchiveNameTooLong)
		}
	})

	t.Run("over-length name is rejected", func(t *testing.T) {
		a := newPackageArchiveNameAllocator()
		long := strings.Repeat("a", packageArchiveZIPMaxNameBytes+1)
		if _, err := a.allocate(long); !errors.Is(err, errPackageArchiveNameTooLong) {
			t.Fatalf("error = %v, want %v", err, errPackageArchiveNameTooLong)
		}
	})
}

func TestPortablePackageArchiveName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain.txt", "plain.txt"},
		{"a/b", "a_b"},
		{`a\b`, "a_b"},
		{"a:b", "a_b"},
		{"a\x00b", "a_b"},
		{"a\nb", "a_b"},
		{"a\tb", "a_b"},
		{"a\x7fb", "a_b"}, // DEL is a control character too
		{"../../etc/passwd", ".._.._etc_passwd"},
		{"", "file"},
		{".", "file"},
		{"..", "file"},
		{"žürich ☃.txt", "žürich ☃.txt"}, // non-ASCII is preserved, not mangled
	}
	for _, tc := range cases {
		if got := portablePackageArchiveName(tc.in); got != tc.want {
			t.Errorf("portablePackageArchiveName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSplitPackageArchiveExtension(t *testing.T) {
	cases := []struct {
		in, stem, ext string
	}{
		{"a.txt", "a", ".txt"},
		{"archive.tar.gz", "archive.tar", ".gz"},
		{"README", "README", ""},
		{".bashrc", ".bashrc", ""},
		{"name.", "name.", ""},
	}
	for _, tc := range cases {
		stem, ext := splitPackageArchiveExtension(tc.in)
		if stem != tc.stem || ext != tc.ext {
			t.Errorf("splitPackageArchiveExtension(%q) = (%q, %q), want (%q, %q)",
				tc.in, stem, ext, tc.stem, tc.ext)
		}
	}
}

func TestPackageArchiveZIPEntryOverhead(t *testing.T) {
	got, err := packageArchiveZIPEntryOverhead("abc")
	if err != nil {
		t.Fatalf("packageArchiveZIPEntryOverhead: %v", err)
	}
	if want := packageArchiveZIPEntryFixedOverhead + 2*3; got != want {
		t.Errorf("overhead = %d, want %d", got, want)
	}

	long := strings.Repeat("a", packageArchiveZIPMaxNameBytes+1)
	if _, err := packageArchiveZIPEntryOverhead(long); !errors.Is(err, errPackageArchiveNameTooLong) {
		t.Errorf("over-length name: error = %v, want %v", err, errPackageArchiveNameTooLong)
	}
}

func TestCheckedPackageArchiveAdd(t *testing.T) {
	if got, err := checkedPackageArchiveAdd(1, 2); err != nil || got != 3 {
		t.Errorf("checkedPackageArchiveAdd(1, 2) = (%d, %v), want (3, nil)", got, err)
	}
	if _, err := checkedPackageArchiveAdd(math.MaxInt64, 1); !errors.Is(err, errPackageArchiveSizeOverflow) {
		t.Errorf("overflow: error = %v, want %v", err, errPackageArchiveSizeOverflow)
	}
	if _, err := checkedPackageArchiveAdd(-1, 1); !errors.Is(err, errPackageArchiveSizeOverflow) {
		t.Errorf("negative operand: error = %v, want %v", err, errPackageArchiveSizeOverflow)
	}
}

// EstimatedSize is documented as an upper bound for a Store archive with zero
// Modified times and no extra fields. Building the planned ZIP for real guards
// the hand-derived overhead constants against archive/zip changes.
func TestPackageArchiveEstimateIsUpperBound(t *testing.T) {
	sources := make([]PackageArchiveSource, packageArchiveDirectFileLimit+1)
	for i := range sources {
		// Duplicate names so the real archive is written with dedup suffixes.
		sources[i] = archiveSource(fmt.Sprintf("u%02d", i), "photo.jpg", int64(10*i))
	}

	plan, err := planPackageArchives(sources)
	if err != nil {
		t.Fatalf("planPackageArchives: %v", err)
	}
	if len(plan.Artifacts) != 1 || plan.Artifacts[0].Kind != PackageArchiveArtifactZIP {
		t.Fatalf("plan = %+v, want a single ZIP", plan.Artifacts)
	}
	artifact := plan.Artifacts[0]

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, member := range artifact.Members {
		fw, err := w.CreateHeader(&zip.FileHeader{
			Name:   member.ArchiveName,
			Method: zip.Store,
		})
		if err != nil {
			t.Fatalf("create %q: %v", member.ArchiveName, err)
		}
		if _, err := fw.Write(bytes.Repeat([]byte{'x'}, int(member.Source.Size))); err != nil {
			t.Fatalf("write %q: %v", member.ArchiveName, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	if int64(buf.Len()) > artifact.EstimatedSize {
		t.Fatalf("real ZIP is %d bytes, exceeds EstimatedSize %d", buf.Len(), artifact.EstimatedSize)
	}
}
