import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import UiDrawer from '../UiDrawer.vue'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const panel = () => document.querySelector('.drawer-panel')
const backdrop = () => document.querySelector('.drawer-backdrop')
const closeBtn = () =>
  document.querySelector<HTMLButtonElement>('.drawer-head .btn')!

// `cancelable: true` is not decoration: Zag blocks a disallowed dismissal by
// calling preventDefault() on this event, and preventDefault() on a
// non-cancelable event is a silent no-op — so the guard would look absent.
const escape = () =>
  document.dispatchEvent(
    new KeyboardEvent('keydown', {
      key: 'Escape',
      bubbles: true,
      cancelable: true,
    }),
  )

function mountDrawer(props: Record<string, unknown> = {}) {
  wrapper = mount(UiDrawer, {
    props,
    slots: {
      head: '<span class="crumb">admin / users</span>',
      default: `<template #default="{ titleProps }">
        <h2 v-bind="titleProps" class="who">Anne Solberg</h2>
        <button class="inner-action">Edit</button>
      </template>`,
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

describe('UiDrawer', () => {
  it('renders a modal dialog with a backdrop', async () => {
    mountDrawer()
    await flush()

    expect(panel()).not.toBeNull()
    expect(backdrop()).not.toBeNull()
    // The hand-rolled drawer this replaces had no role and no aria-modal, so a
    // screen reader announced it as a plain div and kept reading the page.
    expect(panel()!.getAttribute('role')).toBe('dialog')
    expect(panel()!.getAttribute('aria-modal')).toBe('true')
  })

  it('names itself from the heading the caller binds titleProps to', async () => {
    mountDrawer()
    await flush()

    const labelledBy = panel()!.getAttribute('aria-labelledby')
    expect(labelledBy).toBeTruthy()
    expect(document.getElementById(labelledBy!)?.textContent).toBe(
      'Anne Solberg',
    )
  })

  it('closes on Escape', async () => {
    const w = mountDrawer()
    await flush()

    escape()
    await flush()

    // Escape did nothing at all before: there were no keydown handlers in src/.
    expect(w.emitted('close')).toHaveLength(1)
  })

  it('closes from the head button', async () => {
    const w = mountDrawer()
    await flush()

    closeBtn().click()
    await flush()

    expect(w.emitted('close')).toHaveLength(1)
  })

  it('does not close on a click inside the panel', async () => {
    const w = mountDrawer()
    await flush()

    document.querySelector<HTMLButtonElement>('.inner-action')!.click()
    await flush()

    expect(w.emitted('close')).toBeUndefined()
  })

  it('locks scrolling behind the panel', async () => {
    mountDrawer()
    await flush()

    // The old drawer let the page keep scrolling under it.
    expect(document.body.style.overflow).toBe('hidden')
  })

  it('releases the scroll lock once closed', async () => {
    const w = mountDrawer()
    await flush()
    await w.setProps({ open: false })
    await flush()

    expect(panel()).toBeNull()
    expect(document.body.style.overflow).not.toBe('hidden')
  })

  it('anchors right by default and left on request', async () => {
    mountDrawer()
    await flush()
    expect(panel()!.getAttribute('data-side')).toBe('right')
    wrapper!.unmount()

    mountDrawer({ side: 'left' })
    await flush()
    expect(panel()!.getAttribute('data-side')).toBe('left')
  })

  it('applies the width as an inline style', async () => {
    mountDrawer({ width: '480px' })
    await flush()

    expect((panel() as HTMLElement).style.width).toBe('480px')
  })

  it('renders the head slot beside the close button', async () => {
    mountDrawer()
    await flush()

    expect(document.querySelector('.drawer-head .crumb')?.textContent).toBe(
      'admin / users',
    )
  })

  it('teleports to body, escaping any transformed ancestor', async () => {
    const host = document.createElement('div')
    // A transform makes an element the containing block for position: fixed
    // descendants, which is what breaks a backdrop rendered in place.
    host.style.transform = 'translateY(0)'
    document.body.appendChild(host)

    wrapper = mount(UiDrawer, {
      slots: { default: '<h2>Anne Solberg</h2>' },
      attachTo: host,
    })
    await flush()

    expect(panel()!.parentElement!.parentElement).toBe(document.body)
    expect(host.querySelector('.drawer-panel')).toBeNull()
  })
})
