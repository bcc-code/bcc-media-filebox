import { describe, it, expect, afterEach, beforeEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import UiToaster from '../UiToaster.vue'
import { toastStore, notify, notifyError } from '../../../composables/useToast'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

/** Polls until the condition holds — timer-driven state settles unpredictably
 *  under jsdom, so a fixed sleep is either flaky or needlessly slow. */
// Generous ceiling on purpose: the poller returns as soon as the condition
// holds, so this only matters on a loaded machine — where 2.5s was not enough
// and these tests went flaky.
async function waitFor(condition: () => boolean, timeout = 8000) {
  const start = Date.now()
  while (!condition() && Date.now() - start < timeout) {
    await new Promise((r) => setTimeout(r, 25))
    await flush(1)
  }
  return condition()
}

const toasts = () => [...document.querySelectorAll('.toast')]
// A dismissed toast stays mounted in data-state="closed" for the store's
// removeDelay so it can animate out; "gone" from the user's point of view is
// that closed state, not removal from the DOM.
const visible = () =>
  toasts().filter((t) => t.getAttribute('data-state') !== 'closed')
const texts = () =>
  toasts().map((t) => t.querySelector('.toast-title')?.textContent?.trim())
const group = () =>
  document.querySelector('[data-scope="toast"][data-part="group"]')

beforeEach(async () => {
  wrapper = mount(UiToaster, { attachTo: document.body })
  await flush()
})

afterEach(async () => {
  // The store is a module singleton, so clear it or toasts leak between tests.
  toastStore.remove()
  await flush()
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe('UiToaster', () => {
  it('renders nothing while the store is empty', () => {
    expect(toasts()).toHaveLength(0)
  })

  it('renders a toast pushed through notify()', async () => {
    notify('Download link copied')
    await flush()

    expect(texts()).toEqual(['Download link copied'])
  })

  it('types notify() as success and notifyError() as error', async () => {
    notify('Saved')
    await flush()
    expect(toasts()[0].getAttribute('data-type')).toBe('success')

    toastStore.remove()
    await flush()

    notifyError('Failed to save')
    await flush()
    expect(toasts()[0].getAttribute('data-type')).toBe('error')
  })

  it('announces toasts through a polite live region', async () => {
    notify('Saved')
    await flush()

    // The old markup was a bare div — screen readers never announced it.
    expect(group()?.getAttribute('aria-live')).toBe('polite')
    expect(toasts()[0].getAttribute('role')).toBe('status')
  })

  it('teleports the group to <body>', async () => {
    notify('Saved')
    await flush()

    expect(group()?.parentElement).toBe(document.body)
  })

  it('shows several toasts at once instead of replacing the previous one', async () => {
    notify('First')
    notify('Second')
    notify('Third')
    await flush()

    expect(texts()).toEqual(
      expect.arrayContaining(['First', 'Second', 'Third']),
    )
    expect(toasts()).toHaveLength(3)
  })

  it('caps the visible count at the store max', async () => {
    for (let i = 0; i < 8; i++) notify(`Toast ${i}`)
    await flush()

    expect(toasts().length).toBeLessThanOrEqual(4)
  })

  it('dismisses a toast from its close button', async () => {
    notify('Dismiss me')
    await flush()
    expect(toasts()).toHaveLength(1)

    document.querySelector<HTMLButtonElement>('.toast-close')!.click()
    await flush()
    expect(visible()).toHaveLength(0)

    // …and is removed outright once removeDelay has passed.
    expect(await waitFor(() => toasts().length === 0)).toBe(true)
  })

  it('auto-dismisses once the duration elapses', async () => {
    toastStore.create({ title: 'Brief', type: 'success', duration: 60 })
    await flush()
    expect(toasts()).toHaveLength(1)

    expect(await waitFor(() => toasts().length === 0)).toBe(true)
  })

  it('keeps a toast up while it is paused', async () => {
    const id = toastStore.create({
      title: 'Held',
      type: 'success',
      duration: 80,
    })
    await flush()
    toastStore.pause(id)

    // Well past its 80ms duration, but paused, so it must still be up.
    await new Promise((r) => setTimeout(r, 400))
    await flush()
    expect(visible()).toHaveLength(1)

    toastStore.resume(id)
    expect(await waitFor(() => toasts().length === 0)).toBe(true)
  })

  it('renders a description under the title when given', async () => {
    toastStore.create({
      title: 'Upload failed',
      description: 'Disk is full',
      type: 'error',
    })
    await flush()

    expect(
      document.querySelector('.toast-description')?.textContent?.trim(),
    ).toBe('Disk is full')
  })

  it('omits the description element when there is none', async () => {
    notify('No detail')
    await flush()

    expect(document.querySelector('.toast-description')).toBeNull()
  })
})
