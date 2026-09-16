import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import UiButton from '../UiButton.vue'

let wrapper: VueWrapper | null = null

function mountButton(props: Record<string, unknown> = {}, slot = 'Save') {
  wrapper = mount(UiButton, {
    props,
    slots: { default: slot },
    global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
  })
  return wrapper
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('UiButton', () => {
  it('renders a non-submitting button by default', () => {
    const el = mountButton().get('button')
    // Default `type` matters: a bare <button> in a form submits it.
    expect(el.attributes('type')).toBe('button')
    expect(el.classes()).toEqual(['btn'])
    expect(el.text()).toBe('Save')
  })

  it('maps variant, size and shape props onto the shared classes', () => {
    const el = mountButton({
      variant: 'danger',
      size: 'sm',
      block: true,
      icon: true,
      active: true,
    }).get('button')
    expect(el.classes().sort()).toEqual(
      [
        'btn',
        'btn-active',
        'btn-block',
        'btn-danger',
        'btn-icon',
        'btn-sm',
      ].sort(),
    )
  })

  it('emits no class for the default variant and size', () => {
    // 'default'/'md' are the absence of a modifier, not classes of their own —
    // `.btn-default` and `.btn-md` do not exist in components.css.
    const el = mountButton({ variant: 'default', size: 'md' }).get('button')
    expect(el.classes()).toEqual(['btn'])
  })

  it('merges a caller-supplied class instead of dropping it', () => {
    wrapper = mount(UiButton, {
      props: { variant: 'ghost' },
      attrs: { class: 'row-action' },
      slots: { default: 'Edit' },
    })
    expect(wrapper.get('button').classes().sort()).toEqual([
      'btn',
      'btn-ghost',
      'row-action',
    ])
  })

  it('swaps the slot for the pending label and blocks the click while loading', () => {
    const el = mountButton(
      { loading: true, loadingLabel: 'Saving…' },
      'Save',
    ).get('button')
    expect(el.text()).toBe('Saving…')
    expect(el.attributes('disabled')).toBeDefined()
  })

  it('keeps the slot while loading when no pending label is given', () => {
    const el = mountButton({ loading: true }).get('button')
    expect(el.text()).toBe('Save')
    expect(el.attributes('disabled')).toBeDefined()
  })

  it('shows the slot again once loading ends', async () => {
    const w = mountButton({ loading: true, loadingLabel: 'Saving…' })
    await w.setProps({ loading: false })
    expect(w.get('button').text()).toBe('Save')
    expect(w.get('button').attributes('disabled')).toBeUndefined()
  })

  it('renders an anchor for href, without button-only attributes', () => {
    const el = mountButton({ href: '/send?tab=sent', variant: 'primary' }).get(
      'a',
    )
    expect(el.attributes('href')).toBe('/send?tab=sent')
    // `type` and `disabled` are meaningless on an anchor.
    expect(el.attributes('type')).toBeUndefined()
    expect(el.attributes('disabled')).toBeUndefined()
    expect(el.classes()).toContain('btn-primary')
  })

  it('marks a disabled link for assistive tech and drops it from the tab order', () => {
    // An <a> ignores `disabled` entirely, so without this a "disabled" link
    // would still be clickable and focusable.
    const el = mountButton({ href: '/x', disabled: true }).get('a')
    expect(el.attributes('aria-disabled')).toBe('true')
    expect(el.attributes('tabindex')).toBe('-1')
  })

  it('renders a router-link for `to`, without button-only attributes', () => {
    const el = mountButton({ to: '/', variant: 'ghost', size: 'sm' }).get('a')
    expect(el.attributes('to')).toBe('/')
    expect(el.attributes('type')).toBeUndefined()
    expect(el.attributes('disabled')).toBeUndefined()
    expect(el.classes().sort()).toEqual(['btn', 'btn-ghost', 'btn-sm'])
  })

  it('marks a disabled router-link the same way as an anchor', () => {
    const el = mountButton({ to: '/', loading: true }).get('a')
    expect(el.attributes('aria-disabled')).toBe('true')
    expect(el.attributes('tabindex')).toBe('-1')
  })

  it('leaves an enabled link focusable', () => {
    const el = mountButton({ href: '/x' }).get('a')
    expect(el.attributes('aria-disabled')).toBeUndefined()
    expect(el.attributes('tabindex')).toBeUndefined()
  })

  it('forwards clicks and stops them when disabled', async () => {
    let clicks = 0
    wrapper = mount(UiButton, {
      props: { onClick: () => clicks++ },
      slots: { default: 'Go' },
    })
    await wrapper.get('button').trigger('click')
    expect(clicks).toBe(1)

    await wrapper.setProps({ disabled: true })
    await wrapper.get('button').trigger('click')
    expect(clicks).toBe(1)
  })
})
