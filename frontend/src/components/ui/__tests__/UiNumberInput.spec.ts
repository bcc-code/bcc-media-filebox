import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import UiNumberInput from '../UiNumberInput.vue'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const field = () => document.querySelector<HTMLInputElement>('.num-input')!
const steppers = () => [
  ...document.querySelectorAll<HTMLButtonElement>('.num-step'),
]
const inc = () => steppers()[0]
const dec = () => steppers()[1]
const latest = (w: VueWrapper) => {
  const e = w.emitted('update:modelValue')
  return e ? (e.at(-1)![0] as number | '') : undefined
}

function mountNum(props: Record<string, unknown> = {}) {
  wrapper = mount(UiNumberInput, {
    props: { modelValue: 5, ...props },
    attachTo: document.body,
  })
  return wrapper
}

/**
 * Steppers are press-and-hold (pointerdown starts a repeat), not click — so a
 * plain .click() does nothing.
 */
async function press(button: HTMLButtonElement) {
  button.dispatchEvent(
    new PointerEvent('pointerdown', {
      bubbles: true,
      cancelable: true,
      isPrimary: true,
    }),
  )
  await flush(1)
  button.dispatchEvent(
    new PointerEvent('pointerup', {
      bubbles: true,
      cancelable: true,
      isPrimary: true,
    }),
  )
  await flush()
}

async function type(value: string) {
  const el = field()
  el.focus()
  await flush(1)
  el.value = value
  el.dispatchEvent(new Event('input', { bubbles: true }))
  await flush()
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe('UiNumberInput', () => {
  it('shows the current value', async () => {
    mountNum({ modelValue: 7 })
    await flush()

    expect(field().value).toBe('7')
  })

  it('renders an empty field for the unset value', async () => {
    mountNum({ modelValue: '' })
    await flush()

    expect(field().value).toBe('')
  })

  it('exposes spinbutton semantics with its range', async () => {
    mountNum({ modelValue: 5, min: 1, max: 30, ariaLabel: 'More days' })
    await flush()

    expect(field().getAttribute('role')).toBe('spinbutton')
    expect(field().getAttribute('aria-valuemin')).toBe('1')
    expect(field().getAttribute('aria-valuemax')).toBe('30')
    expect(field().getAttribute('aria-label')).toBe('More days')
  })

  it('offers steppers, which a raw number input on macOS does not', async () => {
    mountNum()
    await flush()

    expect(steppers()).toHaveLength(2)
  })

  it('increments and decrements', async () => {
    const w = mountNum({ modelValue: 5 })
    await flush()

    await press(inc())
    expect(latest(w)).toBe(6)

    await w.setProps({ modelValue: 6 })
    await flush()
    await press(dec())
    expect(latest(w)).toBe(5)
  })

  it('increments with the arrow keys', async () => {
    const w = mountNum({ modelValue: 5 })
    await flush()

    field().focus()
    await flush(1)
    field().dispatchEvent(
      new KeyboardEvent('keydown', {
        key: 'ArrowUp',
        bubbles: true,
        cancelable: true,
      }),
    )
    await flush()

    expect(latest(w)).toBe(6)
  })

  it('disables the increment stepper at max', async () => {
    mountNum({ modelValue: 30, min: 1, max: 30 })
    await flush()

    expect(inc().hasAttribute('data-disabled')).toBe(true)
    expect(dec().hasAttribute('data-disabled')).toBe(false)
  })

  it('disables the decrement stepper at min', async () => {
    mountNum({ modelValue: 1, min: 1, max: 30 })
    await flush()

    expect(dec().hasAttribute('data-disabled')).toBe(true)
  })

  it('emits a number, not a string', async () => {
    const w = mountNum({ modelValue: 5 })
    await flush()

    await type('12')

    expect(latest(w)).toBe(12)
    expect(typeof latest(w)).toBe('number')
  })

  it('emits the unset value when cleared, never NaN', async () => {
    const w = mountNum({ modelValue: 5 })
    await flush()

    await type('')

    // Two call sites read '' as "unlimited"; NaN would break them.
    expect(latest(w)).toBe('')
  })

  it('emits the unset value for unparseable input', async () => {
    const w = mountNum({ modelValue: 5 })
    await flush()

    await type('abc')

    expect(Number.isNaN(latest(w))).toBe(false)
  })

  it('shows the placeholder when unset', async () => {
    mountNum({ modelValue: '', placeholder: 'Unlimited' })
    await flush()

    expect(field().getAttribute('placeholder')).toBe('Unlimited')
  })

  it('does not change when disabled', async () => {
    const w = mountNum({ modelValue: 5, disabled: true })
    await flush()

    await press(inc())

    expect(w.emitted('update:modelValue')).toBeFalsy()
  })

  it('leaves the value alone on mouse wheel', async () => {
    const w = mountNum({ modelValue: 5 })
    await flush()

    field().focus()
    await flush(1)
    field().dispatchEvent(
      new WheelEvent('wheel', { deltaY: -100, bubbles: true }),
    )
    await flush()

    // Scrolling a page must never silently edit a field.
    expect(w.emitted('update:modelValue')).toBeFalsy()
  })
})
