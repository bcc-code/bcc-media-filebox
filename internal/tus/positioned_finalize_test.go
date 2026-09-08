package tus

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	db "filebox/internal/db/gen"

	"github.com/tus/tusd/v2/pkg/handler"
)

func TestNeedsAssembly(t *testing.T) {
	cases := []struct {
		name string
		info handler.FileInfo
		want bool
	}{
		{
			name: "positioned parallel upload is already whole",
			info: handler.FileInfo{
				PartialUploads: []string{"a", "b"},
				MetaData:       handler.MetaData{MetaGroup: testGroupID},
			},
			want: false,
		},
		{
			name: "legacy parallel upload still needs the copy pass",
			info: handler.FileInfo{PartialUploads: []string{"a", "b"}},
			want: true,
		},
		{
			name: "single-stream upload has nothing to assemble",
			info: handler.FileInfo{},
			want: false,
		},
		{
			name: "empty group metadata is not a group",
			info: handler.FileInfo{
				PartialUploads: []string{"a"},
				MetaData:       handler.MetaData{MetaGroup: ""},
			},
			want: true,
		},
	}
	for _, c := range cases {
		if got := needsAssembly(c.info); got != c.want {
			t.Errorf("%s: needsAssembly = %v, want %v", c.name, got, c.want)
		}
	}
}

// Finalization of a positioned upload must publish the group file as it stands
// and clean up the parts, without ever looking for a per-partial binary.
func TestFinalizeUploadPublishesPositionedGroupFileWithoutAssembly(t *testing.T) {
	tempDir := t.TempDir()
	uploadDir := t.TempDir()
	queries := newTestDB(t)
	ep := NewEventProcessor(queries, uploadDir, tempDir, nil)
	ctx := context.Background()

	payload := bytes.Repeat([]byte("p"), 4096)
	partSizes := []int64{1024, 1024 + 512, 1536}
	var total int64
	for _, s := range partSizes {
		total += s
	}
	if total != int64(len(payload)) {
		t.Fatalf("fixture: parts sum to %d, payload is %d", total, len(payload))
	}

	// The group file: whole already, exactly as the parts left it.
	if err := os.WriteFile(filepath.Join(tempDir, testGroupID), payload, 0644); err != nil {
		t.Fatal(err)
	}

	info := handler.FileInfo{
		ID:   testGroupID,
		Size: total,
		MetaData: handler.MetaData{
			"filename": "clip.mov",
			MetaGroup:  testGroupID,
			MetaTotal:  strconv.FormatInt(total, 10),
		},
		IsFinal: true,
	}

	// Parts: sidecars and rows, but deliberately no binaries — a positioned
	// part never has one.
	var offset int64
	for i, size := range partSizes {
		id := "part" + strconv.Itoa(i)
		info.PartialUploads = append(info.PartialUploads, id)
		sidecar, err := json.Marshal(handler.FileInfo{
			ID: id, Size: size, Offset: size, IsPartial: true,
			MetaData: handler.MetaData{
				MetaGroup:      testGroupID,
				MetaTotal:      strconv.FormatInt(total, 10),
				MetaPartOffset: strconv.FormatInt(offset, 10),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tempDir, id+".info"), sidecar, 0644); err != nil {
			t.Fatal(err)
		}
		if err := queries.CreatePendingUpload(ctx, db.CreatePendingUploadParams{ID: id, Size: size, IsPartial: 1}); err != nil {
			t.Fatal(err)
		}
		offset += size
	}
	if err := queries.CreatePendingUpload(ctx, db.CreatePendingUploadParams{ID: testGroupID, Size: total}); err != nil {
		t.Fatal(err)
	}
	if err := queries.CompleteUpload(ctx, testGroupID); err != nil {
		t.Fatal(err)
	}

	ep.finalizeUpload(info, time.Now())

	// The bytes must have landed in the default target, unchanged.
	published := filepath.Join(uploadDir, "RawMaterial", "clip.mov")
	got, err := os.ReadFile(published)
	if err != nil {
		t.Fatalf("upload was not published: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("published %d bytes, want %d", len(got), len(payload))
	}

	row, err := queries.GetUpload(ctx, testGroupID)
	if err != nil {
		t.Fatal(err)
	}
	if row.StorageStatus != "ready" {
		t.Fatalf("storage_status = %q, want ready", row.StorageStatus)
	}

	// Part bookkeeping is gone, and so is the group file's own sidecar.
	for _, id := range info.PartialUploads {
		if _, err := os.Stat(filepath.Join(tempDir, id+".info")); !os.IsNotExist(err) {
			t.Errorf("part sidecar %s should have been cleaned up (err=%v)", id, err)
		}
		if _, err := queries.GetUpload(ctx, id); err == nil {
			t.Errorf("part row %s should have been deleted", id)
		}
	}
}

// recoveryFileInfo prefers the sidecar, so group metadata survives a restart.
// When the sidecar is gone it yields no PartialUploads either, so the finalizer
// still treats tempDir/<id> as the whole file — the right outcome, but by
// accident rather than by design, so pin it.
func TestRecoveryOfPositionedUploadSkipsAssemblyWithOrWithoutSidecar(t *testing.T) {
	tempDir := t.TempDir()
	queries := newTestDB(t)
	ep := NewEventProcessor(queries, t.TempDir(), tempDir, nil)
	ctx := context.Background()

	row := db.CreatePendingUploadParams{ID: testGroupID, Size: 8, Filename: "clip.mov"}
	if err := queries.CreatePendingUpload(ctx, row); err != nil {
		t.Fatal(err)
	}
	upload, err := queries.GetUpload(ctx, testGroupID)
	if err != nil {
		t.Fatal(err)
	}

	// With the sidecar: group metadata is read back and assembly is skipped.
	sidecar, err := json.Marshal(handler.FileInfo{
		ID: testGroupID, Size: 8, IsFinal: true,
		PartialUploads: []string{"part0", "part1"},
		MetaData:       handler.MetaData{MetaGroup: testGroupID, MetaTotal: "8"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, testGroupID+".info"), sidecar, 0644); err != nil {
		t.Fatal(err)
	}
	withSidecar := ep.recoveryFileInfo(upload)
	if withSidecar.MetaData[MetaGroup] != testGroupID {
		t.Errorf("group metadata did not survive recovery: %#v", withSidecar.MetaData)
	}
	if needsAssembly(withSidecar) {
		t.Error("recovery of a positioned upload must not re-run assembly")
	}

	// Without it: no PartialUploads either, so still no assembly.
	if err := os.Remove(filepath.Join(tempDir, testGroupID+".info")); err != nil {
		t.Fatal(err)
	}
	withoutSidecar := ep.recoveryFileInfo(upload)
	if needsAssembly(withoutSidecar) {
		t.Error("without a sidecar there are no partials to assemble, so assembly must be skipped")
	}
}
