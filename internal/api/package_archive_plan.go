package api

import (
	"errors"
	"fmt"
	"math"
	"path"
	"sort"
	"strings"
	"unicode"
)

const (
	packageArchiveDirectFileLimit = 10
	packageArchiveMaxSize         = int64(100 << 30)
	packageArchiveSmallFileLimit  = int64(10 << 30)

	// A ZIP made with archive/zip, Store, a zero-value Modified time and no
	// custom extra fields has, at most, the following non-payload bytes per
	// member:
	//
	//   local header       30 + len(name)
	//   ZIP64 descriptor   24
	//   central header     46 + len(name)
	//   ZIP64 extra field  28
	//
	// The regular end record is always present. The ZIP64 end record and
	// locator add another 76 bytes. We reserve all 98 bytes even for a small
	// archive, and reserve the ZIP64 form of every per-entry structure. This is
	// deliberately an upper bound: archive/zip omits ZIP64 structures when the
	// smaller representation is sufficient.
	packageArchiveZIPEntryFixedOverhead = int64(30 + 24 + 46 + 28)
	packageArchiveZIPEndOverhead        = int64(22 + 56 + 20)
	packageArchiveZIPMaxNameBytes       = 1<<16 - 1
)

var (
	errPackageArchiveDuplicateSource = errors.New("duplicate package archive source")
	errPackageArchiveNegativeSize    = errors.New("negative package archive source size")
	errPackageArchiveSizeOverflow    = errors.New("package archive size overflows int64")
	errPackageArchiveNameTooLong     = errors.New("package archive member name is too long")
)

// PackageArchiveArtifactKind describes what a recipient downloads. A direct
// source is represented as "file" to match the persisted/API artifact kind.
type PackageArchiveArtifactKind string

const (
	PackageArchiveArtifactDirect PackageArchiveArtifactKind = "file"
	PackageArchiveArtifactZIP    PackageArchiveArtifactKind = "zip"
)

// PackageArchiveSource is the part of an upload needed to make the delivery
// plan. ID is the upload ID; callers can use it to associate a planned member
// with a subsequently-created share or archive job input.
type PackageArchiveSource struct {
	ID       string
	Filename string
	Size     int64
}

// PackageArchiveMember retains its source identity while recording the name
// that must be used inside a ZIP. For direct artifacts ArchiveName is the
// original filename.
type PackageArchiveMember struct {
	Source      PackageArchiveSource
	ArchiveName string
}

// PackageArchiveArtifact is one downloadable object. ZIP artifacts can have
// several members; direct artifacts always have exactly one.
//
// EstimatedSize is exact for a direct file and a conservative upper bound for
// a Store/ZIP64 archive. A ZIP worker must keep Modified zero and Extra empty
// when creating member headers for that bound to remain valid.
type PackageArchiveArtifact struct {
	Kind          PackageArchiveArtifactKind
	Members       []PackageArchiveMember
	PayloadSize   int64
	EstimatedSize int64
}

// PackageArchivePlan is the complete, ordered set of recipient downloads.
type PackageArchivePlan struct {
	Artifacts []PackageArchiveArtifact
}

type indexedPackageArchiveSource struct {
	source PackageArchiveSource
	index  int
}

type orderedPackageArchiveArtifact struct {
	artifact   PackageArchiveArtifact
	firstIndex int
}

// planPackageArchives applies the package delivery policy without doing I/O.
// Input order is significant and is preserved within each ZIP. Artifact order
// follows the first source represented by each artifact.
func planPackageArchives(sources []PackageArchiveSource) (PackageArchivePlan, error) {
	totalSize, err := validatePackageArchiveSources(sources)
	if err != nil {
		return PackageArchivePlan{}, err
	}

	ordered := make([]orderedPackageArchiveArtifact, 0, len(sources))
	if len(sources) <= packageArchiveDirectFileLimit {
		for i, source := range sources {
			ordered = append(ordered, directPackageArchiveArtifact(source, i))
		}
		return finishPackageArchivePlan(ordered), nil
	}

	zipSources := make([]indexedPackageArchiveSource, 0, len(sources))
	zipEverything := totalSize <= packageArchiveMaxSize
	for i, source := range sources {
		if zipEverything || source.Size < packageArchiveSmallFileLimit {
			zipSources = append(zipSources, indexedPackageArchiveSource{source: source, index: i})
			continue
		}
		ordered = append(ordered, directPackageArchiveArtifact(source, i))
	}

	zips, directFallbacks, err := packPackageArchiveZIPs(zipSources)
	if err != nil {
		return PackageArchivePlan{}, err
	}
	ordered = append(ordered, zips...)
	ordered = append(ordered, directFallbacks...)

	return finishPackageArchivePlan(ordered), nil
}

func validatePackageArchiveSources(sources []PackageArchiveSource) (int64, error) {
	seen := make(map[string]struct{}, len(sources))
	var total int64
	for i, source := range sources {
		if _, duplicate := seen[source.ID]; duplicate {
			return 0, fmt.Errorf("%w: ID %q at index %d", errPackageArchiveDuplicateSource, source.ID, i)
		}
		seen[source.ID] = struct{}{}

		if source.Size < 0 {
			return 0, fmt.Errorf("%w: ID %q has size %d", errPackageArchiveNegativeSize, source.ID, source.Size)
		}
		if source.Size > math.MaxInt64-total {
			return 0, fmt.Errorf("%w while totaling source ID %q", errPackageArchiveSizeOverflow, source.ID)
		}
		total += source.Size
	}
	return total, nil
}

func directPackageArchiveArtifact(source PackageArchiveSource, index int) orderedPackageArchiveArtifact {
	return orderedPackageArchiveArtifact{
		firstIndex: index,
		artifact: PackageArchiveArtifact{
			Kind: PackageArchiveArtifactDirect,
			Members: []PackageArchiveMember{{
				Source:      source,
				ArchiveName: source.Filename,
			}},
			PayloadSize:   source.Size,
			EstimatedSize: source.Size,
		},
	}
}

func packPackageArchiveZIPs(sources []indexedPackageArchiveSource) (
	[]orderedPackageArchiveArtifact,
	[]orderedPackageArchiveArtifact,
	error,
) {
	zips := make([]orderedPackageArchiveArtifact, 0)
	directFallbacks := make([]orderedPackageArchiveArtifact, 0)
	current := newPackageArchiveZIPBuilder()

	for _, source := range sources {
		fits, err := current.tryAppend(source)
		// A maximum-length original name may only become too long after a
		// deduplication suffix is added. Starting a fresh ZIP lets it retain its
		// original valid name. Any error on an empty ZIP is intrinsic to the
		// source and must be returned.
		if err != nil && (current.empty() || !errors.Is(err, errPackageArchiveNameTooLong)) {
			return nil, nil, err
		}
		if err == nil && fits {
			continue
		}

		if !current.empty() {
			zips = append(zips, current.finish())
			current = newPackageArchiveZIPBuilder()

			fits, err = current.tryAppend(source)
			if err != nil {
				return nil, nil, err
			}
		}
		if !fits {
			// This can happen when the package's total payload is at the
			// 100 GiB boundary and one source by itself leaves no room for a
			// valid ZIP envelope. Keeping it direct is the only way to obey the
			// strict final-archive cap without splitting a source file.
			directFallbacks = append(directFallbacks, directPackageArchiveArtifact(source.source, source.index))
			current = newPackageArchiveZIPBuilder()
		}
	}

	if !current.empty() {
		zips = append(zips, current.finish())
	}
	return zips, directFallbacks, nil
}

type packageArchiveZIPBuilder struct {
	members       []PackageArchiveMember
	firstIndex    int
	payloadSize   int64
	estimatedSize int64
	names         *packageArchiveNameAllocator
}

func newPackageArchiveZIPBuilder() packageArchiveZIPBuilder {
	return packageArchiveZIPBuilder{
		firstIndex:    -1,
		estimatedSize: packageArchiveZIPEndOverhead,
		names:         newPackageArchiveNameAllocator(),
	}
}

func (b *packageArchiveZIPBuilder) empty() bool {
	return len(b.members) == 0
}

func (b *packageArchiveZIPBuilder) tryAppend(source indexedPackageArchiveSource) (bool, error) {
	archiveName, err := b.names.allocate(source.source.Filename)
	if err != nil {
		return false, err
	}

	entryOverhead, err := packageArchiveZIPEntryOverhead(archiveName)
	if err != nil {
		return false, err
	}
	nextPayload, err := checkedPackageArchiveAdd(b.payloadSize, source.source.Size)
	if err != nil {
		return false, err
	}
	nextEstimated, err := checkedPackageArchiveAdd(b.estimatedSize, source.source.Size)
	if err != nil {
		return false, err
	}
	nextEstimated, err = checkedPackageArchiveAdd(nextEstimated, entryOverhead)
	if err != nil {
		return false, err
	}
	if nextEstimated > packageArchiveMaxSize {
		return false, nil
	}

	if b.empty() {
		b.firstIndex = source.index
	}
	b.members = append(b.members, PackageArchiveMember{
		Source:      source.source,
		ArchiveName: archiveName,
	})
	b.payloadSize = nextPayload
	b.estimatedSize = nextEstimated
	return true, nil
}

func (b *packageArchiveZIPBuilder) finish() orderedPackageArchiveArtifact {
	return orderedPackageArchiveArtifact{
		firstIndex: b.firstIndex,
		artifact: PackageArchiveArtifact{
			Kind:          PackageArchiveArtifactZIP,
			Members:       b.members,
			PayloadSize:   b.payloadSize,
			EstimatedSize: b.estimatedSize,
		},
	}
}

func finishPackageArchivePlan(ordered []orderedPackageArchiveArtifact) PackageArchivePlan {
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].firstIndex < ordered[j].firstIndex
	})

	artifacts := make([]PackageArchiveArtifact, len(ordered))
	for i := range ordered {
		artifacts[i] = ordered[i].artifact
	}
	return PackageArchivePlan{Artifacts: artifacts}
}

func packageArchiveZIPEntryOverhead(name string) (int64, error) {
	if len(name) > packageArchiveZIPMaxNameBytes {
		return 0, fmt.Errorf("%w: %d bytes", errPackageArchiveNameTooLong, len(name))
	}
	nameBytes := int64(len(name))
	if nameBytes > (math.MaxInt64-packageArchiveZIPEntryFixedOverhead)/2 {
		return 0, errPackageArchiveSizeOverflow
	}
	return packageArchiveZIPEntryFixedOverhead + 2*nameBytes, nil
}

func checkedPackageArchiveAdd(a, b int64) (int64, error) {
	if a < 0 || b < 0 || b > math.MaxInt64-a {
		return 0, errPackageArchiveSizeOverflow
	}
	return a + b, nil
}

type packageArchiveNameAllocator struct {
	used map[string]struct{}
	next map[string]int
}

func newPackageArchiveNameAllocator() *packageArchiveNameAllocator {
	return &packageArchiveNameAllocator{
		used: make(map[string]struct{}),
		next: make(map[string]int),
	}
}

func (a *packageArchiveNameAllocator) allocate(original string) (string, error) {
	original = portablePackageArchiveName(original)
	if _, exists := a.used[original]; !exists {
		if len(original) > packageArchiveZIPMaxNameBytes {
			return "", fmt.Errorf("%w: %d bytes", errPackageArchiveNameTooLong, len(original))
		}
		a.used[original] = struct{}{}
		a.next[original] = 2
		return original, nil
	}

	stem, extension := splitPackageArchiveExtension(original)
	firstSuffix := a.next[original]
	if firstSuffix < 2 {
		// original may already be occupied by a generated name belonging to
		// another source (for example, a second "file.txt" reserves
		// "file (2).txt" before a source with that literal name is seen).
		firstSuffix = 2
	}
	for suffix := firstSuffix; ; suffix++ {
		candidate := fmt.Sprintf("%s (%d)%s", stem, suffix, extension)
		if len(candidate) > packageArchiveZIPMaxNameBytes {
			return "", fmt.Errorf("%w after deduplicating %q: %d bytes", errPackageArchiveNameTooLong, original, len(candidate))
		}
		if _, exists := a.used[candidate]; exists {
			continue
		}
		a.used[candidate] = struct{}{}
		a.next[original] = suffix + 1
		if a.next[candidate] < 2 {
			a.next[candidate] = 2
		}
		return candidate, nil
	}
}

// portablePackageArchiveName turns even a pre-filename-sanitisation upload row
// into one safe ZIP-local basename. ZIP readers commonly interpret both slash
// styles as path separators, so merely relying on path.Clean is insufficient
// for archives opened on a different operating system.
func portablePackageArchiveName(original string) string {
	var clean strings.Builder
	clean.Grow(len(original))
	for _, r := range original {
		switch {
		case r == '/', r == '\\', r == ':', r == 0, unicode.IsControl(r):
			clean.WriteByte('_')
		default:
			clean.WriteRune(r)
		}
	}
	name := clean.String()
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	return name
}

func splitPackageArchiveExtension(name string) (stem, extension string) {
	extension = path.Ext(name)
	// Treat a dotfile as a stem, not as an extension with an empty stem.
	base := path.Base(name)
	if extension == base || extension == "." {
		return name, ""
	}
	return name[:len(name)-len(extension)], extension
}
