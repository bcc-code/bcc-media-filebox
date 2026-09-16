import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import UiConfirm from '../UiConfirm.vue'
import { confirmAction, pending, settle } from '../../../composables/useConfirm'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const panel = () => document.querySelector('.dialog-panel')
const buttons = () => [
  ...document.querySelectorAll<HTMLButtonElement>('.dialog-actions button'),
]
const byText = (re: RegExp) =>
  buttons().find((b) => re.test(b.textContent ?? ''))!
const textOf = (id: string | null) =>
  id ? document.getElementById(id)?.textContent?.trim() : null

function mountHost() {
  wrapper = mount(UiConfirm, { attachTo: document.body })
  return wrapper
}

async function pressEscape() {
  document.dispatchEvent(
    new KeyboardEvent('keydown', {
      key: 'Escape',
      bubbles: true,
      cancelable: true,
    }),
  )
  await flush()
}

afterEach(async () => {
  // The request queue is module state; a pending promise would leak.
  if (pending.value) settle(false)
  await flush(1)
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe('UiConfirm', () => {
  it('renders nothing until something asks', async () => {
    mountHost()
    await flush()

    expect(panel()).toBeNull()
  })

  it('shows the question and the consequence', async () => {
    mountHost()
    await flush()

    void confirmAction({
      title: 'Delete project “Foo”?',
      body: 'Uploads keep their names.',
    })
    await flush()

    const el = panel()!
    expect(textOf(el.getAttribute('aria-labelledby'))).toBe(
      'Delete project “Foo”?',
    )
    expect(textOf(el.getAttribute('aria-describedby'))).toBe(
      'Uploads keep their names.',
    )
  })

  it('is an alertdialog, not a plain dialog', async () => {
    mountHost()
    await flush()

    void confirmAction({ title: 'Sure?' })
    await flush()

    expect(panel()!.getAttribute('role')).toBe('alertdialog')
  })

  it('resolves true when confirmed', async () => {
    mountHost()
    await flush()

    const answer = confirmAction({ title: 'Sure?', confirmLabel: 'Delete it' })
    await flush()
    byText(/Delete it/).click()

    await expect(answer).resolves.toBe(true)
  })

  it('resolves false when cancelled', async () => {
    mountHost()
    await flush()

    const answer = confirmAction({ title: 'Sure?' })
    await flush()
    byText(/Cancel/).click()

    await expect(answer).resolves.toBe(false)
  })

  it('resolves false on Escape — dismissing takes the safe direction', async () => {
    mountHost()
    await flush()

    const answer = confirmAction({ title: 'Sure?' })
    await flush()
    await pressEscape()

    await expect(answer).resolves.toBe(false)
  })

  it('closes once answered', async () => {
    mountHost()
    await flush()

    void confirmAction({ title: 'Sure?' })
    await flush()
    expect(panel()).not.toBeNull()

    byText(/Cancel/).click()
    await flush()

    expect(panel()).toBeNull()
  })

  it('names the action instead of saying OK', async () => {
    mountHost()
    await flush()

    void confirmAction({
      title: 'Sure?',
      confirmLabel: 'Revoke access',
      cancelLabel: 'Keep it',
    })
    await flush()

    expect(buttons().map((b) => b.textContent?.trim())).toEqual([
      'Keep it',
      'Revoke access',
    ])
  })

  it('styles the confirm button as destructive by default', async () => {
    mountHost()
    await flush()

    void confirmAction({ title: 'Sure?' })
    await flush()

    expect(buttons()[1].classList.contains('btn-danger')).toBe(true)
  })

  it('uses the primary style when the action is not destructive', async () => {
    mountHost()
    await flush()

    void confirmAction({ title: 'Proceed?', danger: false })
    await flush()

    expect(buttons()[1].classList.contains('btn-primary')).toBe(true)
    expect(buttons()[1].classList.contains('btn-danger')).toBe(false)
  })

  it('puts Cancel first so the focus trap makes Enter harmless', async () => {
    mountHost()
    await flush()

    void confirmAction({ title: 'Sure?', confirmLabel: 'Delete' })
    await flush()

    // Cancel leads in the DOM, so the trap's initial focus cannot land on the
    // destructive action.
    expect(buttons()[0].textContent?.trim()).toBe('Cancel')
    expect(document.activeElement).not.toBe(byText(/Delete/))
  })

  it('omits the description element when no consequence is given', async () => {
    mountHost()
    await flush()

    void confirmAction({ title: 'Sure?' })
    await flush()

    expect(panel()!.getAttribute('aria-describedby')).toBeNull()
  })

  it('cancels an in-flight request rather than leaving its promise dangling', async () => {
    mountHost()
    await flush()

    const first = confirmAction({ title: 'First?' })
    await flush()
    const second = confirmAction({ title: 'Second?' })
    await flush()

    await expect(first).resolves.toBe(false)
    expect(textOf(panel()!.getAttribute('aria-labelledby'))).toBe('Second?')

    byText(/Confirm/).click()
    await expect(second).resolves.toBe(true)
  })

  it('does not resolve until the user answers', async () => {
    mountHost()
    await flush()

    let settled = false
    void confirmAction({ title: 'Sure?' }).then(() => (settled = true))
    await flush(5)

    expect(settled).toBe(false)
  })
})
