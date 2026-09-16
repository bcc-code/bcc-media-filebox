import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'
import UiDialog from '../UiDialog.vue'

/**
 * UiDialog is a thin wrapper over the Zag.js dialog machine, so these tests
 * cover the contract we depend on rather than re-testing Zag: that the panel is
 * teleported, that the accessible name and description resolve to the right
 * text, that each dismissal route emits `close`, and that `alertdialog` opts out
 * of casual dismissal.
 *
 * Not covered here, because jsdom has no layout engine: real Tab focus order
 * inside the trap, and whether scroll lock visually holds the page. Those need
 * a browser.
 */

let wrapper: VueWrapper | null = null

/**
 * Zag defers work to requestAnimationFrame — `checkRenderedElements` corrects
 * aria-labelledby/describedby a frame after mount, and the dismissable layer
 * arms its listeners on a delay — so tests must flush frames, not just ticks.
 */
async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

/** Zag teleports to <body> and dismissal listeners live on document, so query globally. */
const panel = () => document.querySelector('.dialog-panel')
const textOf = (id: string | null) =>
  id ? document.getElementById(id)?.textContent?.trim() : null

function mountDialog(
  props: Record<string, unknown> = {},
  slots: Record<string, unknown> = {},
) {
  wrapper = mount(UiDialog, {
    props,
    slots: {
      default:
        '<input class="first-field" /><button class="body-btn">Body</button>',
      ...slots,
    },
    attachTo: document.body,
  })
  return wrapper
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

/** Zag's dismissable layer listens for pointerdown on document. */
async function pointerDownOn(target: EventTarget) {
  target.dispatchEvent(
    new PointerEvent('pointerdown', { bubbles: true, cancelable: true }),
  )
  target.dispatchEvent(
    new PointerEvent('pointerup', { bubbles: true, cancelable: true }),
  )
  target.dispatchEvent(
    new MouseEvent('click', { bubbles: true, cancelable: true }),
  )
  await flush()
}

async function pressEscape() {
  document.dispatchEvent(
    new KeyboardEvent('keydown', {
      key: 'Escape',
      bubbles: true,
      cancelable: true,
    }),
  )
  await flush()
}

describe('UiDialog', () => {
  it('is open on mount and teleports the panel to <body>', async () => {
    mountDialog({ title: 'New upload target' })
    await flush()

    const el = panel()
    expect(el).not.toBeNull()
    // The panel must escape any transformed ancestor, which would otherwise
    // become the containing block for its position:fixed overlay.
    expect(el?.closest('.dialog-positioner')?.parentElement).toBe(document.body)
    expect(el?.closest('.admin-root')).toBeNull()
  })

  it('exposes the title as the accessible name', async () => {
    mountDialog({ title: 'New upload target' })
    await flush()

    const el = panel()!
    expect(el.getAttribute('role')).toBe('dialog')
    expect(el.getAttribute('aria-modal')).toBe('true')
    expect(textOf(el.getAttribute('aria-labelledby'))).toBe('New upload target')
  })

  it('exposes the description as the accessible description', async () => {
    mountDialog({
      title: 'T',
      description: 'Maps a friendly name to a folder path.',
    })
    await flush()

    expect(textOf(panel()!.getAttribute('aria-describedby'))).toBe(
      'Maps a friendly name to a folder path.',
    )
  })

  it('accepts a description slot for rich content', async () => {
    mountDialog(
      { title: 'Import sub events' },
      { description: '<span>Paste a list for “Sommerstevne”</span>' },
    )
    await flush()

    expect(textOf(panel()!.getAttribute('aria-describedby'))).toContain(
      'Sommerstevne',
    )
  })

  it('omits the description element entirely when none is given', async () => {
    mountDialog({ title: 'T' })
    await flush()

    expect(panel()!.getAttribute('aria-describedby')).toBeNull()
    expect(document.querySelector('.dialog-description')).toBeNull()
  })

  it('renders body and actions slots', async () => {
    mountDialog(
      { title: 'T' },
      { actions: '<button class="save">Save</button>' },
    )
    await flush()

    expect(document.querySelector('.dialog-panel .first-field')).not.toBeNull()
    expect(document.querySelector('.dialog-actions .save')).not.toBeNull()
  })

  it('omits the actions footer when the slot is unused', async () => {
    mountDialog({ title: 'T' })
    await flush()

    expect(document.querySelector('.dialog-actions')).toBeNull()
  })

  it('applies the width prop to the panel', async () => {
    mountDialog({ title: 'T', width: '560px' })
    await flush()

    expect((panel() as HTMLElement).style.width).toBe('560px')
  })

  it('emits close on Escape', async () => {
    const w = mountDialog({ title: 'T' })
    await flush()

    await pressEscape()
    expect(w.emitted('close')).toBeTruthy()
  })

  it('emits close on an outside pointer interaction', async () => {
    const outside = document.createElement('button')
    document.body.appendChild(outside)
    const w = mountDialog({ title: 'T' })
    await flush()

    await pointerDownOn(outside)
    expect(w.emitted('close')).toBeTruthy()
  })

  it('does not emit close when interacting inside the panel', async () => {
    const w = mountDialog({ title: 'T' })
    await flush()

    await pointerDownOn(document.querySelector('.body-btn')!)
    expect(w.emitted('close')).toBeFalsy()
  })

  it('emits close from the dismissible × button', async () => {
    const w = mountDialog({ title: 'T', dismissible: true })
    await flush()

    const close = document.querySelector<HTMLButtonElement>('.dialog-close')
    expect(close).not.toBeNull()
    close!.click()
    await nextTick()
    expect(w.emitted('close')).toBeTruthy()
  })

  it('hides the × button by default', async () => {
    mountDialog({ title: 'T' })
    await flush()

    expect(document.querySelector('.dialog-close')).toBeNull()
  })

  describe('role="alertdialog"', () => {
    it('uses the alertdialog role', async () => {
      mountDialog({ title: 'Delete target', role: 'alertdialog' })
      await flush()

      expect(panel()!.getAttribute('role')).toBe('alertdialog')
    })

    it('ignores Escape and outside clicks so the choice stays deliberate', async () => {
      const outside = document.createElement('button')
      document.body.appendChild(outside)
      const w = mountDialog({ title: 'Delete target', role: 'alertdialog' })
      await flush()

      await pressEscape()
      await pointerDownOn(outside)
      expect(w.emitted('close')).toBeFalsy()
    })

    it('still honours an explicit closeOnEscape override', async () => {
      const w = mountDialog({
        title: 'Delete',
        role: 'alertdialog',
        closeOnEscape: true,
      })
      await flush()

      await pressEscape()
      expect(w.emitted('close')).toBeTruthy()
    })
  })

  describe('controlled open state', () => {
    it('renders nothing when open is false', async () => {
      mountDialog({ title: 'T', open: false })
      await flush()

      expect(panel()).toBeNull()
    })

    it('opens and closes as the prop changes', async () => {
      const Host = defineComponent({
        props: { open: { type: Boolean, default: false } },
        setup(props) {
          return () => h(UiDialog, { title: 'T', open: props.open })
        },
      })
      wrapper = mount(Host, { props: { open: false }, attachTo: document.body })
      await flush()
      expect(panel()).toBeNull()

      await wrapper.setProps({ open: true })
      await flush()
      expect(panel()).not.toBeNull()

      await wrapper.setProps({ open: false })
      await flush()
      expect(panel()).toBeNull()
    })
  })

  it('removes the panel from the document on unmount', async () => {
    const w = mountDialog({ title: 'T' })
    await flush()
    expect(panel()).not.toBeNull()

    w.unmount()
    wrapper = null
    await nextTick()
    expect(panel()).toBeNull()
  })
})
