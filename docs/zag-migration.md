# Zag.js component migration

Tracking doc for moving hand-rolled interactive UI in `frontend/src` onto
[Zag.js](https://zagjs.com) state machines, under the shared `Ui*` prefix in
`frontend/src/components/ui/`.

Candidates are ranked by **duplication and defects**, not by how basic the
component is — a primitive that already looks right across the app buys little,
while one that is implemented twice and lacks keyboard support buys a lot.

## Conventions

- **Naming:** `Ui*` in `components/ui/`. `App*` is taken (`AppLogo.vue` is a
  brand asset), and `Ui` reads as "UI primitive" rather than naming the
  component's origin. Write `UiButton`, not `UIButton`.
- **Styling:** components consume the shared classes in
  `assets/components.css`. They must not ship their own colours, or the design
  system forks again. Scoped styles are for layout genuinely local to the
  component.
- **Layering:** `z-index` belongs on a floating surface's _content_ element, not
  its positioner — Zag's popper writes `z-index: var(--z-index)` inline on the
  positioner and fills that var from the content's computed value, so a rule on
  the positioner is dead code. See `assets/__tests__/layering.spec.ts`.
- **Formatting:** only run prettier on files that were already
  prettier-clean. Twelve files in `src/` predate the config, so a blanket
  `prettier --write "src/**/*.vue"` rewrites them wholesale and buries the
  actual change — `PackageDownloadScreen.vue` came out at +117/-22 for a
  change it was not part of.
- **Tests:** every component gets a spec, and the spec gets mutation-tested
  (break the behaviour on purpose, confirm a test fails). jsdom gaps are stubbed
  centrally in `src/test/setup.ts`.

## Done

| Component       | Package                 | Replaced                                                                                 | Notes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| --------------- | ----------------------- | ---------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `UiDialog`      | `@zag-js/dialog`        | 6 hand-rolled modal shells in `components/admin/`                                        | Escape didn't close any modal before — there were zero `Escape` handlers in `src/`. Also adds focus trap, focus restore, scroll lock, `aria-modal`. Teleporting fixed a `.fb-fade` transform creating a containing block for `position: fixed`.                                                                                                                                                                                                                                                                                                                                                               |
| `UiMenu`        | `@zag-js/menu`          | `AuthMenu` + `SendUserMenu` (merged into `UserMenu`)                                     | Mouse-only before: no arrow keys, no typeahead, no `aria-expanded`, and `position: absolute` instead of flip/shift.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| `UiToaster`     | `@zag-js/toast`         | ref+timer in `useAdmin.ts` and `SentPackagesList.vue`                                    | Durations had drifted (2400ms vs 2200ms); neither could show two toasts at once; no live region, so nothing was ever announced. Error paths were rendering with a green ✓.                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| `UiSelect`      | `@zag-js/select`        | 5 native `<select>`s                                                                     | Two of them hand-drew a chevron over the native control. Generic SFC keeps `v-model` typed, so `v-model.number` works without the modifier.                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `UiTagsInput`   | `@zag-js/tags-input`    | `RecipientChipInput.vue` + `GroupModal`'s separate implementation                        | Adds chip navigation and edit-in-place.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| `UiCollapsible` | `@zag-js/collapsible`   | `Set<number>` + `toggle()` in `ArrangementsTab.vue`                                      | First `aria-expanded`/`aria-controls` in the codebase.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| `UiFileUpload`  | `@zag-js/file-upload`   | `FileUploader.vue` + `PackageComposeForm`'s inline dropzone                              | Dropzone was a `<div>` with `@click` — unreachable by keyboard. Now reports refused drops instead of silently keeping only the first file.                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| `UiTabs`        | `@zag-js/tabs`          | `views/Admin.vue` (7 tabs) and `views/Send.vue` (3)                                      | No `role="tablist"`, `aria-selected` or `aria-controls` anywhere before, and no arrow keys — the admin bar was 7 separate tab stops, now 1 via roving tabindex. `.tabs` and `.view-tabs` collapsed into one `.tab-list`/`.tab`; their count badges were already byte-identical.                                                                                                                                                                                                                                                                                                                               |
| `UiConfirm`     | none — wraps `UiDialog` | every destructive action: 4 native `confirm()` calls plus 5 that had **no** guard at all | Promise-based `confirmAction()` keeps the `if (!ok) return` control flow. Splits each message into a question and its consequence, names the action instead of "OK", and puts Cancel first so the focus trap makes Enter harmless. Escape cancels (the safe direction); outside-click does not dismiss.                                                                                                                                                                                                                                                                                                       |
| `UiRadioGroup`  | `@zag-js/radio-group`   | `VerificationMethodPicker.vue` plus the two `.seg` switches (`UsersTab`, `GrantModal`)   | No `role="radiogroup"` anywhere before, and no arrow keys: the picker was `<button>`s with a decorative `<span class="radio">`. One machine, two skins — `card` (stacked, with descriptions) and `segmented` (the shared `.seg`). `.verify-opts`/`.verify-card` retired from send.css.                                                                                                                                                                                                                                                                                                                        |
| `UiNumberInput` | `@zag-js/number-input`  | 3 raw `type="number"` inputs (`SentPackageCard` x2, `PackageComposeForm`)                | Adds steppers, min/max clamping and arrow keys; mouse wheel deliberately off so scrolling a page cannot edit a value. Keeps the `''`-means-unlimited contract two call sites rely on, rather than emitting NaN.                                                                                                                                                                                                                                                                                                                                                                                               |
| `UiEditable`    | `@zag-js/editable`      | the two click-to-edit fields in `TargetsTab`                                             | Machine owns the edit session: Escape reverts, Enter/blur commits, focus is managed. Emits `editChange` so the row stays undraggable while its text is open. `ArrangementsTab`'s `.inline-edit` inputs are _not_ editables — always-on inputs with a Save button — and were left alone.                                                                                                                                                                                                                                                                                                                       |
| `UiTooltip`     | `@zag-js/tooltip`       | the two disabled-button reasons (`UploadsTab`, `PackageAccessRequestForm`)               | Opens on hover _and_ focus, wires `aria-describedby`, reachable on touch — a native `title` does none of that. Wrapping the trigger in a span keeps it working over a `disabled` button, which is exactly where the reason matters.                                                                                                                                                                                                                                                                                                                                                                           |
| `UiButton`      | none                    | all 54 `class="btn …"` call sites across 22 files                                        | Not a Zag machine — a consolidation. Seven buttons had each hand-rolled `{{ busy ? 'Xing…' : 'X' }}`; the `loading` prop now owns both the label swap and the disable, so a request in flight cannot be double-fired. `.lg-btn` in `LoginGate` was a fourth button skin whose `.primary` used `--color-surface` where `.btn-primary` uses `--color-accent-ink` — equal only by accident. `href`/`to` render an `<a>`/`<router-link>` instead, so the two button-shaped links stop hand-writing the classes; a disabled link gets `aria-disabled` and leaves the tab order, since an `<a>` ignores `disabled`. |
| `UiDrawer`      | `@zag-js/dialog`        | the hand-rolled shell in `UserDrawer.vue`                                                | Closed on `@click.self` and nothing else: no Escape, no `role`/`aria-modal`, no focus trap, no focus restore, no scroll lock — you could tab straight out of it into the page behind. Its shell CSS was `.admin-root`-prefixed, so the drawer was silently admin-only styling. Teleports into `.admin-root` rather than `<body>` because the content still depends on 15 prefixed classes plus the table element selectors; see docs/frontend-styling.md.                                                                                                                                                     |

## Next up

### Remaining

Only the low-value tail is left: four native `title` attributes that carry no
information a user needs — `PackageFileRow` and `SentPackageCard` (plain buttons
that also have visible labels) and two `inert` spans in
`PackageDownloadScreen`. `TargetsTab`'s drag handle is a `<td>`, which cannot be
wrapped in the trigger span without breaking table semantics, and it already
carries an `aria-label`.

### Candidates to move into `ui/`

Surveyed after the primitives pass, `UiDrawer` now done. Ranked by what
moving actually buys:

- **`TargetSelector.vue`** (97 lines, root level) — a single-select grid of
  cards, i.e. `UiRadioGroup`'s existing `card` variant with a grid layout
  instead of a stack. It currently renders `<button>`s, so no `radiogroup` role
  and no arrow keys. Move it by adding a `layout` option to `UiRadioGroup`
  rather than by writing a third card renderer.
- **`PackageUnavailableScreen.vue`** (12 lines) — the same shape as
  `.empty` plus a heading. Worth folding into a `UiEmptyState` only if a third
  caller shows up; two is not a pattern.

Recommended **against**, recorded so it is not relitigated:

- **`UiInput`** (6 candidate call sites) and **`UiCard`** (21) would wrap a
  single class with no behaviour, no state and no props. `.inp` and `.card`
  already live in `components.css`, which is the single source of truth the
  component would only forward to. That is indirection, not consolidation.
- **Staying put by design:** `AppLogo` (brand asset, not a primitive),
  `LoginGate`, `UserMenu`, `UploadForm`/`UploadList`/`UploadProgress`, and every
  `admin/*Tab`, `admin/*Modal` and `send/Package*` — those are domain
  components with one caller each.

### Deliberately not migrated

- **Clipboard copy** (`composables/useClipboard.ts`). `@zag-js/clipboard` was
  installed and then removed: its machine transitions to "copied"
  _optimistically_ and never surfaces a rejection, so it would have preserved
  the bug — `SentPackagesList` toasted "copied" whether or not anything was
  copied. The one thing it adds is a fallback for when `navigator.clipboard` is
  missing (it is undefined on **any non-HTTPS origin**, so copy silently did
  nothing there), and that fallback is ten lines. The helper does both, and
  `copyLink` now reports real failure.

### Deliberately not confirmed

- **Removing a file from a package you are composing**
  (`components/send/PackageFileRow.vue:40` → `cancelUpload`). This is editing a
  draft, not destroying anything: `useTusUpload.cancelUpload` only aborts a
  transfer still in flight, and for a completed upload it just drops the local
  row (it must not abort one, since it may already back a package). A confirm
  here would add friction to every corrected mis-drop.

### Not worth migrating

- **Progress** — `.progress` in `components.css` is a determinate bar with no
  interaction. Zag's machine adds nothing.
- **Avatar** — `avatarBg()` in `composables/adminHelpers.ts` is a
  hash-to-gradient function, no state.
- **Drag-to-reorder** — `components/admin/TargetsTab.vue:11`. Zag has no
  sortable-list machine; leave it or use a dedicated library.

## jsdom gaps worth knowing

Zag leans on browser APIs jsdom lacks. All stubbed in `src/test/setup.ts`; each
one presented as a confusing test failure rather than a clear error:

| Missing                      | Symptom                                                                                                                          |
| ---------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `ResizeObserver`             | floating-ui throws an unhandled rejection on open, leaving the machine half-wired                                                |
| `Element.prototype.scrollTo` | select aborts its open transition                                                                                                |
| `DragEvent`, `DataTransfer`  | file drops do nothing. The stub's shape is dictated by `@zag-js/file-utils`' `getFileEntries`, which walks `items` (not `files`) |

And four traps that are not jsdom's fault:

- **Zag defers to `requestAnimationFrame`** — `aria-describedby` corrects itself
  a frame after mount, and dismissable layers arm on a frame. Tests must flush
  frames, not just ticks. Every spec has a `flush()` helper.
- **Zag decides which element owns a handler.** Tabs handle keys on the
  _tablist_, not the triggers, and set `aria-controls` only on the _selected_
  trigger. Dispatching to the wrong element, or asserting the wrong invariant,
  reads as a component bug.
- **Selection-follows-focus is browser-only.** With `activationMode:
"automatic"` (the default), arrow keys move DOM focus and the newly focused
  trigger selects itself. jsdom moves the focus but never runs the follow-up, so
  the specs assert roving focus and a browser run confirms the selection.
- **Every item getter derives its own state.** `radio-group` recomputes item
  state inside `getItemProps`, `getItemControlProps` _and_
  `getItemHiddenInputProps`, so a per-item `disabled` must be passed to all of
  them — giving it only to `getItemProps` left the hidden input live and
  clickable while looking disabled.
- **A synthetic KeyboardEvent needs `cancelable: true`.** Zag refuses a
  disallowed dismissal by calling `preventDefault()` on the event, and
  `preventDefault()` on a non-cancelable event is a silent no-op. A
  non-cancelable Escape therefore closes a dialog _even with
  `closeOnEscape: false`_ — the test passes either way and the guard looks
  absent. Found by mutation-testing `UiDrawer`: the Escape mutation survived.
- **`vue-tsc` does not catch an unresolved component.** Replacing a component's
  import instead of adding one left `<UiButton>` rendering as a literal
  `<uibutton>` element, through a clean `vue-tsc -b` and 282 passing tests. Only
  the browser showed it.
- **An in-test timeout must sit below `testTimeout`.** The toast specs are
  duration-driven, so they poll wall-clock time with a ceiling of their own.
  That ceiling was 8s against vitest's 5s default, so on a loaded machine the
  test was killed before the poller could report — surfacing as an
  intermittent "Test timed out", never as an assertion. `testTimeout` is now
  15s in `vite.config.ts`. Reproduce load-sensitive flakes by running three
  suites at once, and capture the output to a file: piping it through `grep`
  for the summary line throws away the failure.
- **Drive components the way a user does.** A tags input ignores typing until it
  has seen `FOCUS`; a delimiter keystroke commits the _stored_ value, so the
  value must be typed before the delimiter; blur-commit runs through
  `trackInteractOutside`, so it needs a real outside interaction; and
  `file-upload` only honours a drop after `dragover`.
