import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import UiTooltip from '../UiTooltip.vue'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const trigger = () => document.querySelector<HTMLElement>('.tooltip-trigger')
const content = () => document.querySelector('.tooltip-content')
const button = () => document.querySelector<HTMLButtonElement>('button')!

function mountTip(props: Record<string, unknown> = {}) {
  wrapper = mount(UiTooltip, {
    props,
    slots: { default: '<button disabled>Webhook</button>' },
    attachTo: document.body,
  })
  return wrapper
}

/** openDelay defaults to 400ms, so hovering needs a real wait. */
async function hover(el: Element) {
  el.dispatchEvent(
    new PointerEvent('pointerenter', { bubbles: false, pointerType: 'mouse' }),
  )
  el.dispatchEvent(
    new PointerEvent('pointermove', { bubbles: true, pointerType: 'mouse' }),
  )
  await new Promise((r) => setTimeout(r, 600))
  await flush()
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe('UiTooltip', () => {
  it('wraps the trigger without replacing it', async () => {
    mountTip({ label: 'Re-send the webhook' })
    await flush()

    expect(trigger()).not.toBeNull()
    expect(trigger()!.querySelector('button')?.textContent).toBe('Webhook')
  })

  it('stays out of the way when there is no label', async () => {
    mountTip({ label: '' })
    await flush()

    // A blank disabledReason must not leave an empty tooltip attached.
    expect(trigger()).toBeNull()
    expect(button().textContent).toBe('Webhook')
  })

  it('treats a whitespace-only label as no label', async () => {
    mountTip({ label: '   ' })
    await flush()

    expect(trigger()).toBeNull()
  })

  it('shows nothing until hovered', async () => {
    mountTip({ label: 'Re-send the webhook' })
    await flush()

    expect(content()).toBeNull()
  })

  it('opens on hover and teleports to body', async () => {
    mountTip({ label: 'Re-send the webhook' })
    await flush()

    await hover(trigger()!)

    expect(content()?.textContent?.trim()).toBe('Re-send the webhook')
    expect(content()!.closest('.tooltip-positioner')!.parentElement).toBe(
      document.body,
    )
  })

  it('works over a disabled control, which is where the reason matters most', async () => {
    mountTip({ label: 'No webhook configured on this target' })
    await flush()

    expect(button().disabled).toBe(true)
    // Pointer events land on the wrapping span, not the inert button.
    await hover(trigger()!)

    expect(content()?.textContent?.trim()).toBe(
      'No webhook configured on this target',
    )
  })

  it('describes the trigger for assistive tech, which a native title does not', async () => {
    mountTip({ label: 'Re-send the webhook' })
    await flush()
    await hover(trigger()!)

    const describedBy = trigger()!.getAttribute('aria-describedby')
    expect(describedBy).toBeTruthy()
    expect(document.getElementById(describedBy!)).not.toBeNull()
  })

  it('closes on Escape', async () => {
    mountTip({ label: 'Re-send the webhook' })
    await flush()
    await hover(trigger()!)
    expect(content()).not.toBeNull()

    document.dispatchEvent(
      new KeyboardEvent('keydown', {
        key: 'Escape',
        bubbles: true,
        cancelable: true,
      }),
    )
    await flush()

    expect(content()).toBeNull()
  })

  it('updates the label reactively', async () => {
    const w = mountTip({ label: 'First' })
    await flush()
    await hover(trigger()!)
    expect(content()?.textContent?.trim()).toBe('First')

    await w.setProps({ label: 'Second' })
    await flush()

    expect(content()?.textContent?.trim()).toBe('Second')
  })
})
