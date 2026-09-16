import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick, type Component } from 'vue'
import UiRadioGroup, { type UiRadioOption } from '../UiRadioGroup.vue'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const root = () => document.querySelector('[data-part="root"]')!
const items = () => [
  ...document.querySelectorAll<HTMLElement>('[data-part="item"]'),
]
const inputs = () => [
  ...document.querySelectorAll<HTMLInputElement>('input[type="radio"]'),
]
const checked = () =>
  items().find((i) => i.getAttribute('data-state') === 'checked')
const labels = () =>
  items().map((i) => i.textContent?.trim().replace(/\s+/g, ' '))

const OPTIONS: UiRadioOption<string>[] = [
  {
    value: 'none',
    label: 'No verification',
    description: 'Anyone with the link can download.',
  },
  { value: 'bcc_login', label: 'BCC login', description: 'Must sign in.' },
  {
    value: 'password',
    label: 'Password',
    description: 'You set a password.',
    disabled: true,
  },
]

function mountGroup(
  props: Record<string, unknown> = {},
  slots: Record<string, string> = {},
) {
  wrapper = mount(UiRadioGroup as unknown as Component, {
    props: { modelValue: 'none', options: OPTIONS, ...props },
    slots,
    attachTo: document.body,
  })
  return wrapper
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe('UiRadioGroup', () => {
  it('exposes radiogroup semantics, which the fake radios never had', async () => {
    mountGroup({ ariaLabel: 'Verification' })
    await flush()

    expect(root().getAttribute('role')).toBe('radiogroup')
    expect(root().getAttribute('aria-label')).toBe('Verification')
    // A real radio input per option, so the group is one tab stop and arrow-navigable.
    expect(inputs()).toHaveLength(3)
    expect(inputs().every((i) => i.type === 'radio')).toBe(true)
  })

  it('renders every option', async () => {
    mountGroup()
    await flush()

    expect(labels()[0]).toContain('No verification')
    expect(items()).toHaveLength(3)
  })

  it('marks the selected option', async () => {
    mountGroup({ modelValue: 'bcc_login' })
    await flush()

    expect(checked()?.textContent).toContain('BCC login')
  })

  it('emits the picked value', async () => {
    const w = mountGroup()
    await flush()

    inputs()[1].click()
    await flush()

    expect(w.emitted('update:modelValue')?.at(-1)).toEqual(['bcc_login'])
  })

  it('will not select a disabled option', async () => {
    const w = mountGroup()
    await flush()

    expect(items()[2].hasAttribute('data-disabled')).toBe(true)
    inputs()[2].click()
    await flush()

    expect(w.emitted('update:modelValue')).toBeFalsy()
  })

  it('selects nothing while the whole group is disabled', async () => {
    const w = mountGroup({ disabled: true })
    await flush()

    inputs()[1].click()
    await flush()

    expect(w.emitted('update:modelValue')).toBeFalsy()
  })

  it('follows a value changed from outside', async () => {
    const w = mountGroup()
    await flush()
    expect(checked()?.textContent).toContain('No verification')

    await w.setProps({ modelValue: 'bcc_login' })
    await flush()

    expect(checked()?.textContent).toContain('BCC login')
  })

  describe('card variant', () => {
    it('shows each description and a radio dot', async () => {
      mountGroup()
      await flush()

      expect(root().classList.contains('radio-cards')).toBe(true)
      expect(document.querySelectorAll('.radio-dot')).toHaveLength(3)
      expect(
        document.querySelector('.radio-description')?.textContent?.trim(),
      ).toBe('Anyone with the link can download.')
    })
  })

  describe('segmented variant', () => {
    it('reuses the shared .seg switch and drops the dots', async () => {
      mountGroup({ variant: 'segmented' })
      await flush()

      expect(root().classList.contains('seg')).toBe(true)
      expect(items()[0].classList.contains('seg-item')).toBe(true)
      // No radio dot, and no descriptions: the selected background carries it.
      expect(document.querySelector('.radio-dot')).toBeNull()
      expect(document.querySelector('.radio-description')).toBeNull()
    })

    it('still reports the checked option for styling', async () => {
      mountGroup({ variant: 'segmented', modelValue: 'bcc_login' })
      await flush()

      expect(checked()?.classList.contains('seg-item')).toBe(true)
    })
  })

  it('renders a per-option icon slot', async () => {
    mountGroup(
      { variant: 'segmented' },
      { 'icon-bcc_login': '<span class="ic">*</span>' },
    )
    await flush()

    expect(items()[1].querySelector('.ic')).not.toBeNull()
    expect(items()[0].querySelector('.ic')).toBeNull()
  })

  it('reflects options replaced after mount', async () => {
    const w = mountGroup()
    await flush()

    await w.setProps({
      options: [
        { value: 'none', label: 'No verification' },
        { value: 'extra', label: 'Something else' },
      ],
    })
    await flush()

    expect(items()).toHaveLength(2)
    expect(labels()[1]).toContain('Something else')
  })
})
