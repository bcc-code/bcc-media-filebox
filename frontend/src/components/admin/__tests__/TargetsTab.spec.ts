import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import type { Grant, Target } from '../../../composables/useAdmin'

const deleteTarget = vi.fn()
const targets = ref<Target[]>([])
const grants = ref<Grant[]>([])

vi.mock('../../../composables/useAdmin', () => ({
  useAdmin: () => ({
    targets,
    grants,
    deleteTarget,
    duplicateTarget: vi.fn(),
    reorderTargets: vi.fn(),
    updateTarget: vi.fn(),
  }),
}))

import TargetsTab from '../TargetsTab.vue'
import { pending, settle } from '../../../composables/useConfirm'

let wrapper: VueWrapper | null = null

async function flush(frames = 2) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const target = (id: number, name: string): Target => ({
  id,
  name,
  path: `/mnt/${name}`,
  formKey: null,
  webhookUrl: null,
  position: id,
  createdAt: '2026-01-01T00:00:00Z',
})

const grant = (id: number, targetIds: number[]): Grant =>
  ({
    id,
    principalKind: 'user',
    principalValue: `u${id}@bcc.no`,
    admin: false,
    allTargets: false,
    targetIds,
    createdAt: '2026-01-01T00:00:00Z',
  }) as unknown as Grant

const deleteButton = () =>
  wrapper!.findAll('button').find((b) => b.text() === 'Delete')!

beforeEach(() => {
  deleteTarget.mockClear()
  targets.value = [target(1, 'Isilon'), target(2, 'Archive')]
  grants.value = []
  if (pending.value) settle(false)
})

afterEach(async () => {
  if (pending.value) settle(false)
  await flush()
  wrapper?.unmount()
  wrapper = null
})

describe('TargetsTab delete', () => {
  it('asks before deleting, instead of firing straight away', async () => {
    wrapper = mount(TargetsTab)
    await flush()

    await deleteButton().trigger('click')
    await flush()

    // This button used to call deleteTarget() directly on click — the most
    // destructive action in the admin UI, with no guard at all.
    expect(deleteTarget).not.toHaveBeenCalled()
    expect(pending.value?.title).toContain('Isilon')
  })

  it('deletes once confirmed', async () => {
    wrapper = mount(TargetsTab)
    await flush()

    await deleteButton().trigger('click')
    await flush()
    settle(true)
    await flush()

    expect(deleteTarget).toHaveBeenCalledWith(1)
  })

  it('leaves the target alone when cancelled', async () => {
    wrapper = mount(TargetsTab)
    await flush()

    await deleteButton().trigger('click')
    await flush()
    settle(false)
    await flush()

    expect(deleteTarget).not.toHaveBeenCalled()
  })

  it('says the folder and its files are untouched', async () => {
    wrapper = mount(TargetsTab)
    await flush()

    await deleteButton().trigger('click')
    await flush()

    expect(pending.value?.body).toContain('untouched')
  })

  it('counts the grants that will lose access', async () => {
    grants.value = [grant(1, [1]), grant(2, [1, 2]), grant(3, [2])]
    wrapper = mount(TargetsTab)
    await flush()

    await deleteButton().trigger('click')
    await flush()

    expect(pending.value?.body).toContain('2 grants will lose access')
  })

  it('does not mention grants when none reference the target', async () => {
    grants.value = [grant(1, [2])]
    wrapper = mount(TargetsTab)
    await flush()

    await deleteButton().trigger('click')
    await flush()

    expect(pending.value?.body).not.toContain('lose access')
  })

  it('ignores blanket grants when counting, since they do not name the target', async () => {
    grants.value = [
      { ...grant(1, []), allTargets: true } as Grant,
      { ...grant(2, []), admin: true } as Grant,
    ]
    wrapper = mount(TargetsTab)
    await flush()

    await deleteButton().trigger('click')
    await flush()

    expect(pending.value?.body).not.toContain('lose access')
  })
})
