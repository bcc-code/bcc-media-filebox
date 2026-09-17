import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import UiFileUpload from '../UiFileUpload.vue'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const zone = () => document.querySelector<HTMLElement>('.dropzone')!
const hidden = () =>
  document.querySelector<HTMLInputElement>('input[type=file]')!
const file = (name: string) => new File(['x'], name, { type: 'text/plain' })

function mountUpload(props: Record<string, unknown> = {}) {
  wrapper = mount(UiFileUpload, { props, attachTo: document.body })
  return wrapper
}

/**
 * Drops files the way the browser does — dragover first. The machine only
 * honours a drop once it has entered the dragging state, so a bare `drop`
 * event silently does nothing.
 */
async function drop(...files: File[]) {
  const transfer = () => {
    const dt = new DataTransfer()
    for (const f of files) dt.items.add(f)
    return dt
  }
  const fire = (type: string) =>
    zone().dispatchEvent(
      new DragEvent(type, {
        dataTransfer: transfer(),
        bubbles: true,
        cancelable: true,
      }),
    )
  fire('dragover')
  await flush(1)
  fire('drop')
  await flush()
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe('UiFileUpload', () => {
  it('is keyboard reachable, unlike the plain div it replaces', async () => {
    mountUpload()
    await flush()

    expect(zone().getAttribute('role')).toBe('button')
    expect(zone().getAttribute('tabindex')).toBe('0')
    expect(zone().getAttribute('aria-label')).toBeTruthy()
  })

  it('renders the label and hint', async () => {
    mountUpload({ label: 'Drop it', hint: 'up to 300 GB' })
    await flush()

    expect(zone().textContent).toContain('Drop it')
    expect(zone().textContent).toContain('up to 300 GB')
  })

  it('emits dropped files', async () => {
    const w = mountUpload()
    await flush()

    await drop(file('a.mov'), file('b.mov'))

    const emitted = w.emitted('files')?.[0]?.[0] as File[]
    expect(emitted.map((f) => f.name)).toEqual(['a.mov', 'b.mov'])
  })

  it('treats maxFiles 0 as unlimited', async () => {
    const w = mountUpload({ maxFiles: 0 })
    await flush()

    await drop(file('a'), file('b'), file('c'), file('d'))

    expect((w.emitted('files')?.[0]?.[0] as File[]).length).toBe(4)
  })

  it('refuses, and explains, a multi-file drop on a single-file target', async () => {
    const w = mountUpload({ maxFiles: 1 })
    await flush()

    await drop(file('first.mov'), file('second.mov'))

    // Zag refuses the whole drop rather than truncating it. The old component
    // silently kept only the first file, leaving the rest unaccounted for.
    expect(w.emitted('files')).toBeFalsy()
    expect(w.emitted('reject')?.[0]?.[0]).toContain('single file')
  })

  it('accepts a single file on a single-file target', async () => {
    const w = mountUpload({ maxFiles: 1 })
    await flush()

    await drop(file('only.mov'))

    expect((w.emitted('files')?.[0]?.[0] as File[]).map((f) => f.name)).toEqual(
      ['only.mov'],
    )
    expect(w.emitted('reject')).toBeFalsy()
  })

  it('accepts nothing while disabled', async () => {
    const w = mountUpload({ disabled: true })
    await flush()

    expect(zone().getAttribute('aria-disabled')).toBe('true')
    await drop(file('a.mov'))

    expect(w.emitted('files')).toBeFalsy()
  })

  it('clears its own list so repeat drops of the same name still emit', async () => {
    const w = mountUpload()
    await flush()

    await drop(file('same.mov'))
    await drop(file('same.mov'))

    expect(w.emitted('files')?.length).toBe(2)
  })

  it('keeps a hidden native input for the click-to-browse path', async () => {
    mountUpload()
    await flush()

    expect(hidden()).not.toBeNull()
    expect(hidden().type).toBe('file')
  })
})
