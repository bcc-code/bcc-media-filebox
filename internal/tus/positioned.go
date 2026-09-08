package tus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/tus/tusd/v2/pkg/filestore"
	"github.com/tus/tusd/v2/pkg/handler"
)

// Metadata keys carrying the positioning contract. They travel in MetaData and
// not in FileInfo.Storage because filestore's NewUpload replaces the Storage
// map wholesale with its own Type/Path/InfoPath, discarding anything a
// pre-create hook put there. MetaData survives: it is persisted into the .info
// sidecar and comes back from GetInfo.
const (
	// MetaGroup names the group file every part of one parallel upload writes
	// into. The final upload's ID is set to this value, so the group file is
	// already at tempDir/<final id> where finalizeUpload expects it.
	MetaGroup = "group"
	// MetaPartOffset is where this partial's bytes start inside the group file.
	// Present on partials only; the pre-create hook derives it by matching
	// Upload-Length against the client's boundary list.
	MetaPartOffset = "partOffset"
	// MetaTotal is the whole upload's size, used to preallocate the group file
	// and to bound every write.
	MetaTotal = "total"
	// MetaBoundaries is the client's part list, as JSON [{start,end},...]. Read
	// by the pre-create hook only; the store works from MetaPartOffset.
	MetaBoundaries = "boundaries"
)

// PositionedStore makes the parts of a parallel upload land in their final
// position as they arrive, so that when the last part finishes there is
// nothing left to concatenate.
//
// It wraps filestore and takes over the Core role. Uploads that carry no group
// metadata are handed straight to filestore, so ordinary single-stream uploads
// and clients predating this change behave exactly as before.
//
// Two properties of filestore make delegation impossible for group uploads,
// rather than merely inconvenient:
//
//   - NewUpload creates the binary through createFile, which opens
//     O_CREATE|O_WRONLY|O_TRUNC. Pointing it at a preallocated group file — or
//     at a finished one, which is what happens when the final upload's ID is
//     the group id — truncates it to zero.
//   - GetUpload ignores the sidecar's offset and reports stat(binPath).Size()
//     instead, turning a missing binary into ErrNotFound. A positioned partial
//     has no binary of its own, so delegating would either 404 every partial or
//     report the whole group's size as each partial's offset. Both break
//     HEAD-based resume and the final POST's sizeOfUploads completeness check.
type PositionedStore struct {
	filestore.FileStore
}

// NewPositionedStore wraps a filestore.
func NewPositionedStore(store filestore.FileStore) PositionedStore {
	return PositionedStore{FileStore: store}
}

// UseIn registers the store's roles on a composer.
//
// It deliberately does not register a Concater. Concatenation only works here
// with DeferredConcater, which the caller installs: filestore's own
// ConcatUploads type-asserts its arguments to *fileUpload and would panic on a
// positioned upload, and copying bytes is the very work this store exists to
// avoid.
func (s PositionedStore) UseIn(composer *handler.StoreComposer) {
	composer.UseCore(s)
	composer.UseTerminater(s)
	composer.UseLengthDeferrer(s)
	composer.UseContentServer(s)
}

// The three As* methods below must be defined even though the embedded
// filestore already provides them: its implementations type-assert their
// argument to *fileUpload, so a positioned upload would panic the handler
// rather than fail a request. A DELETE on one partial of a parallel upload is
// enough to hit it.

// AsTerminatableUpload implements handler.TerminaterDataStore.
func (s PositionedStore) AsTerminatableUpload(upload handler.Upload) handler.TerminatableUpload {
	if positioned, ok := upload.(*positionedUpload); ok {
		return positioned
	}
	return s.FileStore.AsTerminatableUpload(upload)
}

// AsLengthDeclarableUpload implements handler.LengthDeferrerDataStore.
func (s PositionedStore) AsLengthDeclarableUpload(upload handler.Upload) handler.LengthDeclarableUpload {
	if positioned, ok := upload.(*positionedUpload); ok {
		return positioned
	}
	return s.FileStore.AsLengthDeclarableUpload(upload)
}

// AsServableUpload implements handler.ContentServerDataStore.
func (s PositionedStore) AsServableUpload(upload handler.Upload) handler.ServableUpload {
	if positioned, ok := upload.(*positionedUpload); ok {
		return positioned
	}
	return s.FileStore.AsServableUpload(upload)
}

// groupInfo is the positioning contract read back off an upload's metadata.
type groupInfo struct {
	id         string
	total      int64
	partOffset int64
	// isPart distinguishes a partial, which owns a slice of the group file,
	// from the final upload, whose binary *is* the whole group file.
	isPart bool
}

// readGroupInfo extracts the positioning contract, reporting ok=false for an
// upload that is not part of a group. A malformed contract is an error rather
// than a silent fall-through to the legacy path: the hook is the only writer of
// these keys, so anything unparseable means a bug, and guessing could put bytes
// at the wrong offset.
func readGroupInfo(info handler.FileInfo) (groupInfo, bool, error) {
	id := info.MetaData[MetaGroup]
	if id == "" {
		return groupInfo{}, false, nil
	}
	if err := validateGroupID(id); err != nil {
		return groupInfo{}, false, err
	}

	total, err := strconv.ParseInt(info.MetaData[MetaTotal], 10, 64)
	if err != nil || total <= 0 {
		return groupInfo{}, false, fmt.Errorf("group %s: invalid %s metadata %q", id, MetaTotal, info.MetaData[MetaTotal])
	}

	g := groupInfo{id: id, total: total}

	raw, hasOffset := info.MetaData[MetaPartOffset]
	if !hasOffset {
		// No part offset: this is the final upload, which owns the whole file.
		if info.Size != total {
			return groupInfo{}, false, fmt.Errorf("group %s: final upload is %d bytes, expected %d", id, info.Size, total)
		}
		return g, true, nil
	}

	g.isPart = true
	g.partOffset, err = strconv.ParseInt(raw, 10, 64)
	if err != nil || g.partOffset < 0 {
		return groupInfo{}, false, fmt.Errorf("group %s: invalid %s metadata %q", id, MetaPartOffset, raw)
	}
	// The safety property the whole design rests on: filelocker locks per
	// upload id, so concurrent parts writing into one group file are not
	// mutually excluded. Nothing but disjoint, in-bounds ranges keeps them
	// from corrupting each other.
	if info.Size < 0 || g.partOffset+info.Size > total {
		return groupInfo{}, false, fmt.Errorf("group %s: part [%d,%d) does not fit in %d bytes", id, g.partOffset, g.partOffset+info.Size, total)
	}
	return g, true, nil
}

// validateGroupID keeps a group id from escaping the temp directory. The id
// becomes a filename, and tusd's own upload-id validation permits slashes
// (filestore maps them onto subdirectories), so this has to be stricter.
func validateGroupID(id string) error {
	if id == "" || id == "." || id == ".." {
		return fmt.Errorf("invalid group id %q", id)
	}
	if filepath.Base(id) != id || filepath.IsAbs(id) {
		return fmt.Errorf("invalid group id %q", id)
	}
	return nil
}

func (s PositionedStore) groupPath(id string) string {
	return filepath.Join(s.Path, id)
}

func (s PositionedStore) infoPath(id string) string {
	return filepath.Join(s.Path, id+".info")
}

// NewUpload creates a positioned upload for a group member, and delegates
// everything else to filestore.
func (s PositionedStore) NewUpload(ctx context.Context, info handler.FileInfo) (handler.Upload, error) {
	g, ok, err := readGroupInfo(info)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s.FileStore.NewUpload(ctx, info)
	}

	groupPath := s.groupPath(g.id)
	if g.isPart {
		// Like filestore, the store mints the id: tusd leaves info.ID empty
		// unless a hook set it, and only the final upload's id is the hook's
		// to choose.
		if info.ID == "" {
			id, err := newUploadID()
			if err != nil {
				return nil, err
			}
			info.ID = id
		}
		// First part to arrive creates and reserves the file; the others find
		// it already there.
		if err := s.ensureGroupFile(groupPath, g.total); err != nil {
			return nil, err
		}
	} else {
		// The final upload's binary must *be* the group file, or finalizeUpload
		// would look for the assembled bytes at tempDir/<some other id> and
		// find nothing.
		switch info.ID {
		case "":
			info.ID = g.id
		case g.id:
		default:
			return nil, fmt.Errorf("group %s: final upload has id %q, expected the group id", g.id, info.ID)
		}
	}

	// Recorded for parity with filestore and for anything reading the sidecar
	// directly, but this store never reads Path back — see GetUpload.
	info.Storage = map[string]string{
		"Type":     "positioned",
		"Path":     groupPath,
		"InfoPath": s.infoPath(info.ID),
	}

	upload := s.newPositionedUpload(info, g)
	if err := upload.writeInfo(); err != nil {
		return nil, err
	}
	return upload, nil
}

// GetUpload reads an upload back. A positioned partial's offset comes from its
// sidecar, because it has no binary file whose size could stand in for it.
func (s PositionedStore) GetUpload(ctx context.Context, id string) (handler.Upload, error) {
	data, err := os.ReadFile(s.infoPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, handler.ErrNotFound
		}
		return nil, err
	}
	var info handler.FileInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	g, ok, err := readGroupInfo(info)
	if err != nil {
		return nil, err
	}
	// Only partials need our offset handling. The final upload's binary is the
	// whole group file, so filestore's stat-based offset is already right for
	// it, and it is never PATCHed — tusd rejects that with ErrModifyFinal.
	if !ok || !g.isPart {
		return s.FileStore.GetUpload(ctx, id)
	}
	return s.newPositionedUpload(info, g), nil
}

func (s PositionedStore) newPositionedUpload(info handler.FileInfo, g groupInfo) *positionedUpload {
	return &positionedUpload{
		info:      info,
		group:     g,
		groupPath: s.groupPath(g.id),
		infoPath:  s.infoPath(info.ID),
		dirMode:   s.DirModePerm,
		fileMode:  s.FileModePerm,
	}
}

// newUploadID mirrors the ids tusd's filestore mints — 128 bits of randomness
// as hex — so nothing downstream can tell a positioned partial's id apart from
// an ordinary upload's. tusd's own generator lives in an internal package.
func newUploadID() (string, error) {
	buf := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("generate upload id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// ensureGroupFile creates the group file at its full length, once, however many
// parts race to do it.
//
// The loser of the O_EXCL race re-checks the length rather than assuming the
// winner finished: a crash between creating and reserving would otherwise
// leave every later part writing into a zero-length file, losing the ENOSPC
// guarantee silently. Reserving twice is harmless.
func (s PositionedStore) ensureGroupFile(path string, total int64) error {
	dirMode := s.DirModePerm
	if dirMode == 0 {
		dirMode = 0755
	}
	fileMode := s.FileModePerm
	if fileMode == 0 {
		fileMode = 0664
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fileMode)
	switch {
	case err == nil:
		defer f.Close()
		return preallocate(f, total)
	case errors.Is(err, os.ErrExist):
		return s.reserveExistingGroupFile(path, total, fileMode)
	case os.IsNotExist(err):
		if mkErr := os.MkdirAll(filepath.Dir(path), dirMode); mkErr != nil {
			return fmt.Errorf("create temp directory for %s: %w", path, mkErr)
		}
		return s.ensureGroupFile(path, total)
	default:
		return err
	}
}

func (s PositionedStore) reserveExistingGroupFile(path string, total int64, fileMode fs.FileMode) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if stat.Size() >= total {
		return nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY, fileMode)
	if err != nil {
		return err
	}
	defer f.Close()
	return preallocate(f, total)
}

// positionedUpload is one part of a parallel upload, or the final upload that
// stands for all of them. Its bytes live inside the shared group file; it owns
// no binary file of its own.
type positionedUpload struct {
	info      handler.FileInfo
	group     groupInfo
	groupPath string
	infoPath  string
	dirMode   fs.FileMode
	fileMode  fs.FileMode
}

var (
	_ handler.Upload                 = (*positionedUpload)(nil)
	_ handler.TerminatableUpload     = (*positionedUpload)(nil)
	_ handler.LengthDeclarableUpload = (*positionedUpload)(nil)
	_ handler.ServableUpload         = (*positionedUpload)(nil)
)

// WriteChunk writes into the group file at this part's own offset.
//
// tusd validates offset against GetInfo and caps src at the declared size, but
// both bounds are re-checked here. This is the one place where a wrong number
// would corrupt a *different* part of the same file, and the client is the
// party that chose the boundaries.
func (u *positionedUpload) WriteChunk(ctx context.Context, offset int64, src io.Reader) (int64, error) {
	if offset < 0 || offset > u.info.Size {
		return 0, fmt.Errorf("offset %d outside part of %d bytes", offset, u.info.Size)
	}
	remaining := u.info.Size - offset
	if remaining == 0 {
		return 0, nil
	}

	f, err := os.OpenFile(u.groupPath, os.O_WRONLY, u.fileMode)
	if err != nil {
		return 0, err
	}

	// NewOffsetWriter gives pwrite semantics: no shared seek position, so
	// concurrent parts writing to disjoint ranges of the same file cannot
	// interfere.
	n, err := io.Copy(io.NewOffsetWriter(f, u.group.partOffset+offset), io.LimitReader(src, remaining))
	u.info.Offset = offset + n

	// The offset is persisted even on a failed or short write: it is the only
	// record of how far this part got, and a resume reads it back through
	// GetUpload. Losing it would restart the part from zero.
	infoErr := u.writeInfo()
	closeErr := f.Close()

	if err != nil {
		return n, err
	}
	if infoErr != nil {
		return n, infoErr
	}
	return n, closeErr
}

func (u *positionedUpload) GetInfo(ctx context.Context) (handler.FileInfo, error) {
	return u.info, nil
}

// GetReader reads back just this part's slice of the group file.
func (u *positionedUpload) GetReader(ctx context.Context) (io.ReadCloser, error) {
	f, err := os.Open(u.groupPath)
	if err != nil {
		return nil, err
	}
	return sectionReadCloser{
		SectionReader: io.NewSectionReader(f, u.group.partOffset, u.info.Size),
		closer:        f,
	}, nil
}

func (u *positionedUpload) FinishUpload(ctx context.Context) error {
	return nil
}

// Terminate drops the part's bookkeeping and deliberately leaves the group
// file alone: the other parts are still writing into it. An abandoned group
// file is the temp reaper's business.
func (u *positionedUpload) Terminate(ctx context.Context) error {
	if err := os.Remove(u.infoPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// DeclareLength is refused. A part's length is what identifies which boundary
// it belongs to, so it has to be known before the first byte is accepted.
func (u *positionedUpload) DeclareLength(ctx context.Context, length int64) error {
	return handler.NewError("ERR_DEFER_LENGTH_UNSUPPORTED", "a parallel upload part must declare its length up front", http.StatusBadRequest)
}

func (u *positionedUpload) ServeContent(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	f, err := os.Open(u.groupPath)
	if err != nil {
		return err
	}
	defer f.Close()
	http.ServeContent(w, r, u.info.ID, u.modTime(), io.NewSectionReader(f, u.group.partOffset, u.info.Size))
	return nil
}

func (u *positionedUpload) modTime() (t time.Time) {
	if stat, err := os.Stat(u.groupPath); err == nil {
		return stat.ModTime()
	}
	return t
}

// writeInfo persists the sidecar. Written in full each time, like filestore
// does, so the file is never a merge of two states.
func (u *positionedUpload) writeInfo() error {
	data, err := json.Marshal(u.info)
	if err != nil {
		return err
	}
	if err := os.WriteFile(u.infoPath, data, u.fileMode); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if mkErr := os.MkdirAll(filepath.Dir(u.infoPath), u.dirMode); mkErr != nil {
			return mkErr
		}
		return os.WriteFile(u.infoPath, data, u.fileMode)
	}
	return nil
}

// sectionReadCloser closes the underlying file when the reader is closed.
type sectionReadCloser struct {
	*io.SectionReader
	closer io.Closer
}

func (r sectionReadCloser) Close() error { return r.closer.Close() }
