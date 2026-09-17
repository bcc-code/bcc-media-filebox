import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import UiCollapsible from '../UiCollapsible.vue'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const trigger = () => document.querySelector<HTMLButtonElement>('.trig')!
const content = () => document.querySelector('.body')!

function mountCollapsible(props: Record<string, unknown> = {}) {
  wrapper = mount(UiCollapsible, {
    props,
    slots: {
      default: `<template #default="{ trigger, content, open, visible }">
        <button v-bind="trigger" class="trig">{{ open ? 'close' : 'open' }}</button>
        <div v-bind="content" class="body"><span v-if="visible" class="inner">rows</span></div>
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

describe('UiCollapsible', () => {
  it('starts collapsed', async () => {
    mountCollapsible()
    await flush()

    expect(trigger().getAttribute('aria-expanded')).toBe('false')
    expect(content().getAttribute('data-state')).toBe('closed')
  })

  it('reports expansion to assistive tech, which the hand-rolled version never did', async () => {
    mountCollapsible()
    await flush()

    trigger().click()
    await flush()

    expect(trigger().getAttribute('aria-expanded')).toBe('true')
    // aria-controls must point at the content element that actually exists.
    expect(trigger().getAttribute('aria-controls')).toBe(content().id)
  })

  it('toggles content visibility', async () => {
    mountCollapsible()
    await flush()
    expect(document.querySelector('.inner')).toBeNull()

    trigger().click()
    await flush()
    expect(document.querySelector('.inner')).not.toBeNull()

    trigger().click()
    await flush()
    expect(content().getAttribute('data-state')).toBe('closed')
  })

  it('exposes open state to the slot', async () => {
    mountCollapsible()
    await flush()
    expect(trigger().textContent?.trim()).toBe('open')

    trigger().click()
    await flush()
    expect(trigger().textContent?.trim()).toBe('close')
  })

  it('emits update:open so a parent can control it', async () => {
    const w = mountCollapsible()
    await flush()

    trigger().click()
    await flush()

    expect(w.emitted('update:open')?.at(-1)).toEqual([true])
  })

  it('honours a controlled open prop', async () => {
    const w = mountCollapsible({ open: true })
    await flush()

    expect(trigger().getAttribute('aria-expanded')).toBe('true')

    await w.setProps({ open: false })
    await flush()
    expect(trigger().getAttribute('aria-expanded')).toBe('false')
  })

  it('does not toggle when disabled', async () => {
    mountCollapsible({ disabled: true })
    await flush()

    trigger().click()
    await flush()

    expect(trigger().getAttribute('aria-expanded')).toBe('false')
  })
})
