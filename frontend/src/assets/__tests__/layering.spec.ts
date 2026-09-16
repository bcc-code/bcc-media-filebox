import { describe, it, expect } from 'vitest'
// Read from disk rather than importing: Vite's Tailwind plugin owns .css, so a
// `?raw` import of these returns an empty string.
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const read = (rel: string) => readFileSync(resolve(root, rel), 'utf8')

const theme = read('style.css')
const components = read('assets/components.css')
const dialog = read('components/ui/UiDialog.vue')

/**
 * These assert against the CSS source rather than a rendered page, because the
 * component tests run in jsdom with no stylesheet loaded. The invariant is easy
 * to break silently: Zag's popper writes `z-index: var(--z-index)` inline on a
 * positioner and fills that var from the *content* element's computed z-index,
 * so a z-index declared on a positioner is dead code — which is exactly how a
 * select rendered behind the dialog it was opened from.
 */

const token = (name: string) => {
  const m = theme.match(new RegExp(`--z-index-${name}:\\s*(\\d+)`))
  expect(
    m,
    `--z-index-${name} is not defined in the @theme block`,
  ).not.toBeNull()
  return Number(m![1])
}

/** The rule body for a single-class selector. */
const rule = (css: string, selector: string) => {
  const m = css.match(new RegExp(`\\.${selector}\\s*\\{([^}]*)\\}`))
  expect(m, `.${selector} rule not found`).not.toBeNull()
  return m![1]
}

describe('layering', () => {
  it('defines the whole scale as tokens', () => {
    expect(token('dialog-backdrop')).toBeGreaterThan(0)
    expect(token('dialog')).toBeGreaterThan(0)
    expect(token('dropdown')).toBeGreaterThan(0)
  })

  it('stacks dialog above its backdrop', () => {
    expect(token('dialog')).toBeGreaterThan(token('dialog-backdrop'))
  })

  it('stacks dropdowns above dialogs, so a select works inside one', () => {
    expect(token('dropdown')).toBeGreaterThan(token('dialog'))
  })

  it.each(['menu-content', 'select-content'])(
    'declares the z-index on .%s, where popper reads it',
    (selector) => {
      expect(rule(components, selector)).toContain(
        'z-index: var(--z-index-dropdown)',
      )
    },
  )

  it.each(['menu-positioner', 'select-positioner'])(
    'declares no z-index on .%s, where it would be overridden inline',
    (selector) => {
      // Must not match `z-index:` while still allowing `--z-index:`.
      expect(rule(components, selector)).not.toMatch(/(^|[^-])z-index:/)
    },
  )

  it('wires the dialog layers to the tokens rather than bare numbers', () => {
    expect(dialog).toContain('z-index: var(--z-index-dialog-backdrop)')
    expect(dialog).toContain('z-index: var(--z-index-dialog)')
    // A literal z-index here would drift from the scale.
    expect(dialog).not.toMatch(/z-index:\s*\d+/)
  })
})
