// Package translit folds Latin letters to their ASCII base so filenames keep
// "å", "ø", "ü" etc. as readable letters instead of losing them to "_".
//
// Mirrored in the frontend at frontend/src/transliterate.ts. Keep the two in
// sync.
package translit

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// letterFolds covers letters that have no Unicode decomposition to a base
// letter, so stripping combining marks alone wouldn't reach ASCII.
var letterFolds = strings.NewReplacer(
	"æ", "ae", "Æ", "AE",
	"ø", "o", "Ø", "O",
	"ß", "ss", "ẞ", "SS",
	"œ", "oe", "Œ", "OE",
	"đ", "d", "Đ", "D",
	"ð", "d", "Ð", "D",
	"þ", "th", "Þ", "TH",
	"ł", "l", "Ł", "L",
	"ı", "i",
)

// ASCII folds letters to their ASCII base: letterFolds first, then NFD
// decomposition with the combining marks dropped (å→a, ü→u, é→e). Runes with
// no ASCII base (CJK, emoji, punctuation) are left unchanged for the caller to
// handle.
func ASCII(s string) string {
	decomposed := norm.NFD.String(letterFolds.Replace(s))
	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
