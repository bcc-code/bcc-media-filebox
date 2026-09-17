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

Sequenced so each step stands alone: (3) first, on the drawer's own classes,
which also lets `UiDrawer` drop `teleportTo` and default to `<body>` like
`UiDialog`. Then (4) opportunistically. (2) needs no work.

## Whichever way it goes

- `@layer components` loses to _every_ unlayered rule — scoped component styles
  and inline styles included. Anything that must be overridable belongs in the
  layer; anything that must win must not be.
- Verify styling in a browser. jsdom loads no stylesheet, so the 282 component
  tests cannot see a CSS regression. Today's `UiDrawer` work passed the whole
  suite _and_ `vue-tsc` while rendering two buttons as bare `<uibutton>` text,
  because an import got replaced instead of added.
