<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import { useAdmin, type Target, type Group, type Grant, type AdminUser, type AdminUserDetail } from '../composables/useAdmin'
import { initials } from '../composables/adminHelpers'
import TargetsTab from '../components/admin/TargetsTab.vue'
import UsersTab from '../components/admin/UsersTab.vue'
import GroupsTab from '../components/admin/GroupsTab.vue'
import AccessTab from '../components/admin/AccessTab.vue'
import UserDrawer from '../components/admin/UserDrawer.vue'
import TargetModal from '../components/admin/TargetModal.vue'
import GroupModal from '../components/admin/GroupModal.vue'
import GrantModal from '../components/admin/GrantModal.vue'
import AppLogo from '../components/AppLogo.vue'
import '../assets/admin.css'

type Tab = 'targets' | 'users' | 'groups' | 'access'

const router = useRouter()
const { state } = useAuth()
const admin = useAdmin()

const tab = ref<Tab>('targets')
const editingTarget = ref<Target | null>(null)
const targetModalOpen = ref(false)
const editingGroup = ref<Group | null>(null)
const groupModalOpen = ref(false)
const editingGrant = ref<Grant | null>(null)
const grantPrefillEmail = ref<string | undefined>(undefined)
const grantModalOpen = ref(false)
const selectedUser = ref<AdminUserDetail | null>(null)

onMounted(async () => {
  // Bounce non-admins back to the home page; we still render the gate so
  // a flash of admin content isn't possible during the fetch.
  if (!state.authenticated || state.role !== 'admin') {
    router.replace('/')
    return
  }
  await admin.loadAll()
})

const isAdmin = computed(() => state.authenticated && state.role === 'admin')

// ---- target modal ----
function openNewTarget() {
  editingTarget.value = null
  targetModalOpen.value = true
}
async function saveTarget(body: { name: string; path: string }) {
  if (editingTarget.value) {
    await admin.updateTarget(editingTarget.value.id, body)
  } else {
    await admin.createTarget(body)
  }
  targetModalOpen.value = false
}

// ---- group modal ----
function openNewGroup() {
  editingGroup.value = null
  groupModalOpen.value = true
}
function openEditGroup(g: Group) {
  editingGroup.value = g
  groupModalOpen.value = true
}
async function saveGroup(body: { name: string; description: string; members: string[] }) {
  if (editingGroup.value) {
    await admin.updateGroup(editingGroup.value.id, body)
  } else {
    await admin.createGroup(body)
  }
  groupModalOpen.value = false
}

// ---- grant modal ----
function openNewGrant() {
  editingGrant.value = null
  grantPrefillEmail.value = undefined
  grantModalOpen.value = true
}
function openEditGrant(g: Grant) {
  editingGrant.value = g
  grantPrefillEmail.value = undefined
  grantModalOpen.value = true
}
async function saveGrant(body: {
  principalKind: 'user' | 'group'
  principalValue: string
  admin: boolean
  allTargets: boolean
  targetIds: number[]
}) {
  if (editingGrant.value) {
    await admin.updateGrant(editingGrant.value.id, body)
  } else {
    await admin.createGrant(body)
  }
  grantModalOpen.value = false
}

function goToGroups() {
  grantModalOpen.value = false
  tab.value = 'groups'
  openNewGroup()
}

// ---- user drawer ----
async function openUser(u: AdminUser) {
  const detail = await admin.loadUserDetail(u.id)
  if (detail) selectedUser.value = detail
}
function closeUser() {
  selectedUser.value = null
}
async function editAccessFor(u: AdminUserDetail) {
  const existing = admin.grants.value.find(g => g.principalKind === 'user' && g.principalValue.toLowerCase() === u.email.toLowerCase())
  selectedUser.value = null
  if (existing) {
    openEditGrant(existing)
  } else {
    editingGrant.value = null
    grantPrefillEmail.value = u.email
    grantModalOpen.value = true
  }
}
async function revokeUser(u: AdminUserDetail) {
  if (!confirm(`Revoke all access for ${u.name || u.email}? They will keep their upload history but won't be able to upload anymore.`)) return
  const matches = admin.grants.value.filter(g => g.principalKind === 'user' && g.principalValue.toLowerCase() === u.email.toLowerCase())
  for (const m of matches) await admin.deleteGrant(m.id)
  closeUser()
}
</script>

<template>
  <div v-if="isAdmin" class="admin-root">
    <div class="topbar">
      <div class="brand">
        <AppLogo style="width: 22px; height: 22px" />
        <span class="name">FileBox</span>
      </div>
      <div class="crumb"><span>filebox</span><span class="sep">/</span><span class="here">Admin</span></div>
      <div class="spacer"></div>
      <router-link to="/" class="btn btn-ghost btn-sm" style="text-decoration:none">← Back to FileBox</router-link>
      <div class="me">
        <div class="avatar">{{ initials(state.name || state.email) }}</div>
        <span>{{ state.email }}</span>
        <span class="role">Admin</span>
      </div>
    </div>

    <div class="tabs">
      <button :class="{ active: tab === 'targets' }" @click="tab = 'targets'">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
        Upload targets <span class="count">{{ admin.targets.value.length }}</span>
      </button>
      <button :class="{ active: tab === 'users' }" @click="tab = 'users'">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="3.2"/><path d="M5 20c1.5-3.6 4-5 7-5s5.5 1.4 7 5"/></svg>
        Users <span class="count">{{ admin.users.value.length }}</span>
      </button>
      <button :class="{ active: tab === 'groups' }" @click="tab = 'groups'">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="9" cy="8" r="3"/><circle cx="17" cy="9" r="2.5"/><path d="M3 19c1-3 3.5-4.5 6-4.5s5 1.5 6 4.5"/><path d="M15 19c.5-2 2-3 3.5-3s3 1 3.5 3"/></svg>
        Groups <span class="count">{{ admin.groups.value.length }}</span>
      </button>
      <button :class="{ active: tab === 'access' }" @click="tab = 'access'">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2 4 5v6c0 5 3.5 8 8 9 4.5-1 8-4 8-9V5l-8-3z"/><path d="M9 12l2 2 4-4"/></svg>
        Access <span class="count">{{ admin.grants.value.length }}</span>
      </button>
    </div>

    <div class="page">
      <TargetsTab v-if="tab === 'targets'" @new="openNewTarget" @edit="(t) => admin.updateTarget(t.id, { name: t.name, path: t.path })" />
      <UsersTab v-else-if="tab === 'users'" @open="openUser" />
      <GroupsTab v-else-if="tab === 'groups'" @new="openNewGroup" @edit="openEditGroup" />
      <AccessTab v-else @new="openNewGrant" @edit="openEditGrant" />
    </div>

    <TargetModal
      v-if="targetModalOpen"
      :target="editingTarget"
      @cancel="targetModalOpen = false"
      @save="saveTarget"
    />

    <GroupModal
      v-if="groupModalOpen"
      :group="editingGroup"
      @cancel="groupModalOpen = false"
      @save="saveGroup"
    />

    <GrantModal
      v-if="grantModalOpen"
      :grant="editingGrant"
      :prefill-email="grantPrefillEmail"
      :targets="admin.targets.value"
      :builtin-groups="admin.builtinGroups.value"
      :custom-groups="admin.customGroups.value"
      @cancel="grantModalOpen = false"
      @save="saveGrant"
      @go-to-groups="goToGroups"
    />

    <UserDrawer
      v-if="selectedUser"
      :user="selectedUser"
      @close="closeUser"
      @edit-access="editAccessFor"
      @revoke="revokeUser"
    />

    <div v-if="admin.toast.value" class="toast fb-fade" :class="{ danger: admin.toast.value.danger }">
      <span class="dot"></span><span>{{ admin.toast.value.text }}</span>
    </div>
  </div>
</template>
