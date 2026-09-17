import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import UiBadge from '../UiBadge.vue'

let wrapper: VueWrapper | null = null

const mountBadge = (props: Record<string, unknown> = {}, slot = 'Admin') => {
  wrapper = mount(UiBadge, { props, slots: { default: slot } })
  return wrapper
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('UiBadge', () => {
  it('renders the label with the shared .badge chrome', () => {
    const el = mountBadge().get('span')
    expect(el.classes()).toEqual(['badge'])
    expect(el.text()).toBe('Admin')
  })

  it.each([
    ['accent', 'badge-accent'],
    ['ok', 'badge-ok'],
    ['warn', 'badge-warn'],
    ['danger', 'badge-danger'],
  ])('maps variant %s onto .%s', (variant, cls) => {
    expect(mountBadge({ variant }).get('span').classes()).toContain(cls)
  })

  it('emits no modifier for the default variant', () => {
    // `.badge-default` does not exist; default is the absence of a modifier.
    expect(mountBadge({ variant: 'default' }).get('span').classes()).toEqual([
      'badge',
    ])
  })

  it('omits the dot unless asked', () => {
    expect(mountBadge().find('.badge-dot').exists()).toBe(false)
  })

  it('renders the dot before the label', () => {
    const el = mountBadge({ dot: true }).get('span')
    // Compare against the label's position, not firstElementChild: the label is
    // a text node, so the dot is the first *element* whichever side it is on.
    const dot = el.get('.badge-dot').element
    const order = dot.compareDocumentPosition(el.element.lastChild!)
    expect(order & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    // No inline colour: .badge-dot uses currentColor, so the variant tints it.
    expect(dot.getAttribute('style')).toBeNull()
  })

  it('switches to the monospace face on request', () => {
    expect(mountBadge({ mono: true }).get('span').classes()).toContain('mono')
  })

  it('applies a custom tone to the text and border', () => {
    const el = mountBadge({ tone: '#5da5ff' }).get('span')
    const style = el.attributes('style')!
    // jsdom normalises the hex to rgb() and rewrites color-mix's percentage.
    expect(style).toContain('color: rgb(93, 165, 255)')
    expect(style).toMatch(/border-color: color-mix\(.*rgb\(93, 165, 255\)/)
  })

  it('leaves the dot untinted under a tone, since it inherits the colour', () => {
    const el = mountBadge({ tone: '#5da5ff', dot: true }).get('span')
    expect(el.get('.badge-dot').attributes('style')).toBeUndefined()
  })

  it('sets no inline style without a tone', () => {
    expect(mountBadge().get('span').attributes('style')).toBeUndefined()
  })

  it('merges a caller class rather than dropping it', () => {
    wrapper = mount(UiBadge, {
      props: { variant: 'ok' },
      attrs: { class: 'pkg-state' },
      slots: { default: 'Live' },
    })
    expect(wrapper.get('span').classes().sort()).toEqual([
      'badge',
      'badge-ok',
      'pkg-state',
    ])
  })
})
