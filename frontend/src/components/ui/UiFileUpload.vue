<script setup lang="ts">
import * as fileUpload from '@zag-js/file-upload'
import { normalizeProps, useMachine } from '@zag-js/vue'
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    /** 0 means unlimited, matching the form config's convention. */
    maxFiles?: number
    disabled?: boolean
    /** Prompt shown at rest. */
    label?: string
    /** Prompt shown while a drag is over the zone. */
    dragLabel?: string
    /** Supporting line under the prompt. */
    hint?: string
  }>(),
  {
    maxFiles: 0,
    disabled: false,
    label: 'Drag & drop files here, or click to browse',
    dragLabel: 'Drop files here',
    hint: undefined,
  },
)

const emit = defineEmits<{
  files: [files: File[]]
  /** A human-readable reason a drop was refused. */
  reject: [reason: string]
}>()

const id = useId()

const service = useMachine(
  fileUpload.machine,
  computed(() => ({
    id,
    maxFiles: props.maxFiles === 0 ? Infinity : props.maxFiles,
    // Zag's default announces the zone as just "dropzone"; use the visible
    // prompt so a screen reader hears what this one is for.
    translations: { dropzone: props.label },
    disabled: props.disabled,
    onFileAccept: ({ files }: fileUpload.FileAcceptDetails) => {
      if (files.length) emit('files', files)
      // The consumer owns the upload list; the machine keeps no history.
      api.value.clearFiles()
    },
    // Zag refuses the whole drop when it breaks a limit. Say so rather than
    // dropping files on the floor: the hand-rolled version silently kept only
    // the first file, which left the rest unexplained.
    onFileReject: ({ files }: fileUpload.FileRejectDetails) => {
      if (!files.length) return
      const errors = new Set(files.flatMap((f) => f.errors))
      const reason = errors.has('TOO_MANY_FILES')
        ? props.maxFiles === 1
          ? 'This target takes a single file — drop just one.'
          : `At most ${props.maxFiles} files at a time.`
        : errors.has('FILE_TOO_LARGE')
          ? 'That file is too large.'
          : errors.has('FILE_INVALID_TYPE')
            ? 'That file type is not accepted here.'
            : 'Those files were not accepted.'
      emit('reject', reason)
      api.value.clearRejectedFiles()
    },
  })),
)

const api = computed(() => fileUpload.connect(service, normalizeProps))
</script>

<template>
  <div v-bind="api.getRootProps()">
    <div
      v-bind="api.getDropzoneProps()"
      class="dropzone"
      :class="{ dragover: api.dragging, disabled }"
    >
      <span class="ic">
        <svg
          width="40"
          height="40"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="1.5"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5"
          />
        </svg>
      </span>
      <p class="t">{{ api.dragging ? dragLabel : label }}</p>
      <p v-if="hint" class="s">{{ hint }}</p>
    </div>
    <input v-bind="api.getHiddenInputProps()" />
  </div>
</template>

<style scoped>
/* Colocated from components.css: used only by this component. The @layer
   wrapper is kept so precedence against Tailwind utilities is unchanged.
   Shared primitives stay in assets/components.css. */
@layer components {
  .dropzone {
    border: 1.5px dashed var(--color-line-2);
    border-radius: 14px;
    padding: 40px 24px;
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    cursor: pointer;
    transition:
      border-color 0.15s ease,
      background 0.15s ease;
  }
  .dropzone:hover {
    border-color: color-mix(in oklch, var(--color-accent), transparent 40%);
    background: color-mix(in oklch, var(--color-accent), transparent 96%);
  }
  .dropzone .ic {
    color: var(--color-ink-2);
    margin-bottom: 12px;
  }
  .dropzone:disabled,
  .dropzone.disabled {
    opacity: 0.5;
    cursor: not-allowed;
    border-color: var(--color-line);
    background: transparent;
  }
  .dropzone:disabled:hover,
  .dropzone.disabled:hover {
    border-color: var(--color-line);
    background: transparent;
  }

  /* Same scope as .dropzone: as a global rule this tied on specificity with
     the scoped base and the winner came down to bundle order. */
  .dropzone.dragover {
    border-color: var(--color-accent);
    background: color-mix(in oklch, var(--color-accent), transparent 90%);
  }
}
</style>
