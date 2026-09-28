import { describe, it, expect } from 'vitest'
import { sanitizeFilename } from '../useTusUpload'

// Mirrors TestSanitizeFilename in internal/tus/hooks_test.go — the two
// implementations must agree.
describe('sanitizeFilename', () => {
  it.each([
    ['normal.mp4', 'normal.mp4'],
    ['weird name (1).mov', 'weird_name__1_.mov'],
    ['file.tar.gz', 'file_tar.gz'],
    ['../../etc/passwd', '____._etc_passwd'],
    ['/abs/path', '_abs_path'],
    ['foo/bar.txt', 'foo_bar.txt'],
    ['foo\\bar.txt', 'foo_bar.txt'],
    ['a\x00b', 'a_b'],
    ['é.mov', 'e.mov'],
    ['é.mov', 'e.mov'],
    ['Påske øvelse.mp4', 'Paske_ovelse.mp4'],
    ['Ærlig.mov', 'AErlig.mov'],
    ['Größe.txt', 'Grosse.txt'],
    ['Hyvää yötä.wav', 'Hyvaa_yota.wav'],
    ['日本.mp4', '__.mp4'],
    ['.hidden', '_hidden'],
    ['.tar.gz', '_tar.gz'],
    ['no_extension', 'no_extension'],
  ])('%j → %j', (input, want) => {
    expect(sanitizeFilename(input)).toEqual({ name: want, error: null })
  })

  it.each(['', '.', '..'])('rejects %j', (input) => {
    expect(sanitizeFilename(input).error).not.toBeNull()
  })
})
