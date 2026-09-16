<script setup lang="ts">
import { computed } from 'vue'
import { buildFilename, type Form, type Option } from '../forms'

const props = defineProps<{
  form: Form
  modelValue: Record<string, string>
  // Options for select fields whose optionsSource is DB-backed (keyed by field key).
  dynamicOptions?: Record<string, Option[]>
  // Autocomplete suggestions for free-text fields (keyed by field key).
  suggestions?: Record<string, string[]>
}>()

const emit = defineEmits<{
  'update:modelValue': [values: Record<string, string>]
}>()

function setField(key: string, value: string) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}

function optionsFor(fieldKey: string, source: string | undefined, staticOpts: Option[] | undefined): Option[] {
  if (source) return props.dynamicOptions?.[fieldKey] ?? []
  return staticOpts ?? []
}

function tooShort(field: { minLength?: number }, value: string): boolean {
  return !!field.minLength && value.trim() !== '' && value.trim().length < field.minLength
}

// Live preview of the derived filename. The backend re-derives it
// authoritatively on upload; this is purely informational.
const previewName = computed(() => buildFilename(props.form, props.modelValue, '.ext'))
</script>

<template>
  <div class="card form-card">
    <div class="form-head">
      <span class="badge badge-accent">
        <span class="badge-dot" />
        {{ form.label }}
      </span>
      <span class="form-desc">{{ form.description }}</span>
    </div>

    <div class="form-grid">
      <div v-for="field in form.fields" :key="field.key" class="field">
        <label>
          {{ field.label }}
          <span v-if="field.required" class="req">*</span>
          <span v-else class="opt">Optional</span>
          <span v-if="field.maxLength" class="count">
            {{ (modelValue[field.key] ?? '').length }}/{{ field.maxLength }}
          </span>
        </label>

        <select
          v-if="field.type === 'select'"
          :value="modelValue[field.key] ?? ''"
          @change="setField(field.key, ($event.target as HTMLSelectElement).value)"
        >
          <option v-if="field.placeholder" value="">{{ field.placeholder }}</option>
          <option v-for="opt in optionsFor(field.key, field.optionsSource, field.options)" :key="opt.code" :value="opt.code">{{ opt.label }}</option>
        </select>

        <template v-else>
          <input
            :type="field.type === 'number' ? 'number' : 'text'"
            :value="modelValue[field.key] ?? ''"
            :placeholder="field.placeholder"
            :maxlength="field.maxLength || undefined"
            :list="field.suggest ? `${form.key}-${field.key}-list` : undefined"
            :class="{ 'inp-warn': tooShort(field, modelValue[field.key] ?? '') }"
            @input="setField(field.key, ($event.target as HTMLInputElement).value)"
          />
          <datalist v-if="field.suggest" :id="`${form.key}-${field.key}-list`">
            <option v-for="s in suggestions?.[field.key] ?? []" :key="s" :value="s" />
          </datalist>
          <span v-if="tooShort(field, modelValue[field.key] ?? '')" class="hint hint-warn">
            At least {{ field.minLength }} characters.
          </span>
        </template>
      </div>
    </div>

    <div class="filename-preview">
      <span class="l">Resulting filename</span>
      <span class="v mono">{{ previewName }}</span>
    </div>
  </div>
</template>

<style scoped>
.form-card { padding: 18px; }

.form-head { display: flex; align-items: center; gap: 10px; margin-bottom: 18px; flex-wrap: wrap; }
.form-desc { font-size: 13.5px; color: var(--color-ink-2); }

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 4px 14px;
}

/* The shared `.field > label` is an uppercase micro-label; this form needs it
   to also carry the required marker and the character counter on one row. */
.field > label { display: flex; align-items: center; gap: 6px; }
.field > label .req { color: var(--color-accent); }
.field > label .opt { font-size: 10px; color: var(--color-ink-3); }
.field > label .count {
  margin-left: auto;
  font-size: 11px; letter-spacing: 0; text-transform: none;
  color: var(--color-ink-3);
}

/* Scoped styles are unlayered, so they beat the `@layer components` input
   rule without needing extra specificity. */
.inp-warn { border-color: var(--color-warn); }
.hint-warn { color: var(--color-warn); }

.filename-preview {
  display: flex; align-items: center; gap: 12px; flex-wrap: wrap;
  margin-top: 18px; padding: 12px 14px; border-radius: 10px;
  background: var(--color-surface-3);
  border: 1px solid var(--color-line);
}
.filename-preview .l {
  font-size: 10px; font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.08em;
  color: var(--color-ink-3);
}
.filename-preview .v { font-size: 13px; color: var(--color-ink-2); }
</style>
