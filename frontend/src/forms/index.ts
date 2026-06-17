// Hardcoded upload-form definitions. A target may reference a form by key
// (the `formKey` field returned by /api/targets); when it does, the uploader
// must fill the form before uploading and the resulting filename is derived
// from the submitted values.
//
// This is a mirror of the backend registry in internal/forms/forms.go. Keep the
// two in sync — same keys, same fields, same template — when editing. The
// backend is authoritative for the final filename; buildFilename here only
// drives the live preview.

export type FieldType = 'text' | 'number' | 'select'

export interface Option {
  code: string // goes into the filename
  label: string // shown in the dropdown
}

export interface Field {
  key: string
  label: string
  type: FieldType
  required: boolean
  minLength?: number
  maxLength?: number
  placeholder?: string
  options?: Option[]
  // optionsSource names a DB-backed catalog (e.g. "projects", "arrangements")
  // the select options come from instead of the static `options` above.
  optionsSource?: string
  // optionsScope makes the select dependent on another field: its options are
  // fetched for that field's value (e.g. sub-events for the chosen arrangement).
  optionsScope?: string
  // suggest enables free-text autocomplete sourced from prior uploads, scoped by
  // the value of the suggestScope field (e.g. season suggestions per project).
  suggest?: boolean
  suggestScope?: string
}

export interface Form {
  key: string
  label: string
  description: string
  maxFiles: number // 1 = single file; 0 = unlimited
  fields: Field[]
  template: string // e.g. "{arrangement}_{subEvent}_{navn}"
  // resetFields are cleared after each successful upload.
  resetFields?: string[]
}

export const registry: Record<string, Form> = {
  bcc_media: {
    key: 'bcc_media',
    label: 'BCC Media',
    description: 'Add event details before uploading',
    maxFiles: 1,
    template: '{arrangement}_{subEvent}_{navn}',
    fields: [
      {
        key: 'arrangement',
        label: 'Arrangement',
        type: 'select',
        required: true,
        placeholder: 'Velg arrangement...',
        options: [
          { code: 'ARR', label: 'Arrangement' },
          { code: 'SMR', label: 'Sommerstevne' },
          { code: 'VIN', label: 'Vinterstevne' },
        ],
      },
      {
        key: 'subEvent',
        label: 'Sub event',
        type: 'select',
        required: true,
        placeholder: 'Velg arrangement først',
        options: [
          { code: 'SUB', label: 'Sub event' },
          { code: 'MØT', label: 'Møte' },
          { code: 'SEM', label: 'Seminar' },
        ],
      },
      { key: 'post', label: 'Post-nr.', type: 'number', required: false },
      {
        key: 'type',
        label: 'Type',
        type: 'select',
        required: false,
        options: [
          { code: '', label: '— Ingen —' },
          { code: 'VID', label: 'Video' },
          { code: 'AUD', label: 'Audio' },
        ],
      },
      {
        key: 'navn',
        label: 'Navn',
        type: 'text',
        required: true,
        maxLength: 50,
        placeholder: 'For example: temafilm',
      },
    ],
  },
  camera_dailies: {
    key: 'camera_dailies',
    label: 'BCC Media Masters',
    description: 'Add project details before uploading',
    maxFiles: 1,
    template: '{project}_{season}_{episode}_{title}',
    resetFields: ['episode', 'title'],
    fields: [
      {
        key: 'project',
        label: 'Project',
        type: 'select',
        required: true,
        placeholder: 'Select project...',
        optionsSource: 'projects',
      },
      { key: 'season', label: 'Season', type: 'text', required: false, suggest: true, suggestScope: 'project' },
      { key: 'episode', label: 'Episode', type: 'text', required: false, suggest: true, suggestScope: 'project' },
      {
        key: 'title',
        label: 'Title',
        type: 'text',
        required: true,
        minLength: 5,
        maxLength: 50,
        placeholder: 'For example: cold open',
      },
    ],
  },
  oslofjord_delivery: {
    key: 'oslofjord_delivery',
    label: 'Oslofjord Delivery',
    description: 'Add event details before uploading',
    maxFiles: 1,
    template: '{arrangement}_{subEvent}_{post}_{type}_{navn}',
    resetFields: ['post', 'navn'],
    fields: [
      {
        key: 'arrangement',
        label: 'Arrangement',
        type: 'select',
        required: true,
        placeholder: 'Velg arrangement...',
        optionsSource: 'arrangements',
      },
      {
        // Not required: the "-" (empty) choice means none and is dropped from
        // the filename. Options are the sub-events of the chosen arrangement.
        key: 'subEvent',
        label: 'Sub event',
        type: 'select',
        required: false,
        placeholder: '-',
        optionsSource: 'subEvents',
        optionsScope: 'arrangement',
      },
      { key: 'post', label: 'Post-nr.', type: 'text', required: false, maxLength: 10 },
      {
        key: 'type',
        label: 'Type',
        type: 'select',
        required: false,
        placeholder: '— Ingen —',
        options: [
          { code: 'VIDEO', label: 'Video' },
          { code: 'LED', label: 'LED Background' },
          { code: 'CLICK', label: 'ClickTrack' },
          { code: 'PRES', label: 'Presentation' },
        ],
      },
      {
        key: 'navn',
        label: 'Navn',
        type: 'text',
        required: true,
        minLength: 5,
        maxLength: 50,
        placeholder: 'For example: temafilm',
      },
    ],
  },
}

export function getForm(key: string | null | undefined): Form | null {
  if (!key) return null
  return registry[key] ?? null
}

export function formKeys(): string[] {
  return Object.keys(registry).sort()
}

// slug mirrors the backend slug(): keep [A-Za-z0-9-], collapse anything else to
// a single "_", trim leading/trailing underscores.
function slug(s: string): string {
  return s
    .replace(/[^A-Za-z0-9-]+/g, '_')
    .replace(/^_+|_+$/g, '')
}

function optionCode(field: Field, raw: string): string {
  if (!raw) return ''
  const match = field.options?.find((o) => o.code === raw || o.label === raw)
  return match ? match.code : slug(raw)
}

// buildFilename mirrors the backend BuildFilename for the live preview. ext
// should include the leading dot (e.g. ".mov") or be empty.
export function buildFilename(form: Form, values: Record<string, string>, ext: string): string {
  const codeByKey: Record<string, string> = {}
  for (const field of form.fields) {
    const raw = (values[field.key] ?? '').trim()
    codeByKey[field.key] = field.type === 'select' ? optionCode(field, raw) : slug(raw)
  }
  const tokens = [...form.template.matchAll(/\{([^}]+)\}/g)].map((m) => m[1])
  const parts = tokens.map((t) => codeByKey[t]).filter((v) => v)
  const base = parts.join('_') || 'upload'
  return base + ext
}

// isFormValid reports whether all required fields have a non-empty value and
// every field meets its minLength.
export function isFormValid(form: Form, values: Record<string, string>): boolean {
  return form.fields.every((f) => {
    const v = (values[f.key] ?? '').trim()
    if (f.required && v === '') return false
    if (f.minLength && v !== '' && v.length < f.minLength) return false
    return true
  })
}
