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

## The palette comes from admin-web

Ported from `bcc-media-platform/admin-web` (`app/assets/css/main.css`), which
carries the bcc-media-play Flutter design system. What was taken:

- **Colours.** Raw values sit on `:root` as `--ds-*`, exposed in `@theme` under
  admin-web's semantic names (`surface-default`, `text-muted`,
  `primary-default`, `semantic-error`, …) so the two apps read the same. The
  accent is a muted blue — the hue of `#2b7fff` at 70% of its chroma and a
  little darker, `oklch(0.56 0.145 259)` = **`#3c72c8`**. It is split across
  admin-web's two primary tokens rather than using one value everywhere,
  because FileBox uses the accent both as a fill and as text:

  - `primary-default` `#3c72c8` — every **fill**: button backgrounds, borders,
    focus rings, `accent-color`, and the `color-mix` tints. 3.51:1 on the
    ground, past the 3:1 floor for non-text.
  - `primary-contrast` `#6092e1` — the accent as **text**, the same hue lifted
    in lightness: 5.3:1 on the ground, where the fill manages only 3.51:1. All
    16 `color:` usages point here; fills, borders and `accent-color` stay on
    `primary-default`. This is what the two tokens mean in admin-web — their
    light theme _deepens_ the contrast variant against white, ours _lightens_
    it against black.
  - `on-primary` white — the label on a blue fill, **4.74:1**. The lower
    lightness of the muted blue is what buys this; the fully saturated
    `#2b7fff` managed only 3.76:1 and failed AA for 17px text.

  The provider tints do not follow the accent, since Azure and BCC have to stay
  distinguishable — BCC takes admin-web's `data-brand="bcc"` dark primary
  (`#a0cec8`).
  FileBox's own names are kept as **aliases onto the same values** — 537
  references across the component CSS resolve through them, so the palette moves
  in one place rather than in 537.

- **Type scale**, 1:1: `--text-heading-1` … `--text-caption-2`, each bundling
  size, line height, weight and tracking. In plain CSS reference all four
  sub-properties; Tailwind emits them as real custom properties.
- **Radii**, matching their Tailwind usage: `--radius-item` 8px (badges, list
  rows), `--radius-surface` 12px (fields, cards, popups), and the wide button
  radii 16/24/32px. Their badges are `rounded-lg`, not pills.
- **Elevation and motion**: `--shadow-resting` on surfaces that sit on the page,
  `--shadow-floating` on anything that opens over it, `--ease-out-expo` with a
  200ms duration, and `scale(0.95)` on button press.
- **`gradient-border` / `gradient-border-dark`** as `@utility` definitions,
  verbatim. A 1px gradient band painted by a masked `::before`, so it needs no
  extra element.

  **The band replaces the border — an element must not carry both.** Keeping
  both drew a solid hairline with a second, lighter one just inside it. So
  `.card`, `.menu-content`, `.select-content`, `.dialog-panel` and
  `.drawer-panel` dropped their `border` (and the drawer its edge border) when
  they took the band. `.card` carries the declarations in its own rule rather
  than the utility class, because it is applied in 21 templates — the one place
  the block is duplicated.

  `.select-trigger` has the band, and no border — see the select note below.

  `gradient-border-dark` goes on the primary button, whose fill is light enough
  that the band has to darken rather than lighten. That button keeps `.btn`'s
  1px border — its colour equals the fill, so nothing extra shows, and dropping
  it would shrink the button 2px against its siblings in an action row.

- **Archivo**, their family, fetched from Google Fonts (OFL-1.1) and
  self-hosted. It is a variable font, so one file per subset covers 100–900 —
  hence `font-weight: 100 900` and `format('woff2-variations')`.

Two gaps where FileBox has more structure than admin-web: their surface ladder
is two steps plus a translucent indent, where FileBox's components need four
(`--ds-surface-2`/`-4` fill the gaps on the same neutral); and their single
`border-1` is too faint for an input edge, so `--ds-border-2` is a stronger
hairline for control borders.

### The select panel is raised; its trigger is a field

admin-web's `DesignInput` has **no background class at all** — their text
fields are flat and transparent — while `DesignSelect` is deliberately raised:
`bg-surface-raise`, `shadow-resting`, the gradient band instead of a border,
`hover:bg-surface-indent`, a popup at the same level as the trigger, and a
highlighted row that **recesses** to `surface-indent` rather than lightening.

FileBox takes half of that, on purpose:

- **The panel follows admin-web.** `.select-content` is `surface-raise`
  (`#353836`) with the band and `shadow-floating`, and
  `.select-item[data-highlighted]` recesses to `rgb(0 0 0 / 0.5)`. Before the
  port the popup sat a step _above_ the trigger and the highlight lightened —
  both inverted from theirs.
- **The trigger follows its neighbours.** It stays in the `.inp` selector
  family, so it renders identically to the text input and the number control
  beside it in a form — same ground, 1px border, radius, `body-3` text and
  focus ring. Raising only the select made it read as a different kind of
  control in a row of fields.

The difference in reasoning: admin-web can raise its select because _all_ its
fields are flat, so the raised one is the odd one out by design. FileBox's
fields are filled, so raising one would just look inconsistent. Matching the
panel gets the depth without that cost.

`--ds-surface-indent` was added for the highlight; it was not in the original
port, since FileBox's own ladder only steps upward.

### Deliberately not ported

- **Light mode and the `data-brand` variants.** admin-web is light-first with a
  `.dark` class. FileBox is dark-only by decision; only the dark values were
  carried over. Adding light mode means re-checking every colocated component
  style against a light ground.
- **CVA + utility-class components.** admin-web expresses component styling as
  Tailwind utility strings through `cva`. FileBox uses semantic classes
  (`.btn`, `.badge`) that the `Ui*` components own — see the colocation above.
  Matching their architecture would mean rewriting all 17 components.
- **Metric-matched fallback faces for Archivo.** Those are generator output and
  none exist for it, so the sans stack falls back to `system-ui` plainly and
  text reflows slightly when the webfont swaps in. The mono family keeps its
  matched faces. Running the same generator over Archivo would close this.

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
