package tus

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"normal.mp4", "normal.mp4", false},
		{"weird name (1).mov", "weird_name__1_.mov", false},
		{"file.tar.gz", "file_tar.gz", false},
		{"../../etc/passwd", "____._etc_passwd", false},
		{"/abs/path", "_abs_path", false},
		{"foo/bar.txt", "foo_bar.txt", false},
		{"foo\\bar.txt", "foo_bar.txt", false},
		{"a\x00b", "a_b", false},
		{"é.mov", "_.mov", false},
		{".hidden", "_hidden", false},
		{".tar.gz", "_tar.gz", false},
		{"no_extension", "no_extension", false},
		{"..", "", true},
		{".", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := SanitizeFilename(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("SanitizeFilename(%q) = %q, want error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("SanitizeFilename(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRenameUploadConcurrentSameNameNeverReplaces(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := t.TempDir()
	ep := &EventProcessor{tempDir: tempDir}

	const uploads = 48
	type result struct {
		path    string
		payload string
	}
	results := make(chan result, uploads)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < uploads; i++ {
		id := fmt.Sprintf("upload-%02d", i)
		payload := fmt.Sprintf("unique payload %02d", i)
		if err := os.WriteFile(filepath.Join(tempDir, id), []byte(payload), 0644); err != nil {
			t.Fatalf("write source %s: %v", id, err)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- result{path: ep.renameUpload(id, "same-name.bin", targetDir), payload: payload}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	seen := make(map[string]string, uploads)
	for got := range results {
		if got.path == "" {
			t.Fatal("concurrent rename failed")
		}
		if previous, duplicate := seen[got.path]; duplicate {
			t.Fatalf("destination %q was claimed twice for %q and %q", got.path, previous, got.payload)
		}
		seen[got.path] = got.payload
		contents, err := os.ReadFile(got.path)
		if err != nil {
			t.Fatalf("read destination %q: %v", got.path, err)
		}
		if string(contents) != got.payload {
			t.Fatalf("destination %q contains %q, want %q", got.path, contents, got.payload)
		}
	}
	if len(seen) != uploads {
		t.Fatalf("published %d unique files, want %d", len(seen), uploads)
	}
}

func TestCrossDeviceMoveConcurrentSameNameNeverReplaces(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	const uploads = 24
	type result struct {
		path    string
		payload string
		err     error
	}
	results := make(chan result, uploads)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < uploads; i++ {
		src := filepath.Join(sourceDir, fmt.Sprintf("upload-%02d", i))
		payload := fmt.Sprintf("cross-device payload %02d", i)
		if err := os.WriteFile(src, []byte(payload), 0644); err != nil {
			t.Fatalf("write source %q: %v", src, err)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			path, err := crossDeviceMove(src, targetDir, "same-name.bin")
			results <- result{path: path, payload: payload, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	seen := make(map[string]string, uploads)
	for got := range results {
		if got.err != nil {
			t.Fatalf("cross-device publication failed: %v", got.err)
		}
		if previous, duplicate := seen[got.path]; duplicate {
			t.Fatalf("destination %q was claimed twice for %q and %q", got.path, previous, got.payload)
		}
		seen[got.path] = got.payload
		contents, err := os.ReadFile(got.path)
		if err != nil {
			t.Fatalf("read destination %q: %v", got.path, err)
		}
		if string(contents) != got.payload {
			t.Fatalf("destination %q contains %q, want %q", got.path, contents, got.payload)
		}
	}
	if len(seen) != uploads {
		t.Fatalf("published %d unique files, want %d", len(seen), uploads)
	}
}
