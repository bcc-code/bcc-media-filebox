import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import UiTabs, { type UiTabEntry } from '../UiTabs.vue'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const triggers = () => [...document.querySelectorAll<HTMLButtonElement>('.tab')]
const labels = () =>
  triggers().map((t) => t.textContent?.trim().replace(/\s+/g, ' '))
const selected = () => triggers().find((t) => t.hasAttribute('data-selected'))
const panels = () => [
  ...document.querySelectorAll<HTMLElement>('[data-part="content"]'),
]
const visiblePanel = () => panels().find((p) => !p.hasAttribute('hidden'))

const TABS: UiTabEntry[] = [
  { value: 'compose', label: 'New package' },
  { value: 'sent', label: 'Sent packages', count: 4 },
  { value: 'preview', label: 'Recipient preview' },
]

function mountTabs(
  props: Record<string, unknown> = {},
  extraSlots: Record<string, string> = {},
) {
  wrapper = mount(UiTabs, {
    props: { modelValue: 'compose', tabs: TABS, ...props },
    slots: {
      compose: '<p class="p-compose">compose</p>',
      sent: '<p class="p-sent">sent</p>',
      preview: '<p class="p-preview">preview</p>',
      ...extraSlots,
    },
    attachTo: document.body,
  })
  return wrapper
}

/** Zag handles tab keys on the tablist, not on the individual triggers. */
async function key(k: string) {
  selected()?.focus()
  await flush(1)
  document
    .querySelector('.tab-list')!
    .dispatchEvent(
      new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }),
    )
  await flush()
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe('UiTabs', () => {
  it('renders a trigger per tab, with counts where given', async () => {
    mountTabs()
    await flush()

    expect(labels()).toEqual([
      'New package',
      'Sent packages 4',
      'Recipient preview',
    ])
  })

  it('exposes tablist semantics the hand-rolled bars never had', async () => {
    mountTabs()
    await flush()

    expect(document.querySelector('.tab-list')?.getAttribute('role')).toBe(
      'tablist',
    )
    expect(triggers().map((t) => t.getAttribute('role'))).toEqual([
      'tab',
      'tab',
      'tab',
    ])
    expect(triggers()[0].getAttribute('aria-selected')).toBe('true')
    expect(triggers()[1].getAttribute('aria-selected')).toBe('false')
  })

  it('links each trigger to its panel', async () => {
    mountTabs()
    await flush()

    const controls = triggers()[0].getAttribute('aria-controls')
    expect(controls).toBeTruthy()
    const panel = document.getElementById(controls!)
    expect(panel).not.toBeNull()
    expect(panel!.getAttribute('role')).toBe('tabpanel')
  })

  it('marks the selected tab for styling', async () => {
    mountTabs({ modelValue: 'sent' })
    await flush()

    expect(selected()?.textContent?.trim()).toContain('Sent packages')
  })

  it('shows only the selected panel', async () => {
    mountTabs()
    await flush()

    expect(document.querySelector('.p-compose')).not.toBeNull()
    expect(visiblePanel()).toBe(panels()[0])
  })

  it('mounts only the selected panel, so sibling tabs do not fetch on load', async () => {
    mountTabs()
    await flush()

    // The v-if/v-else-if chain this replaces never mounted inactive tabs; all
    // seven admin tabs mounting at once would fire their loaders up front.
    expect(document.querySelector('.p-compose')).not.toBeNull()
    expect(document.querySelector('.p-sent')).toBeNull()
    expect(document.querySelector('.p-preview')).toBeNull()
  })

  it('keeps every panel element present, and links the selected one', async () => {
    mountTabs()
    await flush()

    expect(panels()).toHaveLength(3)
    // Zag sets aria-controls only on the selected trigger — an unselected tab
    // has no active panel to point at.
    const withControls = triggers().filter((t) =>
      t.hasAttribute('aria-controls'),
    )
    expect(withControls).toHaveLength(1)
    expect(
      document.getElementById(withControls[0].getAttribute('aria-controls')!),
    ).not.toBeNull()
  })

  it('emits the new value on click', async () => {
    const w = mountTabs()
    await flush()

    triggers()[1].click()
    await flush()

    expect(w.emitted('update:modelValue')?.at(-1)).toEqual(['sent'])
  })

  it('swaps the panel when the model changes', async () => {
    const w = mountTabs()
    await flush()

    await w.setProps({ modelValue: 'preview' })
    await flush()

    expect(document.querySelector('.p-preview')).not.toBeNull()
    expect(document.querySelector('.p-compose')).toBeNull()
  })

  describe('keyboard', () => {
    // Roving focus is the accessibility win here: before this, Tab was the only
    // way through the bar, one stop per tab.
    it('moves focus to the next tab with ArrowRight', async () => {
      mountTabs()
      await flush()
      triggers()[0].focus()
      await flush(1)

      await key('ArrowRight')

      expect(document.activeElement).toBe(triggers()[1])
    })

    it('moves focus to the previous tab with ArrowLeft', async () => {
      mountTabs({ modelValue: 'sent' })
      await flush()
      triggers()[1].focus()
      await flush(1)

      await key('ArrowLeft')

      expect(document.activeElement).toBe(triggers()[0])
    })

    it('jumps to the last and first tab with End and Home', async () => {
      mountTabs()
      await flush()
      triggers()[0].focus()
      await flush(1)

      await key('End')
      expect(document.activeElement).toBe(triggers()[2])

      await key('Home')
      expect(document.activeElement).toBe(triggers()[0])
    })

    it('uses roving tabindex so the bar is a single tab stop', async () => {
      mountTabs({ modelValue: 'sent' })
      await flush()

      expect(triggers().map((t) => t.getAttribute('tabindex'))).toEqual([
        '-1',
        '0',
        '-1',
      ])
    })
  })

  // Selection-follows-focus (activationMode "automatic") is not asserted here:
  // jsdom moves focus but does not run the follow-up selection. Verified in a
  // real browser instead.

  it('renders a per-tab icon slot', async () => {
    mountTabs({}, { 'icon-sent': '<span class="ic-sent">*</span>' })
    await flush()

    expect(triggers()[1].querySelector('.ic-sent')).not.toBeNull()
    expect(triggers()[0].querySelector('.ic-sent')).toBeNull()
  })

  it('applies listClass and panelClass for surface-specific spacing', async () => {
    mountTabs({ listClass: 'tab-list-inset', panelClass: 'page' })
    await flush()

    expect(
      document.querySelector('.tab-list')?.classList.contains('tab-list-inset'),
    ).toBe(true)
    expect(panels()[0].classList.contains('page')).toBe(true)
  })

  it('reflects counts that change after mount', async () => {
    const w = mountTabs()
    await flush()

    await w.setProps({
      tabs: TABS.map((t) => (t.value === 'sent' ? { ...t, count: 9 } : t)),
    })
    await flush()

    expect(labels()[1]).toBe('Sent packages 9')
  })
})
