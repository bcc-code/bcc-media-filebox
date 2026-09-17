import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import UiTagsInput from '../UiTagsInput.vue'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const field = () =>
  document.querySelector<HTMLInputElement>('.token-input > input')!
const tokens = () => [...document.querySelectorAll('.token')]
const tokenText = () =>
  tokens().map((t) => t.querySelector('span')?.textContent?.trim())
const latest = (w: VueWrapper) => {
  const e = w.emitted('update:modelValue')
  return e ? (e.at(-1)![0] as string[]) : undefined
}

function mountTags(props: Record<string, unknown> = {}) {
  wrapper = mount(UiTagsInput, {
    props: { modelValue: [], ...props },
    attachTo: document.body,
  })
  return wrapper
}

/**
 * Types into the input and commits the way a user would. The focus matters:
 * the machine ignores TYPE/Enter until it has seen FOCUS.
 */
async function type(text: string, commitKey = 'Enter') {
  const el = field()
  el.focus()
  await flush(1)
  el.value = text
  el.dispatchEvent(new Event('input', { bubbles: true }))
  await flush(1)
  el.dispatchEvent(
    new KeyboardEvent('keydown', {
      key: commitKey,
      bubbles: true,
      cancelable: true,
    }),
  )
  await flush()
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe('UiTagsInput', () => {
  it('renders the current values as tokens', async () => {
    mountTags({ modelValue: ['a@b.no', 'c@d.no'] })
    await flush()

    expect(tokenText()).toEqual(['a@b.no', 'c@d.no'])
  })

  it('adds a value on Enter', async () => {
    const w = mountTags()
    await flush()

    await type('a@b.no')

    expect(latest(w)).toEqual(['a@b.no'])
  })

  it.each([',', ';', ' '])(
    'commits the pending value when %j is typed',
    async (delim) => {
      const w = mountTags()
      await flush()

      const el = field()
      el.focus()
      await flush(1)
      // Two steps, as real typing does it: the machine stores the value on TYPE,
      // then the delimiter keystroke commits whatever it stored. Jumping straight
      // to "a@b.no," would commit an empty value.
      el.value = 'a@b.no'
      el.dispatchEvent(new Event('input', { bubbles: true }))
      await flush(1)
      el.value = `a@b.no${delim}`
      el.dispatchEvent(new Event('input', { bubbles: true }))
      await flush()

      expect(latest(w)).toEqual(['a@b.no'])
    },
  )

  it('refuses duplicates', async () => {
    const w = mountTags({ modelValue: ['a@b.no'] })
    await flush()

    await type('a@b.no')

    // Either no emit at all, or an unchanged list — never a second copy.
    expect(latest(w) ?? ['a@b.no']).toEqual(['a@b.no'])
  })

  it('removes the last value on Backspace in an empty field', async () => {
    const w = mountTags({ modelValue: ['a@b.no', 'c@d.no'] })
    await flush()

    const el = field()
    el.focus()
    await flush(1)
    el.dispatchEvent(
      new KeyboardEvent('keydown', {
        key: 'Backspace',
        bubbles: true,
        cancelable: true,
      }),
    )
    await flush()
    // Zag highlights the last tag first, then deletes on a second press.
    el.dispatchEvent(
      new KeyboardEvent('keydown', {
        key: 'Backspace',
        bubbles: true,
        cancelable: true,
      }),
    )
    await flush()

    expect(latest(w)).toEqual(['a@b.no'])
  })

  it('removes a value from its delete button', async () => {
    const w = mountTags({ modelValue: ['a@b.no', 'c@d.no'] })
    await flush()

    tokens()[0].querySelector<HTMLButtonElement>('button')!.click()
    await flush()

    expect(latest(w)).toEqual(['c@d.no'])
  })

  it('commits a half-typed value when the user interacts outside', async () => {
    const outside = document.createElement('button')
    document.body.appendChild(outside)
    const w = mountTags()
    await flush()

    const el = field()
    el.focus()
    await flush(1)
    el.value = 'a@b.no'
    el.dispatchEvent(new Event('input', { bubbles: true }))
    await flush(1)

    // There is no onBlur handler: the machine watches for interaction outside
    // its root, so a bare el.blur() proves nothing.
    outside.dispatchEvent(
      new PointerEvent('pointerdown', { bubbles: true, cancelable: true }),
    )
    outside.focus()
    await flush()

    expect(latest(w)).toEqual(['a@b.no'])
  })

  it('splits a pasted list into separate values', async () => {
    const w = mountTags()
    await flush()

    const el = field()
    el.focus()
    await flush(1)
    // jsdom's ClipboardEvent carries no clipboardData, so reproduce what the
    // connect handler does on a paste: set the value and report insertFromPaste.
    el.value = 'a@b.no, c@d.no; e@f.no'
    el.dispatchEvent(
      new InputEvent('input', { bubbles: true, inputType: 'insertFromPaste' }),
    )
    await flush()

    expect(latest(w)).toEqual(['a@b.no', 'c@d.no', 'e@f.no'])
  })

  it('marks values the caller reports invalid, without dropping them', async () => {
    const isEmail = (v: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v)
    mountTags({
      modelValue: ['good@b.no', 'oops'],
      isInvalid: (v: string) => !isEmail(v),
    })
    await flush()

    // A typo must still be visible — dropping it silently is how a recipient
    // ends up with no mail and no way to request the package.
    expect(tokenText()).toEqual(['good@b.no', 'oops'])
    expect(tokens()[0].classList.contains('token-invalid')).toBe(false)
    expect(tokens()[1].classList.contains('token-invalid')).toBe(true)
  })

  it('exposes a listbox-ish label and placeholder on the field', async () => {
    mountTags({
      placeholder: 'Add email and press Enter…',
      ariaLabel: 'Recipients',
    })
    await flush()

    expect(field().getAttribute('placeholder')).toBe(
      'Add email and press Enter…',
    )
    expect(field().getAttribute('aria-label')).toBe('Recipients')
  })

  it('does not accept input when disabled', async () => {
    const w = mountTags({ disabled: true })
    await flush()

    await type('a@b.no')

    expect(latest(w)).toBeUndefined()
  })

  it('reflects values replaced from outside', async () => {
    const w = mountTags({ modelValue: ['a@b.no'] })
    await flush()

    await w.setProps({ modelValue: ['x@y.no', 'z@w.no'] })
    await flush()

    expect(tokenText()).toEqual(['x@y.no', 'z@w.no'])
  })
})
