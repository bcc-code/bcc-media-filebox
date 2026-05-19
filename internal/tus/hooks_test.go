package tus

import "testing"

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
