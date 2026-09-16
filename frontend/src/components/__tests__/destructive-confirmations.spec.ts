import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import type { Arrangement, Grant } from '../../composables/useAdmin'

/**
 * Every destructive action in the app must ask first. These are integration
 * tests per call site rather than unit tests of UiConfirm: the regression that
 * matters is a button wired straight to its mutation, which is what all of
 * these were before.
 */

// ---- useAdmin ----
const deleteGrant = vi.fn()
const deleteSubEvent = vi.fn()
const grants = ref<Grant[]>([])
const arrangements = ref<Arrangement[]>([])

vi.mock('../../composables/useAdmin', () => ({
  useAdmin: () => ({
    grants,
    arrangements,
    targets: ref([]),
    groups: ref([]),
    deleteGrant,
    deleteSubEvent,
    deleteArrangement: vi.fn(),
    createSubEvent: vi.fn(),
    updateSubEvent: vi.fn(),
  }),
}))

// ---- usePackages ----
const revokePackage = vi.fn()
const dismissAccessRequest = vi.fn()
const packages = ref<any[]>([])

vi.mock('../../composables/usePackages', () => ({
  usePackages: () => ({
    packages,
    total: ref(packages.value.length),
    loading: ref(false),
    loadMorePackages: vi.fn(),
    revokePackage,
    extendPackage: vi.fn(),
    dismissAccessRequest,
    setNotifyOnDownload: vi.fn(),
    startPreparationPolling: vi.fn(),
    stopPreparationPolling: vi.fn(),
  }),
}))

import AccessTab from '../admin/AccessTab.vue'
import ArrangementsTab from '../admin/ArrangementsTab.vue'
import SentPackagesList from '../send/SentPackagesList.vue'
import { pending, settle } from '../../composables/useConfirm'

let wrapper: VueWrapper | null = null

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

const clickByText = async (text: string) => {
  const btn = wrapper!.findAll('button').find((b) => b.text().trim() === text)
  expect(btn, `no button labelled ${text}`).toBeTruthy()
  await btn!.trigger('click')
  await flush()
}

beforeEach(() => {
  vi.clearAllMocks()
  if (pending.value) settle(false)
  grants.value = []
  arrangements.value = []
  packages.value = []
})

afterEach(async () => {
  if (pending.value) settle(false)
  await flush(1)
  wrapper?.unmount()
  wrapper = null
})

describe('AccessTab — removing a grant', () => {
  const grant = (over: Partial<Grant> = {}): Grant =>
    ({
      id: 7,
      principalKind: 'user',
      principalValue: 'someone@bcc.no',
      admin: false,
      allTargets: true,
      targetIds: [],
      createdAt: '2026-01-01T00:00:00Z',
      ...over,
    }) as Grant

  it('asks before revoking access', async () => {
    grants.value = [grant()]
    wrapper = mount(AccessTab)
    await flush()

    await clickByText('Remove')

    expect(deleteGrant).not.toHaveBeenCalled()
    expect(pending.value?.title).toContain('someone@bcc.no')
  })

  it('removes once confirmed', async () => {
    grants.value = [grant()]
    wrapper = mount(AccessTab)
    await flush()

    await clickByText('Remove')
    settle(true)
    await flush()

    expect(deleteGrant).toHaveBeenCalledWith(7)
  })

  it('keeps the grant when cancelled', async () => {
    grants.value = [grant()]
    wrapper = mount(AccessTab)
    await flush()

    await clickByText('Remove')
    settle(false)
    await flush()

    expect(deleteGrant).not.toHaveBeenCalled()
  })

  it('says past uploads are kept', async () => {
    grants.value = [grant()]
    wrapper = mount(AccessTab)
    await flush()

    await clickByText('Remove')

    expect(pending.value?.body).toContain('kept')
  })

  it('speaks of members when the grant is for a group', async () => {
    grants.value = [
      grant({ principalKind: 'group', principalValue: 'Camera dept.' }),
    ]
    wrapper = mount(AccessTab)
    await flush()

    await clickByText('Remove')

    expect(pending.value?.body).toContain('members')
  })
})

describe('ArrangementsTab — deleting a sub event', () => {
  beforeEach(() => {
    arrangements.value = [
      {
        id: 1,
        name: 'Sommerstevne',
        code: 'SMR',
        createdAt: '2026-01-01T00:00:00Z',
        subEvents: [{ id: 5, name: 'Åpningsmøte', code: 'OPEN' }],
      } as Arrangement,
    ]
  })

  it('asks before deleting, naming the sub event', async () => {
    wrapper = mount(ArrangementsTab)
    await flush()
    // The rows live inside a collapsible; open it first.
    await wrapper.find('.chev').trigger('click')
    await flush()

    const del = wrapper
      .findAll('.sub-row button')
      .find((b) => b.text().trim() === 'Delete')
    expect(del, 'no Delete button in the sub-event row').toBeTruthy()
    await del!.trigger('click')
    await flush()

    expect(deleteSubEvent).not.toHaveBeenCalled()
    expect(pending.value?.title).toContain('Åpningsmøte')
  })

  it('deletes once confirmed', async () => {
    wrapper = mount(ArrangementsTab)
    await flush()
    await wrapper.find('.chev').trigger('click')
    await flush()

    const del = wrapper
      .findAll('.sub-row button')
      .find((b) => b.text().trim() === 'Delete')!
    await del.trigger('click')
    await flush()
    settle(true)
    await flush()

    expect(deleteSubEvent).toHaveBeenCalledWith(1, 5)
  })
})

describe('SentPackagesList — revoking a package', () => {
  const pkg = (over: Record<string, unknown> = {}) => ({
    packageId: 'pkg-1',
    name: 'Sommerstevne masters',
    status: 'active',
    isExpired: false,
    permanentlyExpired: false,
    isDownloadLimitHit: false,
    pendingRequests: [],
    artifactCount: 1,
    preparationStatus: 'ready',
    preparationProgress: 100,
    recipients: [],
    createdAt: '2026-01-01T00:00:00Z',
    expiresAt: '2027-01-01T00:00:00Z',
    filesDeletedAt: '2027-06-01T00:00:00Z',
    notifyOnDownload: false,
    ...over,
  })

  it('asks before revoking, naming the package', async () => {
    packages.value = [pkg()]
    wrapper = mount(SentPackagesList)
    await flush()

    await clickByText('Revoke')

    expect(revokePackage).not.toHaveBeenCalled()
    expect(pending.value?.title).toContain('Sommerstevne masters')
  })

  it('tells the author it can be reopened, because ExtendPackage clears revoked', async () => {
    packages.value = [pkg()]
    wrapper = mount(SentPackagesList)
    await flush()

    await clickByText('Revoke')

    expect(pending.value?.body).toContain('reopen')
  })

  it('revokes once confirmed', async () => {
    packages.value = [pkg()]
    wrapper = mount(SentPackagesList)
    await flush()

    await clickByText('Revoke')
    settle(true)
    await flush()

    expect(revokePackage).toHaveBeenCalledWith('pkg-1')
  })

  it('keeps the package live when cancelled', async () => {
    packages.value = [pkg()]
    wrapper = mount(SentPackagesList)
    await flush()

    await clickByText('Revoke')
    settle(false)
    await flush()

    expect(revokePackage).not.toHaveBeenCalled()
  })

  it('names the requester when dismissing a request', async () => {
    packages.value = [
      pkg({
        pendingRequests: [
          {
            id: 'req-9',
            email: 'asker@example.com',
            reason: 'expired',
            createdAt: '2026-01-02T00:00:00Z',
          },
        ],
      }),
    ]
    wrapper = mount(SentPackagesList)
    await flush()

    const x = wrapper.find('.ask-x')
    expect(x.exists()).toBe(true)
    await x.trigger('click')
    await flush()

    expect(dismissAccessRequest).not.toHaveBeenCalled()
    expect(pending.value?.title).toContain('asker@example.com')
    expect(pending.value?.body).toContain('not notified')
  })

  it('keeps the request when cancelled', async () => {
    // The only assertion that catches a dropped `if (!ok) return`: until the
    // confirm resolves, execution is parked at the await either way.
    packages.value = [
      pkg({
        pendingRequests: [
          {
            id: 'req-9',
            email: 'asker@example.com',
            reason: 'expired',
            createdAt: '2026-01-02T00:00:00Z',
          },
        ],
      }),
    ]
    wrapper = mount(SentPackagesList)
    await flush()

    await wrapper.find('.ask-x').trigger('click')
    await flush()
    settle(false)
    await flush()

    expect(dismissAccessRequest).not.toHaveBeenCalled()
  })

  it('dismisses once confirmed', async () => {
    packages.value = [
      pkg({
        pendingRequests: [
          {
            id: 'req-9',
            email: 'asker@example.com',
            reason: 'expired',
            createdAt: '2026-01-02T00:00:00Z',
          },
        ],
      }),
    ]
    wrapper = mount(SentPackagesList)
    await flush()

    await wrapper.find('.ask-x').trigger('click')
    await flush()
    settle(true)
    await flush()

    expect(dismissAccessRequest).toHaveBeenCalledWith('pkg-1', 'req-9')
  })
})
