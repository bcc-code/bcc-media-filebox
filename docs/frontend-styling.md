# Frontend styling: where it lives, and where it should go

Open question, written down after the `UiDrawer` extraction forced a design
compromise. No decision made yet — this records the evidence so the decision
can be made once.

## Where styling lives

Decided and done: **tokens global, shared primitives global, everything
component-specific in the component**.

| Home                                   | Holds                                                                                                                       | Size     |
| -------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- | -------- |
| `style.css` — Tailwind `@theme`        | The tokens. Never redeclare one elsewhere.                                                                                  | 101      |
| `assets/components.css`                | Primitives more than one component applies: `.inp`, `.card`, `.badge`, `.field`, `.section-head`, the `.card table` family. | 506      |
| `assets/send.css` / `assets/admin.css` | The page surface itself, plus classes handed across a component boundary (see below).                                       | 223 / 88 |
| `<style scoped>` in 38 components      | Everything used by exactly one component.                                                                                   | —        |

`assets/` went from 2856 lines to 918. `UiSelect` owns its 19 rules, `UiMenu`
11, `UiToaster` 10, `UiNumberInput` 10, `UiButton` 16; `SentPackageCard` 44,
`PackageDownloadScreen` 19.

Moved rules keep their `@layer components` wrapper inside the SFC. The layer
exists so a Tailwind utility can still override a primitive; dropping it would
have silently made every moved rule beat utilities.

## Two rules that decide where a rule can live

Both were learned the hard way, and both now have a test.

### 1. A class handed to a child cannot be styled by the parent

`panel-class="page"`, `preview-class="path"`, `list-class="tab-list-inset"` —
the class is applied to the **child's** element and carries the child's scope
id, so the parent's `<style scoped>` never matches it. Four rules were
colocated into a parent that could no longer reach them and the admin page lost
all its content padding. Such classes stay global, or move into the component
that renders the element. Guarded by `assets/__tests__/scoping.spec.ts`.

### 2. Scoping raises specificity, so a class family cannot be split

`<style scoped>` appends a `[data-v-x]` attribute to every selector. Move the
base rule into a component and leave its variants global, and the base starts
outranking its own variants:

- `.btn` scoped (0,2,0) beat `.btn-sm` and `.btn-primary` (0,1,0) — **every
  button lost its size and variant.** The modifiers were left behind because
  `UiButton` composes them (`` `btn-${variant}` ``), so the names never appear
  as literals for a search to find.
- `.app-nav a` scoped (0,2,1) tied with `.app-nav a.active` (0,2,1) — the
  active nav item's colour and underline came down to bundle order. Same for
  `.app-brand .mark` / `.app-brand .name`.
- Found by the guard afterwards, none of them yet visible: `.seg .seg-item[data-state='checked']`,
  `.progress .fill.warn`/`.danger`, `.summary`, `.req-note.blocked`,
  `.preparation-state.failed`, `.pkg-card.gone`, the `.pill.*` states,
  `.dropzone.dragover`, `.user-trigger.open`.

`assets/__tests__/css-ownership.spec.ts` asserts two things: no global rule
whose subject is a scoped rule's subject **plus extra classes** at equal or
lower specificity (that superset condition is what separates a real conflict
from `.field .hint` vs `.field > label`, which target different descendants),
and no dynamically-built family (`` `x-${…}` ``) with modifiers in a global
sheet.

## What is left

- `send.css` (223) and `admin.css` (88) still hold the surface wrappers, the
  boundary-crossing classes, and rules mixing classes owned by different
  components — `.pkg-card.gone` style splits that are safe today because the
  global selector is more specific, but worth tidying when those components are
  next touched.
- `.inp` stays in `components.css` until the 23 raw `<input>`s in `admin/*`
  move to `UiInput`/`UiTextarea`; both components need it, and scoped CSS
  cannot be shared.

## Self-hosted fonts

Wired up, and then silently not working for a while: only the `latin-ext`
subset had been downloaded, so every `@font-face` carried a `unicode-range`
starting at `U+0100` and the browser skipped all of them for ordinary text.
FileBox rendered in `system-ui` — which on a Mac is SF Pro, so it looked
deliberate. Both subsets are now present, 8 faces per family (Inter and
JetBrains Mono, weights 400/700, each with a metric-matched Arial/Courier
fallback).

Two traps, both of which produce no error at all:

- **Downloading a subset replaces the stylesheet, it does not extend it.** Only
  the newly downloaded subset's faces survive, so `latin` alone drops Ā/Ē/Č and
  `latin-ext` alone drops the entire alphabet. Keep both sets of faces.
- **The woff2 files must be in `public/fonts/`.** The `url('/fonts/…')` paths
  are absolute. A file under `src/assets/` is not served there — Vite's dev
  server answers the request with `index.html` and a **200**, so the face is
  discarded as an invalid font with nothing logged.

`assets/__tests__/fonts.spec.ts` guards both, plus the `:root` clash (the
generator emits `:root { --font-sans: … }`, which is unlayered and outranks the
`@theme` token, leaving it defined twice). It asserts that some `unicode-range`
covers `A a 0 space æ ø å Ā`, that every referenced woff2 exists under
`public/fonts/`, and that the `@theme` stack still names the fallback families.

## Page surfaces are unlayered, and that beats the component layer

`admin.css` and `send.css` are not in `@layer components`, and an unlayered
rule beats a layered one at _any_ specificity. So a surface-wide restyle of a
bare element silently overrides a shared primitive, on that page only:
`.admin-root button { color: inherit }` flattened `.btn-danger`'s red, and
`.send-root a { color: accent }` recoloured the whole app header on Send but not
Upload. `assets/__tests__/layering.spec.ts` now forbids `.<surface> <element>`
for `a`, `button`, `input`, `select`, `textarea`, `table`, `th`, `td`; a
qualified `element.class` like `select.inp` stays legal, as does the `*`
box-sizing reset. The replacement for a blanket rule is a named primitive —
`.link` carries the accent text-link styling.

## Whichever way it goes

- `@layer components` loses to _every_ unlayered rule — scoped component styles
  and inline styles included. Anything that must be overridable belongs in the
  layer; anything that must win must not be.
- Verify styling in a browser. jsdom loads no stylesheet, so the 282 component
  tests cannot see a CSS regression. Today's `UiDrawer` work passed the whole
  suite _and_ `vue-tsc` while rendering two buttons as bare `<uibutton>` text,
  because an import got replaced instead of added.
