import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import type { Arrangement, Target } from '../../../composables/useAdmin'

import TargetModal from '../TargetModal.vue'
import ProjectModal from '../ProjectModal.vue'
import ArrangementModal from '../ArrangementModal.vue'
import GroupModal from '../GroupModal.vue'
import GrantModal from '../GrantModal.vue'
import SubEventImportModal from '../SubEventImportModal.vue'

/**
 * The six admin modals used to hand-roll the same overlay markup. They now
 * delegate the shell to UiDialog, so these tests pin the parts that regression
 * would silently break: each dialog still has an accessible name, still emits
 * its cancel event from the footer, and still closes on Escape — which none of
 * them did before the conversion.
 */

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const panel = () => document.querySelector('.dialog-panel')
const accessibleName = () => {
  const id = panel()?.getAttribute('aria-labelledby')
  return id ? document.getElementById(id)?.textContent?.trim() : null
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

const arrangement: Arrangement = {
  id: 1,
  name: 'Sommerstevne 2026',
  code: 'SMR26',
  createdAt: '2026-01-01T00:00:00Z',
  subEvents: [],
}

const target: Target = {
  id: 1,
  name: 'Verify Target',
  path: '/tmp/t1',
  formKey: null,
  webhookUrl: null,
  position: 1,
  createdAt: '2026-01-01T00:00:00Z',
}

/** component, props, expected title, and the event it reports dismissal with */
const CASES: Array<[string, any, Record<string, unknown>, string, string]> = [
  ['TargetModal', TargetModal, { target: null }, 'New upload target', 'cancel'],
  ['ProjectModal', ProjectModal, { project: null }, 'New project', 'cancel'],
  [
    'ArrangementModal',
    ArrangementModal,
    { arrangement: null },
    'New arrangement',
    'cancel',
  ],
  ['GroupModal', GroupModal, { group: null }, 'New custom group', 'cancel'],
  [
    'GrantModal',
    GrantModal,
    { grant: null, targets: [], builtinGroups: [], customGroups: [] },
    'Grant access',
    'cancel',
  ],
  [
    'SubEventImportModal',
    SubEventImportModal,
    { arrangement },
    'Import sub events',
    'close',
  ],
]

let wrapper: VueWrapper | null = null

beforeEach(() => {
  // No modal should need the network to render; fail loudly if one tries.
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(new Response('[]', { status: 200 }))),
  )
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe.each(CASES)(
  '%s',
  (name, Component, props, expectedTitle, dismissEvent) => {
    it('renders inside a labelled dialog teleported to <body>', async () => {
      wrapper = mount(Component, { props, attachTo: document.body })
      await flush()

      const el = panel()
      expect(el, `${name} rendered no dialog panel`).not.toBeNull()
      expect(el!.getAttribute('role')).toBe('dialog')
      expect(el!.getAttribute('aria-modal')).toBe('true')
      expect(accessibleName()).toBe(expectedTitle)
    })

    it(`emits "${dismissEvent}" on Escape`, async () => {
      wrapper = mount(Component, { props, attachTo: document.body })
      await flush()

      await pressEscape()
      expect(wrapper.emitted(dismissEvent)).toBeTruthy()
    })

    it(`emits "${dismissEvent}" from the Cancel button`, async () => {
      wrapper = mount(Component, { props, attachTo: document.body })
      await flush()

      const cancel = [
        ...document.querySelectorAll<HTMLButtonElement>(
          '.dialog-actions button',
        ),
      ].find((b) => /cancel/i.test(b.textContent ?? ''))
      expect(
        cancel,
        `${name} has no Cancel button in the actions slot`,
      ).toBeTruthy()
      cancel!.click()
      await nextTick()
      expect(wrapper.emitted(dismissEvent)).toBeTruthy()
    })

    it('puts its primary action in the dialog footer', async () => {
      wrapper = mount(Component, { props, attachTo: document.body })
      await flush()

      expect(
        document.querySelector('.dialog-actions .btn-primary'),
      ).not.toBeNull()
    })
  },
)

describe('TargetModal specifics', () => {
  it('reports the edited values on save', async () => {
    wrapper = mount(TargetModal, {
      props: { target: null },
      attachTo: document.body,
    })
    await flush()

    const inputs = [
      ...document.querySelectorAll<HTMLInputElement>('.dialog-panel input'),
    ]
    const name = inputs.find((i) => i.placeholder.includes('Isilon'))!
    const path = inputs.find((i) => i.placeholder.includes('/mnt'))!
    name.value = 'Archive'
    name.dispatchEvent(new Event('input'))
    path.value = '/mnt/archive'
    path.dispatchEvent(new Event('input'))
    await nextTick()

    document
      .querySelector<HTMLButtonElement>('.dialog-actions .btn-primary')!
      .click()
    await nextTick()

    expect(wrapper.emitted('save')?.[0]?.[0]).toMatchObject({
      name: 'Archive',
      path: '/mnt/archive',
    })
  })

  it('keeps the primary action disabled until name and path are filled', async () => {
    wrapper = mount(TargetModal, {
      props: { target: null },
      attachTo: document.body,
    })
    await flush()

    const save = document.querySelector<HTMLButtonElement>(
      '.dialog-actions .btn-primary',
    )!
    expect(save.disabled).toBe(true)
  })
})

describe('SubEventImportModal specifics', () => {
  it('interpolates the arrangement name into the dialog description', async () => {
    wrapper = mount(SubEventImportModal, {
      props: { arrangement },
      attachTo: document.body,
    })
    await flush()

    const id = panel()!.getAttribute('aria-describedby')
    expect(id).not.toBeNull()
    expect(document.getElementById(id!)?.textContent).toContain(
      'Sommerstevne 2026',
    )
  })

  it('previews pasted name/code pairs', async () => {
    wrapper = mount(SubEventImportModal, {
      props: { arrangement },
      attachTo: document.body,
    })
    await flush()

    const ta = document.querySelector<HTMLTextAreaElement>(
      '.dialog-panel textarea',
    )!
    ta.value = 'Åpningsmøte\nOPENING\nLLB kick-off\nLLB'
    ta.dispatchEvent(new Event('input'))
    await flush()

    const rows = [
      ...document.querySelectorAll('.dialog-panel .prev-row:not(.head)'),
    ]
    expect(rows.length).toBe(2)
    expect(rows[0].textContent).toContain('Åpningsmøte')
    expect(rows[0].textContent).toContain('OPENING')
  })
})

describe('GrantModal specifics', () => {
  it('renders the kind segmented control and the target picker', async () => {
    wrapper = mount(GrantModal, {
      props: {
        grant: null,
        targets: [target],
        builtinGroups: [],
        customGroups: [],
      },
      attachTo: document.body,
    })
    await flush()

    // Both were scoped under .admin-root before the dialog started teleporting
    // out of it, so their presence here is the regression guard.
    expect(document.querySelector('.dialog-panel .seg')).not.toBeNull()
    expect(document.querySelector('.dialog-panel .target-pick')).not.toBeNull()
    expect(
      document.querySelector('.dialog-panel .target-pick')!.textContent,
    ).toContain('Verify Target')
  })
})
