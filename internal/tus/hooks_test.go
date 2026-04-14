package tus

import "testing"

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"normal.mp4", "normal.mp4", false},
		{"weird name (1).mov", "weird name (1).mov", false},
		{"../../etc/passwd", "", true},
		{"/abs/path", "", true},
		{"foo/bar.txt", "", true},
		{"foo\\bar.txt", "", true},
		{"..", "", true},
		{".", "", true},
		{"", "", true},
		{"a\x00b", "", true},
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
