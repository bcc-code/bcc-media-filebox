package tus

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tus/tusd/v2/pkg/filestore"
	"github.com/tus/tusd/v2/pkg/handler"
)

const testGroupID = "01HZZZTESTGROUP000000000"

// distinctBoundaries is the reference implementation of the client-side
// boundary rule, mirroring boundariesFor() in useTusUpload.ts. It lives here
// so the server's validation is tested against exactly the shape the client is
// specified to produce; the two must be changed together.
//
// Why the lengths have to differ: tus-js-client hands every part the same
// metadata object and the same headers, so Upload-Length is the only per-part
// signal the server gets. The split is therefore equal-ish but deliberately
// uneven — each of the first N-1 parts is one more 4 KiB unit than the last,
// which makes the interior lengths strictly increasing, and the final part
// smaller than all of them. Alignment keeps the reflink option open.
//
// ok is false when the file is too small to split this way at all, in which
// case the client must not use parallel mode.
func distinctBoundaries(total int64, parts int) ([]Boundary, bool) {
	const align = 4096
	if parts <= 1 || total <= 0 {
		return nil, false
	}
	// Each interior part is base + (i+1)*align, so the interior parts consume
	// (parts-1)*base + align*parts*(parts-1)/2 and the remainder is the last
	// part. It must be positive, and smaller than every interior part.
	base := total / int64(parts) / align * align
	shift := int64(align) * int64(parts) * int64(parts-1) / 2
	if base <= 0 {
		return nil, false
	}
	interior := int64(parts-1)*base + shift
	last := total - interior
	if last <= 0 || last >= base+align {
		return nil, false
	}

	out := make([]Boundary, parts)
	var start int64
	for i := 0; i < parts-1; i++ {
		end := start + base + int64(i+1)*align
		out[i] = Boundary{Start: start, End: end}
		start = end
	}
	out[parts-1] = Boundary{Start: start, End: total}
	return out, true
}

func boundariesJSON(t *testing.T, bs []Boundary) string {
	t.Helper()
	parts := make([]string, len(bs))
	for i, b := range bs {
		parts[i] = fmt.Sprintf(`{"start":%d,"end":%d}`, b.Start, b.End)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func encodeMeta(pairs map[string]string) string {
	out := make([]string, 0, len(pairs))
	for k, v := range pairs {
		out = append(out, k+" "+base64.StdEncoding.EncodeToString([]byte(v)))
	}
	return strings.Join(out, ",")
}

// positionedFixture stands up a real tusd handler over the positioned store,
// with a pre-create callback that mirrors what Server.resolveUploadGroup does.
type positionedFixture struct {
	tempDir   string
	srv       *httptest.Server
	completed chan handler.HookEvent
}

func newPositionedFixture(t *testing.T) positionedFixture {
	t.Helper()
	tempDir := t.TempDir()
	store := NewPositionedStore(filestore.New(tempDir))
	composer := handler.NewStoreComposer()
	store.UseIn(composer)
	composer.UseConcater(DeferredConcater{})

	h, err := handler.NewHandler(handler.Config{
		BasePath:              "/files/",
		StoreComposer:         composer,
		NotifyCompleteUploads: true,
		PreUploadCreateCallback: func(hook handler.HookEvent) (handler.HTTPResponse, handler.FileInfoChanges, error) {
			meta := hook.Upload.MetaData
			if meta[MetaGroup] == "" {
				return handler.HTTPResponse{}, handler.FileInfoChanges{}, nil
			}
			total, err := strconv.ParseInt(meta[MetaTotal], 10, 64)
			if err != nil {
				return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("ERR_INVALID_GROUP", err.Error(), http.StatusBadRequest)
			}
			if hook.Upload.IsFinal {
				return handler.HTTPResponse{}, handler.FileInfoChanges{ID: meta[MetaGroup], MetaData: meta}, nil
			}
			boundaries, err := ParseBoundaries(meta[MetaBoundaries], total)
			if err != nil {
				return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("ERR_INVALID_GROUP", err.Error(), http.StatusBadRequest)
			}
			match, err := MatchBoundary(boundaries, hook.Upload.Size)
			if err != nil {
				return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("ERR_INVALID_GROUP", err.Error(), http.StatusBadRequest)
			}
			meta[MetaPartOffset] = FormatInt(match.Start)
			return handler.HTTPResponse{}, handler.FileInfoChanges{MetaData: meta}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	completed := make(chan handler.HookEvent, 16)
	go func() {
		for ev := range h.CompleteUploads {
			completed <- ev
		}
	}()

	srv := httptest.NewServer(http.StripPrefix("/files/", h))
	t.Cleanup(srv.Close)
	return positionedFixture{tempDir: tempDir, srv: srv, completed: completed}
}

func (f positionedFixture) do(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	req.Header.Set("Tus-Resumable", "1.0.0")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res
}

// createPart sends the partial creation POST and returns its upload URL.
func (f positionedFixture) createPart(t *testing.T, length int64, meta map[string]string) (string, int) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, f.srv.URL+"/files/", nil)
	req.Header.Set("Upload-Length", strconv.FormatInt(length, 10))
	req.Header.Set("Upload-Concat", "partial")
	req.Header.Set("Upload-Metadata", encodeMeta(meta))
	res := f.do(t, req)
	return res.Header.Get("Location"), res.StatusCode
}

func (f positionedFixture) patch(t *testing.T, url string, offset int64, body []byte) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	req.Header.Set("Upload-Offset", strconv.FormatInt(offset, 10))
	return f.do(t, req).StatusCode
}

func (f positionedFixture) groupBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.tempDir, testGroupID))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// groupUpload drives a whole parallel upload and returns the payload it sent.
func (f positionedFixture) groupUpload(t *testing.T, partSizes []int64, patchOrder []int) ([]byte, []string) {
	t.Helper()
	var total int64
	bounds := make([]Boundary, len(partSizes))
	var start int64
	for i, size := range partSizes {
		bounds[i] = Boundary{Start: start, End: start + size}
		start += size
		total += size
	}
	meta := map[string]string{
		MetaGroup:      testGroupID,
		MetaTotal:      strconv.FormatInt(total, 10),
		MetaBoundaries: boundariesJSON(t, bounds),
	}

	payload := make([]byte, 0, total)
	chunks := make([][]byte, len(partSizes))
	for i, size := range partSizes {
		chunks[i] = bytes.Repeat([]byte{byte('a' + i)}, int(size))
		payload = append(payload, chunks[i]...)
	}

	urls := make([]string, len(partSizes))
	for i, size := range partSizes {
		url, status := f.createPart(t, size, meta)
		if status != http.StatusCreated {
			t.Fatalf("create part %d: status %d", i, status)
		}
		urls[i] = url
	}
	for _, i := range patchOrder {
		if status := f.patch(t, urls[i], 0, chunks[i]); status != http.StatusNoContent {
			t.Fatalf("patch part %d: status %d", i, status)
		}
		<-f.completed
	}
	return payload, urls
}

func (f positionedFixture) finalPost(t *testing.T, urls []string, total int64) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, f.srv.URL+"/files/", nil)
	req.Header.Set("Upload-Concat", "final;"+strings.Join(urls, " "))
	req.Header.Set("Upload-Metadata", encodeMeta(map[string]string{
		MetaGroup: testGroupID,
		MetaTotal: strconv.FormatInt(total, 10),
	}))
	req.Header.Set("Tus-Resumable", "1.0.0")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res
}

// The headline behaviour: parts arriving in any order land in their declared
// positions, and the final POST finds the file already whole.
func TestPositionedPartsLandInOrderRegardlessOfArrival(t *testing.T) {
	f := newPositionedFixture(t)
	sizes := []int64{10, 7, 13}
	payload, urls := f.groupUpload(t, sizes, []int{2, 0, 1})

	if got := f.groupBytes(t); !bytes.Equal(got, payload) {
		t.Fatalf("group file = %q, want %q", got, payload)
	}

	// No per-partial binary may exist: that is the whole point, and it is also
	// what keeps peak temp disk at one file size.
	for _, u := range urls {
		id := path(u)
		if _, err := os.Stat(filepath.Join(f.tempDir, id)); !os.IsNotExist(err) {
			t.Errorf("partial %s must have no binary of its own (err=%v)", id, err)
		}
		if _, err := os.Stat(filepath.Join(f.tempDir, id+".info")); err != nil {
			t.Errorf("partial %s sidecar should exist: %v", id, err)
		}
	}

	res := f.finalPost(t, urls, int64(len(payload)))
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("final POST: status %d", res.StatusCode)
	}
	ev := <-f.completed
	if ev.Upload.ID != testGroupID {
		t.Fatalf("final upload id = %q, want the group id", ev.Upload.ID)
	}
	if got := f.groupBytes(t); !bytes.Equal(got, payload) {
		t.Fatalf("group file after final POST = %q, want %q", got, payload)
	}
}

// The regression the original design note would have shipped: filestore's
// NewUpload opens the binary O_TRUNC, so routing the final upload through it
// would zero a fully uploaded file at the moment of the final POST.
func TestPositionedFinalPostDoesNotTruncateTheGroupFile(t *testing.T) {
	f := newPositionedFixture(t)
	payload, urls := f.groupUpload(t, []int64{9, 4, 17}, []int{0, 1, 2})

	before := f.groupBytes(t)
	if !bytes.Equal(before, payload) {
		t.Fatalf("precondition: group file = %q, want %q", before, payload)
	}

	if res := f.finalPost(t, urls, int64(len(payload))); res.StatusCode != http.StatusCreated {
		t.Fatalf("final POST: status %d", res.StatusCode)
	}
	<-f.completed

	after := f.groupBytes(t)
	if len(after) == 0 {
		t.Fatal("the group file was truncated to zero by the final POST")
	}
	if !bytes.Equal(after, payload) {
		t.Fatalf("group file = %q, want %q", after, payload)
	}
}

// filestore reports stat(binPath).Size() as the offset and 404s on a missing
// binary. A positioned partial has neither, so the store has to answer from the
// sidecar — HEAD-based resume and the final POST's completeness check both
// depend on it.
func TestPositionedPartialReportsItsOwnOffsetNotTheGroupSize(t *testing.T) {
	f := newPositionedFixture(t)
	sizes := []int64{10, 7, 13}
	var total int64
	bounds := make([]Boundary, len(sizes))
	var start int64
	for i, size := range sizes {
		bounds[i] = Boundary{Start: start, End: start + size}
		start += size
		total += size
	}
	meta := map[string]string{
		MetaGroup:      testGroupID,
		MetaTotal:      strconv.FormatInt(total, 10),
		MetaBoundaries: boundariesJSON(t, bounds),
	}

	// Second part, so its offset inside the group file is non-zero: a
	// stat-based answer would be indistinguishable from a correct one at
	// offset 0.
	url, status := f.createPart(t, sizes[1], meta)
	if status != http.StatusCreated {
		t.Fatalf("create part: status %d", status)
	}

	head := func() (offset, length string, code int) {
		req, _ := http.NewRequest(http.MethodHead, url, nil)
		res := f.do(t, req)
		return res.Header.Get("Upload-Offset"), res.Header.Get("Upload-Length"), res.StatusCode
	}

	offset, length, code := head()
	if code != http.StatusOK {
		t.Fatalf("HEAD before any write: status %d (a positioned partial must not 404)", code)
	}
	if offset != "0" {
		t.Errorf("Upload-Offset = %q before any write, want 0", offset)
	}
	if length != strconv.FormatInt(sizes[1], 10) {
		t.Errorf("Upload-Length = %q, want %d (the part's size, not the group's)", length, sizes[1])
	}

	// Write half the part, then confirm resume sees exactly that.
	half := int(sizes[1] / 2)
	if status := f.patch(t, url, 0, bytes.Repeat([]byte("b"), half)); status != http.StatusNoContent {
		t.Fatalf("partial patch: status %d", status)
	}
	offset, _, code = head()
	if code != http.StatusOK {
		t.Fatalf("HEAD after partial write: status %d", code)
	}
	if offset != strconv.Itoa(half) {
		t.Fatalf("Upload-Offset = %q after writing %d bytes, want %d", offset, half, half)
	}
}

// A final POST before every part is complete must be refused, or the group
// file would be published with holes in it.
func TestPositionedFinalPostRejectedWhileAPartIsIncomplete(t *testing.T) {
	f := newPositionedFixture(t)
	sizes := []int64{10, 7}
	total := sizes[0] + sizes[1]
	meta := map[string]string{
		MetaGroup:      testGroupID,
		MetaTotal:      strconv.FormatInt(total, 10),
		MetaBoundaries: boundariesJSON(t, []Boundary{{0, 10}, {10, 17}}),
	}

	var urls []string
	for _, size := range sizes {
		url, status := f.createPart(t, size, meta)
		if status != http.StatusCreated {
			t.Fatalf("create part: status %d", status)
		}
		urls = append(urls, url)
	}
	// Only the first part is filled in.
	if status := f.patch(t, urls[0], 0, bytes.Repeat([]byte("a"), 10)); status != http.StatusNoContent {
		t.Fatalf("patch: status %d", status)
	}
	<-f.completed

	res := f.finalPost(t, urls, total)
	if res.StatusCode == http.StatusCreated {
		t.Fatal("final POST succeeded with an incomplete part")
	}
}

// A part may never write outside its own range: that is the only thing keeping
// concurrent parts from corrupting each other, since filelocker locks per
// upload id and not per group file.
func TestPositionedWriteChunkRefusesToLeaveItsRange(t *testing.T) {
	tempDir := t.TempDir()
	store := NewPositionedStore(filestore.New(tempDir))

	info := handler.FileInfo{
		ID:   "part-a",
		Size: 10,
		MetaData: handler.MetaData{
			MetaGroup:      testGroupID,
			MetaTotal:      "30",
			MetaPartOffset: "10",
		},
	}
	upload, err := store.NewUpload(context.Background(), info)
	if err != nil {
		t.Fatalf("new upload: %v", err)
	}

	// An offset past the part's own length is refused outright.
	if _, err := upload.WriteChunk(context.Background(), 11, bytes.NewReader([]byte("x"))); err == nil {
		t.Error("a write at an offset beyond the part's size must fail")
	}

	// A source longer than the part is clamped, never spilled into the
	// neighbouring part's range.
	n, err := upload.WriteChunk(context.Background(), 0, bytes.NewReader(bytes.Repeat([]byte("b"), 100)))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != 10 {
		t.Fatalf("wrote %d bytes, want the part's 10", n)
	}
	data, err := os.ReadFile(filepath.Join(tempDir, testGroupID))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 30 {
		t.Fatalf("group file is %d bytes, want the preallocated 30", len(data))
	}
	want := append(append(make([]byte, 10), bytes.Repeat([]byte("b"), 10)...), make([]byte, 10)...)
	if !bytes.Equal(data, want) {
		t.Fatalf("group file = %q, want %q", data, want)
	}
}

// A contract the hook could never produce, but the store is the last line of
// defence for it.
func TestPositionedNewUploadRejectsAPartThatDoesNotFit(t *testing.T) {
	store := NewPositionedStore(filestore.New(t.TempDir()))
	info := handler.FileInfo{
		ID:   "part-a",
		Size: 10,
		MetaData: handler.MetaData{
			MetaGroup:      testGroupID,
			MetaTotal:      "15",
			MetaPartOffset: "10",
		},
	}
	if _, err := store.NewUpload(context.Background(), info); err == nil {
		t.Fatal("a part running past the declared total must be refused")
	}
}

// A group id becomes a filename, and tusd's own id validation allows slashes.
func TestPositionedRejectsTraversalInGroupID(t *testing.T) {
	store := NewPositionedStore(filestore.New(t.TempDir()))
	for _, id := range []string{"../escape", "sub/dir", ".", ".."} {
		info := handler.FileInfo{
			ID:   "part-a",
			Size: 1,
			MetaData: handler.MetaData{
				MetaGroup:      id,
				MetaTotal:      "1",
				MetaPartOffset: "0",
			},
		}
		if _, err := store.NewUpload(context.Background(), info); err == nil {
			t.Errorf("group id %q must be refused", id)
		}
	}
}

// Uploads with no group metadata must be untouched by any of this.
func TestPositionedDelegatesPlainUploadsToFilestore(t *testing.T) {
	tempDir := t.TempDir()
	store := NewPositionedStore(filestore.New(tempDir))
	ctx := context.Background()

	upload, err := store.NewUpload(ctx, handler.FileInfo{ID: "plain", Size: 5})
	if err != nil {
		t.Fatalf("new upload: %v", err)
	}
	if _, err := upload.WriteChunk(ctx, 0, strings.NewReader("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tempDir, "plain"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("plain upload wrote %q", data)
	}

	// And it must come back through GetUpload with filestore's semantics.
	again, err := store.GetUpload(ctx, "plain")
	if err != nil {
		t.Fatalf("get upload: %v", err)
	}
	info, err := again.GetInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.Offset != 5 {
		t.Fatalf("offset = %d, want 5", info.Offset)
	}
}

// Terminating one part must not disturb the bytes the other parts have already
// written into the shared file.
func TestPositionedTerminateLeavesTheGroupFileAlone(t *testing.T) {
	tempDir := t.TempDir()
	store := NewPositionedStore(filestore.New(tempDir))
	ctx := context.Background()

	mk := func(id string, offset, size int64) handler.Upload {
		upload, err := store.NewUpload(ctx, handler.FileInfo{
			ID:   id,
			Size: size,
			MetaData: handler.MetaData{
				MetaGroup:      testGroupID,
				MetaTotal:      "20",
				MetaPartOffset: FormatInt(offset),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return upload
	}

	keep := mk("keep", 0, 10)
	drop := mk("drop", 10, 10)
	if _, err := keep.WriteChunk(ctx, 0, bytes.NewReader(bytes.Repeat([]byte("a"), 10))); err != nil {
		t.Fatal(err)
	}

	if err := store.AsTerminatableUpload(drop).Terminate(ctx); err != nil {
		t.Fatalf("terminate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, testGroupID)); err != nil {
		t.Fatalf("group file must survive a part's termination: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tempDir, testGroupID))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data[:10], bytes.Repeat([]byte("a"), 10)) {
		t.Fatalf("surviving part's bytes were disturbed: %q", data[:10])
	}
	if _, err := os.Stat(filepath.Join(tempDir, "drop.info")); !os.IsNotExist(err) {
		t.Errorf("terminated part's sidecar should be gone (err=%v)", err)
	}
}

func TestPreallocateMakesTheFileFullLength(t *testing.T) {
	path := filepath.Join(t.TempDir(), "group")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	const size = 3 << 20
	if err := preallocate(f, size); err != nil {
		t.Fatalf("preallocate: %v", err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Size() != size {
		t.Fatalf("size = %d, want %d", stat.Size(), size)
	}
}

// The loser of the creation race must not re-reserve over live bytes, and must
// repair a file the winner created but never sized.
func TestEnsureGroupFileIsIdempotentAndRepairsAShortFile(t *testing.T) {
	tempDir := t.TempDir()
	store := NewPositionedStore(filestore.New(tempDir))
	path := filepath.Join(tempDir, testGroupID)

	if err := store.ensureGroupFile(path, 1024); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("z"), 8), 0644); err != nil {
		t.Fatal(err)
	}
	// Simulates a crash between O_EXCL create and preallocate: the file exists
	// but is far too short.
	if err := store.ensureGroupFile(path, 1024); err != nil {
		t.Fatalf("second: %v", err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Size() != 1024 {
		t.Fatalf("size = %d, want the file repaired to 1024", stat.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data[:8], bytes.Repeat([]byte("z"), 8)) {
		t.Fatalf("existing bytes were clobbered: %q", data[:8])
	}
}

// The server must accept exactly the shape the client is specified to produce,
// for every file size worth splitting.
func TestDistinctBoundariesAreAcceptedByTheServer(t *testing.T) {
	sizes := []int64{
		1 << 30,    // 1 GiB
		264 << 30,  // the size that motivated the whole design
		100 << 20,  // 100 MiB
		100003,     // prime, not aligned to anything
		1 << 20,    // 1 MiB
		6*4096 + 1, // barely above the alignment floor
	}
	for _, total := range sizes {
		for _, parts := range []int{2, 3, 6} {
			bs, ok := distinctBoundaries(total, parts)
			if !ok {
				continue
			}
			parsed, err := ParseBoundaries(boundariesJSON(t, bs), total)
			if err != nil {
				t.Errorf("total=%d parts=%d: server rejected the client's split: %v", total, parts, err)
				continue
			}
			if len(parsed) != parts {
				t.Errorf("total=%d parts=%d: got %d parts", total, parts, len(parsed))
			}
			// And every part must be matchable by its length alone, which is
			// the only signal the server has.
			for i, b := range parsed {
				match, err := MatchBoundary(parsed, b.Size())
				if err != nil {
					t.Errorf("total=%d parts=%d: part %d unmatchable: %v", total, parts, i, err)
					continue
				}
				if match.Start != b.Start {
					t.Errorf("total=%d parts=%d: length %d matched offset %d, want %d", total, parts, b.Size(), match.Start, b.Start)
				}
			}
		}
	}
}

// A file too small to split into distinct aligned parts must be reported as
// such, so the client falls back to a single stream instead of emitting a list
// the server will reject.
func TestDistinctBoundariesRefusesFilesTooSmallToSplit(t *testing.T) {
	for _, c := range []struct {
		total int64
		parts int
	}{
		{total: 1, parts: 6},
		{total: 4095, parts: 2},
		{total: 0, parts: 3},
		{total: 1 << 30, parts: 1},
	} {
		if _, ok := distinctBoundaries(c.total, c.parts); ok {
			t.Errorf("total=%d parts=%d should be refused", c.total, c.parts)
		}
	}
}
