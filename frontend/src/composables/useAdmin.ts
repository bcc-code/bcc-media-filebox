import { ref, computed } from 'vue'

export interface Target {
  id: number
  name: string
  path: string
  formKey: string | null
  position: number
  createdAt: string
}

export interface Project {
  id: number
  name: string
  code: string
  createdAt: string
}

export interface SubEvent {
  id: number
  name: string
  code: string
}

export interface Arrangement {
  id: number
  name: string
  code: string
  createdAt: string
  subEvents: SubEvent[]
}

export interface Group {
  id: number
  name: string
  kind: 'builtin' | 'custom'
  description: string
  createdAt: string
  memberCount: number
  members: string[]
}

export interface Grant {
  id: number
  principalKind: 'user' | 'group'
  principalValue: string
  admin: boolean
  allTargets: boolean
  targetIds: number[]
  createdAt: string
}

export interface AdminUser {
  id: number
  provider: string
  email: string
  name: string
  role: string
  createdAt: string
  lastLoginAt: string
  uploads: number
  uploadsThisMonth: number
  totalBytes: number
  bytesThisMonth: number
  failures: number
  active: boolean
  groups: string[]
}

export interface RecentUpload {
  id: string
  filename: string
  size: number
  targetName: string
  when: string
}

export interface AdminUserDetail extends AdminUser {
  recent: RecentUpload[]
  directGrants: Grant[]
  effectiveTargetIds: number[]
  effectiveAll: boolean
}

const targets = ref<Target[]>([])
const projects = ref<Project[]>([])
const arrangements = ref<Arrangement[]>([])
const groups = ref<Group[]>([])
const grants = ref<Grant[]>([])
const users = ref<AdminUser[]>([])
const loading = ref(false)
const lastError = ref<string | null>(null)

const toast = ref<{ text: string; danger?: boolean } | null>(null)
let toastTimer: number | null = null
function showToast(text: string, danger = false) {
  toast.value = { text, danger }
  if (toastTimer) window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => {
    toast.value = null
  }, 2400)
}

async function jsonFetch<T>(input: string, init?: RequestInit): Promise<T> {
  const res = await fetch(input, {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    let msg = `Request failed (${res.status})`
    try {
      const body = await res.json()
      if (body?.error) msg = body.error
    } catch {
      /* ignore */
    }
    throw new Error(msg)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

async function loadAll() {
  loading.value = true
  lastError.value = null
  try {
    const [t, p, ar, g, gr, u] = await Promise.all([
      jsonFetch<Target[]>('/api/admin/targets'),
      jsonFetch<Project[]>('/api/admin/projects'),
      jsonFetch<Arrangement[]>('/api/admin/arrangements'),
      jsonFetch<Group[]>('/api/admin/groups'),
      jsonFetch<Grant[]>('/api/admin/grants'),
      jsonFetch<AdminUser[]>('/api/admin/users'),
    ])
    targets.value = t
    projects.value = p
    arrangements.value = ar
    groups.value = g
    grants.value = gr
    users.value = u
  } catch (e) {
    lastError.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

// Targets ---------------------------------------------------------------

async function createTarget(body: { name: string; path: string; formKey?: string | null }) {
  try {
    const t = await jsonFetch<Target>('/api/admin/targets', { method: 'POST', body: JSON.stringify(body) })
    targets.value.push(t)
    showToast(`Added target “${t.name}”`)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function updateTarget(id: number, body: { name: string; path: string; formKey?: string | null }) {
  try {
    const t = await jsonFetch<Target>(`/api/admin/targets/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
    const i = targets.value.findIndex(x => x.id === id)
    if (i >= 0) targets.value[i] = t
    showToast(`Saved “${t.name}”`)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function deleteTarget(id: number) {
  const t = targets.value.find(x => x.id === id)
  try {
    await jsonFetch(`/api/admin/targets/${id}`, { method: 'DELETE' })
    targets.value = targets.value.filter(x => x.id !== id)
    // The server cascade-clears grant_targets but we hold a stale local copy.
    grants.value.forEach(g => {
      g.targetIds = g.targetIds.filter(tid => tid !== id)
    })
    showToast(`Removed “${t?.name ?? 'target'}”`, true)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function duplicateTarget(t: Target) {
  await createTarget({ name: `${t.name} (copy)`, path: t.path, formKey: t.formKey })
}

async function reorderTargets(ids: number[]) {
  const prev = targets.value
  // Optimistic: reorder locally so the row jumps immediately under the cursor.
  const byId = new Map(prev.map(t => [t.id, t]))
  targets.value = ids.map(id => byId.get(id)).filter((t): t is Target => !!t)
  try {
    const t = await jsonFetch<Target[]>('/api/admin/targets/reorder', {
      method: 'POST',
      body: JSON.stringify({ ids }),
    })
    targets.value = t
    showToast('Reordered targets')
  } catch (e) {
    targets.value = prev
    showToast((e as Error).message, true)
  }
}

// Projects --------------------------------------------------------------

async function createProject(body: { name: string; code: string }) {
  try {
    const p = await jsonFetch<Project>('/api/admin/projects', { method: 'POST', body: JSON.stringify(body) })
    projects.value.push(p)
    showToast(`Added project “${p.name}”`)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function updateProject(id: number, body: { name: string; code: string }) {
  try {
    const p = await jsonFetch<Project>(`/api/admin/projects/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
    const i = projects.value.findIndex(x => x.id === id)
    if (i >= 0) projects.value[i] = p
    showToast(`Saved “${p.name}”`)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function deleteProject(id: number) {
  const p = projects.value.find(x => x.id === id)
  try {
    await jsonFetch(`/api/admin/projects/${id}`, { method: 'DELETE' })
    projects.value = projects.value.filter(x => x.id !== id)
    showToast(`Removed “${p?.name ?? 'project'}”`, true)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

// Arrangements & sub-events --------------------------------------------

async function createArrangement(body: { name: string; code: string }) {
  try {
    const a = await jsonFetch<Arrangement>('/api/admin/arrangements', { method: 'POST', body: JSON.stringify(body) })
    arrangements.value.push({ ...a, subEvents: a.subEvents ?? [] })
    showToast(`Added arrangement “${a.name}”`)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function updateArrangement(id: number, body: { name: string; code: string }) {
  try {
    const a = await jsonFetch<Arrangement>(`/api/admin/arrangements/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
    const i = arrangements.value.findIndex(x => x.id === id)
    // The PATCH response carries no sub-events; keep the ones we already hold.
    if (i >= 0) arrangements.value[i] = { ...a, subEvents: arrangements.value[i].subEvents }
    showToast(`Saved “${a.name}”`)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function deleteArrangement(id: number) {
  const a = arrangements.value.find(x => x.id === id)
  try {
    await jsonFetch(`/api/admin/arrangements/${id}`, { method: 'DELETE' })
    arrangements.value = arrangements.value.filter(x => x.id !== id)
    showToast(`Removed “${a?.name ?? 'arrangement'}”`, true)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function createSubEvent(arrangementId: number, body: { name: string; code: string }) {
  try {
    const s = await jsonFetch<SubEvent>(`/api/admin/arrangements/${arrangementId}/sub-events`, { method: 'POST', body: JSON.stringify(body) })
    const arr = arrangements.value.find(x => x.id === arrangementId)
    if (arr) arr.subEvents.push(s)
    showToast(`Added sub event “${s.name}”`)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function updateSubEvent(arrangementId: number, id: number, body: { name: string; code: string }) {
  try {
    const s = await jsonFetch<SubEvent>(`/api/admin/sub-events/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
    const arr = arrangements.value.find(x => x.id === arrangementId)
    if (arr) {
      const i = arr.subEvents.findIndex(x => x.id === id)
      if (i >= 0) arr.subEvents[i] = s
    }
    showToast(`Saved “${s.name}”`)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function deleteSubEvent(arrangementId: number, id: number) {
  try {
    await jsonFetch(`/api/admin/sub-events/${id}`, { method: 'DELETE' })
    const arr = arrangements.value.find(x => x.id === arrangementId)
    if (arr) arr.subEvents = arr.subEvents.filter(x => x.id !== id)
    showToast('Removed sub event', true)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

// Groups ----------------------------------------------------------------

async function createGroup(body: { name: string; description: string; members: string[] }) {
  try {
    const g = await jsonFetch<Group>('/api/admin/groups', { method: 'POST', body: JSON.stringify(body) })
    groups.value.push(g)
    showToast(`Created group “${g.name}”`)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function updateGroup(id: number, body: { name: string; description: string; members: string[] }) {
  try {
    const g = await jsonFetch<Group>(`/api/admin/groups/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
    const i = groups.value.findIndex(x => x.id === id)
    if (i >= 0) groups.value[i] = g
    // Renames cascade to grants on the backend; reflect that locally too.
    showToast(`Saved group “${g.name}”`)
    // Reload grants since principal_value may have changed for several rows.
    grants.value = await jsonFetch<Grant[]>('/api/admin/grants')
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function deleteGroup(id: number) {
  const g = groups.value.find(x => x.id === id)
  try {
    await jsonFetch(`/api/admin/groups/${id}`, { method: 'DELETE' })
    groups.value = groups.value.filter(x => x.id !== id)
    if (g) grants.value = grants.value.filter(gr => !(gr.principalKind === 'group' && gr.principalValue === g.name))
    showToast(`Removed group “${g?.name ?? 'group'}”`, true)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

// Grants ----------------------------------------------------------------

interface GrantWrite {
  principalKind: 'user' | 'group'
  principalValue: string
  admin: boolean
  allTargets: boolean
  targetIds: number[]
}

async function createGrant(body: GrantWrite) {
  try {
    const g = await jsonFetch<Grant>('/api/admin/grants', { method: 'POST', body: JSON.stringify(body) })
    grants.value.push(g)
    showToast(`Granted access to ${g.principalValue}`)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function updateGrant(id: number, body: GrantWrite) {
  try {
    const g = await jsonFetch<Grant>(`/api/admin/grants/${id}`, { method: 'PATCH', body: JSON.stringify(body) })
    const i = grants.value.findIndex(x => x.id === id)
    if (i >= 0) grants.value[i] = g
    showToast(`Saved access for ${g.principalValue}`)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

async function deleteGrant(id: number) {
  const g = grants.value.find(x => x.id === id)
  try {
    await jsonFetch(`/api/admin/grants/${id}`, { method: 'DELETE' })
    grants.value = grants.value.filter(x => x.id !== id)
    showToast(`Removed access for ${g?.principalValue ?? 'principal'}`, true)
  } catch (e) {
    showToast((e as Error).message, true)
  }
}

// Users -----------------------------------------------------------------

async function loadUserDetail(id: number): Promise<AdminUserDetail | null> {
  try {
    return await jsonFetch<AdminUserDetail>(`/api/admin/users/${id}`)
  } catch (e) {
    showToast((e as Error).message, true)
    return null
  }
}

// Derived ---------------------------------------------------------------

const builtinGroups = computed(() => groups.value.filter(g => g.kind === 'builtin'))
const customGroups = computed(() => groups.value.filter(g => g.kind === 'custom'))

export function useAdmin() {
  return {
    targets,
    projects,
    arrangements,
    groups,
    grants,
    users,
    builtinGroups,
    customGroups,
    loading,
    lastError,
    toast,
    showToast,
    loadAll,
    createTarget,
    updateTarget,
    deleteTarget,
    duplicateTarget,
    reorderTargets,
    createProject,
    updateProject,
    deleteProject,
    createArrangement,
    updateArrangement,
    deleteArrangement,
    createSubEvent,
    updateSubEvent,
    deleteSubEvent,
    createGroup,
    updateGroup,
    deleteGroup,
    createGrant,
    updateGrant,
    deleteGrant,
    loadUserDetail,
  }
}
