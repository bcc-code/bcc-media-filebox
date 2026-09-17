import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import UiInput from '../UiInput.vue'
import UiTextarea from '../UiTextarea.vue'

let wrapper: VueWrapper | null = null

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe.each([
  ['UiInput', UiInput, 'input'],
  ['UiTextarea', UiTextarea, 'textarea'],
])('%s', (_name, Component, tag) => {
  const mountIt = (props: Record<string, unknown> = {}, attrs = {}) => {
    wrapper = mount(Component, {
      props: { modelValue: 'hello', ...props },
      attrs,
    })
    return wrapper
  }

  it('carries the shared .inp chrome', () => {
    // The whole point: one definition of the field's background, border and
    // radius, so a stacked form cannot show one field on a different surface.
    expect(mountIt().get(tag).classes()).toContain('inp')
  })

  it('renders the model value', () => {
    expect((mountIt().get(tag).element as HTMLInputElement).value).toBe('hello')
  })

  it('emits on input', async () => {
    const w = mountIt()
    const el = w.get(tag)
    ;(el.element as HTMLInputElement).value = 'typed'
    await el.trigger('input')
    expect(w.emitted('update:modelValue')).toEqual([['typed']])
  })

  it('follows a value changed from outside', async () => {
    const w = mountIt()
    await w.setProps({ modelValue: 'from parent' })
    expect((w.get(tag).element as HTMLInputElement).value).toBe('from parent')
  })

  it('marks invalid for assistive tech, not just styling', () => {
    const el = mountIt({ invalid: true }).get(tag)
    expect(el.attributes('aria-invalid')).toBe('true')
    expect(el.classes()).toContain('inp-invalid')
  })

  it('omits aria-invalid when valid', () => {
    // `aria-invalid="false"` is noise; absent is the correct default.
    expect(mountIt().get(tag).attributes('aria-invalid')).toBeUndefined()
  })

  it('merges a caller class rather than dropping it', () => {
    const el = mountIt({}, { class: 'pw-verify' }).get(tag)
    expect(el.classes().sort()).toEqual(['inp', 'pw-verify'])
  })

  it('passes unknown attributes straight through', () => {
    // Everything the component does not own — placeholder, disabled, maxlength,
    // autocomplete, type — must reach the element untouched.
    const el = mountIt(
      {},
      { placeholder: 'Enter it', disabled: '', maxlength: '80' },
    ).get(tag)
    expect(el.attributes('placeholder')).toBe('Enter it')
    expect(el.attributes('disabled')).toBeDefined()
    expect(el.attributes('maxlength')).toBe('80')
  })
})

describe('UiInput only', () => {
  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
  })

  it('takes its type from a fallthrough attribute', () => {
    wrapper = mount(UiInput, {
      props: { modelValue: '' },
      attrs: { type: 'password' },
    })
    expect(wrapper.get('input').attributes('type')).toBe('password')
  })

  it('switches to the monospace face on request', () => {
    wrapper = mount(UiInput, { props: { modelValue: 'SMR26', mono: true } })
    expect(wrapper.get('input').classes()).toContain('mono')
  })
})
