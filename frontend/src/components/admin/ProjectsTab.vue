<script setup lang="ts">
import { useAdmin, type Project } from '../../composables/useAdmin'

const emit = defineEmits<{ (e: 'new'): void; (e: 'edit', p: Project): void }>()
const { projects, deleteProject } = useAdmin()

async function onDelete(p: Project) {
  if (!confirm(`Delete project “${p.name}”? Existing uploads keep their filenames; new uploads can no longer pick it.`)) {
    return
  }
  await deleteProject(p.id)
}
</script>

<template>
  <div class="fb-fade">
    <div class="section-head">
      <div>
        <h1>Projects</h1>
        <div class="sub">Projects power the BCC Media Masters upload form. Each has a display name people pick from, and a short code that goes into the resulting filename.</div>
      </div>
      <button class="btn btn-primary" @click="emit('new')">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>
        New project
      </button>
    </div>

    <div v-if="projects.length === 0" class="empty">
      No projects yet. Add one so people can select it in the upload form.
    </div>

    <div v-else class="card">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Code</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in projects" :key="p.id">
            <td>
              <div class="name-cell">
                <div class="swatch">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
                </div>
                <div class="primary">{{ p.name }}</div>
              </div>
            </td>
            <td><span class="chip mono">{{ p.code }}</span></td>
            <td class="actions">
              <button class="btn btn-sm btn-ghost" @click="emit('edit', p)">Edit</button>
              <button class="btn btn-sm btn-danger" @click="onDelete(p)">Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
