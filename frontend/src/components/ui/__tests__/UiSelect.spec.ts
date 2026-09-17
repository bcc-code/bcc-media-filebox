import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick, type Component } from 'vue'
import UiSelect, { type UiSelectEntry } from '../UiSelect.vue'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const trigger = () =>
  document.querySelector<HTMLButtonElement>('.select-trigger')!
const content = () => document.querySelector('.select-content')
// Unlike UiDialog/UiMenu, Zag's select keeps its content mounted and toggles
// `hidden` — it needs the element present to measure the trigger width and
// restore scroll — so "open" is a state, not the element's existence.
const isOpen = () => content()?.getAttribute('data-state') === 'open'
const items = () => [...document.querySelectorAll<HTMLElement>('.select-item')]
const labels = () =>
  items().map((el) => el.textContent?.replace('✓', '').trim())
const valueText = () =>
  document.querySelector('.select-value')?.textContent?.trim()

const FLAT: UiSelectEntry<string>[] = [
  { value: 'a', label: 'Alpha' },
  { value: 'b', label: 'Bravo' },
  { value: 'c', label: 'Charlie', disabled: true },
]

const GROUPED: UiSelectEntry<string>[] = [
  {
    label: 'Built-in directory groups',
    options: [{ value: 'bcc', label: 'All BCC members' }],
  },
  {
    label: 'Custom groups',
    options: [{ value: 'cam', label: 'Camera dept.' }],
  },
]

function mountSelect(props: Record<string, unknown>) {
  wrapper = mount(UiSelect as unknown as Component, {
    props,
    attachTo: document.body,
  })
  return wrapper
}

async function open() {
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

describe('UiSelect', () => {
  it('shows the placeholder when nothing is selected', async () => {
    mountSelect({
      modelValue: null,
      options: FLAT,
      placeholder: 'Choose a group…',
    })
    await flush()

    expect(valueText()).toBe('Choose a group…')
    expect(document.querySelector('.select-placeholder')).not.toBeNull()
  })

  it('shows the selected option label, not its value', async () => {
    mountSelect({ modelValue: 'b', options: FLAT })
    await flush()

    expect(valueText()).toBe('Bravo')
    expect(document.querySelector('.select-placeholder')).toBeNull()
  })

  it('falls back to the placeholder when the value matches no option', async () => {
    // GrantModal's draft starts at '' with no '' option — this used to render an
    // empty trigger rather than the placeholder.
    mountSelect({
      modelValue: '',
      options: FLAT,
      placeholder: 'Choose a group…',
    })
    await flush()

    expect(valueText()).toBe('Choose a group…')
    expect(document.querySelector('.select-placeholder')).not.toBeNull()
  })

  it('treats an empty-string option as a real selection when one exists', async () => {
    // TargetModal's '' means "no form" and must resolve to its label.
    mountSelect({
      modelValue: '',
      options: [{ value: '', label: 'None — free upload' }, ...FLAT],
      placeholder: 'Select…',
    })
    await flush()

    expect(valueText()).toBe('None — free upload')
    expect(document.querySelector('.select-placeholder')).toBeNull()
  })

  it('stays closed until the trigger is used', async () => {
    mountSelect({ modelValue: null, options: FLAT })
    await flush()
    expect(isOpen()).toBe(false)

    await open()
    expect(isOpen()).toBe(true)
  })

  it('exposes combobox/listbox semantics', async () => {
    mountSelect({ modelValue: null, options: FLAT })
    await flush()
    expect(trigger().getAttribute('aria-expanded')).toBe('false')

    await open()
    expect(trigger().getAttribute('aria-expanded')).toBe('true')
    // Zag puts role=listbox on the content, not on the inner <ul>.
    expect(content()?.getAttribute('role')).toBe('listbox')
    expect(items()[0].getAttribute('role')).toBe('option')
  })

  it('teleports the list to <body> so it escapes clipped ancestors', async () => {
    mountSelect({ modelValue: null, options: FLAT })
    await open()

    expect(content()!.closest('.select-positioner')!.parentElement).toBe(
      document.body,
    )
  })

  it('renders every option', async () => {
    mountSelect({ modelValue: null, options: FLAT })
    await open()

    expect(labels()).toEqual(['Alpha', 'Bravo', 'Charlie'])
  })

  it('emits the picked value on click', async () => {
    const w = mountSelect({ modelValue: null, options: FLAT })
    await open()

    items()[1].click()
    await flush()

    expect(w.emitted('update:modelValue')?.[0]).toEqual(['b'])
  })

  it('closes after picking', async () => {
    mountSelect({ modelValue: null, options: FLAT })
    await open()

    items()[0].click()
    await flush()

    expect(isOpen()).toBe(false)
  })

  it('marks the selected option as checked', async () => {
    mountSelect({ modelValue: 'b', options: FLAT })
    await open()

    expect(items()[1].getAttribute('data-state')).toBe('checked')
    expect(items()[0].getAttribute('data-state')).toBe('unchecked')
  })

  it('will not pick a disabled option', async () => {
    const w = mountSelect({ modelValue: null, options: FLAT })
    await open()

    const charlie = items()[2]
    expect(charlie.hasAttribute('data-disabled')).toBe(true)
    charlie.click()
    await flush()

    expect(w.emitted('update:modelValue')).toBeFalsy()
  })

  it('does not open when disabled', async () => {
    mountSelect({ modelValue: null, options: FLAT, disabled: true })
    await flush()

    await open()
    expect(isOpen()).toBe(false)
  })

  describe('numeric values', () => {
    const DAYS: UiSelectEntry<number>[] = [
      { value: 1, label: 'Expires in 1 day' },
      { value: 7, label: 'Expires in 7 days' },
      { value: 30, label: 'Expires in 30 days' },
    ]

    it('emits a number, not a stringified one', async () => {
      const w = mountSelect({ modelValue: 7, options: DAYS })
      await open()

      items()[2].click()
      await flush()

      const emitted = w.emitted('update:modelValue')?.[0]?.[0]
      expect(emitted).toBe(30)
      expect(typeof emitted).toBe('number')
    })

    it('resolves a numeric modelValue to its label', async () => {
      mountSelect({ modelValue: 7, options: DAYS })
      await flush()

      expect(valueText()).toBe('Expires in 7 days')
    })
  })

  describe('grouped options', () => {
    it('renders a label per group', async () => {
      mountSelect({ modelValue: null, options: GROUPED })
      await open()

      const groupLabels = [
        ...document.querySelectorAll('.select-group-label'),
      ].map((e) => e.textContent?.trim())
      expect(groupLabels).toEqual([
        'Built-in directory groups',
        'Custom groups',
      ])
    })

    it('keeps group labels out of the option list', async () => {
      mountSelect({ modelValue: null, options: GROUPED })
      await open()

      expect(labels()).toEqual(['All BCC members', 'Camera dept.'])
    })

    it('selects an option from inside a group', async () => {
      const w = mountSelect({ modelValue: null, options: GROUPED })
      await open()

      items()[1].click()
      await flush()

      expect(w.emitted('update:modelValue')?.[0]).toEqual(['cam'])
    })
  })

  describe('keyboard', () => {
    it('highlights with the arrow keys and picks with Enter', async () => {
      const w = mountSelect({ modelValue: null, options: FLAT })
      await open()

      await key('ArrowDown')
      const highlighted = items().findIndex((el) =>
        el.hasAttribute('data-highlighted'),
      )
      expect(highlighted).toBeGreaterThanOrEqual(0)

      await key('Enter')
      expect(w.emitted('update:modelValue')).toBeTruthy()
    })

    it('closes on Escape without picking', async () => {
      const w = mountSelect({ modelValue: null, options: FLAT })
      await open()

      await key('Escape')

      expect(isOpen()).toBe(false)
      expect(w.emitted('update:modelValue')).toBeFalsy()
    })
  })

  it('reflects options that change after mount', async () => {
    const w = mountSelect({ modelValue: null, options: FLAT })
    await open()
    expect(items()).toHaveLength(3)

    await w.setProps({ options: [...FLAT, { value: 'd', label: 'Delta' }] })
    await flush()

    expect(labels()).toContain('Delta')
  })

  it('applies the invalid border when validation fails', async () => {
    mountSelect({ modelValue: null, options: FLAT, invalid: true })
    await flush()

    expect(trigger().classList.contains('select-invalid')).toBe(true)
  })

  it('uses ariaLabel as the accessible name when the label sits outside', async () => {
    mountSelect({ modelValue: null, options: FLAT, ariaLabel: 'Upload form' })
    await flush()

    expect(trigger().getAttribute('aria-label')).toBe('Upload form')
  })
})
