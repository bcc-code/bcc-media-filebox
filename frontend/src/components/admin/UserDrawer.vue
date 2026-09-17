<script setup lang="ts">
import { computed } from 'vue'
import { useAdmin, type AdminUserDetail } from '../../composables/useAdmin'
import {
  formatBytes,
  relTime,
  initials,
  providerLabel,
  providerColor,
  avatarBg,
} from '../../composables/adminHelpers'
import UiButton from '../ui/UiButton.vue'
import UiDrawer from '../ui/UiDrawer.vue'
import UiBadge from '../ui/UiBadge.vue'

const props = defineProps<{ user: AdminUserDetail }>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'edit-access', u: AdminUserDetail): void
  (e: 'revoke', u: AdminUserDetail): void
}>()

const { targets } = useAdmin()

function targetName(id: number) {
  return targets.value.find((t) => t.id === id)?.name ?? '—'
}

const effectiveTargetNames = computed(() => {
  if (props.user.effectiveAll) return targets.value.map((t) => t.name)
  return props.user.effectiveTargetIds.map(targetName)
})

const role = computed(() => {
  if (props.user.role === 'admin') return 'admin'
  if (props.user.provider === 'guest' || props.user.role === 'guest')
    return 'guest'
  return 'uploader'
})

const failureRate = computed(() => {
  const total = props.user.uploads + props.user.failures
  if (total === 0) return '—'
  return ((props.user.failures / total) * 100).toFixed(1) + '% rate'
})
</script>

<template>
  <UiDrawer @close="emit('close')">
    <template #head>
      <div class="crumb" style="margin-left: 8px">
        filebox / admin / users /
        <span style="color: var(--color-ink-2)">{{ user.id }}</span>
      </div>
    </template>

    <template #default="{ titleProps }">
      <div class="drawer-id">
        <div
          class="avatar-xl"
          :style="{ background: avatarBg(user.email || user.name) }"
        >
          {{ initials(user.name || user.email) }}
        </div>
        <div style="flex: 1; min-width: 0">
          <h2 v-bind="titleProps">{{ user.name || user.email }}</h2>
          <div class="drawer-id-row">
            <span
              class="mono"
              style="color: var(--color-ink-2); font-size: 13px"
              >{{ user.email }}</span
            >
            <UiBadge :tone="providerColor(user.provider)" dot>
              {{ providerLabel(user.provider) }}
            </UiBadge>
            <UiBadge v-if="role === 'admin'" variant="accent" dot
              >Admin</UiBadge
            >
            <UiBadge v-else-if="role === 'guest'" variant="warn" dot
              >Guest</UiBadge
            >
            <UiBadge v-else dot>Uploader</UiBadge>
            <UiBadge v-if="user.active" variant="ok" dot>Active</UiBadge>
            <UiBadge v-else tone="var(--color-ink-3)" dot>Dormant</UiBadge>
          </div>
        </div>
        <div style="display: flex; gap: 8px; flex-shrink: 0">
          <UiButton size="sm" @click="emit('edit-access', user)"
            >Edit access</UiButton
          >
          <UiButton
            v-if="role !== 'guest'"
            size="sm"
            variant="danger"
            @click="emit('revoke', user)"
            >Revoke</UiButton
          >
        </div>
      </div>

      <div class="stat-grid">
        <div class="stat-block">
          <div class="l">Last login</div>
          <div class="n">{{ relTime(user.lastLoginAt) }}</div>
          <div class="s mono">{{ user.lastLoginAt.slice(0, 10) }}</div>
        </div>
        <div class="stat-block">
          <div class="l">First seen</div>
          <div class="n">{{ relTime(user.createdAt) }}</div>
          <div class="s mono">{{ user.createdAt.slice(0, 10) }}</div>
        </div>
        <div class="stat-block">
          <div class="l">Total uploads</div>
          <div class="n">{{ user.uploads.toLocaleString() }}</div>
          <div class="s">{{ user.uploadsThisMonth }} this month</div>
        </div>
        <div class="stat-block">
          <div class="l">Data uploaded</div>
          <div class="n">{{ formatBytes(user.totalBytes) }}</div>
          <div class="s">
            avg {{ formatBytes(user.totalBytes / Math.max(user.uploads, 1)) }} /
            file
          </div>
        </div>
        <div class="stat-block">
          <div class="l">Sessions, 30d</div>
          <div class="n">—</div>
          <div class="s">not tracked yet</div>
        </div>
        <div class="stat-block">
          <div class="l">Failed uploads</div>
          <div class="n">{{ user.failures }}</div>
          <div class="s">{{ failureRate }}</div>
        </div>
      </div>

      <div class="drawer-section">
        <div class="drawer-section-head">
          <h3>Recent uploads</h3>
          <span class="mono" style="font-size: 12px; color: var(--color-ink-3)">
            last {{ user.recent.length }} of {{ user.uploads.toLocaleString() }}
          </span>
        </div>
        <div class="card" v-if="user.recent.length">
          <table>
            <thead>
              <tr>
                <th>File</th>
                <th>Target</th>
                <th style="text-align: right">Size</th>
                <th>When</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in user.recent" :key="r.id">
                <td>
                  <span class="mono" style="font-size: 12.5px">{{
                    r.filename
                  }}</span>
                </td>
                <td>
                  <UiBadge>{{ r.targetName || '—' }}</UiBadge>
                </td>
                <td style="text-align: right">
                  <span class="mono" style="font-size: 12.5px">{{
                    formatBytes(r.size)
                  }}</span>
                </td>
                <td>
                  <span style="font-size: 13px; color: var(--color-ink-2)">{{
                    relTime(r.when)
                  }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="empty" style="padding: 24px">No uploads yet.</div>
      </div>

      <div class="drawer-section">
        <div class="drawer-section-head"><h3>Access</h3></div>
        <div class="access-grid">
          <div class="access-block">
            <div class="l">Groups</div>
            <div class="badges" v-if="user.groups.length">
              <UiBadge v-for="gn in user.groups" :key="gn"
                ><svg
                  width="11"
                  height="11"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <circle cx="9" cy="8" r="3" />
                  <circle cx="17" cy="9" r="2.5" />
                  <path d="M3 19c1-3 3.5-4.5 6-4.5s5 1.5 6 4.5" />
                </svg>
                {{ gn }}</UiBadge
              >
            </div>
            <div v-else style="color: var(--color-ink-3); font-size: 13px">
              Not in any groups.
            </div>
          </div>
          <div class="access-block">
            <div class="l">Direct grants</div>
            <div class="badges" v-if="user.directGrants.length">
              <UiBadge
                v-for="g in user.directGrants"
                :key="g.id"
                variant="accent"
                >{{
                  g.admin
                    ? 'Admin'
                    : g.allTargets
                      ? 'All targets'
                      : g.targetIds.map(targetName).join(', ') || 'No targets'
                }}</UiBadge
              >
            </div>
            <div v-else style="color: var(--color-ink-3); font-size: 13px">
              None — access comes from group membership.
            </div>
          </div>
          <div class="access-block">
            <div class="l">Effective targets</div>
            <div class="badges">
              <UiBadge v-if="user.effectiveAll" variant="ok" dot
                >All targets</UiBadge
              >
              <template v-else>
                <UiBadge
                  v-for="name in effectiveTargetNames"
                  :key="name"
                  variant="ok"
                  >{{ name }}</UiBadge
                >
                <span
                  v-if="effectiveTargetNames.length === 0"
                  style="color: var(--color-ink-3); font-size: 13px"
                  >No upload access.</span
                >
              </template>
            </div>
          </div>
        </div>
      </div>
    </template>
  </UiDrawer>
</template>

<style scoped>
/* Promoted out of admin.css. These are this drawer's own content styles, so
   they belong with the markup that uses them — and scoping them here means the
   panel keeps its look wherever UiDrawer teleports it. */
.avatar-xl {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  font-size: 18px;
  font-weight: 600;
  color: var(--color-accent-ink);
  flex-shrink: 0;
}
.stat-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin-top: 24px;
  margin-bottom: 28px;
}
.stat-block {
  background: var(--color-surface-2);
  border: 1px solid var(--color-line);
  border-radius: 10px;
  padding: 14px 16px;
}
.stat-block .l {
  font-size: 10.5px;
  color: var(--color-ink-3);
  margin-bottom: 6px;
}
.stat-block .n {
  font-size: 20px;
  font-weight: 600;
  letter-spacing: -0.3px;
}
.stat-block .s {
  font-size: 11.5px;
  color: var(--color-ink-3);
  margin-top: 3px;
}
.drawer-id {
  display: flex;
  align-items: center;
  gap: 16px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--color-line);
}
.drawer-id h2 {
  margin: 0 0 6px;
  font-size: 22px;
  font-weight: 600;
  letter-spacing: -0.3px;
}
.drawer-id-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}
.drawer-section {
  margin-top: 32px;
}
.drawer-section-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 12px;
}
.drawer-section-head h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-ink-2);
}
.access-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 12px;
}
.access-block {
  background: var(--color-surface-2);
  border: 1px solid var(--color-line);
  border-radius: 10px;
  padding: 14px 16px;
}
.access-block .l {
  font-size: 10.5px;
  color: var(--color-ink-3);
  margin-bottom: 10px;
}
</style>
