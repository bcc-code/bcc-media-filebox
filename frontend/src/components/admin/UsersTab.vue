<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAdmin, type AdminUser } from '../../composables/useAdmin'
import {
  formatBytes,
  relTime,
  initials,
  providerLabel,
  providerColor,
  avatarBg,
} from '../../composables/adminHelpers'
import UiRadioGroup, { type UiRadioOption } from '../ui/UiRadioGroup.vue'

const emit = defineEmits<{ (e: 'open', u: AdminUser): void }>()

const { users, targets } = useAdmin()
const userQuery = ref('')
const userFilter = ref<'all' | 'admin' | 'guest'>('all')

const userFilterOptions: UiRadioOption<'all' | 'admin' | 'guest'>[] = [
  { value: 'all', label: 'All' },
  { value: 'admin', label: 'Admins' },
  { value: 'guest', label: 'Guests' },
]

const filteredUsers = computed(() => {
  const q = userQuery.value.trim().toLowerCase()
  return users.value.filter((u) => {
    if (userFilter.value === 'admin' && u.role !== 'admin') return false
    if (
      userFilter.value === 'guest' &&
      u.role !== 'guest' &&
      u.provider !== 'guest'
    )
      return false
    if (
      q &&
      !u.name.toLowerCase().includes(q) &&
      !u.email.toLowerCase().includes(q)
    )
      return false
    return true
  })
})

const activeCount = computed(() => users.value.filter((u) => u.active).length)
const totalUploads = computed(() =>
  users.value.reduce((s, u) => s + u.uploads, 0),
)
const totalBytes = computed(() =>
  users.value.reduce((s, u) => s + u.totalBytes, 0),
)
const bytesThisMonth = computed(() =>
  users.value.reduce((s, u) => s + u.bytesThisMonth, 0),
)
const avgPerUser = computed(() =>
  users.value.length === 0 ? 0 : totalBytes.value / users.value.length,
)

function role(u: AdminUser) {
  if (u.role === 'admin') return 'admin'
  if (u.provider === 'guest' || u.role === 'guest') return 'guest'
  return 'uploader'
}
</script>

<template>
  <div class="fb-fade">
    <div class="section-head">
      <div>
        <h1>Users</h1>
        <div class="sub">
          Everyone who has signed into FileBox at least once, with their
          lifetime upload stats. Click a row to see their full activity.
        </div>
      </div>
      <div style="display: flex; gap: 8px; align-items: center">
        <UiRadioGroup
          v-model="userFilter"
          :options="userFilterOptions"
          variant="segmented"
          aria-label="Filter users"
        />
        <input v-model="userQuery" class="user-search" placeholder="Search…" />
      </div>
    </div>

    <div class="stat-strip">
      <div class="stat-card">
        <div class="l">Total users</div>
        <div class="n">{{ users.length }}</div>
        <div class="s">{{ activeCount }} active this month</div>
      </div>
      <div class="stat-card">
        <div class="l">Uploads, all time</div>
        <div class="n">{{ totalUploads.toLocaleString() }}</div>
        <div class="s">
          Across {{ targets.length }}
          {{ targets.length === 1 ? 'target' : 'targets' }}
        </div>
      </div>
      <div class="stat-card">
        <div class="l">Data uploaded</div>
        <div class="n">{{ formatBytes(totalBytes) }}</div>
        <div class="s">{{ formatBytes(bytesThisMonth) }} this month</div>
      </div>
      <div class="stat-card">
        <div class="l">Avg per user</div>
        <div class="n">{{ formatBytes(avgPerUser) }}</div>
        <div class="s">·</div>
      </div>
    </div>

    <div class="card">
      <table>
        <thead>
          <tr>
            <th>User</th>
            <th>Role</th>
            <th>Last login</th>
            <th style="text-align: right">Uploads</th>
            <th style="text-align: right">Volume</th>
            <th>Status</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="u in filteredUsers"
            :key="u.id"
            @click="emit('open', u)"
            style="cursor: pointer"
          >
            <td>
              <div class="name-cell">
                <div
                  class="avatar-md"
                  :style="{ background: avatarBg(u.email || u.name) }"
                >
                  {{ initials(u.name || u.email) }}
                </div>
                <div>
                  <div class="primary">{{ u.name || u.email }}</div>
                  <div class="secondary">
                    {{ u.email }} ·
                    <span :style="{ color: providerColor(u.provider) }">{{
                      providerLabel(u.provider)
                    }}</span>
                  </div>
                </div>
              </div>
            </td>
            <td>
              <span v-if="role(u) === 'admin'" class="badge badge-accent"
                ><span class="badge-dot"></span>Admin</span
              >
              <span v-else-if="role(u) === 'guest'" class="badge badge-warn"
                ><span class="badge-dot"></span>Guest</span
              >
              <span v-else class="badge"
                ><span class="badge-dot"></span>Uploader</span
              >
            </td>
            <td>
              <div style="font-size: 13.5px">{{ relTime(u.lastLoginAt) }}</div>
              <div
                class="mono"
                style="
                  font-size: 11.5px;
                  color: var(--color-ink-3);
                  margin-top: 2px;
                "
              >
                {{ u.lastLoginAt.slice(0, 10) }}
              </div>
            </td>
            <td style="text-align: right">
              <span class="mono" style="font-size: 13px">{{
                u.uploads.toLocaleString()
              }}</span>
            </td>
            <td style="text-align: right">
              <span class="mono" style="font-size: 13px">{{
                formatBytes(u.totalBytes)
              }}</span>
            </td>
            <td>
              <span v-if="u.active" class="badge badge-ok"
                ><span class="badge-dot"></span>Active</span
              >
              <span v-else class="badge" style="color: var(--color-ink-3)"
                ><span
                  class="badge-dot"
                  style="background: var(--color-ink-3)"
                ></span
                >Dormant</span
              >
            </td>
            <td class="actions">
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                style="color: var(--color-ink-3)"
              >
                <path d="M9 6l6 6-6 6" />
              </svg>
            </td>
          </tr>
          <tr v-if="filteredUsers.length === 0">
            <td
              colspan="7"
              style="
                padding: 48px;
                text-align: center;
                color: var(--color-ink-3);
              "
            >
              No users match.
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
/* Colocated from admin.css: used only by this component. The @layer
   wrapper is kept so precedence against Tailwind utilities is unchanged.
   Shared primitives stay in assets/components.css. */
.avatar-md {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-accent-ink);
  flex-shrink: 0;
}
.user-search {
  width: 220px;
  padding: 8px 12px;
  background: var(--color-surface-2);
  border: 1px solid var(--color-line-2);
  border-radius: 8px;
  color: var(--color-ink);
  font-size: 13px;
  outline: none;
  transition: border-color 0.15s ease;
}
.user-search:focus {
  border-color: var(--color-accent);
}
.stat-strip {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 24px;
}
.stat-card {
  background: var(--color-surface-2);
  border: 1px solid var(--color-line);
  border-radius: 12px;
  padding: 16px 18px;
}
</style>
