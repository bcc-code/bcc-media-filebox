# Telling Upload and Share apart

Working note. First cut implemented (see "Status / next"); the rest unbuilt.

The two top-level tabs read as two bare verbs of equal weight. For freelance and volunteer
contributors shooting video and stills for BCC Media, "Upload" and "Send" — as the tabs were
then labelled — do not partition the space in everyday language. Uploading *is* sending;
sending *involves* uploading. Picking right requires knowing what the words mean *here*, which
is precisely what a first-time contributor does not have.

## Correction: the audiences are disjoint

The framing above is wrong in a way that changes the recommendation. **Share is an internal
tool** — staff getting material from the onsite file system to named recipients, who may be
inside or outside BCC Media. Upload is the contributor-facing one, bringing material in.

Note the asymmetry: *who may use* Share is restricted (staff), but *who may receive* is not
(anyone with an email address). Those are different questions, and conflating them produces
labels like "Share externally" that are wrong about the second while trying to signal the
first.

| | Upload | Share |
| --- | --- | --- |
| Who may use it | Freelancers, volunteers — outside contributors | Internal staff only |
| Who receives | BCC Media itself — no named recipient | Named people, inside or outside |
| Does the other audience need it? | Staff use it too | Contributors — essentially never |

This was never one audience weighing two options. **The best fix for a decision most users
shouldn't be asked to make is to stop asking them.** Every sentence of explanation is tuition
for a choice a freelancer has no business making, and today the nav puts that choice in front
of all of them: Share shows for every non-guest (`AppHeader.vue:12`), freelancers included.

Consequences:

- Gating Share to staff (below) outranks every copy idea in section C. Hide it from
  contributors and their nav collapses to one destination — no distinction left to explain,
  and the problem class disappears for most users.
- The Upload → Share cross-link (D8) **actively misdirects** once the audiences are disjoint:
  it points Upload's main audience at a tool that isn't theirs. Removed. It should only return
  behind the same signal that shows the tab. Share → Upload stays — a staff member on Share may
  well mean to hand material in.
- Section C copy still pays off, but for a smaller group: staff, who see both surfaces and do
  have a real choice.
- The labels cannot carry the staff-only signal. "Share" is defined by *having a named
  recipient*, not by that recipient being external, so there is no honest wording that also
  says "not for you" to a contributor. That job belongs to gating, and nothing else can do it.

## Gating Share

**Deferred to its own branch and PR.** Recorded here because it bounds what the clarity work
can achieve: an honest label for Share says it reaches named people, which is no signal at all
about who may use it. Naming cannot stand in for gating here, even partly. Until gating lands,
every contributor sees a tab that isn't theirs and nothing in the words will tell them so.

### The signal partly exists

Identifying staff needs no new identity plumbing. Three builtin groups already exist, and
`queries/grants.sql:78-80` maps each to a provider:

| Group | Matches | Grants in prod |
| --- | --- | --- |
| All BCC members | `provider = 'bcc'` (BCC Login — the wide community) | 2 |
| All bcc.media employees | `provider = 'azure'` (the staff Entra tenant) | **0** |
| All guests | `provider = 'guest'` | 1 |

Membership is directory-managed: it falls out of which IdP you signed in with, and no admin
can hand-edit it. Custom groups (`kind='custom'`) are the opposite — explicit email membership.

### The catch

**The employees group carries no grants in production.** Whoever is sending today is therefore
reaching Share some other way: a direct user grant, admin status, or — most likely — by signing
in through BCC Login rather than Entra, which puts staff in *All BCC members* alongside every
volunteer. If that's what's happening, `provider = 'azure'` is the wrong signal entirely and
gating on it would quietly cut staff off from the tool.

So the first move is not a migration, it's a question answered from data: **who actually sends
today, and under which provider?** Against prod:

```sql
SELECT u.provider,
       COUNT(DISTINCT u.id) AS senders,
       COUNT(p.id)          AS packages
FROM packages p
JOIN users u ON u.id = p.created_by_user_id
GROUP BY u.provider;
```

If the senders are all `azure`, the builtin group is the right principal. If they're `bcc`,
the distinction we need isn't in the directory at all and has to be an explicit list.

### Shape, once that's known

1. Migration: `ALTER TABLE grants ADD COLUMN can_send INTEGER NOT NULL DEFAULT 0`.
2. Seed it to match **current reality, not intended policy** — whatever the query above says is
   actually sending — so nobody loses access on deploy day. Narrowing afterwards is an admin
   action with a visible blast radius; a migration that silently revokes is not.
3. Resolve alongside target resolution in `EffectiveTargetIDs` (`api/access.go:32`), with
   admin implying it the way `admin` already implies `all_targets`.
4. Expose `canSend` on `/api/me`; gate the nav on it rather than `provider !== 'guest'`.
5. Checkbox in `GrantModal.vue`, beside the existing admin / all-targets controls.

Routing it through grants rather than a hardcoded provider check buys the escape hatch: a
freelancer who genuinely needs to send — rushes to an external colourist, say — joins a custom
group instead of requiring a code change.

## Adjacent gap: staff re-upload bytes BCC Media already has

"Sharing things from our onsite file system" is literal — the material is already on storage,
often in the very directories Upload deposits into. But Share's compose form only takes files
through the browser file picker, so a staff member sending a 40 GB folder pushes it up again
from a mounted drive. A "pick from a target" source in the compose form would skip an entire
upload pass. Separate feature from the clarity question, same user journey, and plausibly a
bigger win than any wording change.

## What's actually wrong

**1. The decision point carries no information.** The nav is two words. Every explanation we
have lives on the pages themselves (`Home.vue:245`, `Send.vue:57`) — one click *past* the
choice. The user decides with nothing, then reads the description that would have helped.

**2. The copy explains mechanics, not purpose.**

| Today | What it answers | What the user asked |
| --- | --- | --- |
| "Pick a target, fill in what it needs, add your files, then press Upload." | How do I operate this screen? | Is this the screen I want? |
| "Bundle files into a single package and send a secure download link to anyone by email." | What does this screen do? | Is this the screen I want? |

The Send line is the better of the two — it at least names an outcome (a link, by email). The
Upload line is pure procedure, and leads with **"target"**, which is internal jargon: it's a
server directory from `TARGET_N_DIR`. Nothing in it says what Upload is *for* or who ends up
with the files.

**3. Nothing distinguishes them by audience, which is the only distinction that matters.**
Both flows open with "add your files". The difference is entirely in *who receives them*, and
neither screen leads with that.

**4. A wrong guess is silent.** Pick the wrong tab and you get a plausible-looking file picker
either way. There's no moment where the product says "this probably isn't what you meant."

**5. Guests get a hidden tab with no explanation** (`AppHeader.vue:12`). Send is removed
rather than disabled, so a guest who was told "send it through FileBox" sees no Send at all.

## The distinction in contributor language

|  | Upload | Send |
| --- | --- | --- |
| Who gets the files | BCC — the archive and production teams | Specific people you name, by email |
| Do you name a recipient? | No | Yes, that's the point |
| What you must know | Which project / event it belongs to | Who should get it |
| What happens after | It's filed. You're done, no link back | Recipients get a link; it expires |
| Mental model | **Handing material in** | **Sharing with someone** |

The useful sentence pair, roughly:

- **Upload** — "Give your material to BCC, so it can be used and archived."
- **Send** — "Give files to specific people, who get a download link by email."

## Ideas

### A. Name the destination, not the action

The cheapest fix with the most leverage. A verb alone can't carry this; the noun after it can.

1. **"Upload to BCC" / "Send to people"** — minimal edit, keeps both familiar verbs, adds the
   one word that disambiguates. Probably the highest value-per-character change available.
2. **"Deliver to BCC" / "Share with people"** — stronger separation (no shared verb at all),
   bigger change to muscle memory for existing users.
3. **Keep the labels, add a sub-label** — two-line nav items, e.g. `Upload` / *to BCC* and
   `Send` / *to people*. Costs header height; works on the narrow layout only if the nav wraps.
4. **Directional icons** — a box going into a shelf vs. an arrow to a person. Supports the
   words; never carries the meaning alone.

### B. Put the explanation where the choice happens

5. **One-line descriptor under each nav item** (A3, dressed as small cards). The decision stops
   needing a click to inform itself.
6. **Hover tooltip** — cheap, and weak: no touch support, and nobody hovers to discover a thing
   they don't know exists. Not worth it alone.
7. **First-run chooser** — on first visit, a screen asking "What do you want to do?" with two
   plain-language cards, remembered afterwards (`localStorage`), changeable from the nav.
   Teaches the distinction once, properly. Risk is a stale remembered choice; mitigate by
   keeping both tabs visible afterwards.

### C. Rewrite the page copy to lead with audience

Concrete replacements for the two lines in the screenshots:

- `Home.vue:246` — "Your files go to BCC, where the media team can use and archive them. Pick
  what this material belongs to, add the files, and upload. You don't need to send anyone a
  link."
- `Send.vue:70` — "For getting files to specific people. They get an email with a download
  link — nothing is filed into BCC's archive."

Each names the recipient in the first clause, and each says what the *other* one does so a
misrouted user recognises it. Also worth replacing the **"Target"** field label (`Home.vue:252`)
with "Where should this go?" or "What is this for?" — "target" is ours, not theirs.

### D. Catch the wrong choice in flow

8. **Reciprocal escape hatches.** Under the target selector: "Need to get files to a specific
   person instead? → Send." Under Send's recipient field: "Handing material in to BCC? →
   Upload." Catches the error at the exact moment of doubt, costs two lines.
9. **Use the empty states.** A first-time visitor to Send (no sent packages) is the ideal place
   for the full two-column comparison — it's dead space today.
10. **The no-targets case is a free signal.** A contributor with no grants gets an empty target
    list, which is the strongest possible evidence they're on the wrong screen. That empty state
    should say so and point at Send, rather than showing an empty picker.
11. **Guests:** show Send disabled with a reason ("Sign in with your BCC account to send files
    to people") rather than hiding it. A missing tab is unexplainable; a disabled one isn't.

### E. Structural — remove the up-front decision

12. **Files first, destination second.** One landing screen: drop your files, *then* answer
    "Where are these going?" with two clear choices. This matches how contributors actually
    arrive ("I have 40 clips, now what") instead of demanding the taxonomy before they've done
    anything. Upload and Send become two answers to one question rather than two parallel apps.
    Biggest change here, and the only idea that makes the wrong choice structurally impossible —
    everything in A–D is mitigation of a decision we're still asking people to make cold.
13. **Send as an action, not a place.** Upload is the default screen; sending becomes something
    you do *to* files from the list. Fits "I uploaded it, now I want Ola to have it", but
    misfits sending files that should never be archived.

## Status / next

**Done — clarity pass (this branch):**

| | Before | After |
| --- | --- | --- |
| Nav / titles | Upload · Send | Upload to BCC Media · Share with people |
| Upload intro | "Pick a target, fill in what it needs…" | "Your files go to BCC Media, where the team can use and archive them…" |
| Upload field | "Target" | "Where should this go?" |
| Share intro | "Bundle files into a single package…" | "For getting files to specific people, inside or outside BCC Media…" |

Plus: "Nobody receives a download link — this is a hand-in" on Upload, a Share → Upload
cross-link, and `flex-wrap` on the header so longer labels don't overflow narrow windows.

Three decisions worth keeping:

- **"Share with people", not "Send to people".** Both halves were contested. *With people*
  survived two wrong alternatives — "to people" (vague) and "externally" (plainly false: Share
  is used to reach internal recipients too). What the label has to name is that there *is* a
  named recipient, in contrast to Upload, where the organisation itself is the destination.
  *Share* over *send* was the owner's call — the argument for keeping "send" was that the
  feature mails a copy with an expiry rather than granting standing access, and that the
  codebase says send throughout; the argument that won is that "share" is simply the word
  people reach for, and it sits further from "upload" than "send" does, which is the whole
  problem this document is about.
- **The rename stops at the user-visible staff surface.** Nav, titles, intro, the "Shared
  packages" tab, "Share package" buttons, empty states. Route (`/send`), tab query value
  (`?tab=sent`), component paths, the `send` tus target and every API field keep their names —
  renaming those is churn with no user benefit, and the target name is wired into upload
  routing. **Recipient-facing copy still says "sent"**: `PackageDownloadScreen.vue:75` and the
  mail subject/headline (`mail/templates.go:133`, `:210`). Open decision — it's a different
  audience, and changing a subject line affects threading for mail already delivered.
- **"BCC Media", not "BCC".** The files go to the media organisation; *BCC* is the wider
  community that BCC Login authenticates, and conflating them in the one label a contributor
  reads first is the wrong place to be loose.
- **No Upload → Share cross-link**, per the correction above.

**Next, in order:**

1. *(separate branch/PR)* Run the sender query, then `can_send` on grants, seeded to match who
   sends today; gate the nav on it.
2. Once gated, ask whether contributors need a nav at all — one destination needs no tabs.
3. Leave B7 (first-run chooser) and E12 (files-first) alone unless misroutes persist *after*
   gating. Both solve a decision that gating may delete outright.

## Open questions

- **Do staff sign in through Entra or BCC Login?** The employees group having zero grants
  suggests BCC Login, which would make the directory useless as a staff signal. Settles the
  whole gating design, and only data answers it.
- Does anyone outside the staff tenant legitimately need Share? If yes they need a custom group
  before gating ships, or they lose access that day.
- What share of real users are guests? If it's most of them, D11 outranks everything above it.
- Worth 20 minutes with 3 actual freelancers: "You've just filmed an event for BCC and have 20
  clips to hand over. Which tab?" Cheapest possible validation, and it settles A1 vs A2.
