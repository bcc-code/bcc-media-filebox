import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import TargetSelector from '../TargetSelector.vue'
import type { TargetInfo } from '../../types'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const TARGETS = [
  { name: 'Sunday Service', form: [] },
  { name: 'Conference', form: [] },
] as unknown as TargetInfo[]

const items = () => [
  ...document.querySelectorAll<HTMLElement>('[data-part="item"]'),
]

function mountSelector(modelValue = 'Sunday Service') {
  wrapper = mount(TargetSelector, {
    props: { modelValue, targets: TARGETS },
    attachTo: document.body,
  })
  return wrapper
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe('TargetSelector', () => {
  it('renders a named radiogroup, which the fake cards never had', async () => {
    mountSelector()
    await flush()

    const root = document.querySelector('[data-part="root"]')!
    expect(root.getAttribute('role')).toBe('radiogroup')
    expect(root.getAttribute('aria-label')).toBe('Upload target')
    expect(root.classList.contains('radio-cards-grid')).toBe(true)
  })

  it('renders one card per target, labelled by name', async () => {
    mountSelector()
    await flush()

    expect(items()).toHaveLength(2)
    expect(items().map((i) => i.textContent?.trim())).toEqual([
      'Sunday Service',
      'Conference',
    ])
  })

  it('marks the selected target', async () => {
    mountSelector('Conference')
    await flush()

    expect(items()[1].getAttribute('data-state')).toBe('checked')
    expect(items()[0].getAttribute('data-state')).toBe('unchecked')
  })

  it('emits the target name on pick', async () => {
    const w = mountSelector()
    await flush()

    items()[1].click()
    await flush()

    expect(w.emitted('update:modelValue')).toEqual([['Conference']])
  })

  it('gives every target its own icon tile on the card top row', async () => {
    mountSelector()
    await flush()

    // One tile per card, in the top row rather than inside the label.
    expect(
      document.querySelectorAll('.radio-card-top .target-ic'),
    ).toHaveLength(2)
    expect(document.querySelector('.radio-label .target-ic')).toBeNull()
  })

  it('reflects targets loaded after mount', async () => {
    const w = mountSelector('')
    await flush()
    expect(items()).toHaveLength(2)

    await w.setProps({
      targets: [{ name: 'Only one', form: [] }] as unknown as TargetInfo[],
    })
    await flush()

    expect(items().map((i) => i.textContent?.trim())).toEqual(['Only one'])
  })
})
