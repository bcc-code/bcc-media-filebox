<script setup lang="ts">
import { computed } from 'vue'
import { useAdmin, type AdminUserDetail } from '../../composables/useAdmin'
import { formatBytes, relTime, initials, providerLabel, providerColor, avatarBg } from '../../composables/adminHelpers'
import UiButton from '../ui/UiButton.vue'

const props = defineProps<{ user: AdminUserDetail }>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'edit-access', u: AdminUserDetail): void
  (e: 'revoke', u: AdminUserDetail): void
}>()

const { targets } = useAdmin()

function targetName(id: number) {
  return targets.value.find(t => t.id === id)?.name ?? '—'
}

const effectiveTargetNames = computed(() => {
  if (props.user.effectiveAll) return targets.value.map(t => t.name)
  return props.user.effectiveTargetIds.map(targetName)
})

const role = computed(() => {
  if (props.user.role === 'admin') return 'admin'
  if (props.user.provider === 'guest' || props.user.role === 'guest') return 'guest'
  return 'uploader'
})

const failureRate = computed(() => {
  const total = props.user.uploads + props.user.failures
  if (total === 0) return '—'
  return ((props.user.failures / total) * 100).toFixed(1) + '% rate'
})
</script>

<template>
  <div class="drawer-bg" @click.self="emit('close')">
    <div class="drawer fb-slide">
      <div class="drawer-head">
        <UiButton icon variant="ghost" @click="emit('close')" aria-label="Close">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
        </UiButton>
        <div class="crumb" style="margin-left:8px">filebox / admin / users / <span style="color:var(--color-ink-2)">{{ user.id }}</span></div>
      </div>

      <div class="drawer-body">
        <div class="drawer-id">
          <div class="avatar-xl" :style="{ background: avatarBg(user.email || user.name) }">{{ initials(user.name || user.email) }}</div>
          <div style="flex:1; min-width:0">
            <h2>{{ user.name || user.email }}</h2>
            <div class="drawer-id-row">
              <span class="mono" style="color:var(--color-ink-2);font-size:13px">{{ user.email }}</span>
              <span
                class="badge"
                :style="{
                  color: providerColor(user.provider),
                  borderColor: 'color-mix(in oklch, ' + providerColor(user.provider) + ', transparent 60%)',
                }"
              >
                <span class="badge-dot" :style="{ background: providerColor(user.provider) }"></span>
                {{ providerLabel(user.provider) }}
              </span>
              <span v-if="role === 'admin'" class="badge badge-accent"><span class="badge-dot"></span>Admin</span>
              <span v-else-if="role === 'guest'" class="badge badge-warn"><span class="badge-dot"></span>Guest</span>
              <span v-else class="badge"><span class="badge-dot"></span>Uploader</span>
              <span v-if="user.active" class="badge badge-ok"><span class="badge-dot"></span>Active</span>
              <span v-else class="badge" style="color:var(--color-ink-3)"><span class="badge-dot" style="background:var(--color-ink-3)"></span>Dormant</span>
            </div>
          </div>
          <div style="display:flex;gap:8px;flex-shrink:0">
            <UiButton size="sm" @click="emit('edit-access', user)">Edit access</UiButton>
            <UiButton v-if="role !== 'guest'" size="sm" variant="danger" @click="emit('revoke', user)">Revoke</UiButton>
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
            <div class="s">avg {{ formatBytes(user.totalBytes / Math.max(user.uploads, 1)) }} / file</div>
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
            <span class="mono" style="font-size:12px;color:var(--color-ink-3)">
              last {{ user.recent.length }} of {{ user.uploads.toLocaleString() }}
            </span>
          </div>
          <div class="card" v-if="user.recent.length">
            <table>
              <thead>
                <tr><th>File</th><th>Target</th><th style="text-align:right">Size</th><th>When</th></tr>
              </thead>
              <tbody>
                <tr v-for="r in user.recent" :key="r.id">
                  <td><span class="mono" style="font-size:12.5px">{{ r.filename }}</span></td>
                  <td><span class="badge">{{ r.targetName || '—' }}</span></td>
                  <td style="text-align:right"><span class="mono" style="font-size:12.5px">{{ formatBytes(r.size) }}</span></td>
                  <td><span style="font-size:13px;color:var(--color-ink-2)">{{ relTime(r.when) }}</span></td>
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
                <span v-for="gn in user.groups" :key="gn" class="badge">
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="9" cy="8" r="3"/><circle cx="17" cy="9" r="2.5"/><path d="M3 19c1-3 3.5-4.5 6-4.5s5 1.5 6 4.5"/></svg>
                  {{ gn }}
                </span>
              </div>
              <div v-else style="color:var(--color-ink-3);font-size:13px">Not in any groups.</div>
            </div>
            <div class="access-block">
              <div class="l">Direct grants</div>
              <div class="badges" v-if="user.directGrants.length">
                <span v-for="g in user.directGrants" :key="g.id" class="badge badge-accent">
                  {{ g.admin ? 'Admin' : g.allTargets ? 'All targets' : g.targetIds.map(targetName).join(', ') || 'No targets' }}
                </span>
              </div>
              <div v-else style="color:var(--color-ink-3);font-size:13px">None — access comes from group membership.</div>
            </div>
            <div class="access-block">
              <div class="l">Effective targets</div>
              <div class="badges">
                <span v-if="user.effectiveAll" class="badge badge-ok"><span class="badge-dot"></span>All targets</span>
                <template v-else>
                  <span v-for="name in effectiveTargetNames" :key="name" class="badge badge-ok">{{ name }}</span>
                  <span v-if="effectiveTargetNames.length === 0" style="color:var(--color-ink-3);font-size:13px">No upload access.</span>
                </template>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
