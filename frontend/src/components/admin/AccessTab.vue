<script setup lang="ts">
import { computed } from 'vue'
import { useAdmin, type Grant } from '../../composables/useAdmin'
import { confirmAction } from '../../composables/useConfirm'
import UiButton from '../ui/UiButton.vue'

const emit = defineEmits<{ (e: 'new'): void; (e: 'edit', g: Grant): void }>()
const { grants, targets, groups, deleteGrant } = useAdmin()

async function onRemove(g: Grant) {
  const who =
    g.principalKind === 'group'
      ? `the group “${g.principalValue}”`
      : g.principalValue
  const ok = await confirmAction({
    title: `Remove access for ${g.principalValue}?`,
    body:
      `Uploads already made by ${who} are kept, but ` +
      (g.principalKind === 'group' ? 'its members' : 'they') +
      ' will not be able to upload until granted again.',
    confirmLabel: 'Remove access',
  })
  if (!ok) return
  await deleteGrant(g.id)
}

function targetName(id: number) {
  return targets.value.find((t) => t.id === id)?.name ?? '—'
}

function secondaryFor(g: Grant): string {
  if (g.principalKind === 'group') {
    const gr = groups.value.find((x) => x.name === g.principalValue)
    if (!gr) return 'Group'
    if (gr.kind === 'builtin') return 'Built-in group'
    return `Custom · ${gr.members.length} ${gr.members.length === 1 ? 'person' : 'ppl'}`
  }
  if (g.principalValue.toLowerCase().endsWith('@bcc.media')) return 'Azure AD'
  if (g.principalValue.toLowerCase().endsWith('@bcc.no')) return 'BCC Login'
  return 'BCC Login · external'
}

const sortedGrants = computed(() =>
  [...grants.value].sort((a, b) => a.id - b.id),
)
</script>

<template>
  <div class="fb-fade">
    <div class="section-head">
      <div>
        <h1>Access</h1>
        <div class="sub">
          Who can sign into the admin, and which upload targets each person or
          group can use. Groups expand to everyone they include.
        </div>
      </div>
      <UiButton variant="primary" @click="emit('new')">
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.5"
          stroke-linecap="round"
        >
          <path d="M12 5v14M5 12h14" />
        </svg>
        Grant access
      </UiButton>
    </div>

    <div v-if="grants.length === 0" class="empty">
      No grants yet. Anyone signing in will hit a permission wall.
    </div>

    <div v-else class="card">
      <table>
        <thead>
          <tr>
            <th>Principal</th>
            <th>Role</th>
            <th>Allowed targets</th>
            <th>Added</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="g in sortedGrants" :key="g.id">
            <td>
              <div class="name-cell">
                <div
                  class="swatch"
                  :class="{ group: g.principalKind === 'group' }"
                >
                  <svg
                    v-if="g.principalKind === 'user'"
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.8"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <circle cx="12" cy="8" r="3.2" />
                    <path d="M5 20c1.5-3.6 4-5 7-5s5.5 1.4 7 5" />
                  </svg>
                  <svg
                    v-else
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.8"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <circle cx="9" cy="8" r="3" />
                    <circle cx="17" cy="9" r="2.5" />
                    <path d="M3 19c1-3 3.5-4.5 6-4.5s5 1.5 6 4.5" />
                    <path d="M15 19c.5-2 2-3 3.5-3s3 1 3.5 3" />
                  </svg>
                </div>
                <div>
                  <div class="primary">{{ g.principalValue }}</div>
                  <div class="secondary">{{ secondaryFor(g) }}</div>
                </div>
              </div>
            </td>
            <td>
              <span class="badge badge-accent" v-if="g.admin"
                ><span class="badge-dot"></span>Admin</span
              >
              <span class="badge" v-else style="color: var(--color-ink-3)"
                >Uploader</span
              >
            </td>
            <td>
              <div class="badges">
                <span v-if="g.admin || g.allTargets" class="badge badge-ok"
                  ><span class="badge-dot"></span>All targets</span
                >
                <template v-else>
                  <span v-for="tid in g.targetIds" :key="tid" class="badge">{{
                    targetName(tid)
                  }}</span>
                  <span
                    v-if="g.targetIds.length === 0"
                    class="badge"
                    style="color: var(--color-ink-3)"
                    >No targets</span
                  >
                </template>
              </div>
            </td>
            <td>
              <span
                class="mono"
                style="font-size: 12px; color: var(--color-ink-3)"
                >{{ g.createdAt.slice(0, 10) }}</span
              >
            </td>
            <td class="actions">
              <UiButton size="sm" variant="ghost" @click="emit('edit', g)">
                Edit
              </UiButton>
              <UiButton size="sm" variant="danger" @click="onRemove(g)">
                Remove
              </UiButton>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
