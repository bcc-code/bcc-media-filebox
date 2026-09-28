// Folds Latin letters to their ASCII base so filenames keep "å", "ø", "ü" etc.
// as readable letters instead of losing them to "_".
//
// Mirrors internal/translit/translit.go. Keep the two in sync.

// Letters with no Unicode decomposition to a base letter, so stripping
// combining marks alone wouldn't reach ASCII.
const LETTER_FOLDS: Record<string, string> = {
  æ: 'ae', Æ: 'AE',
  ø: 'o', Ø: 'O',
  ß: 'ss', ẞ: 'SS',
  œ: 'oe', Œ: 'OE',
  đ: 'd', Đ: 'D',
  ð: 'd', Ð: 'D',
  þ: 'th', Þ: 'TH',
  ł: 'l', Ł: 'L',
  ı: 'i',
}

// transliterate folds letters to their ASCII base: LETTER_FOLDS first, then NFD
// decomposition with the combining marks dropped (å→a, ü→u, é→e). Characters
// with no ASCII base (CJK, emoji, punctuation) are left for the caller.
export function transliterate(s: string): string {
  const folded = Array.from(s, (ch) => LETTER_FOLDS[ch] ?? ch).join('')
  return folded.normalize('NFD').replace(/\p{Mn}/gu, '')
}
