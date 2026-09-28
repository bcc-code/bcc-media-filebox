package translit

import "testing"

func TestASCII(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Påske øvelse", "Paske ovelse"},
		{"Ærlig", "AErlig"},
		{"Größe", "Grosse"},
		{"Hyvää yötä", "Hyvaa yota"},
		{"é", "e"},
		{"日本", "日本"},
	}
	for _, c := range cases {
		if got := ASCII(c.in); got != c.want {
			t.Errorf("ASCII(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
