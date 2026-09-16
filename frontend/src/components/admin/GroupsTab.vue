<script setup lang="ts">
import { useAdmin, type Group } from '../../composables/useAdmin'
import { confirmAction } from '../../composables/useConfirm'
import UiButton from '../ui/UiButton.vue'

const emit = defineEmits<{ (e: 'new'): void; (e: 'edit', g: Group): void }>()
const { groups, grants, deleteGroup } = useAdmin()

function countGrantsForGroup(name: string) {
  return grants.value.filter(
    (g) => g.principalKind === 'group' && g.principalValue === name,
  ).length
}

async function onDelete(gr: Group) {
  const inUse = countGrantsForGroup(gr.name)
  if (inUse > 0) {
    const ok = await confirmAction({
      title: `Delete “${gr.name}”?`,
      body: `It is used by ${inUse} grant${inUse === 1 ? '' : 's'}, which will be deleted too.`,
      confirmLabel: 'Delete group and grants',
    })
    if (!ok) return
  }
  await deleteGroup(gr.id)
}
</script>

<template>
  <div class="fb-fade">
    <div class="section-head">
      <div>
        <h1>Groups</h1>
        <div class="sub">
          Reusable bundles of users. Grant access to a group once instead of
          listing every member. Built-in groups come from the directory and
          update automatically; custom groups you maintain here.
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
        New custom group
      </UiButton>
    </div>

    <div class="card">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Members</th>
            <th>Used by</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="gr in groups" :key="gr.id">
            <td>
              <div class="name-cell">
                <div class="swatch group">
                  <svg
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
                  <div class="primary">{{ gr.name }}</div>
                  <div class="secondary">{{ gr.description }}</div>
                </div>
              </div>
            </td>
            <td>
              <span v-if="gr.kind === 'builtin'" class="badge"
                ><span class="badge-dot"></span>Built-in</span
              >
              <span v-else class="badge badge-accent"
                ><span class="badge-dot"></span>Custom</span
              >
            </td>
            <td>
              <span
                v-if="gr.kind === 'builtin'"
                class="mono"
                style="font-size: 12px; color: var(--color-ink-3)"
                >Directory-managed</span
              >
              <span
                v-else
                class="mono"
                style="font-size: 12.5px; color: var(--color-ink-2)"
              >
                {{ gr.members.length }}
                {{ gr.members.length === 1 ? 'member' : 'members' }}
              </span>
            </td>
            <td>
              <span
                v-if="countGrantsForGroup(gr.name) === 0"
                class="badge"
                style="color: var(--color-ink-3)"
                >No grants</span
              >
              <span v-else class="badge badge-ok"
                >{{ countGrantsForGroup(gr.name) }}
                {{
                  countGrantsForGroup(gr.name) === 1 ? 'grant' : 'grants'
                }}</span
              >
            </td>
            <td class="actions">
              <template v-if="gr.kind === 'custom'">
                <UiButton size="sm" variant="ghost" @click="emit('edit', gr)">
                  Edit
                </UiButton>
                <UiButton size="sm" variant="danger" @click="onDelete(gr)">
                  Delete
                </UiButton>
              </template>
              <span
                v-else
                class="mono"
                style="font-size: 11px; color: var(--color-ink-3)"
                >Managed by directory</span
              >
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
