import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import UiEditable from '../UiEditable.vue'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const preview = () =>
  document.querySelector<HTMLElement>('[data-part="preview"]')!
const input = () =>
  document.querySelector<HTMLInputElement>('[data-part="input"]')!
const editing = () => !input().hasAttribute('hidden')

function mountEditable(props: Record<string, unknown> = {}) {
  wrapper = mount(UiEditable, {
    props: { value: 'Isilon', ...props },
    attachTo: document.body,
  })
  return wrapper
}

async function startEdit() {
  preview().click()
  await flush()
}

async function type(value: string) {
  const el = input()
  el.value = value
  el.dispatchEvent(new Event('input', { bubbles: true }))
  await flush(1)
}

async function key(k: string) {
  input().dispatchEvent(
    new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }),
  )
  await flush()
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe('UiEditable', () => {
  it('shows the value as text until clicked', async () => {
    mountEditable()
    await flush()

    expect(preview().textContent?.trim()).toBe('Isilon')
    expect(editing()).toBe(false)
  })

  it('opens the editor on click', async () => {
    mountEditable()
    await flush()

    await startEdit()

    expect(editing()).toBe(true)
    expect(input().value).toBe('Isilon')
  })

  it('commits on Enter', async () => {
    const w = mountEditable()
    await flush()
    await startEdit()

    await type('Archive')
    await key('Enter')

    expect(w.emitted('commit')?.at(-1)).toEqual(['Archive'])
  })

  it('cancels on Escape without committing', async () => {
    const w = mountEditable()
    await flush()
    await startEdit()

    await type('Archive')
    await key('Escape')

    // The hand-rolled version also did this; it is the reason for a real
    // machine rather than a bare input.
    expect(w.emitted('commit')).toBeFalsy()
    expect(editing()).toBe(false)
  })

  it('does not commit an unchanged value', async () => {
    const w = mountEditable()
    await flush()
    await startEdit()

    await key('Enter')

    expect(w.emitted('commit')).toBeFalsy()
  })

  it('treats an emptied field as no change', async () => {
    const w = mountEditable()
    await flush()
    await startEdit()

    await type('   ')
    await key('Enter')

    expect(w.emitted('commit')).toBeFalsy()
  })

  it('trims what it commits', async () => {
    const w = mountEditable()
    await flush()
    await startEdit()

    await type('  Archive  ')
    await key('Enter')

    expect(w.emitted('commit')?.at(-1)).toEqual(['Archive'])
  })

  it('reports when editing opens and closes', async () => {
    const w = mountEditable()
    await flush()

    await startEdit()
    expect(w.emitted('editChange')?.at(-1)).toEqual([true])

    await key('Escape')
    expect(w.emitted('editChange')?.at(-1)).toEqual([false])
  })

  it('applies caller classes to the preview and the input', async () => {
    mountEditable({ previewClass: 'primary', inputClass: 'mono' })
    await flush()

    expect(preview().classList.contains('primary')).toBe(true)
    expect(input().classList.contains('mono')).toBe(true)
    expect(input().classList.contains('inline-edit')).toBe(true)
  })

  it('does not open when disabled', async () => {
    mountEditable({ disabled: true })
    await flush()

    await startEdit()

    expect(editing()).toBe(false)
  })

  it('follows a value replaced from outside', async () => {
    const w = mountEditable()
    await flush()

    await w.setProps({ value: 'Renamed' })
    await flush()

    expect(preview().textContent?.trim()).toBe('Renamed')
  })
})
