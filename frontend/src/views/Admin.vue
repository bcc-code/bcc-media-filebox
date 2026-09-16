<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import {
  useAdmin,
  type Target,
  type Project,
  type Arrangement,
  type Group,
  type Grant,
  type AdminUser,
  type AdminUserDetail,
} from '../composables/useAdmin'
import { initials } from '../composables/adminHelpers'
import TargetsTab from '../components/admin/TargetsTab.vue'
import ProjectsTab from '../components/admin/ProjectsTab.vue'
import ArrangementsTab from '../components/admin/ArrangementsTab.vue'
import UploadsTab from '../components/admin/UploadsTab.vue'
import UsersTab from '../components/admin/UsersTab.vue'
import GroupsTab from '../components/admin/GroupsTab.vue'
import AccessTab from '../components/admin/AccessTab.vue'
import UserDrawer from '../components/admin/UserDrawer.vue'
import TargetModal from '../components/admin/TargetModal.vue'
import ProjectModal from '../components/admin/ProjectModal.vue'
import ArrangementModal from '../components/admin/ArrangementModal.vue'
import GroupModal from '../components/admin/GroupModal.vue'
import GrantModal from '../components/admin/GrantModal.vue'
import AppLogo from '../components/AppLogo.vue'
import '../assets/admin.css'
import UiTabs, { type UiTabEntry } from '../components/ui/UiTabs.vue'
import { confirmAction } from '../composables/useConfirm'
import UiButton from '../components/ui/UiButton.vue'

type Tab =
  | 'targets'
  | 'projects'
  | 'arrangements'
  | 'uploads'
  | 'users'
  | 'groups'
  | 'access'

const router = useRouter()
const { state } = useAuth()
const admin = useAdmin()

const tab = ref<Tab>('targets')

const adminTabs = computed<UiTabEntry[]>(() => [
  {
    value: 'targets',
    label: 'Upload targets',
    count: admin.targets.value.length,
  },
  { value: 'projects', label: 'Projects', count: admin.projects.value.length },
  {
    value: 'arrangements',
    label: 'Arrangements',
    count: admin.arrangements.value.length,
  },
  {
    value: 'uploads',
    label: 'Uploads',
    count: admin.adminUploads.value.length,
  },
  { value: 'users', label: 'Users', count: admin.users.value.length },
  { value: 'groups', label: 'Groups', count: admin.groups.value.length },
  { value: 'access', label: 'Access', count: admin.grants.value.length },
])
const editingTarget = ref<Target | null>(null)
const targetModalOpen = ref(false)
const editingProject = ref<Project | null>(null)
const projectModalOpen = ref(false)
const editingArrangement = ref<Arrangement | null>(null)
const arrangementModalOpen = ref(false)
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
function openEditTarget(t: Target) {
  editingTarget.value = t
  targetModalOpen.value = true
}

// ---- project modal ----
function openNewProject() {
  editingProject.value = null
  projectModalOpen.value = true
}
function openEditProject(p: Project) {
  editingProject.value = p
  projectModalOpen.value = true
}
async function saveProject(body: { name: string; code: string }) {
  if (editingProject.value) {
    await admin.updateProject(editingProject.value.id, body)
  } else {
    await admin.createProject(body)
  }
  projectModalOpen.value = false
}

// ---- arrangement modal ----
function openNewArrangement() {
  editingArrangement.value = null
  arrangementModalOpen.value = true
}
function openEditArrangement(a: Arrangement) {
  editingArrangement.value = a
  arrangementModalOpen.value = true
}
async function saveArrangement(body: { name: string; code: string }) {
  if (editingArrangement.value) {
    await admin.updateArrangement(editingArrangement.value.id, body)
  } else {
    await admin.createArrangement(body)
  }
  arrangementModalOpen.value = false
}
async function saveTarget(body: {
  name: string
  path: string
  formKey: string | null
  webhookUrl: string | null
}) {
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
async function saveGroup(body: {
  name: string
  description: string
  members: string[]
}) {
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
  const existing = admin.grants.value.find(
    (g) =>
      g.principalKind === 'user' &&
      g.principalValue.toLowerCase() === u.email.toLowerCase(),
  )
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
  const ok = await confirmAction({
    title: `Revoke all access for ${u.name || u.email}?`,
    body: "They keep their upload history but won't be able to upload anymore.",
    confirmLabel: 'Revoke access',
  })
  if (!ok) return
  const matches = admin.grants.value.filter(
    (g) =>
      g.principalKind === 'user' &&
      g.principalValue.toLowerCase() === u.email.toLowerCase(),
  )
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
      <div class="crumb">
        <span>filebox</span><span class="sep">/</span
        ><span class="here">Admin</span>
      </div>
      <div class="spacer"></div>
      <UiButton to="/" variant="ghost" size="sm">← Back to FileBox</UiButton>
      <div class="me">
        <div class="avatar">{{ initials(state.name || state.email) }}</div>
        <span>{{ state.email }}</span>
        <span class="role">Admin</span>
      </div>
    </div>

    <UiTabs
      v-model="tab"
      :tabs="adminTabs"
      list-class="tab-list-inset"
      panel-class="page"
    >
      <template #icon-targets>
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path
            d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"
          />
        </svg>
      </template>
      <template #targets>
        <TargetsTab
          @new="openNewTarget"
          @open="openEditTarget"
          @edit="
            (t) =>
              admin.updateTarget(t.id, {
                name: t.name,
                path: t.path,
                formKey: t.formKey,
                webhookUrl: t.webhookUrl,
              })
          "
        />
      </template>
      <template #icon-projects>
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path
            d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"
          />
        </svg>
      </template>
      <template #projects>
        <ProjectsTab @new="openNewProject" @edit="openEditProject" />
      </template>
      <template #icon-arrangements>
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <rect x="3" y="4" width="18" height="16" rx="2" />
          <path d="M3 9h18M9 9v11" />
        </svg>
      </template>
      <template #arrangements>
        <ArrangementsTab
          @new="openNewArrangement"
          @edit="openEditArrangement"
        />
      </template>
      <template #icon-uploads>
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M12 3v12M7 8l5-5 5 5" />
          <path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2" />
        </svg>
      </template>
      <template #uploads>
        <UploadsTab />
      </template>
      <template #icon-users>
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <circle cx="12" cy="8" r="3.2" />
          <path d="M5 20c1.5-3.6 4-5 7-5s5.5 1.4 7 5" />
        </svg>
      </template>
      <template #users>
        <UsersTab @open="openUser" />
      </template>
      <template #icon-groups>
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <circle cx="9" cy="8" r="3" />
          <circle cx="17" cy="9" r="2.5" />
          <path d="M3 19c1-3 3.5-4.5 6-4.5s5 1.5 6 4.5" />
          <path d="M15 19c.5-2 2-3 3.5-3s3 1 3.5 3" />
        </svg>
      </template>
      <template #groups>
        <GroupsTab @new="openNewGroup" @edit="openEditGroup" />
      </template>
      <template #icon-access>
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M12 2 4 5v6c0 5 3.5 8 8 9 4.5-1 8-4 8-9V5l-8-3z" />
          <path d="M9 12l2 2 4-4" />
        </svg>
      </template>
      <template #access>
        <AccessTab @new="openNewGrant" @edit="openEditGrant" />
      </template>
    </UiTabs>

    <TargetModal
      v-if="targetModalOpen"
      :target="editingTarget"
      @cancel="targetModalOpen = false"
      @save="saveTarget"
    />

    <ProjectModal
      v-if="projectModalOpen"
      :project="editingProject"
      @cancel="projectModalOpen = false"
      @save="saveProject"
    />

    <ArrangementModal
      v-if="arrangementModalOpen"
      :arrangement="editingArrangement"
      @cancel="arrangementModalOpen = false"
      @save="saveArrangement"
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
  </div>
</template>
