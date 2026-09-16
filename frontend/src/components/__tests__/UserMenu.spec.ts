import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick, reactive } from 'vue'

const authState = reactive({
  authenticated: false,
  name: '',
  email: '',
  provider: '',
  role: 'user',
})
const providers = reactive<{ id: string; displayName: string }[]>([])
const signIn = vi.fn()
const signOut = vi.fn()
const changeUser = vi.fn()
const push = vi.fn()
const route = reactive({ path: '/' })

vi.mock('../../composables/useAuth', () => ({
  useAuth: () => ({ state: authState, signIn, signOut, changeUser }),
}))
vi.mock('../../composables/useProviders', () => ({
  useProviders: () => providers,
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push }),
  useRoute: () => route,
}))

import UserMenu from '../UserMenu.vue'

async function flush(frames = 3) {
  for (let i = 0; i < frames; i++) {
    await new Promise((r) => requestAnimationFrame(() => r(null)))
    await new Promise((r) => setTimeout(r, 0))
    await nextTick()
  }
}

let wrapper: VueWrapper | null = null
const labels = () =>
  [...document.querySelectorAll('.menu-item')].map((e) => e.textContent?.trim())

async function open() {
  wrapper = mount(UserMenu, { attachTo: document.body })
  await flush()
  document.querySelector<HTMLButtonElement>('.menu-trigger-reset')!.click()
  await flush()
}

beforeEach(() => {
  Object.assign(authState, {
    authenticated: false,
    name: '',
    email: '',
    provider: '',
    role: 'user',
  })
  providers.splice(0, providers.length)
  route.path = '/'
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

describe('UserMenu', () => {
  it('offers each sign-in provider to a guest', async () => {
    providers.push(
      { id: 'bcc', displayName: 'BCC Login' },
      { id: 'azure', displayName: 'Azure AD' },
    )
    await open()

    expect(labels()).toEqual([
      'Sign in with BCC Login',
      'Sign in with Azure AD',
      'Change user',
    ])
    expect(document.querySelector('.menu-separator')).not.toBeNull()
  })

  it('shows the guest identity in the header and no Sign out', async () => {
    await open()

    expect(document.querySelector('.menu-header')?.textContent).toContain(
      'Guest',
    )
    expect(labels()).not.toContain('Sign out')
  })

  it('omits the separator when no providers are configured', async () => {
    await open()

    expect(document.querySelector('.menu-separator')).toBeNull()
  })

  it('signs in with the chosen provider', async () => {
    providers.push({ id: 'bcc', displayName: 'BCC Login' })
    await open()

    const item = [...document.querySelectorAll<HTMLElement>('.menu-item')].find(
      (e) => e.textContent?.includes('BCC Login'),
    )!
    item.click()
    await flush()

    expect(signIn).toHaveBeenCalledWith('bcc')
  })

  it('shows the signed-in identity and Sign out', async () => {
    Object.assign(authState, {
      authenticated: true,
      name: 'Verify Admin',
      email: 'admin@test.local',
      provider: 'bcc',
      role: 'user',
    })
    await open()

    const header = document.querySelector('.menu-header')!.textContent ?? ''
    expect(header).toContain('Verify Admin')
    expect(header).toContain('admin@test.local')
    expect(labels()).toEqual(['Change user', 'Sign out'])
  })

  it('offers Admin to an admin who is not already on the admin page', async () => {
    Object.assign(authState, {
      authenticated: true,
      role: 'admin',
      name: 'A',
      email: 'a@b.c',
    })
    await open()

    expect(labels()).toContain('Admin')
  })

  it('hides Admin while on the admin page', async () => {
    Object.assign(authState, {
      authenticated: true,
      role: 'admin',
      name: 'A',
      email: 'a@b.c',
    })
    route.path = '/admin'
    await open()

    expect(labels()).not.toContain('Admin')
  })

  it('navigates to /admin when Admin is chosen', async () => {
    Object.assign(authState, {
      authenticated: true,
      role: 'admin',
      name: 'A',
      email: 'a@b.c',
    })
    await open()

    const item = [...document.querySelectorAll<HTMLElement>('.menu-item')].find(
      (e) => e.textContent?.trim() === 'Admin',
    )!
    item.click()
    await flush()

    expect(push).toHaveBeenCalledWith('/admin')
  })

  it('signs out', async () => {
    Object.assign(authState, {
      authenticated: true,
      name: 'A',
      email: 'a@b.c',
      provider: 'bcc',
    })
    await open()

    const item = [...document.querySelectorAll<HTMLElement>('.menu-item')].find(
      (e) => e.textContent?.trim() === 'Sign out',
    )!
    item.click()
    await flush()

    expect(signOut).toHaveBeenCalledOnce()
  })

  it('shows the initial of the display name on the trigger', async () => {
    Object.assign(authState, {
      authenticated: true,
      name: 'Verify Admin',
      email: 'a@b.c',
    })
    wrapper = mount(UserMenu, { attachTo: document.body })
    await flush()

    expect(
      document.querySelector('.user-trigger .avatar')?.textContent?.trim(),
    ).toBe('V')
    expect(
      document
        .querySelector('.user-trigger .avatar')
        ?.classList.contains('guest'),
    ).toBe(false)
  })

  it('marks the guest avatar so it reads as having no account', async () => {
    wrapper = mount(UserMenu, { attachTo: document.body })
    await flush()

    const avatar = document.querySelector('.user-trigger .avatar')!
    expect(avatar.textContent?.trim()).toBe('G')
    expect(avatar.classList.contains('guest')).toBe(true)
  })
})
