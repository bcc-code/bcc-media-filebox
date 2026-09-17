# Frontend styling: where it lives, and where it should go

Open question, written down after the `UiDrawer` extraction forced a design
compromise. No decision made yet — this records the evidence so the decision
can be made once.

## Where styling lives today

Five mechanisms, all active at the same time:

| Mechanism                                            | Size                                      | Role                                                                                                                     |
| ---------------------------------------------------- | ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `style.css` — Tailwind v4 `@theme`                   | 101 lines                                 | The token source. Generates both `--color-*` vars and utilities.                                                         |
| `assets/components.css` — `@layer components`        | 1248 lines                                | Shared primitives: `.btn`, `.inp`, `.card`, `.badge`, `.tab-list`…                                                       |
| `assets/admin.css` / `assets/send.css` — page-scoped | 378 + 1027 lines; 56 + 152 prefixed rules | Page-specific styling, hand-scoped by an `.admin-root` / page-class prefix.                                              |
| `<style scoped>` in a component                      | 14 components                             | Layout local to one component. The newest shared components (`UiDialog`, `UiMenu`, `UiDrawer`) put _all_ their CSS here. |
| Inline `style=` in templates                         | 91 attributes                             | One-off nudges — margins, widths, a colour.                                                                              |

Tailwind is installed and is already the token source, but its **utilities are
used in only 5 places**. So "we use Tailwind" is true of the tokens and false of
everything else.

## What the UiDrawer work exposed

Three concrete costs, all traceable to the page-scoped prefix scheme:

1. **It dictated an architectural choice.** `UiDialog` teleports to `<body>`.
   `UiDrawer` cannot: the drawer's content depends on 15 classes plus 6
   element-selector rule groups (`.admin-root table`, `.admin-root thead th`, …)
   that only match inside `.admin-root`. Teleporting to `<body>` would render it
   unstyled, so `UiDrawer` needed a `teleportTo` prop and `UserDrawer` passes
   `.admin-root`. A styling decision reached into the component API.

2. **Prefixed rules beat scoped ones.** `.admin-root .drawer-head` and a
   component's scoped `.drawer-head[data-v-x]` are both specificity (0,2,0), so
   the winner is whichever comes later in the bundle. The old shell rules had to
   be _deleted_, not merely overridden. Same family as the earlier
   `.btn-danger` bug, where an unlayered `.admin-root button { color: inherit }`
   outranked all of `@layer components`.

3. **A component cannot style its own slot content.** Scoped CSS matches only
   elements carrying that component's `data-v` attribute, and slot content
   carries the _caller's_. So `UiDrawer` owns the backdrop, panel, sticky head
   and body padding, while `.drawer-id` / `.drawer-section` had to stay with
   `UserDrawer`. **This is the constraint that decides the question below** — it
   is a property of scoped CSS, not something a convention can fix.

## The two options

**A. Move CSS into each `.vue` file.** Retire `admin.css` / `send.css`; each
component carries its own `<style scoped>`.

- Deletes the prefix scheme, which is the thing actually causing trouble, and
  matches what the newest components already do.
- Cannot be total, because of constraint 3: any component that takes a slot
  needs its content's classes to live somewhere the caller can reach. A global
  sheet for shared layout survives either way.
- Risk: the same card/table styling copy-pasted into N components, with no
  single place to change it — which is how the four style regimes started.

**B. Go 100% Tailwind utilities.** Markup carries utilities; no component
stylesheets.

- One mechanism. No specificity ladder, no `@layer` ordering surprises, no dead
  rules, and nothing to keep in sync.
- Solves constraint 3 cleanly: utilities are written at the call site, so a
  caller styles its own slot content naturally.
- But it is a rewrite, not a migration: 2754 lines of CSS against 5 current
  utility usages. And repeated patterns need a discipline — `@apply` puts us
  back in the layering question, so the honest answer is a component per
  pattern, which is what `.btn` → `UiButton` already did.
- The 91 inline `style=` attributes are hand-rolled utilities already, so those
  convert almost for free.

## Leaning

Neither extreme. The split that falls out of the evidence:

1. **Tokens stay in `@theme`.** Already true, and it serves both options.
2. **Shared primitives stay semantic and global** (`components.css`). `.btn` was
   used 54 times before it became `UiButton`; that reuse is what made the
   consolidation mechanical. A design contract wants a name.
3. **Everything page-specific moves into the component that renders it.** The
   `.admin-root` / page-prefix scheme buys nothing that scoping does not buy
   better, and it costs the three things above. **This is the highest-value
   step: 208 prefixed rules, and it is what caused today's compromise.**
4. **One-off layout uses Tailwind utilities, not a new class and not a new
   inline `style=`.** That gives the 91 inline attributes somewhere to go.

Sequenced so each step stands alone. (4) opportunistically; (2) needs no work.

### Progress on (3)

`admin.css` is down from 56 `.admin-root` rules to 24. The first slice, done:

- **Promoted to `components.css`** (17 rules): the `.section-head` family, the
  `.name-cell` family, `.inline-edit`, and the table styling. Every caller is a
  component rather than a page, and a teleported drawer has to reach them from
  outside `.admin-root`. The table rules hang off **`.card`** instead of being
  bare element selectors — every `<table>` in the app already sits in a
  `<div class="card">`, so that is opt-in by structure with no markup change,
  and a stray table elsewhere stays unstyled.
- **Moved into `UserDrawer.vue`'s scoped block** (15 rules): `.drawer-id*`,
  `.drawer-section*`, `.access-*`, `.stat-grid`, `.stat-block*`, `.avatar-xl`.
- **`UiDrawer` now teleports to `<body>` unconditionally** and `teleportTo` is
  gone. It existed only to keep the panel inside `.admin-root`; with the CSS
  moved, every computed style is unchanged with the panel in `<body>`.

A false alarm worth recording: a name-based scan flagged 19 "shared" classes
(`.l`, `.n`, `.s`, `.name`, `.primary`, `.sub`…) apparently reused across
unrelated components. They are not — every one is nested under a block class
(`.stat-card .l`, `.stat-block .l`, `.access-block .l` are three different
rules). The names only _look_ generic because the `.admin-root` prefix made
them safe. Scoping per component preserves that safety; deleting the prefix
without scoping would not.

The remaining 24 rules are all single-component: the `.topbar` family and the
`.admin-root` surface/reset (Admin.vue, 11), the users-table extras
`.stat-strip`/`.stat-card*`/`.avatar-md`/`.user-search` (UsersTab.vue, 8), and
`.path` (TargetsTab.vue, 1). Mechanical, and none of them block anything.

`send.css` — 152 prefixed rules — has not been touched.

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

## The failure mode this keeps producing

`admin.css` and `send.css` are **unlayered**, and an unlayered rule beats
`@layer components` at _any_ specificity. So a surface-wide restyle of a bare
element silently overrides a shared primitive, on that page only:

- `.admin-root button { color: inherit }` flattened `.btn-danger`'s red.
- `.send-root a { color: var(--color-accent) }` recoloured the entire app
  header — brand, nav links, and the text inside the active pill — on Send but
  not Upload. It existed for exactly one link ("Back to Upload"), and the pill
  background still came from the layered rule, so the result was a dark pill
  with accent text and no single rule saying so.
- Two more were sitting there unfired: `.send-root .draft-restore-error button`,
  plus the whole `.admin-root table` family before it moved to `.card table`.

`assets/__tests__/layering.spec.ts` now asserts neither surface declares
`.<surface> <element>` for `a`, `button`, `input`, `select`, `textarea`,
`table`, `th` or `td`. Two shapes stay legal: the `*` box-sizing reset, and a
qualified `element.class` like `select.inp`, which refines a primitive on
elements already carrying it instead of overriding every element of that type.
The replacement for a blanket rule is a named primitive — `.link` now carries
the accent text-link styling, opt-in and layered.

## Whichever way it goes

- `@layer components` loses to _every_ unlayered rule — scoped component styles
  and inline styles included. Anything that must be overridable belongs in the
  layer; anything that must win must not be.
- Verify styling in a browser. jsdom loads no stylesheet, so the 282 component
  tests cannot see a CSS regression. Today's `UiDrawer` work passed the whole
  suite _and_ `vue-tsc` while rendering two buttons as bare `<uibutton>` text,
  because an import got replaced instead of added.
