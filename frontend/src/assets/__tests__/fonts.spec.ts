import { describe, it, expect } from 'vitest'
// Read from disk rather than importing: Vite's Tailwind plugin owns .css, so a
// `?raw` import of these returns an empty string.
import { readFileSync, existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const read = (rel: string) => readFileSync(resolve(root, rel), 'utf8')

const sheets = {
  sans: read('assets/styles/font-sans.css'),
  mono: read('assets/styles/font-mono.css'),
}
const theme = read('style.css')

/**
 * Self-hosted fonts fail *silently*: a missing subset just means the browser
 * skips the face, and a woff2 under src/assets rather than public/ is answered
 * by Vite's SPA fallback with index.html and a 200. FileBox shipped for a while
 * rendering entirely in system-ui because only the `latin-ext` subset had been
 * downloaded — no error anywhere. These assert the two things that were wrong.
 */

/** Every codepoint covered by any `unicode-range` in a sheet. */
const covers = (css: string, codepoint: number) =>
  [...css.matchAll(/unicode-range:\s*([^;]+);/g)].some((m) =>
    m[1].split(',').some((part) => {
      const range = part.trim().replace(/^U\+/i, '')
      const [lo, hi = lo] = range.split('-')
      return codepoint >= parseInt(lo, 16) && codepoint <= parseInt(hi, 16)
    }),
  )

describe.each(Object.entries(sheets))('font-%s.css', (_kind, css) => {
  it.each([
    ['A', 0x41],
    ['a', 0x61],
    ['0', 0x30],
    ['space', 0x20],
    // Norwegian: the app's own language, and all three sit in Latin-1.
    ['æ', 0xe6],
    ['ø', 0xf8],
    ['å', 0xe5],
    // latin-ext, so a `latin`-only download does not silently drop it either.
    ['Ā', 0x100],
  ])('covers %s with some unicode-range', (_label, cp) => {
    expect(covers(css, cp)).toBe(true)
  })

  it('references only woff2 files that exist in public/fonts', () => {
    const urls = [...css.matchAll(/url\(['"]([^'"]+)['"]\)/g)].map((m) => m[1])
    expect(urls.length).toBeGreaterThan(0)
    for (const url of urls) {
      // An absolute /fonts/… URL resolves to public/fonts at dev and build time.
      expect(
        url.startsWith('/fonts/'),
        `${url} must be served from /fonts/`,
      ).toBe(true)
      const onDisk = resolve(root, '../public', url.replace(/^\//, ''))
      expect(existsSync(onDisk), `${url} is not in public/fonts`).toBe(true)
    }
  })

  it('declares no :root block, which would outrank the @theme token', () => {
    // The generator emits `:root { --font-sans: … }`. Unlayered, it beats
    // @theme's output and leaves the token defined twice.
    expect(css).not.toMatch(/^:root/m)
  })
})

describe('@theme font tokens', () => {
  it.each([
    ['--font-sans', 'Archivo'],
    ['--font-mono', "'JetBrains Mono'"],
  ])('names the %s family the @font-face rules define', (token, family) => {
    const m = theme.match(new RegExp(`${token}:\\s*([^;]+);`))
    expect(m, `${token} missing from @theme`).not.toBeNull()
    expect(m![1]).toContain(family.replace(/'/g, ''))
  })

  it('keeps the mono metric-matched fallback in the stack', () => {
    // Without it the line box shifts when the webfont swaps in. The sans
    // family has no equivalent: those faces are generator output and none were
    // produced for Archivo, so its stack falls back to system-ui plainly.
    expect(theme).toContain('JetBrains Mono fallback: Courier New')
  })

  it('declares a variable weight range for the sans faces', () => {
    // Archivo ships as one variable file per subset; without the range the
    // browser synthesises bold instead of using the weight axis.
    const faces = sheets.sans.match(/@font-face \{[^}]*\}/g) ?? []
    expect(faces.length).toBeGreaterThan(0)
    for (const face of faces) {
      expect(face).toMatch(/font-weight:\s*100 900/)
      expect(face).toContain('woff2-variations')
    }
  })
})
