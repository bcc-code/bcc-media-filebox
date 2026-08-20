package objectstore

import "testing"

func TestArchiveMultipartPartSizeSupportsHundredGiBStream(t *testing.T) {
	const archiveLimit = int64(100 << 30)
	parts := (archiveLimit + archiveMultipartPartSize - 1) / archiveMultipartPartSize
	if parts > 10_000 {
		t.Fatalf("100 GiB needs %d parts at %d bytes, exceeding S3's limit", parts, archiveMultipartPartSize)
	}
	if archiveMultipartPartSize < 5<<20 {
		t.Fatalf("part size %d is below S3's 5 MiB multipart minimum", archiveMultipartPartSize)
	}
}
