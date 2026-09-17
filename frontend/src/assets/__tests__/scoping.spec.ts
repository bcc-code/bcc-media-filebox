import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve, join, relative } from 'node:path'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')

function vueFiles(dir = root): string[] {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) return vueFiles(full)
    return name.endsWith('.vue') ? [full] : []
  })
}

/**
 * A class handed to a child component through a prop — `panel-class="page"`,
 * `preview-class="path"`, `list-class="tab-list-inset"` — is applied to the
 * *child's* element and carries the child's scope id. The parent's
 * `<style scoped>` can never match it.
 *
 * Colocating CSS into components made this easy to get wrong: four rules moved
 * into a parent that could no longer reach them, and the admin page silently
 * lost its content padding. jsdom loads no stylesheet, so nothing else here
 * catches it.
 */
describe('scoped styles never target a class passed to a child', () => {
  const files = vueFiles()
  const offences: string[] = []

  for (const file of files) {
    const src = readFileSync(file, 'utf8')
    if (!src.includes('<template>')) continue
    const template = src.slice(src.indexOf('<template>'))
    const style = src.includes('<style') ? src.slice(src.indexOf('<style')) : ''
    if (!style) continue

    const passed = new Set<string>()
    for (const m of template.matchAll(/\b[a-z]+-class="([^"]+)"/g)) {
      for (const cls of m[1].split(/\s+/)) passed.add(cls)
    }
    for (const cls of passed) {
      const declares = new RegExp(`^\\s*\\.${cls}(?![\\w-])`, 'm')
      if (declares.test(style)) {
        offences.push(
          `${relative(root, file)} styles .${cls}, which it passes to a child`,
        )
      }
    }
  }

  it('finds at least one component that passes a class prop', () => {
    // Guards the guard: if the pattern disappears the assertion below is vacuous.
    const passing = files.filter((f) =>
      /\b[a-z]+-class="/.test(readFileSync(f, 'utf8')),
    )
    expect(passing.length).toBeGreaterThan(0)
  })

  it('has no component styling a class it hands to a child', () => {
    expect(offences).toEqual([])
  })
})
