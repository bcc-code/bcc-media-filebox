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
// These moved out of components.css into the component that owns them, so the
// z-index invariant is now asserted against the component's own style block.
const floating: Record<string, string> = {
  menu: read('components/ui/UiMenu.vue'),
  select: read('components/ui/UiSelect.vue'),
  tooltip: read('components/ui/UiTooltip.vue'),
}
const dialog = read('components/ui/UiDialog.vue')
const drawer = read('components/ui/UiDrawer.vue')
const surfaces = {
  'admin.css': read('assets/admin.css'),
  'send.css': read('assets/send.css'),
}

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

/**
 * The page-surface stylesheets are unlayered, and an unlayered rule beats
 * @layer components at ANY specificity. So a surface-wide restyle of a bare
 * element silently overrides a shared primitive. This has happened twice:
 * `.admin-root button { color: inherit }` flattened `.btn-danger`'s red, and
 * `.send-root a { color: accent }` recoloured the whole app header — brand,
 * nav links and the active pill — on the Send page only.
 */
describe.each(Object.entries(surfaces))(
  '%s restyles no bare element',
  (_name, css) => {
    // Anything components.css owns. Two shapes are deliberately allowed: the
    // `*` box-sizing reset, which sets no property a primitive competes for,
    // and a qualified `element.class` such as `select.inp`, which refines a
    // primitive on elements already carrying it instead of overriding every
    // element of that type.
    it.each([
      'a',
      'button',
      'input',
      'select',
      'textarea',
      'table',
      'th',
      'td',
    ])('declares no `.<surface> %s` rule', (element) => {
      const bare = new RegExp(
        `^\\.[a-z-]+-root(\\s+[.:#\\[][^\\s{,]*)*\\s+${element}(?![-\\w.])`,
        'm',
      )
      const hit = css.match(bare)
      expect(
        hit,
        `${hit?.[0]} beats @layer components; give it a class instead`,
      ).toBeNull()
    })
  },
)

describe('layering', () => {
  it('defines the whole scale as tokens', () => {
    expect(token('drawer-backdrop')).toBeGreaterThan(0)
    expect(token('drawer')).toBeGreaterThan(0)
    expect(token('dialog-backdrop')).toBeGreaterThan(0)
    expect(token('dialog')).toBeGreaterThan(0)
    expect(token('dropdown')).toBeGreaterThan(0)
  })

  it('stacks dialog above its backdrop', () => {
    expect(token('dialog')).toBeGreaterThan(token('dialog-backdrop'))
  })

  it('stacks a drawer above its backdrop', () => {
    expect(token('drawer')).toBeGreaterThan(token('drawer-backdrop'))
  })

  it('stacks dialogs above drawers, so a modal opened from one lands on top', () => {
    // No flow stacks them today — Admin.vue clears selectedUser before opening
    // GrantModal — but a drawer is page furniture and a dialog interrupts, so
    // the order is fixed here rather than left to whoever writes the next one.
    expect(token('dialog-backdrop')).toBeGreaterThan(token('drawer'))
  })

  it('stacks dropdowns above dialogs, so a select works inside one', () => {
    expect(token('dropdown')).toBeGreaterThan(token('dialog'))
  })

  it.each(['menu', 'select', 'tooltip'])(
    'declares the z-index on .%s-content, where popper reads it',
    (part) => {
      expect(rule(floating[part], `${part}-content`)).toContain(
        'z-index: var(--z-index-dropdown)',
      )
    },
  )

  it.each(['menu', 'select', 'tooltip'])(
    'declares no z-index on .%s-positioner, where it would be overridden inline',
    (part) => {
      // Must not match `z-index:` while still allowing `--z-index:`.
      expect(rule(floating[part], `${part}-positioner`)).not.toMatch(
        /(^|[^-])z-index:/,
      )
    },
  )

  it.each(['menu', 'select', 'tooltip'])(
    'keeps .%s-* out of components.css now that it is colocated',
    (part) => {
      // Two definitions of a floating surface's z-index is how the original
      // bug got in. One home only.
      expect(components).not.toContain(`.${part}-content`)
      expect(components).not.toContain(`.${part}-positioner`)
    },
  )

  it('wires the drawer layers to the tokens rather than bare numbers', () => {
    expect(drawer).toContain('z-index: var(--z-index-drawer-backdrop)')
    expect(drawer).toContain('z-index: var(--z-index-drawer)')
    // The sticky head's `z-index: 2` is local to the panel's stacking context,
    // so it is the one literal allowed here.
    expect(drawer.match(/z-index:\s*\d+/g)).toEqual(['z-index: 2'])
  })

  it('wires the dialog layers to the tokens rather than bare numbers', () => {
    expect(dialog).toContain('z-index: var(--z-index-dialog-backdrop)')
    expect(dialog).toContain('z-index: var(--z-index-dialog)')
    // A literal z-index here would drift from the scale.
    expect(dialog).not.toMatch(/z-index:\s*\d+/)
  })
})
