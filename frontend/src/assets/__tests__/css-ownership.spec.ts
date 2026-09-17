import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve, join, relative } from 'node:path'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const read = (rel: string) => readFileSync(resolve(root, rel), 'utf8')

function vueFiles(dir = root): string[] {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) return vueFiles(full)
    return name.endsWith('.vue') ? [full] : []
  })
}

// Page-surface wrappers are not context in this sense: every rule on the page
// carries one, and no component style mentions them.
const SURFACE_CLASSES = new Set(['admin-root', 'send-root', 'upload-root'])

const SHEETS = [
  'assets/components.css',
  'assets/send.css',
  'assets/admin.css',
] as const

/** Selectors declared in a stylesheet or a <style> block. */
const selectors = (css: string) =>
  [...css.matchAll(/^[ \t]*([.#][^{}\n]*?)[ \t]*\{/gm)].map((m) =>
    m[1].split(/\s+/).join(' '),
  )

/** The compound the rule actually targets — the last part of the selector. */
const subject = (sel: string) => sel.split(/[\s>+~]+/).pop()!

/**
 * Scoped CSS adds a [data-v-x] attribute to a rule's subject, which RAISES its
 * specificity. So splitting one class family across a component's scoped block
 * and a global sheet inverts the intended cascade: `.btn` scoped (0,2,0) beat
 * the global `.btn-sm` and `.btn-primary` (0,1,0), and every button silently
 * lost its size and variant.
 *
 * A family therefore has to live in exactly one place.
 */
describe('css ownership', () => {
  const globals = new Map(SHEETS.map((f) => [f, selectors(read(f))]))

  /** Class/attribute/pseudo-class count, then element count. */
  const spec = (sel: string): [number, number] => [
    (sel.match(/\.[a-zA-Z][\w-]*/g) ?? []).length +
      (sel.match(/\[[^\]]+\]/g) ?? []).length +
      (sel.match(/:(?!:)[a-z-]+/g) ?? []).length,
    (sel.match(/(?:^|[\s>+~])[a-z]+(?![\w-])/g) ?? []).length,
  ]
  const lte = (a: [number, number], b: [number, number]) =>
    a[0] < b[0] || (a[0] === b[0] && a[1] <= b[1])
  /** The parts of one compound: element name, classes, attributes, pseudos. */
  const tokens = (compound: string) =>
    new Set(compound.match(/^[a-z]+|\.[\w-]+|\[[^\]]+\]|:(?!:)[a-z-]+/g) ?? [])
  /** First class in the selector — the family it belongs to. */
  const rootClass = (sel: string) => sel.match(/\.([a-zA-Z][\w-]*)/)?.[1] ?? ''

  it('never leaves a global variant that its own scoped base outranks', () => {
    // The shape that bit twice: a global rule written as a MORE specific
    // variant of a class the component also styles, which after scoping ties
    // with or loses to the base. `.app-nav a.active` is (0,2,1) and the scoped
    // `.app-nav a` becomes (0,2,1) too, so bundle order decided whether the
    // active nav item got its colour and underline.
    //
    // A global rule with the same or fewer classes is not this bug — that is an
    // ordinary base rule the component deliberately overrides.
    const offences: string[] = []
    for (const file of vueFiles()) {
      const src = readFileSync(file, 'utf8')
      if (!src.includes('<style')) continue
      for (const mine of selectors(src.slice(src.indexOf('<style')))) {
        const base = rootClass(mine)
        if (!base) continue
        const mineSpec = spec(mine)
        const scopedSpec: [number, number] = [mineSpec[0] + 1, mineSpec[1]]
        const myTokens = tokens(subject(mine))
        for (const [sheet, sels] of globals) {
          for (const sel of sels) {
            const stripped = sel
              .split(/[\s>+~]+/)
              .filter((part) => !SURFACE_CLASSES.has(part.replace(/^\./, '')))
              .join(' ')
            if (rootClass(stripped) !== base) continue
            // Same element, plus modifiers: the global rule's subject compound
            // must strictly contain the scoped rule's. That excludes rules
            // targeting a different descendant (`.field .hint` vs
            // `.field > label`), which never compete.
            const theirs = tokens(subject(stripped))
            const isVariant =
              theirs.size > myTokens.size &&
              [...myTokens].every((t) => theirs.has(t))
            if (isVariant && lte(spec(stripped), scopedSpec)) {
              offences.push(
                `${relative(root, file)} "${mine}" outranks ${sheet} "${sel}"`,
              )
            }
          }
        }
      }
    }
    expect([...new Set(offences)]).toEqual([])
  })

  it('keeps a dynamically-built class family with the component that builds it', () => {
    // UiButton composes `btn-${variant}` / `btn-${size}`, so the modifier names
    // never appear as literals — which is exactly why they were left behind.
    const offences: string[] = []
    for (const file of vueFiles()) {
      const src = readFileSync(file, 'utf8')
      if (!src.includes('<style')) continue
      const prefixes = [...src.matchAll(/`([a-zA-Z][\w-]*)-\$\{/g)].map(
        (m) => m[1],
      )
      for (const prefix of prefixes) {
        for (const [sheet, sels] of globals) {
          const hit = sels.find((s) =>
            new RegExp(`\\.${prefix}-[\\w-]+`).test(subject(s)),
          )
          if (hit) {
            offences.push(
              `${relative(root, file)} builds .${prefix}-* but ${sheet} styles "${hit}"`,
            )
          }
        }
      }
    }
    expect(offences).toEqual([])
  })
})
