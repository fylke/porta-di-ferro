# Porta di Ferro — Design

> **Status:** proposed, not yet agreed. Tech stack deliberately not chosen here — see §10.
>
> **This is a living document.** Everything in it is current best understanding rather than a
> commitment. Real use will overturn parts of it — a club night, the 15 November event, watching how
> another club runs theirs, or simply a better idea arriving mid-build — and that is the plan working
> rather than failing. Anything recorded as settled can be reopened; writing decisions down is meant
> to make disagreement concrete, not to freeze it.

---

## 1. What this is

A HEMA tournament application: score matches at the mat, run pools, show results. Built by two members
of **MSL — Medeltida Stridsteknik Linköping IF**, a club in Linköping affiliated with Svenska
HEMA-förbundet.

**First target:** MSL's own club-internal event, **15 November 2026**. Small, forgiving, and a real
deadline.

### Definitions

**Milestones**

Defined first, because the names are used throughout.

| Term | Meaning |
|---|---|
| **MVP** | Milestone 1 — the smallest thing that can run the 15 November event |
| **Stretch** | Milestone 2 — the rest of what a full club event needs. Later than MVP, **not optional** |
| **Future** | Milestone 3 — an unsorted idea dump. Not commitments |

**Structure**

| Term | Meaning |
|---|---|
| **Event** | A whole occasion, such as MSL's club event on 15 November. May contain several tournaments. **Not modelled in MVP** — one run of the application is one tournament |
| **Tournament** | A single self-contained competition, assigned exactly one discipline. Owns its own competitors, pools and results |
| **Discipline** | The weapon and ruleset a tournament is fought under — *open steel longsword*, for instance. Hardcoded in MVP |
| **Pool** | A group of competitors within a tournament who each fence all the others once |
| **Match** | One competitor against another. Run to 8 points or 3 minutes |
| **Exchange** | A single scoring action within a match, ending when the head referee breaks and announces the points |
| **Elimination** | The knockout stage after the pools. Milestone 2 |
| **Mat** | The physical fencing area. Matches are assigned to mats, and a pool runs on one mat |

**People**

| Term | Meaning |
|---|---|
| **Competitor** | A person competing in a tournament. Red or blue in any given match |
| **Event organizer** | Runs the event and operates the server — registers competitors, sets up tournaments, generates pools, publishes results |
| **Head referee** | Controls the match on the mat: starts and stops it, announces the points for each exchange, issues warnings. **The application records their decisions and interprets nothing** |
| **Assistant referee** | Assesses hits and signals them to the head referee. **Not represented in the app** — how many there are, and how they resolve disagreement, never reaches it |
| **Score keeper** | Operates the score keeper client, recording what the head referee announces and running the timer. The *sekretariat* of the Swedish rules |
| **Coach** | One per competitor, permitted to advise during a match. Penalties for a coach's conduct fall on their competitor. Not modelled |
| **Audience** | Spectators. Read the scoreboard and audience displays, and never interact with the app |

**Scoring**

Two things called "points" exist, and confusing them is the easiest mistake to make here:

| Term | Meaning |
|---|---|
| **Points** | Awarded in an exchange, in accordance with the ruleset in use. Usually 1 or 2 |
| **Match points** | The pool-standings value of a *result* — 9 for a win, 6 a draw, 3 a loss. **Not the same as points scored** |
| **Warning** | A penalty issued by the head referee. The second deducts a point, the third loses the match 0–8. The count resets each match |
| **Forfeit** | A match conceded, recorded 0–8 |
| **Disqualification** | A severe violation ending the match. Recorded 8–0 with no match points. Removing the competitor from the tournament is done separately, by withdrawing them |
| **Withdrawal** | A competitor leaving the tournament during the pools. Their results are voided as though they never entered |

**Application**

| Term | Meaning |
|---|---|
| **Server** | The instance running on the organizer's PC. Source of truth, and serves the web app to the clients |
| **Score keeper client** | The web client at the mat, one per mat |
| **Scoreboard** | The display surface on a secondary monitor |

### Why build it at all

The incumbents are good software with a distribution problem.

- **HEMA Scorecard** is open source, but installs via Docker Compose over PHP and MySQL, and in
  practice only works against its author's own server. The install wall is where a club organizer
  gives up.
- **HEMAGON** has no public repositories and is hosted-only, operated from Spain. Your event data
  lives on someone else's infrastructure.

So the open one can't practically be self-hosted, and the self-hostable one doesn't exist.

> **The differentiator is that a club can actually run it themselves.** Any club, any laptop, no
> external service, no account, no internet.

Testable form: **download to running tournament in under 5 minutes**, by a non-programmer, with no
prior tooling installed. Installability is an acceptance criterion, not end-stage packaging.

### Distribution

The promise above needs something to actually download, so **every release publishes install files as
assets on the repository's GitHub Releases page**. That page is the distribution channel: an organizer
downloads one file and runs it. No git clone, no build step, no toolchain.

- **MVP: a Windows installer.** The server runs on the organizer's PC, which in practice means Windows.
- **Linux install arrives with the web/cloud server** (Milestone 3), where a Linux host becomes the
  normal deployment target rather than an unusual one.
- **A plain install file on every platform.** A container image is a fallback only where a native
  installer genuinely isn't practical — some cloud hosts — and never the primary path. The governing
  principle is the easiest possible installation, and "install Docker first" is exactly the wall this
  project exists to avoid.
- macOS is not currently planned. Worth revisiting if the packaging turns out to be near-free once the
  stack is chosen.

Two properties matter beyond simply having a download:

- **Versioned and pinnable.** An organizer should be able to take a specific known-good build a week
  before their event and not have it move underneath them. Nobody wants to discover a regression the
  morning of a tournament.
- **Release notes written for organizers, not developers.** The audience is a club volunteer deciding
  whether to upgrade before Saturday.

The exact packaging format follows from the stack decision (§10) — but the requirement that a
downloadable installer exists does not, and it constrains that choice.

---

## 2. Decisions

| # | Decision |
|---|---|
| 1 | **The score keeper client records the head referee's final decision.** It does not capture individual judge signals |
| 2 | **Hardcoded MSL rules for MVP.** Data-driven rulesets are a future milestone |
| 3 | **Venue LAN** for MVP and stretch. Organizer's PC is the server and the source of truth. Cloud is a future milestone |
| 4 | **Web clients** — standard web, so any modern browser. Android is the tested target for MVP. Nothing to install on a client |
| 5 | **Exchange log**, append-only, with timestamps. Score is derived, never stored directly |
| 6 | **One writer per match.** Handover is a stretch goal |
| 7 | **Competitors entered locally.** No public registration, no payments |
| 8 | **Local JSON files** as the database. The organizer owns and can read their data |
| 9 | **Club-agnostic.** Nothing hardcoded to MSL except the ruleset in MVP |
| 10 | **MIT licensed** |
| 11 | **English UI for MVP**, Swedish localisation a stretch goal. **Internal identifiers are English** regardless |

---

## 3. Architecture

### Three surfaces

| Surface | Runs on | Role |
|---|---|---|
| **Server** | Organizer's PC | Source of truth; competitor registration, tournament setup, pool generation, results |
| **Score keeper client** | Tablet or phone at the mat | The score keeper's tool — the *sekretariat* of the Swedish rules. Scores one match at a time |
| **Display** | Any browser on the LAN | Live scoreboard for one mat, both mats, or the match roster |

Clients join over the venue LAN via a printed URL or QR code. **The server also serves the web app
itself** — at a venue with no internet, that's how a device that has never opened it gets it.

### Displays

A display is just a page the server serves, so **what renders it is not an application concern**:

| URL | Shows |
|---|---|
| `/display/mat/1`, `/display/mat/2` | The live scoreboard for one mat |
| `/display/mats` | Every active mat on a single screen |
| `/display/mats?ids=1,2` | A chosen subset of mats |
| `/display/roster` | The match roster |

**URL addressing is permanent, not an MVP stopgap.** Server-assigned displays (Milestone 2) are a
convenience layer *on top of* these URLs, never a replacement for them. Being able to point any
browser at a known address stays useful indefinitely — for setting a screen up in seconds, for
checking what a display should be showing when it isn't, and for later consumers such as the
streaming overlay, which is just another page reading the same data.

Display URLs are **unauthenticated and read-only**. Nothing here is secret: it is the same
information the audience is already watching on a screen. That is safe on a venue LAN and needs
revisiting only if the cloud server (Milestone 3) ever exposes an event to the open internet, where
the question becomes whether an event should be publicly viewable rather than whether it is secret.

Open the URL in a browser and fullscreen it. A second monitor on the organizer's PC, a spare laptop
beside the mat, a Raspberry Pi, an old tablet, or a venue TV with a built-in browser are all the same
thing to the application. **MVP and the "displays away from the server PC" stretch case are therefore
the same feature** — the difference is only what hardware the organizer plugs in.

**The combined `/display/mats` view is what keeps MVP hardware-cheap.** If the organizers can find one
spot visible from both mats, the whole event needs a single extra screen on a single video output,
which almost any laptop already has. The trade is legibility: two mats sharing a screen means each
gets half the width, so the display has to be closer, larger, or both.

**Scaling the combined view past two mats** is a layout problem, not a plumbing one. Four full
scoreboards in a 2×2 grid are unreadable from across a hall, so the view adapts to how many mats it
is asked to show:

| Mats shown | Layout |
|---|---|
| 1 | The full scoreboard |
| 2 | Two full scoreboards side by side |
| 3 or more | **One compact row per mat** — the two competitors, the score, the time — like a departures board. Wide and short rows stay legible at a distance in a way that shrunken scoreboards do not |

The `?ids=` parameter matters more as mats are added, because it maps onto how a hall is actually
laid out: a screen between mats 1 and 2 shows those two full-size, and another between mats 3 and 4
shows the other pair. That beats one screen trying to serve the whole hall.

*(A rotating carousel cycling one mat at a time is the obvious alternative. Rejected as the default:
it keeps type large but guarantees the mat someone cares about is off-screen exactly when they look.)*

That matters because **the MVP must not depend on the server PC having several video outputs.** The
workarounds for a one-HDMI laptop are real but each has a catch:

| Approach | Catch |
|---|---|
| USB-C with DisplayPort Alt Mode | A genuine second output, if the laptop's port supports it. Many don't |
| USB-C dock with two HDMI | Usually relies on MST; laptops without MST support silently mirror instead |
| DisplayLink adapter | Works almost anywhere, but needs a driver installed |
| DisplayPort daisy-chain | Needs the first monitor to have a DP *output*. Rare outside business monitors |

None of this is worth fighting, because **any spare computer on the LAN is also a display**. That
sidesteps cable length too: passive HDMI is dependable to roughly 10–15 m, and mats in a sports hall
are easily further from the organizer's desk than that.

**What a display shows**

- **During a match** — competitor names and colours, scores, warning triangles, the match time, and
  the winner with final scores once decided.
- **Between matches** — the next match on that mat. Pool standings were considered and dropped:
  there is never enough time between matches for anyone to read them. The **on-deck list**
  (Milestone 2) is the richer version of this.
- The device must be **kept awake** with a screen wake lock.

**Not Google Cast.** Cast devices generally need an internet connection to set up and work reliably,
which collides directly with a LAN-only venue, and tab casting adds latency and softens text — the
opposite of what a scoreboard needs. A small networked device at each monitor beats it on every axis
that matters here.

**Not a phone driving its own monitor either.** Some Android devices can output video over USB-C via
DisplayPort Alt Mode, but Chrome on Android *mirrors* — a web page cannot render different content to
an attached display. Showing the score keeper view on the phone and a scoreboard on the monitor would
need Android's native `Presentation` API, so it is only reachable by abandoning web-only clients
(decision 4). Not worth it: a dedicated cheap device per monitor is simpler, and cannot take the score
keeper's screen down with it when it fails.

### Sync

The network is never in the scoring path. A tap writes to the score keeper client's own local log and
returns immediately; pushing to the server is asynchronous. The score keeper never waits on the LAN.

Each match is an **append-only log with exactly one writer**, so there is nothing to merge — the
server orders and stores. This is log shipping, not distributed consensus. Events carry a per-match
sequence number; reconnecting means *"here is everything after sequence N"*. The primary key is
`(match_id, sequence)`, so retries are idempotent and need no deduplication logic.

**Push after every confirmed exchange**, not at match end, so a lost device costs at most one
exchange.

Corrections are appended as new events rather than mutating history. MVP has no correction UI (§7),
but building the log this way means adding one later is a UI change rather than a data migration.

### The exchange log

Recorded per match:

- every confirmed exchange — timestamp, points awarded to each competitor
- every warning — timestamp, competitor
- **timer events** — started, stopped at timestamp X, resumed after Y seconds

Nothing else. This is enough to reconstruct a match completely and to produce post-event statistics
later without changing the schema.

Because it stores each competitor's **raw assessed value** rather than the resulting score, the
differential-versus-additive scoring question (§5) is purely an engine concern — matches recorded
under one mode stay fully interpretable under the other, with no data migration.

---

## 4. The score keeper view

The most important screen in the application. Large, unambiguous buttons; usable at a glance under
time pressure.

**Tablet, landscape**

```
+--------------------------------------------------------------------+
|    +--------+                                            +---+     |
|    |  UNDO  |                                            |...|     |
|    +--------+                                            +---+     |
|                                                                    |
|          RED                                         BLUE          |
|    Competitor name                             Competitor name     |
|                                                                    |
|           5  /!\ /!\          02:47                   3            |
|                                                                    |
|    ################                            +--------------+    |
|    #      2       #                            |      2       |    |
|    ################    +------------------+    +--------------+    |
|    +--------------+    |   PLAY / PAUSE   |    +--------------+    |
|    |      1       |    +------------------+    |      1       |    |
|    +--------------+                            +--------------+    |
|                                                                    |
|      +----------+                                +----------+      |
|      | WARNING! |                                | WARNING! |      |
|      +----------+                                +----------+      |
|                                                                    |
|    +----------------------------------------------------------+    |
|    |                     CONFIRM EXCHANGE                     |    |
|    +----------------------------------------------------------+    |
|                                                                    |
+--------------------------------------------------------------------+
```

Boxes drawn with `#` are selected; `+--+` are unselected.

**Phone, portrait**

```
+--------------------------------------------+
|   +--------+                       +---+   |
|   |  UNDO  |                       |...|   |
|   +--------+                       +---+   |
|                                            |
|                   02:47                    |
|                                            |
|            +------------------+            |
|            |   PLAY / PAUSE   |            |
|            +------------------+            |
|                                            |
|          RED                 BLUE          |
|    Competitor name     Competitor name     |
|                                            |
|           5  /!\ /!\          3            |
|                                            |
|   ##################  +----------------+   |
|   #       2        #  |       2        |   |
|   ##################  +----------------+   |
|   +----------------+  +----------------+   |
|   |       1        |  |       1        |   |
|   +----------------+  +----------------+   |
|     +------------+      +------------+     |
|     |  WARNING!  |      |  WARNING!  |     |
|     +------------+      +------------+     |
|                                            |
|   +------------------------------------+   |
|   |          CONFIRM EXCHANGE          |   |
|   +------------------------------------+   |
|                                            |
+--------------------------------------------+
```

The layout adapts to the device rather than shrinking one design onto a smaller screen. On a
phone the timer and its controls move to the top and the two competitors sit closer together
beneath, with *Confirm exchange* spanning the full width at the bottom.

The ordering follows from how often each control is used: **the most-pressed control belongs
where the thumb already is.** *Confirm exchange* is pressed every single exchange, so it takes
the bottom of the screen. Start and stop are pressed once per match plus the occasional
time-out, making them the least-used controls and the right ones to exile to the top.

**Red stays on the left and blue on the right in every layout.** That mapping mirrors the mat
and must never move, whatever the screen size — swapping sides is a deliberate action (§7), not
something a device rotation should do.

**Behaviour**

- Red competitor on one side, blue on the other, mirroring the wristbands. **Sides are fixed in MVP**;
  swapping them is a stretch goal.
- Each side shows current score, a **2-point** button, a **1-point** button, and a **warning** button.
- **Confirmed warnings show as a warning triangle beside that competitor's score** — one triangle per
  warning. The count is what matters, not merely that a warning exists: the score keeper needs to see
  at a glance whether the next warning costs a point or ends the match. Provisional, unconfirmed
  warnings never appear here.
- **Nothing takes effect until *Confirm exchange*.** Points and warnings alike are only selections
  until then — the score doesn't move, the warning isn't counted, and nothing is written to the log.

**Selection model**, per competitor:

- The two point buttons are **mutually exclusive**. Selecting one deselects the other, so a competitor
  holds at most one of {1, 2}.
- **Pressing an already-selected button deselects it.** True of the point buttons and the warning
  button alike, so any mis-tap is undone by tapping it again.
- The **warning toggles independently** of the points — a competitor can be given points and a warning in
  the same exchange, or either alone.
- **Selected state must be unmistakable at a glance.** A selected warning shows red or equally
  prominent; a selected point button is clearly distinct from an unselected one. The score keeper has
  to be able to check the state in the moment before confirming, without studying it.

This also enforces a rule for free: **a competitor can be awarded at most 2 points in one exchange**,
which is exactly the ruleset's maximum for a single hit.

**On confirmation**

- Both competitors can be selected before confirming. Scoring is **differential** (§5), so selecting 2
  for red and 1 for blue awards **+1 to red** — afterblows and doubles need no special handling
  because they net out.
- **Confirming with nothing selected records a no-score exchange.** An exchange where neither competitor
  scored is a real event and is logged as such, not discarded.
- **A third warning ends the match**, so it asks for confirmation before committing (§5).

**Timer**

- **Counts up from 00:00**, toward the 3-minute match time.
- At **02:50 — ten seconds remaining — it flashes** to signal the final exchange. Flashing rather
  than a static colour change, to catch the eye of a score keeper who is watching the mat. Needs real
  testing.
- **It does not stop at 03:00.** It keeps ticking past the limit until the **final exchange is
  confirmed**, which is what ends the match. A completed match's clock therefore routinely reads more
  than three minutes.
- It is **not paused for scoring**, matching the ruleset. The timer control exists for the head
  referee's time-outs, not for ordinary exchanges.
- **One play/pause toggle**, generously sized. It is the only control that must be hit fast, so it is
  among the largest on the screen.
- **The time readout is large too**, not just its button. The score keeper is watching the mat, so the
  clock must be readable at a glance rather than looked at directly — prominent, but **not
  oppressively so**. Giving the number half the screen starves the scoring controls, which matter just
  as much.

**Colour**

Each competitor's half of the screen is tinted with their colour — red one side, blue the other —
matching the wristbands, so the score keeper's eye lands on the right half without reading anything.

Two collisions fall out of that, and both are resolved by one rule: **hue means identity, never
state.**

- *A selected warning cannot be red*, because the red competitor's side is already red. **Selection is
  shown by fill and border weight, not by hue** — the same treatment on both sides, so a selected
  button looks selected regardless of which competitor it belongs to.
- *The timer turning red at 02:50 is the one place red doesn't mean the red competitor.* It works because
  the timer sits in the neutral centre column and the whole area floods at once, which reads as an
  alarm rather than as identity. Keep it a full-area change rather than colouring the digits alone.

Warning triangles and the warning buttons use **amber**, distinct from both competitor colours and
conventional for the meaning.

Accessibility work is a future milestone, but the layout already carries the redundancy that matters:
fixed sides, RED and BLUE labels, and competitor names. Colour is never the only cue.

The scoreboard and audience displays use the same colour language, so a spectator glancing between
screens doesn't have to relearn it.

### Why atomic per-exchange commit

1. **Clean logs.**
2. **The final-exchange edge case is handled for free** — see below.
3. **One screen update per exchange, not three.** Dealing points one at a time has been observed to
   confuse a head referee mid-match.
4. **Easier to read the whole exchange back** when echoing it to the referee.
5. **Far fewer score keeper errors** — no awarding 3 when 2 was meant, no forgetting whether blue was
   already given a point.

### Match-ending dialogs

**Final exchange.** Once the clock is past the final-exchange threshold, confirming an exchange raises
a dialog:

- **End match** — the match ends, and **the timestamp of that final confirmed exchange is the match end
  time**. The running clock is disregarded from that point.
- **Continue one more exchange** — play continues, and the dialog appears again after the next
  confirmation.

The clock keeps running in the background throughout; nothing special happens to the timer itself. The
decision is only ever about *which confirmation ends the match*.

This is what "handled for free" means: because scoring commits atomically per exchange, there is always
a well-defined moment to ask and a well-defined timestamp to record.

**Point cap.** Confirming an exchange that takes either competitor **to or past 8** raises a dialog
announcing the result — *"Red wins 9–3"* — with **End match** or **Undo last exchange**.

**Warning cap.** A warning that would take a competitor to the match-loss level raises the same dialog.

The three share a shape but **not their second action** — build them as one component parameterised on
it, not as one identical dialog:

| Trigger | Choices | Why |
|---|---|---|
| Final exchange (past time) | **End match** / **Continue one more exchange** | Play may legitimately continue; the head referee decides |
| Point cap reached | **End match** / **Undo last exchange** | The rules end it, so the only alternative is that the entry was a mistake |
| Warning cap reached | **End match** / **Undo last exchange** | Same |

After time expires, continuing is a legitimate call. After either cap the match is over by rule, and
the second option exists only as a safety net against a mis-tap. If one confirmation trips both
conditions, **the cap takes precedence**.

**Overshoot:** at 7, a 2-versus-nothing exchange gives 9. **Record the actual 9 rather than clamping to
8** — point difference feeds two of the four ranking indices, so clamping would quietly distort the
standings. That leaves forfeits (recorded 8–0) as the only place 8 is a hard number.

### Corner controls — rare, destructive, out of the way

Two controls sit outside the main grid, because they are unusual and damaging and should not compete
for space with per-exchange controls:

- **Undo — upper left.** Confirmation dialog before it applies.
- **Severe warnings — a `…` overflow menu, upper right.** Immediate escalation to a double warning, a
  match loss, or disqualification is rare and needs no immediate access. **No confirmation dialog.**

**The `…` menu is the home for rare per-match controls generally**. It holds only severe warnings at first; later it gains the colour and side options from
Milestone 2. Establishing the slot now avoids reopening a deliberately full grid to make room later.

**Severe warnings commit through *Confirm exchange*, with no extra dialog.** Choosing one from the menu
makes it a pending selection; it commits with the rest of the exchange. Reaching into a buried menu is
already deliberate and the normal confirm is the second gate.

**A pending severe warning shows on that competitor's warning button.** Labels carry their own
emphasis, with the exclamation count matching the level:

| Level | Label |
|---|---|
| 1 | `WARNING!` |
| 2 | `DOUBLE!!` |
| 3 | `TRIPLE!!!` |

The punctuation encodes severity, so the label reads as more alarming exactly as the consequence gets
worse — a third cue alongside colour and size.

**That button is also how a severe warning is cancelled**, following the toggle rule used everywhere
else:

| Warning button state | Tap does |
|---|---|
| `DOUBLE!!` / `TRIPLE!!!`, selected | Clears it; reverts to an ordinary, unselected `WARNING!` |
| `WARNING!`, unselected | Selects an ordinary single warning |
| `WARNING!`, selected | Deselects it |

Cancelling a mis-picked severe warning never means going back into the menu.

### Layout direction

Agreed in review, and **to be settled by prototyping rather than on paper**:

- **Shrink the warning buttons.** Rarely used, currently equal in area to the scoring controls.
- **Remove dead space** — an edge-to-edge grid where every region is clickable.
- **Strong press feedback**: the whole region changes hue, not a subtle border.
- **Point buttons follow the ruleset.** A rapier thrust worth 3 appears as a 3 button. Prevents the
  failure Holmgång had where competitors forgot their own written rule — and pulls a slice of
  data-driven rules earlier than Milestone 3 assumed.
- **Themed backgrounds**: red's region red-tinted, blue's blue, warning areas amber, **with buttons
  kept clearly contrasted against them**. This has to coexist with selection being shown by fill and
  border rather than hue — selected and unselected must stay obviously different *on top of* a themed
  background, which only a real prototype will settle.

**Build two or three variants and test them.** The authors' instincts differ — bordered buttons versus
edge-to-edge clickable panels — which is a good reason to try both. If more than one works, ship them
as a score keeper preference.

---

## 5. MVP ruleset (hardcoded)

MSL's SM ruleset. Longsword scoring is used for all weapons at this stage.

| Rule | Value |
|---|---|
| Point values | 1 or 2, as announced by the head referee |
| Exchange scoring | **Differential** — the difference between the two assessments is awarded |
| Point cap | 8 |
| Match time | 3 minutes |
| Final-exchange warning | 10 seconds remaining (02:50) |
| Result types | Win / loss / **draw** (draws are possible in pools) |
| Pool match points | Win **9**, draw **6**, loss **3** |
| Forfeit | Recorded 0–8; winner takes 9 match points, forfeiter 0 |
| Withdrawal during pools | Treated as if the competitor never participated — results retroactively voided |
| Red / blue assignment | Fixed at pool creation, for every match in the pool |

### Exchange scoring is differential

**The difference between the two competitors' hits is awarded, not both values.** A 2 against a 1
gives the winner **1 point** and the other **nothing**. A 2 against a 2 gives **nobody anything**.
This is the mechanism that makes afterblows and doubles self-handling — there is no special case for
them, they simply net out.

Two consequences:

- **The score keeper's selections are inputs, not outcomes.** Tapping 2 for red and 1 for blue applies
  **+1 to red**. What is tapped is no longer what is added, which the prototypes should account for.
- **The log stays raw.** It records each competitor's assessed value; the net is derived, per
  decision 5.

**Scoring mode is a ruleset parameter**, deferred to the data-driven work in Milestone 3. Many
tournaments instead use a **12-point cap with additive scoring**, where a 2–1 exchange raises *both*
scores. That is different arithmetic, not just different numbers.

### Warnings

**Warnings** escalate automatically, per competitor, and the count **resets each match**:

| Warning | Consequence |
|---|---|
| First | Recorded and displayed. No score effect |
| Second | **Point deduction** — one point off the warned competitor |
| Third | **Match loss, 0–8** against the warned competitor, earning them no match points |

Because the third warning ends the match, the score keeper view confirms before committing it.

**A tournament-level warning count runs alongside the per-match one.** The per-match level governs
in-match consequences and resets every match, but the running total across the tournament is recorded
and visible, so a competitor collecting warnings match after match is apparent to the organizer rather
than invisible by design.

In MVP the ladder only ever advances one step at a time. **Immediate escalation is a stretch goal**
(Milestone 2, item 2): a head referee may judge a violation severe enough to jump straight to a point
deduction, a lost match, or disqualification without passing through the earlier steps. Mechanically
this is just **applying more than one warning in a single exchange** — the level model below already
expresses it, so no separate penalty concept is needed.

That is best modelled as a **penalty level per competitor per match** rather than a count of warnings:

| Level | Effect |
|---|---|
| 0 | Clean |
| 1 | Warning — recorded, no score effect |
| 2 | Point deduction — one point off |
| 3 | Match lost 0–8, no match points |

The warning button advances the level by one. Immediate escalation advance it two or three levels
respectively
**Disqualification is match-scoped while the rules are hardcoded** — that is, throughout MVP and
Stretch. It is recorded exactly as level 3 is: **the match lost 0–8, with no match points.** The
application does not model removal from the tournament, consistent with decision 1 — it records the
outcome the head referee announced and interprets nothing beyond it.

If the competitor genuinely leaves the event, the organizer marks them **withdrawn**, which already
voids their results as though they never participated. The two mechanisms compose: disqualify the
match, then withdraw the competitor. Nothing extra needs building, and the ranking indices stay
correct because withdrawal removes them from the arithmetic entirely.

Note that a penalty loss earns **no match points**, unlike an ordinary loss which is worth 3. That
applies to level 3 and disqualification alike.

**Once rules are data-driven** (Milestone 3), the treatment of a disqualification becomes an organizer
choice — score it as a match loss, or void the competitor retroactively as MSL's written rule does.

**Pool ranking**, in order, all divided by matches *completed*:

1. Match point index — match points ÷ matches completed
2. Victory index — wins ÷ matches completed
3. Score index — (points scored − points conceded) ÷ matches completed
4. Reception index — points conceded ÷ matches completed (**lowest** wins)
5. Head-to-head result
6. Random draw

Dividing by *completed* matches is what makes retroactive withdrawal work correctly.

---

## 6. Milestone 1 — MVP: Club Event Basics

Target: run the 15 November club event.

1. **Server (organizer) + score keeper clients** over the LAN.
2. **Hardcoded MSL rules** per §5.
3. **Score keeper view** per §4.
4. **Up to 2 mats** concurrently.
5. **Up to 4 pools, up to 7 competitors each** — a ceiling of 28 competitors per run.
6. **Pools only** — no eliminations.
7. **Server views:**
   - **7.1 Competitors** — registration and status. Stored as a local JSON database file.
   - **7.2 Tournament** — number of mats, min/max competitors per pool, generate pools.
   - **7.3 Pools** — generated matches per pool with status; results as JSON.
8. **Displays, addressed by URL** — `/display/mat/N` for one mat, `/display/mats` for both on a
   single screen, `/display/roster` for the match roster. Rendered by whatever is convenient: a
   second monitor on the server PC, or any spare machine on the LAN. Between matches a mat display
   shows the next match on that mat.
9. **Pool generation** honouring min/max size, with **uneven pool sizes accepted**, and match ordering
   that minimises consecutive matches on a best-effort basis, **reporting any remaining violations**
   rather than guaranteeing none. Generation also **assigns red and blue for every match**, aiming to
   give each competitor a roughly even split across their own matches — best-effort, like the ordering.
10. **Export results as JSON.**
11. **Printable pool sheets** as a paper fallback, listing each match with its assigned colours.
12. **English UI.**
13. **Undo of the last confirmed exchange.** Moved into MVP during review: it is what makes the
    confirm-step model acceptable, and both authors treated easy correction as essential.
14. **A Windows installer published to GitHub Releases.** Listed as a deliverable rather than assumed,
    because it is the acceptance criterion for the whole premise (§1).

### Mat assignment

Fixed and predictable, because confusion at the mat costs more than throughput:

- **Mat 1 runs pools 1 and 3. Mat 2 runs pools 2 and 4.** Odd pools to mat 1, even to mat 2.
- The **first two pools start together**. When a pool finishes, that mat picks up its next pool.
- **No organizer override in MVP** — the mapping is fixed. Override is a stretch goal.

### Running several disciplines

MVP has no concept of disciplines or divisions, and caps at 28 competitors. Multiple disciplines are
handled by **starting a separate run of the application per discipline** — up to four, each with its
own JSON database.

**MVP runs them sequentially, one at a time.** Running disciplines concurrently is a stretch goal,
and only then does the question of separate ports and data directories arise.

### Accepted MVP limitations

Deliberate, and listed so nobody is surprised on the day:

- **Correction is one step deep.** Undo covers the *last* confirmed exchange (item 13). An error
  noticed several exchanges later still has no route in the UI, and the escape hatch remains the
  **local JSON the organizer can hand-edit**. Full history editing stays out of MVP.
- **No eliminations, no finals.** Pools produce a ranking; anything beyond that is run on paper.
- **No handover.** If a score keeper client dies mid-match, the match is re-entered.

---

## 7. Milestone 2 — Club Event Stretch Goals

1. **Full history editing** — correct any exchange in a finished or running match, not just the last
   one. Undo of the last exchange is in MVP; this is the deeper version.
2. **Immediate penalty escalation** — the head referee may jump straight to a point deduction, a lost
   match, or disqualification. Reached through a penalty menu on the warning control rather than more
   buttons on the main view, with confirmation on every level above a plain warning.
3. **Eliminations** — top 8 from the pools.
4. **Server-assigned displays** — a device opens `/display` and the organizer chooses what it shows,
   reassigning on the fly and seeing which screens are live. Added *alongside* URL addressing, which
   remains supported. Cheap by this point, because the connected-client registry already exists for
   handover (item 10).
5. **Audience display**, a richer variant of the mat display:
   - the **winner and final scores, prominently**, when a match is decided
   - the **upcoming match** — competitor names, colour-coded red and blue — in a smaller but still clearly
     legible font
   - an **"on deck" panel down the side** listing the next 3–5 matches, with red and blue background
     colour-coding per competitor. During pools this is what tells a competitor whether there is time
     to refill a water bottle or take a jacket off
6. **Competitor colour and side options**, reached from the `…` overflow menu (§4): change the
   competitors' colours away from the red/blue default, and swap which side each occupies — on the
   score keeper view and on the display **independently of each other**. Swapping sides risks
   cognitive dissonance against the physical corners, so changing colours is often the better answer
   to the same problem.
7. **Up to 4 mats, up to 8 pools** — 56 competitors per run. Mat assignment generalises to pool *N* on
   mat *((N−1) mod mats) + 1*.
8. **Organizer override of mat assignment.**
9. **Concurrent disciplines** — several runs at once, which requires a distinct port and data
   directory per instance.
10. **Score keeper client handover** — graceful (planned: bathroom break, shift change) and ungraceful
   (device died). Graceful flushes before releasing so nothing is lost; ungraceful increments a
   writer epoch, and any late events from the old device are quarantined and shown to the organizer
   rather than silently dropped.
11. **Swedish localisation** alongside English.
12. **PDF export** alongside JSON.
13. **Club balancing in pool generation** — distribute competitors from the same club as evenly as
    possible (issue #3). No effect at a club-internal event, so it needs synthetic testing.

---

## 8. Milestone 3 — Future

Deliberately unrefined. An idea dump to be sorted later, not a commitment. Grouped only to stay
navigable.

**Three structural notes.** Most competitor-facing entries below need a **persistent identity and data
that outlives a single event**, which is a real departure from MVP's one-JSON-file-per-tournament
model rather than a feature bolted onto it. Most of them also need the cloud server (1), because they
assume something reachable before and after the event, not a laptop switched on that morning.

Third, and more limiting: **anything requiring cross-club data is structurally out of reach.** A
self-hosted server only ever holds the events its own club ran, so league-wide ratings and a global
opponent database — the things a centralised platform does well — cannot work here without becoming
the very central service this project rejects. Career statistics, opponent history and achievements
are therefore scoped to whatever a given server has seen. **The escape hatch is HEMA Ratings export**: exporting to
HEMA Ratings feeds the shared community database without Porta di Ferro having to be a platform, and
that is the right division of labour.

### Deployment and access

1. **Web/cloud server** — an easily deployed droplet or a web server on a PC, with clients connecting
   over the internet rather than the LAN. **A Linux install ships alongside the Windows one** from
   this point, since a Linux host becomes the normal deployment target.
2. **Observer role.**
3. **Per-mat scoreboard clients** driving their own monitors, possibly handhelds.

### Before the event

4. **Events divided into tournaments**, with organizer options.
5. **Event page** — one public page carrying everything about an event: venue, schedule, rules,
   entry lists, and the pools once drawn.
6. **Participant self-registration** — one-click entry to an event and to individual tournaments per
   discipline.
7. **Entry administration** — approve applications, track payment and attendance, assign categories,
   and seed entrants. The organizer's desk work before a competitor ever reaches a mat.
8. **Disciplines** — weapon and ruleset selection per tournament (issues #2, #4).
9. **Categories** — open, women's, novice and so on, as an axis separate from discipline.
10. **Touch-friendly pool building** — drag competitors between pools on a phone or a PC, with
    filters and seeding, rather than only accepting what the generator produced.
11. **Publish pools and structures ahead of the event**, so competitors arrive knowing their group.

### Rules and formats

12. **Fully data-driven rulesets** — replacing the hardcoded MVP rules. Includes making the
    disqualification policy an organizer choice: score it as a match loss, or void the competitor
    retroactively as MSL's written rule does.
13. **Finals — best of three**, won by two wins or one win and two draws, then sudden death.
14. **Configurable elimination cut** — how many advance, set when organizing the tournament.
15. **Team events.**
16. **Non-match events** — cutting, forms, solo.

### During the event

17. **Staff** — roles, availability, assignment, and the *pliktdomarsystem* under which competing
    obliges you to staff another discipline (issue #5).
18. **Timetable** — generation, with personalised per-competitor schedules (issue #6).
19. **Exchange feedback on displays** — a per-tournament option for how the result of an exchange is
    shown, so a watcher can tell what just happened rather than only seeing a number change.

    | Mode | Behaviour |
    |---|---|
    | **None** | Current score only. What most existing apps do, and the default |
    | **Parenthesised delta** | The change precedes the score — `(+1) 5`, `(+2) 5`, `(−1) 4`, nothing if the score did not move. Clears after a configurable time, **default 5 seconds**, replaced immediately if another exchange lands first. Duration configurable including infinite, which still yields to the next exchange |
    | **Floating delta** | A transient `+1`, `+2` or `−1` drifts beside the score and fades, the way games show XP gains. Configurable time and speed |

    **Deductions are shown too, not only gains** — a warning costing a point produces `(−1)`. This is
    the case that needs it most: a score going *up* is self-explanatory, while a score going *down*
    with nothing to explain it is baffling to watch. In the floating mode deductions should drift
    **downward** and gains upward, so direction carries the sign as well as the glyph.

    **Hard layout constraint for the parenthesised mode: the score must not move.** Reserve the
    delta's space permanently and show it empty when idle. A score that shifts sideways every
    exchange is worse than no feedback at all, and laying the delta out inline is the obvious way to
    get this wrong.

    **This matters more here than in most rulesets.** With differential scoring (§5), a spectator who
    watched a clean 2 land sees the score advance by 1 with nothing explaining why.

    Worth prototyping a fourth mode: showing the **raw assessments** — `2–1` — rather than the net. It
    explains the differential directly and the log already holds both values. Note it describes
    exchanges only, so warnings would need their own representation.
20. **Streaming overlay** — names, clubs, score, time, penalties, bracket context.
21. **Shareable follow link** — a competitor sends friends a link that follows their matches live,
    rather than making them hunt through a results page.

### After the event

22. **Persistent public results** and competitor profiles.
23. **Career statistics** — a competitor's record across events, not just the current one.
24. **Opponent history** — look up a potential opponent's record and the head-to-head against them,
    across the events *this* server holds.
25. **Statistics over the exchange log** — per competitor and per category, including the timing and
    warning data MVP already records.
26. **Achievements** — general and annual, earned across the events this server holds.
27. **HEMA Ratings export** — one-button extraction in a form the community database can ingest.

### Cross-cutting

28. **Club logos** — for the hosting club and individual competitors, on scoreboards and displays.
    Runtime import plus a database in the repo.
29. **Accessibility** — WCAG 2.1 AA as a stated goal.
30. **GDPR and data retention options** — organizer chooses to store results indefinitely or push to
    HEMA Ratings, with all participants consenting at signup. Grows more significant with (21)–(26),
    which all imply keeping personal data long after the event.

---

## 9. GitHub issues

The issues take precedence over this document where they conflict. Not all are viable as written in
an MVP context, so several are reduced or deferred.

| Issue | Milestone | Notes |
|---|---|---|
| **#1 Add a competitor** | **MVP**, reduced | Name and club only. Picture, phone number and club crest move to Future. Push notifications are out — they need internet, which a LAN-only server doesn't have |
| **#2 Add a tournament** | **MVP**, reduced | Mats, min/max pool size, generate pools. Name, logo and discipline linkage move to Future |
| **#3 Generate pools** | **MVP**, partly | Pool sizing and match ordering in MVP. Club balancing is a stretch goal. Staff assignment is Future |
| **#4 Add a discipline** | **Future** | MVP hardcodes one ruleset; disciplines only matter once rules are data-driven |
| **#5 Add staff** | **Future** | Roles are irrelevant while the score keeper simply records the head referee's decision |
| **#6 Generate a timetable** | **Future** | The largest single piece of work in the issue set |

---

## 10. Still to decide

### The stack

The next thing to settle. Two constraints dominate:

1. **Installability.** A single self-contained artifact the organizer starts on a PC. No separate
   database server, no container runtime, no pre-installed language runtime. This is the whole
   premise, and it eliminates entire families of otherwise reasonable choices.
2. **Shared scoring logic.** The score keeper client needs it offline to show a live score; the server needs
   it as the authority. That means one language on both sides, a shared spec implemented twice, or a
   core compiled to WebAssembly.

These pull against each other — the easy answer to (2) is one language everywhere, while the easy
answer to (1) is a compiled binary. That tension is the substance of the decision.

Two things change the usual calculus: the Python `.gitignore` in the repo was an artefact and carries
no signal, and the stack will be **built with Claude Code rather than chosen for existing
familiarity** — so it should be optimised for the two constraints above and for long-term
maintainability, not for what either of us has written most of before.

---

## 11. Risks

| Risk | Assessment |
|---|---|
| **Installability treated as a late chore** | Would forfeit the entire premise. Test the 5-minute install from the first week, on a machine that isn't the developer's |
| **Correction is one step deep in MVP** | Undo covers the last confirmed exchange only. An error noticed later still needs hand-editing JSON, which keeps the paper fallback valuable and makes full history editing the first Milestone 2 item |
| **The score keeper view is unsettled** | Deliberately so — §4 gives a direction and a set of constraints, not a finished layout, and expects two or three prototypes. It is the most-used screen in the application, so leaving it open is a considered risk rather than an oversight. **Prototype early; it gates nothing else but everything depends on it being right** |
| **Scope creep from Future** | Milestone 3 is an idea dump, not a queue. Nothing moves out of it without being cut down first |

---

## 12. Verification

- **Pure logic** — unit and property tests on scoring, pool generation, ranking and the four indices.
  Deterministic, no UI or network needed, and painful to debug live at an event.
- **Ranking suite** — the index chain, head-to-head fallback, 8–0 forfeits, and **retroactive voiding
  of a withdrawn competitor** with everyone else's indices recomputing correctly.
- **Timer suite** — the 02:50 warning, running past 03:00, ending only on confirmation of the final
  exchange, and stop/start behaviour around time-outs.
- **Warning escalation suite** — first warning scores nothing, second deducts a point, third forces a
  0–8 loss; the count resets each match; and a provisional warning cleared before confirmation has no
  effect at all.
- **No-score exchanges** — confirming with nothing selected appends an exchange to the log and leaves
  both scores untouched.
- **Installability test**, treated as an acceptance test — clean machine, not a developer's, with a
  stopwatch, **installing the published asset from the Releases page rather than a local build**. That
  is the path a real organizer takes, and it is the only one worth measuring.
  **If this fails, the release fails.**
- **Simulated event** — enter competitors, generate pools, score every match, produce standings, asserting
  invariants end to end.
- **Offline test** — disconnect a score keeper client mid-match, score a full pool, reconnect, and assert the
  server's log matches the device's exactly.
- **iOS / WebKit check** — every browser on iOS is WebKit, so there is no port, only compatibility.
  Verify the screen wake lock (Safari 16.4+ only), local storage surviving a match, and Add to Home
  Screen. Element fullscreen is unavailable on iPhone Safari, so treat iPhones as score keeper devices
  rather than displays.
- **Club-night trials** — put the score keeper view in front of real competitors weekly. Worth more than any
  amount of synthetic testing.
- **LAN dress rehearsal** before the event — real tablets, real server PC, real venue wifi, a mock
  pool. This is the acceptance test for 15 November, not the unit suite.
- **Paper fallback drill** — confirm a pool can be run on printed sheets and entered afterwards.

### Fallback ladder

The MVP is built so partial completion still leaves something usable:

| Tier | Run the event on |
|---|---|
| 1 | Full system — server, score keeper clients, scoreboard |
| 2 | Score keeper clients alone, printed pool sheets, standings by hand |
| 3 | All paper |

Pick the tier a week out, not on the morning.
