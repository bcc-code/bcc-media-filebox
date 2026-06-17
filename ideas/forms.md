# Hardcoded upload forms per target

## Context

FileBox currently lets a user pick a destination (target) and drop files. Filenames are
sanitized but otherwise free-form, and nothing is captured about *what* the file is. We want
certain destinations to require **structured metadata before upload** and to **derive the
filename from that metadata**. Two real examples (from the design mockups):

- **BCC Media (Isilon)** — collects Arrangement, Sub event, Post-nr (optional), Type (optional),
  Navn → resulting filename `ARR_SUB_navn.ext`. Allows **only one file at a time**.
- **Raw camera dailies** — collects Project, Season (optional), Episode (optional), Title →
  resulting filename `PROJ_tittel.ext`.

On top of that, two operational requirements:

1. **Write the submitted form data into a JSON sidecar** next to the uploaded file.
2. **Use that data to rename the incoming file** — the rename is authoritative on the backend,
   not just a client-side convenience.

These forms are **hardcoded** — not admin-editable. Admins only choose *which* hardcoded form
(if any) a target uses, via a dropdown. A target with no form behaves exactly as today.

**Decisions already locked in:**
- Form definitions are hardcoded in **both** Go and TypeScript (no API-driven schema).
- A target gets a nullable `form_key`; admins pick the form from a dropdown of known keys.
- Filename-validation regex is **out of scope** — tracked as a separate future feature.

**Complexity verdict:** medium. New `internal/forms` package + a mirror TS module, one migration
(one nullable column), a small enrichment of `/api/targets`, one new Vue component, and ~40 lines
of new logic in the existing TUS finalize path for the rename + sidecar. The trickiest parts are
(a) carrying form values through to the async finalize goroutine and (b) keeping the Go and TS
definitions in sync.

## Concept

A *form profile* is a hardcoded definition attached to a target. It declares:

- **Fields** — each with `key`, `label`, `type` (`text` | `number` | `select`), `required`,
  `maxLength`, `placeholder`, and (for selects) `options` — each option carrying a short `code`
  used in the filename plus a human label shown in the dropdown.
- **Filename template** — e.g. `{arrangement}_{subEvent}_{navn}`. Selects contribute their
  option `code` (ARR, SUB…); text fields contribute a slugified value; optional empty fields are
  skipped (see "Optional fields" below).
- **maxFiles** — `1` for single-file forms (BCC Media), `0`/unset for unlimited.
- **key / label / description** — stable id + the "Add event details before uploading" header.

Targets with no `form_key` behave exactly as today: free upload, multiple files, sanitized
original filename, no sidecar.

## Data model

### Migration: `internal/db/migrations/00010_add_target_form.sql`

```sql
-- +goose Up
ALTER TABLE targets ADD COLUMN form_key TEXT;   -- NULL = plain upload, no form

-- +goose Down
ALTER TABLE targets DROP COLUMN form_key;
```

No backfill — existing targets stay `form_key = NULL`.

The submitted form **values** also need to reach the async finalize goroutine (see Backend
section). Cleanest option: add a column to `uploads` to stash them at create time:

```sql
ALTER TABLE uploads ADD COLUMN form_data TEXT;  -- raw JSON of submitted field values
```

(Alternative: carry the values only through TUS metadata and read them back in `finalizeUpload`
without a column. A column is simpler to reason about and gives us provenance for free — pick one
at implementation time. This doc assumes the column.)

### sqlc

- `internal/db/queries/targets.sql` — `CreateTarget` / `UpdateTarget` set `form_key`;
  `ListTargets` / `GetTargetByName` return it.
- `internal/db/queries/uploads.sql` — `CreateUpload` writes `form_data`; the finalize path reads
  it (it already loads the upload row).
- Run `make generate` after editing the `.sql` files.

`targetDTO` (`internal/api/admin_handlers.go:82`) gains `FormKey *string`.

## Go form registry: `internal/forms/`

Single source of truth on the backend — and the **authoritative renamer**.

```go
package forms

type FieldType string

const (
    FieldText   FieldType = "text"
    FieldNumber FieldType = "number"
    FieldSelect FieldType = "select"
)

type Option struct {
    Code  string // goes into the filename, e.g. "ARR"
    Label string // shown in the dropdown
}

type Field struct {
    Key         string
    Label       string
    Type        FieldType
    Required    bool
    MaxLength   int
    Placeholder string
    Options     []Option // select only
}

type Form struct {
    Key         string
    Label       string
    Description string // "Add event details before uploading"
    MaxFiles    int    // 1 = single file; 0 = unlimited
    Fields      []Field
    Template    string // "{arrangement}_{subEvent}_{navn}"
}

var Registry = map[string]Form{
    "bcc_media":     { /* ... */ },
    "camera_dailies": { /* ... */ },
}

func Get(key string) (Form, bool) { f, ok := Registry[key]; return f, ok }
func Keys() []string              { /* sorted keys for the admin dropdown */ }

// BuildFilename is the authoritative renamer. It resolves each {token} in the
// template from values (select -> option code, text -> slug), skips empty
// optional tokens, then appends ext. The result is still run through
// SanitizeFilename + uniquePath by the caller.
func BuildFilename(f Form, values map[string]string, ext string) string { /* ... */ }
```

- Admin `CreateTarget` / `UpdateTarget` validation rejects an unknown `form_key`
  (`validateTarget`, `internal/api/admin_handlers.go:237`).

## TS form registry: `frontend/src/forms/index.ts`

A mirror of the Go structs and registry, keyed identically, plus `buildFilename(form, values, ext)`
for the live "RESULTING FILENAME" preview shown in the form.

**Sync risk (explicit, because the definitions are duplicated by choice):** the Go and TS
registries can drift. Mitigations, cheapest first:
- A comment in each file pointing at the other as its mirror.
- A unit test asserting the **key sets** match (the frontend would have to expose its keys; or a
  tiny `/api/forms/keys` endpoint the Go test hits — overkill, skip unless drift bites).
- Longer term: generate the TS module from the Go registry (`go generate`) so there's one source.
  Out of scope here; note it as the natural follow-up if the form count grows.

## API change

`/api/targets` (`internal/api/handlers.go:68`) currently returns `string[]` (commit `3ae51d3`
dropped paths from the cards). Enrich it back to objects so the selector knows which form to
render:

```json
[{ "name": "BCC Media (Isilon)", "formKey": "bcc_media" },
 { "name": "Raw camera dailies",  "formKey": "camera_dailies" },
 { "name": "Audio masters (S3)",   "formKey": null }]
```

`TargetSelector.vue` and `Home.vue` consume the new shape. (Flag for the implementer: this reverses
the recent string-array simplification — update the frontend types in `frontend/src/types/`.)

## Frontend flow

- **New `UploadForm.vue`** — takes a form definition + `v-model` of field values. Renders fields by
  type (text input, number input, select), enforces `required` + `maxLength`, and shows the live
  resulting-filename preview. Mirrors the mockups (header pill, two-column grid, "RESULTING
  FILENAME" footer).
- **`Home.vue`** — when the selected target has a `formKey`, render `UploadForm` below the
  `TargetSelector` and **gate file acceptance** until required fields are valid.
- **`FileUploader.vue`** — honor `maxFiles`: drop the `multiple` attribute and reject >1 file when
  the form is single-file.
- **`useTusUpload.ts` (`addFiles`)** — send the submitted field values to the backend as TUS
  metadata (a `formdata` JSON blob alongside the existing `target`). The client-computed filename
  is only a **preview**; the backend re-derives the authoritative name. Keep the existing client
  sanitization as a display guard.

## Backend: server-side rename + JSON sidecar

This is where both follow-ups land. The TUS finalize path already renames temp → target dir
(`finalizeUpload` / `renameUpload` in `internal/tus/hooks.go:140`, with collision handling via
`uniquePath:316` and a containment check at `:241`). Extend it:

1. **Capture form data at create time.** In `handleCreated` (`internal/tus/hooks.go:65`), read the
   `formdata` metadata and persist it to `uploads.form_data` alongside the existing fields. Also
   resolve the target's `form_key` (it can be looked up from the target name we already store).

2. **Server-side rename (follow-up #2).** In `finalizeUpload`, when the target has a form:
   - Parse `form_data` JSON into `map[string]string`.
   - `final := forms.BuildFilename(form, values, ext)` where `ext` comes from the uploaded file.
   - Run `final` through `SanitizeFilename` (`:338`) and `uniquePath` (`:316`) exactly as the
     current code does for client filenames. This replaces the client name as the rename target.
   - When the target has no form, behavior is unchanged.

3. **JSON sidecar (follow-up #1).** After the file lands at its final (possibly de-duped) path,
   write `<finalName>.json` next to it containing the submitted field values plus provenance:

   ```json
   {
     "originalFilename": "MVI_0432.mov",
     "target": "Raw camera dailies",
     "formKey": "camera_dailies",
     "fields": { "project": "PROJ", "title": "cold open", "episode": "3" },
     "uploaderId": "azure:abc123",
     "sha256": "…",
     "uploadedAt": "2026-06-17T11:40:00Z"
   }
   ```

   Reuse the existing containment check so the sidecar stays inside the target dir, and derive its
   name from the **final** de-duped file name (so `clip(1).mov` → `clip(1).mov.json`).

### Edge cases to decide at implementation time

- **Sidecar name must track the de-duped file name** — compute it *after* `uniquePath`, not before.
- **Integrity failure** — if the SHA-256 check fails and `FailUpload` runs, skip (or remove) the
  sidecar; don't leave an orphan describing a file that was rejected.
- **Sidecar write failure** — fail the whole upload, or log-and-continue leaving the file? Lean
  toward log-and-continue (the file is the primary artifact) but mark it visibly. Decide explicitly.
- **Missing/invalid form data** for a form target — reject at `handleCreated` so a bad upload never
  starts, rather than discovering it in the async finalize goroutine.

## Other design considerations (call out, don't resolve here)

- **One template ⇒ one filename.** A filename-generating form naturally implies a single file per
  submission. BCC Media's "1 file only" fits this directly. For a multi-file form target you'd need
  a numbering scheme (`_001`, `_002`); simplest first cut is to make all form targets single-file.
- **Optional fields in the template.** Define skip-when-empty: `{post}` with no value collapses
  cleanly without leaving `__` in the name. The mockup's `ARR_SUB_navn` omits the optional Post-nr
  and Type entirely, which confirms skip-when-empty.
- **Sidecar naming convention.** `<file>.json` (sibling, visible) vs `.<file>.json` (hidden) vs a
  `_meta/` subdir. Visible sibling is simplest and easiest for downstream tools to find; pick one
  and apply consistently.

## Critical files

- `internal/db/migrations/00010_add_target_form.sql` (new)
- `internal/db/queries/targets.sql`, `internal/db/queries/uploads.sql` (then `make generate`)
- `internal/forms/forms.go` (new package — registry + `BuildFilename`)
- `internal/api/admin_handlers.go` (`targetDTO`, `validateTarget`, create/update accept `form_key`)
- `internal/api/handlers.go` (`ListTargets` returns `{name, formKey}`)
- `internal/tus/hooks.go` (`handleCreated`: capture form data; `finalizeUpload`: rename + sidecar)
- `frontend/src/forms/index.ts` (new — TS mirror)
- `frontend/src/components/UploadForm.vue` (new)
- `frontend/src/components/TargetSelector.vue`, `FileUploader.vue`, `views/Home.vue`,
  `composables/useTusUpload.ts`, `types/index.ts`

## Existing utilities to reuse

- `SanitizeFilename` — `internal/tus/hooks.go:338`. Run `BuildFilename` output through it.
- `uniquePath` — `internal/tus/hooks.go:316`. Collision handling for both file and sidecar.
- Containment check pattern — `internal/tus/hooks.go:241-255`. Apply to the sidecar destination.
- `crossDeviceMove` — `internal/tus/hooks.go:278`. Already used by the rename; unchanged.
- Drag-reorder + inline-edit admin patterns — `frontend/src/components/admin/TargetsTab.vue` — the
  natural home for the new `form_key` dropdown.
- Formatting helpers (`formatBytes`, etc.) — `frontend/src/composables/adminHelpers.ts`.

## Verification

1. **Go unit tests:** `internal/forms/forms_test.go` — `BuildFilename` for both registered forms,
   including optional-field skipping and select-code resolution; unknown-key lookup returns
   `(_, false)`.
2. **Admin flow:** create/edit a target, pick a form from the dropdown, confirm `form_key`
   persists and an unknown key is rejected.
3. **Upload flow (form target):** select BCC Media, confirm the form appears, the file picker is
   gated until required fields are filled, only one file is accepted, and the preview matches
   `ARR_SUB_navn.ext`. After upload: the file in the target dir is named per the template, and a
   matching `<finalName>.json` sidecar sits next to it with the submitted fields + provenance.
4. **Upload flow (no-form target):** Audio masters behaves exactly as today — multiple files,
   sanitized original names, no sidecar.
5. **De-dup edge case:** upload two submissions that resolve to the same name; confirm the second
   becomes `name(1).ext` and its sidecar is `name(1).ext.json`.
