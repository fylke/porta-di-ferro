# Porta di Ferro — Design

> **Status:** proposed, not yet agreed. Tech stack deliberately not chosen here — see §10.

---

## 1. What this is

A HEMA tournament application: score bouts at the mat, run pools, show results. Built by two members
of **MSL — Medeltida Stridsteknik Linköping IF**, a club in Linköping affiliated with Svenska
HEMA-förbundet.

**First target:** MSL's own club-internal event, **1 October 2026**. Small, forgiving, and a real
deadline.

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
| 1 | **The score keeper client records the ring judge's final decision.** It does not capture individual judge signals |
| 2 | **Hardcoded MSL rules for MVP.** Data-driven rulesets are a future milestone |
| 3 | **Venue LAN** for MVP and stretch. Organizer's PC is the server and the source of truth. Cloud is a future milestone |
| 4 | **Web clients** — Android browsers for MVP, iOS a stretch goal. Nothing to install on a client |
| 5 | **Exchange log**, append-only, with timestamps. Score is derived, never stored directly |
| 6 | **One writer per bout.** Handover is a stretch goal |
| 7 | **Fencers entered locally.** No public registration, no payments |
| 8 | **Local JSON files** as the database. The organizer owns and can read their data |
| 9 | **Club-agnostic.** Nothing hardcoded to MSL except the ruleset in MVP |
| 10 | **MIT licensed** |
| 11 | **Swedish UI for MVP**, English localisation a stretch goal. **Internal identifiers are English** regardless |

---

## 3. Architecture

### Three surfaces

| Surface | Runs on | Role |
|---|---|---|
| **Server** | Organizer's PC | Source of truth; fencer registration, tournament setup, pool generation, results |
| **Score keeper client** | Tablet or phone at the mat | The score keeper's tool — the *sekretariat* of the Swedish rules. Scores one bout at a time |
| **Scoreboard** | Secondary monitor on the server PC | Live match display |

Clients join over the venue LAN via a printed URL or QR code. **The server also serves the web app
itself** — at a venue with no internet, that's how a device that has never opened it gets it.

### Sync

The network is never in the scoring path. A tap writes to the score keeper client's own local log and
returns immediately; pushing to the server is asynchronous. The score keeper never waits on the LAN.

Each bout is an **append-only log with exactly one writer**, so there is nothing to merge — the
server orders and stores. This is log shipping, not distributed consensus. Events carry a per-bout
sequence number; reconnecting means *"here is everything after sequence N"*. The primary key is
`(bout_id, sequence)`, so retries are idempotent and need no deduplication logic.

**Push after every confirmed exchange**, not at bout end, so a lost device costs at most one
exchange.

Corrections are appended as new events rather than mutating history. MVP has no correction UI (§7),
but building the log this way means adding one later is a UI change rather than a data migration.

### The exchange log

Recorded per bout:

- every confirmed exchange — timestamp, points awarded to each fencer
- every warning — timestamp, fencer
- **timer events** — started, stopped at timestamp X, resumed after Y seconds

Nothing else. This is enough to reconstruct a bout completely and to produce post-event statistics
later without changing the schema.

---

## 4. The score keeper view

The most important screen in the application. Large, unambiguous buttons; usable at a glance under
time pressure.

```
+--------------------------------------------------------------------+
|                                                                    |
|       RED                     02:47                     BLUE       |
|   Fencer name                                       Fencer name    |
|                                                                    |
|        5   /!\ /!\                                       3         |
|                                                                    |
|   ############           +--------------+           +----------+   |
|   #    2     #           |    START     |           |    2     |   |
|   ############           +--------------+           +----------+   |
|                                                                    |
|   +----------+           +--------------+           +----------+   |
|   |    1     |           |     STOP     |           |    1     |   |
|   +----------+           +--------------+           +----------+   |
|                                                                    |
|   +----------+                                      ############   |
|   | WARNING  |                                      # WARNING  #   |
|   +----------+                                      ############   |
|                                                                    |
|                  +------------------------------+                  |
|                  |       CONFIRM EXCHANGE       |                  |
|                  +------------------------------+                  |
|                                                                    |
+--------------------------------------------------------------------+
```

Boxes drawn with `#` are selected; `+--+` are unselected.

**Behaviour**

- Red fencer on one side, blue on the other, mirroring the wristbands. **Sides are fixed in MVP**;
  swapping them is a stretch goal.
- Each side shows current score, a **2-point** button, a **1-point** button, and a **warning** button.
- **Confirmed warnings show as a warning triangle beside that fencer's score** — one triangle per
  warning. The count is what matters, not merely that a warning exists: the score keeper needs to see
  at a glance whether the next warning costs a point or ends the bout. Provisional, unconfirmed
  warnings never appear here.
- **Nothing takes effect until *Confirm exchange*.** Points and warnings alike are only selections
  until then — the score doesn't move, the warning isn't counted, and nothing is written to the log.

**Selection model**, per fencer:

- The two point buttons are **mutually exclusive**. Selecting one deselects the other, so a fencer
  holds at most one of {1, 2}.
- **Pressing an already-selected button deselects it.** True of the point buttons and the warning
  button alike, so any mis-tap is undone by tapping it again.
- The **warning toggles independently** of the points — a fencer can be given points and a warning in
  the same exchange, or either alone.
- **Selected state must be unmistakable at a glance.** A selected warning shows red or equally
  prominent; a selected point button is clearly distinct from an unselected one. The score keeper has
  to be able to check the state in the moment before confirming, without studying it.

This also enforces a rule for free: **a fencer can be awarded at most 2 points in one exchange**,
which is exactly the ruleset's maximum for a single hit.

**On confirmation**

- Both fencers can be selected before confirming, which is how afterblows and doubles are entered.
- **Confirming with nothing selected records a no-score exchange.** An exchange where neither fencer
  scored is a real event and is logged as such, not discarded.
- **A third warning ends the bout**, so it asks for confirmation before committing (§5).

**Timer**

- **Counts up from 00:00**, toward the 3-minute match time.
- At **02:50 — ten seconds remaining — it turns red** (or is made equally unmissable) to signal the
  final exchange.
- **It does not stop at 03:00.** It keeps ticking past the limit until the **final exchange is
  confirmed**, which is what ends the bout. A completed bout's clock therefore routinely reads more
  than three minutes.
- It is **not paused for scoring**, matching the ruleset. The timer controls exist for the ring
  judge's time-outs, not for ordinary exchanges.
- **Start and stop are two separate full-size buttons**, the same size as the scoring buttons, with
  **exactly one enabled at a time** — start is disabled while running, stop is disabled while
  stopped. Two dedicated buttons rather than one toggle, so the control never depends on the
  operator having read the current state correctly before pressing.

**Colour**

Each fencer's half of the screen is tinted with their colour — red one side, blue the other —
matching the wristbands, so the score keeper's eye lands on the right half without reading anything.

Two collisions fall out of that, and both are resolved by one rule: **hue means identity, never
state.**

- *A selected warning cannot be red*, because the red fencer's side is already red. **Selection is
  shown by fill and border weight, not by hue** — the same treatment on both sides, so a selected
  button looks selected regardless of which fencer it belongs to.
- *The timer turning red at 02:50 is the one place red doesn't mean the red fencer.* It works because
  the timer sits in the neutral centre column and the whole area floods at once, which reads as an
  alarm rather than as identity. Keep it a full-area change rather than colouring the digits alone.

Warning triangles and the warning buttons use **amber**, distinct from both fencer colours and
conventional for the meaning.

Accessibility work is a future milestone, but the layout already carries the redundancy that matters:
fixed sides, RED and BLUE labels, and fencer names. Colour is never the only cue.

The scoreboard and audience displays use the same colour language, so a spectator glancing between
screens doesn't have to relearn it.

---

## 5. MVP ruleset (hardcoded)

MSL's SM ruleset. Longsword scoring is used for all weapons at this stage.

| Rule | Value |
|---|---|
| Point values | 1 or 2, as announced by the ring judge |
| Point cap | 8 |
| Match time | 3 minutes |
| Final-exchange warning | 10 seconds remaining (02:50) |
| Result types | Win / loss / **draw** (draws are possible in pools) |
| Pool match points | Win **9**, draw **6**, loss **3** |
| Forfeit | Recorded 8–0; winner takes 9 match points, forfeiter 0 |
| Withdrawal during pools | Treated as if the fencer never participated — results retroactively voided |
| Red / blue assignment | Assigned when the match is about to start |

**Warnings** escalate automatically, per fencer, and the count **resets each bout**:

| Warning | Consequence |
|---|---|
| First | Recorded and displayed. No score effect |
| Second | **Point deduction** — one point off the warned fencer |
| Third | **Match loss, 0–8** against the warned fencer |

Because the third warning ends the bout, the score keeper view confirms before committing it.

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

Target: run the 1 October club event.

1. **Server (organizer) + score keeper clients** over the LAN.
2. **Hardcoded MSL rules** per §5.
3. **Score keeper view** per §4.
4. **Up to 2 mats** concurrently.
5. **Up to 4 pools, up to 7 fencers each** — a ceiling of 28 fencers per run.
6. **Pools only** — no eliminations.
7. **Server views:**
   - **7.1 Fencers** — registration and status. Stored as a local JSON database file.
   - **7.2 Tournament** — number of mats, min/max fencers per pool, generate pools.
   - **7.3 Pools** — generated matches per pool with status; results as JSON.
8. **Scoreboard on a secondary monitor** — current points and warnings for red and blue, match time,
   and the winner when decided.
9. **Pool generation** honouring min/max size, with **uneven pool sizes accepted**, and bout ordering
   that minimises consecutive bouts on a best-effort basis, **reporting any remaining violations**
   rather than guaranteeing none.
10. **Export results as JSON.**
11. **Printable blank pool sheets** as a paper fallback.
12. **Swedish UI.**
13. **A Windows installer published to GitHub Releases.** Listed as a deliverable rather than assumed,
    because it is the acceptance criterion for the whole premise (§1).

### Mat assignment

Fixed and predictable, because confusion at the mat costs more than throughput:

- **Mat 1 runs pools 1 and 3. Mat 2 runs pools 2 and 4.** Odd pools to mat 1, even to mat 2.
- The **first two pools start together**. When a pool finishes, that mat picks up its next pool.
- **No organizer override in MVP** — the mapping is fixed. Override is a stretch goal.

### Running several disciplines

MVP has no concept of disciplines or divisions, and caps at 28 fencers. Multiple disciplines are
handled by **starting a separate run of the application per discipline** — up to four, each with its
own JSON database.

**MVP runs them sequentially, one at a time.** Running disciplines concurrently is a stretch goal,
and only then does the question of separate ports and data directories arise.

### Accepted MVP limitations

Deliberate, and listed so nobody is surprised on the day:

- **No correction path.** There is no undo and no minus button. A mis-tap noticed after *Confirm
  exchange* cannot be fixed in the UI, and a ring judge's point deduction has no direct entry.
  The escape hatch is that the database is **local JSON the organizer can hand-edit**. Proper
  correction is the first stretch goal.
- **No eliminations, no finals.** Pools produce a ranking; anything beyond that is run on paper.
- **No handover.** If a score keeper client dies mid-bout, the bout is re-entered.

---

## 7. Milestone 2 — Club Event Stretch Goals

1. **Correction path** — undo the last confirmed exchange, and a route for ring-judge point
   deductions. The highest-value item in this milestone.
2. **Eliminations** — top 8 from the pools.
3. **Audience display** on a secondary monitor:
   - the **winner and final scores, prominently**, when a bout is decided
   - the **upcoming match** — fencer names, colour-coded red and blue — in a smaller but still clearly
     legible font
   - an **"on deck" panel down the side** listing matches still to come, with red and blue background
     colour-coding per fencer
4. **Swap fencer sides** when the fencers are oriented the other way round from the score keeper's point
   of view. **Score keeper view and audience view swap independently** of each other.
5. **Up to 4 mats, up to 8 pools** — 56 fencers per run. Mat assignment generalises to pool *N* on
   mat *((N−1) mod mats) + 1*.
6. **Organizer override of mat assignment.**
7. **Concurrent disciplines** — several runs at once, which requires a distinct port and data
   directory per instance.
8. **Score keeper client handover** — graceful (planned: bathroom break, shift change) and ungraceful
   (device died). Graceful flushes before releasing so nothing is lost; ungraceful increments a
   writer epoch, and any late events from the old device are quarantined and shown to the organizer
   rather than silently dropped.
9. **English localisation** alongside Swedish.
10. **PDF export** alongside JSON.
11. **iOS client support.**
12. **Club balancing in pool generation** — distribute fencers from the same club as evenly as
    possible (issue #3). No effect at a club-internal event, so it needs synthetic testing.

---

## 8. Milestone 3 — Future

Deliberately unrefined. An idea dump to be sorted later, not a commitment.

1. **Web/cloud server** — an easily deployed droplet or a web server on a PC, with clients connecting
   over the internet rather than the LAN. **A Linux install ships alongside the Windows one** from
   this point, since a Linux host becomes the normal deployment target.
2. **Participant self-registration** — participants sign themselves up to an event and to
   individual tournaments per discipline. **Depends on (1)**, since it needs a web-hosted server
   standing before the event rather than a laptop switched on that morning.
3. **Observer role.**
4. **Fully data-driven rulesets** — replacing the hardcoded MVP rules.
5. **Finals — best of three**, won by two wins or one win and two draws, then sudden death.
6. **Events divided into tournaments**, with organizer options.
7. **Disciplines** — weapon and ruleset selection per tournament (issues #2, #4).
8. **Configurable elimination cut** — how many advance, set when organizing the tournament.
9. **Club logos** — for the hosting club and individual fencers, on scoreboards and displays.
   Runtime import plus a database in the repo.
10. **Staff** — roles, availability, assignment, and the *pliktdomarsystem* under which competing
    obliges you to staff another discipline (issue #5).
11. **Timetable** — generation, with personalised per-competitor schedules (issue #6).
12. **Per-mat scoreboard clients** driving their own monitors, possibly handhelds.
13. **Team events.**
14. **Non-bout events** — cutting, forms, solo.
15. **Persistent public results** and fencer profiles.
16. **Streaming overlay** — names, clubs, score, time, penalties, bracket context.
17. **HEMA Ratings export.**
18. **Statistics** over the exchange log.
19. **Accessibility** — WCAG 2.1 AA as a stated goal.
20. **GDPR / data retention options** — organizer chooses to store results indefinitely or push to
    HEMA Ratings, with all participants consenting at signup.

---

## 9. GitHub issues

The issues take precedence over this document where they conflict. Not all are viable as written in
an MVP context, so several are reduced or deferred.

| Issue | Milestone | Notes |
|---|---|---|
| **#1 Add a competitor** | **MVP**, reduced | Name and club only. Picture, phone number and club crest move to Future. Push notifications are out — they need internet, which a LAN-only server doesn't have |
| **#2 Add a tournament** | **MVP**, reduced | Mats, min/max pool size, generate pools. Name, logo and discipline linkage move to Future |
| **#3 Generate pools** | **MVP**, partly | Pool sizing and bout ordering in MVP. Club balancing is a stretch goal. Staff assignment is Future |
| **#4 Add a discipline** | **Future** | MVP hardcodes one ruleset; disciplines only matter once rules are data-driven |
| **#5 Add staff** | **Future** | Roles are irrelevant while the score keeper simply records the ring judge's decision |
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
| **No correction path in MVP** | Accepted deliberately (§6), but it means a mis-tap survives to the results. Hand-editing JSON is the only recourse. Raises the value of the paper fallback, and makes the stretch correction path the first thing to build afterwards |
| **The warning button is destructive** | With automatic escalation, three taps end a bout 0–8 — and MVP has no undo. Mitigated by keeping warnings provisional until *Confirm exchange* and confirming the third one, but it remains the single most damaging control on the screen. Worth extra care in layout so it cannot be hit by accident |
| **Timer semantics past zero** | A count-up clock that ignores its own limit until an external event confirms is unusual and easy to get subtly wrong. Test the boundary explicitly |
| **Club balancing untested** | Everyone shares a club at a club-internal event, so this path gets no real exercise. Cover synthetically |
| **Scope creep from Future** | Milestone 3 is an idea dump, not a queue. Nothing moves out of it without being cut down first |

---

## 12. Verification

- **Pure logic** — unit and property tests on scoring, pool generation, ranking and the four indices.
  Deterministic, no UI or network needed, and painful to debug live at an event.
- **Ranking suite** — the index chain, head-to-head fallback, 8–0 forfeits, and **retroactive voiding
  of a withdrawn fencer** with everyone else's indices recomputing correctly.
- **Timer suite** — the 02:50 warning, running past 03:00, ending only on confirmation of the final
  exchange, and stop/start behaviour around time-outs.
- **Warning escalation suite** — first warning scores nothing, second deducts a point, third forces a
  0–8 loss; the count resets each bout; and a provisional warning cleared before confirmation has no
  effect at all.
- **No-score exchanges** — confirming with nothing selected appends an exchange to the log and leaves
  both scores untouched.
- **Installability test**, treated as an acceptance test — clean machine, not a developer's, with a
  stopwatch, **installing the published asset from the Releases page rather than a local build**. That
  is the path a real organizer takes, and it is the only one worth measuring.
  **If this fails, the release fails.**
- **Simulated event** — enter fencers, generate pools, score every bout, produce standings, asserting
  invariants end to end.
- **Offline test** — disconnect a score keeper client mid-bout, score a full pool, reconnect, and assert the
  server's log matches the device's exactly.
- **Club-night trials** — put the score keeper view in front of real fencers weekly. Worth more than any
  amount of synthetic testing.
- **LAN dress rehearsal** before the event — real tablets, real server PC, real venue wifi, a mock
  pool. This is the acceptance test for 1 October, not the unit suite.
- **Paper fallback drill** — confirm a pool can be run on printed sheets and entered afterwards.

### Fallback ladder

The MVP is built so partial completion still leaves something usable:

| Tier | Run the event on |
|---|---|
| 1 | Full system — server, score keeper clients, scoreboard |
| 2 | Score keeper clients alone, printed pool sheets, standings by hand |
| 3 | All paper |

Pick the tier a week out, not on the morning.
