import { describe, it, expect, afterEach, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import UiMenu, { type UiMenuEntry } from '../UiMenu.vue'

let wrapper: VueWrapper | null = null

// Zag defers to requestAnimationFrame (positioning, listener arming), so tests
// must flush frames rather than ticks.
async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const trigger = () =>
  document.querySelector<HTMLButtonElement>('.menu-trigger-reset')!
const content = () => document.querySelector('.menu-content')
const itemEls = () => [...document.querySelectorAll<HTMLElement>('.menu-item')]

function mountMenu(items: UiMenuEntry[], props: Record<string, unknown> = {}) {
  wrapper = mount(UiMenu, {
    props: { items, ...props },
    slots: { trigger: '<span class="trigger-label">Open</span>' },
    attachTo: document.body,
  })
  return wrapper
}

async function openMenu() {
  trigger().click()
  await flush()
}

async function key(k: string) {
  const el = content() ?? trigger()
  el.dispatchEvent(
    new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }),
  )
  await flush()
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe('UiMenu', () => {
  it('renders the trigger slot and stays closed initially', async () => {
    mountMenu([{ label: 'A', onSelect: vi.fn() }])
    await flush()

    expect(document.querySelector('.trigger-label')).not.toBeNull()
    expect(content()).toBeNull()
  })

  it('marks the trigger with collapsed/expanded state', async () => {
    mountMenu([{ label: 'A', onSelect: vi.fn() }])
    await flush()
    expect(trigger().getAttribute('aria-expanded')).toBe('false')

    await openMenu()
    expect(trigger().getAttribute('aria-expanded')).toBe('true')
    expect(trigger().getAttribute('aria-controls')).toBe(content()!.id)
  })

  it('teleports the content to <body> so it escapes clipped ancestors', async () => {
    mountMenu([{ label: 'A', onSelect: vi.fn() }])
    await openMenu()

    expect(content()!.closest('.menu-positioner')!.parentElement).toBe(
      document.body,
    )
  })

  it('gives the content and items correct menu roles', async () => {
    mountMenu([
      { label: 'A', onSelect: vi.fn() },
      { label: 'B', onSelect: vi.fn() },
    ])
    await openMenu()

    expect(content()!.getAttribute('role')).toBe('menu')
    expect(itemEls().map((el) => el.getAttribute('role'))).toEqual([
      'menuitem',
      'menuitem',
    ])
  })

  it('renders separators as separators, not as items', async () => {
    mountMenu([
      { label: 'A', onSelect: vi.fn() },
      { type: 'separator' },
      { label: 'B', onSelect: vi.fn() },
    ])
    await openMenu()

    expect(itemEls()).toHaveLength(2)
    const sep = document.querySelector('.menu-separator')
    expect(sep).not.toBeNull()
    expect(sep!.getAttribute('role')).toBe('separator')
  })

  it('renders the header slot outside the item list', async () => {
    wrapper = mount(UiMenu, {
      props: { items: [{ label: 'A', onSelect: vi.fn() }] },
      slots: {
        trigger: '<span>Open</span>',
        header: '<div class="who">signed in</div>',
      },
      attachTo: document.body,
    })
    await openMenu()

    expect(document.querySelector('.menu-header .who')).not.toBeNull()
    // The header must never become a focus stop.
    expect(
      document.querySelector('.menu-header')!.getAttribute('role'),
    ).toBeNull()
    expect(itemEls()).toHaveLength(1)
  })

  it('calls the matching entry handler on click', async () => {
    const a = vi.fn()
    const b = vi.fn()
    mountMenu([
      { label: 'A', onSelect: a },
      { label: 'B', onSelect: b },
    ])
    await openMenu()

    itemEls()[1].click()
    await flush()

    expect(b).toHaveBeenCalledOnce()
    expect(a).not.toHaveBeenCalled()
  })

  it('dispatches to the right handler when separators shift the indices', async () => {
    const last = vi.fn()
    mountMenu([
      { label: 'A', onSelect: vi.fn() },
      { type: 'separator' },
      { label: 'B', onSelect: vi.fn() },
      { type: 'separator' },
      { label: 'C', onSelect: last },
    ])
    await openMenu()

    itemEls()[2].click()
    await flush()

    expect(last).toHaveBeenCalledOnce()
  })

  it('closes after a selection', async () => {
    mountMenu([{ label: 'A', onSelect: vi.fn() }])
    await openMenu()

    itemEls()[0].click()
    await flush()

    expect(content()).toBeNull()
  })

  it('does not fire a disabled entry', async () => {
    const onSelect = vi.fn()
    mountMenu([{ label: 'Nope', onSelect, disabled: true }])
    await openMenu()

    const item = itemEls()[0]
    expect(item.hasAttribute('data-disabled')).toBe(true)
    item.click()
    await flush()

    expect(onSelect).not.toHaveBeenCalled()
  })

  it('applies the danger class to destructive entries', async () => {
    mountMenu([{ label: 'Delete', onSelect: vi.fn(), danger: true }])
    await openMenu()

    expect(itemEls()[0].classList.contains('menu-item-danger')).toBe(true)
  })

  it('closes on Escape', async () => {
    mountMenu([{ label: 'A', onSelect: vi.fn() }])
    await openMenu()
    expect(content()).not.toBeNull()

    await key('Escape')
    expect(content()).toBeNull()
  })

  it('closes on an outside pointer interaction', async () => {
    const outside = document.createElement('button')
    document.body.appendChild(outside)
    mountMenu([{ label: 'A', onSelect: vi.fn() }])
    await openMenu()

    outside.dispatchEvent(
      new PointerEvent('pointerdown', { bubbles: true, cancelable: true }),
    )
    outside.dispatchEvent(
      new MouseEvent('click', { bubbles: true, cancelable: true }),
    )
    await flush()

    expect(content()).toBeNull()
  })

  it('highlights items with the arrow keys', async () => {
    mountMenu([
      { label: 'A', onSelect: vi.fn() },
      { label: 'B', onSelect: vi.fn() },
    ])
    await openMenu()

    await key('ArrowDown')
    const highlighted = () =>
      itemEls().findIndex((el) => el.hasAttribute('data-highlighted'))
    const first = highlighted()
    expect(first).toBeGreaterThanOrEqual(0)

    await key('ArrowDown')
    expect(highlighted()).not.toBe(first)
  })

  it('selects the highlighted item with Enter', async () => {
    const a = vi.fn()
    mountMenu([{ label: 'A', onSelect: a }])
    await openMenu()

    await key('ArrowDown')
    await key('Enter')

    expect(a).toHaveBeenCalledOnce()
  })

  it('emits openChange as the menu toggles', async () => {
    const w = mountMenu([{ label: 'A', onSelect: vi.fn() }])
    await openMenu()
    expect(w.emitted('openChange')?.at(-1)).toEqual([true])

    await key('Escape')
    expect(w.emitted('openChange')?.at(-1)).toEqual([false])
  })

  it('applies the width prop to the content', async () => {
    mountMenu([{ label: 'A', onSelect: vi.fn() }], { width: '240px' })
    await openMenu()

    expect((content() as HTMLElement).style.width).toBe('240px')
  })

  it('reflects entries that appear after mount', async () => {
    const w = mountMenu([{ label: 'A', onSelect: vi.fn() }])
    await openMenu()
    expect(itemEls()).toHaveLength(1)

    const added = vi.fn()
    await w.setProps({
      items: [
        { label: 'A', onSelect: vi.fn() },
        { label: 'B', onSelect: added },
      ],
    })
    await flush()
    expect(itemEls()).toHaveLength(2)

    itemEls()[1].click()
    await flush()
    expect(added).toHaveBeenCalledOnce()
  })
})
