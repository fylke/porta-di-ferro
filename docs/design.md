# Porta di Ferro — Design

> **Companion:** [`open-questions.md`](./open-questions.md) — what's still undecided.
>
> **Status:** proposed, not yet agreed. Tech stack deliberately not chosen here.

---

## 1. What this is

A HEMA tournament application: score bouts at the mat, run pools, show results. Built by two
members of **MSL — Medeltida Stridsteknik Linköping IF**, a club in Linköping affiliated with
Svenska HEMA-förbundet.

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

---

## 2. Decisions

| # | Decision |
|---|---|
| 1 | **The table client records the ring judge's final decision.** It does not capture individual judge signals |
| 2 | **Hardcoded MSL rules for MVP.** Data-driven rulesets are a future milestone |
| 3 | **Venue LAN.** Organizer's PC is the server and the source of truth. No cloud |
| 4 | **Web clients** — Android browsers for MVP, iOS a stretch goal. Nothing to install on a client |
| 5 | **Exchange log**, append-only, with timestamps. Score is derived, never stored directly |
| 6 | **One writer per bout.** Handover is a stretch goal |
| 7 | **Fencers imported/entered locally.** No public registration, no payments |
| 8 | **Local JSON files** as the database. The organizer owns and can read their data |
| 9 | **Club-agnostic.** Nothing hardcoded to MSL except the ruleset in MVP |
| 10 | **MIT licensed** |

### The decision that removed the most work

Decision 1 collapses almost the entire ruleset surface. Because the table waits for the ring judge's
announced award, the application never needs to know about:

- how many judges there are, or how their votes resolve
- flag signal vocabularies
- target zones or which weapon is in use
- afterblows, double hits, or grappling actions

All of it arrives as *"2 to red"* or *"1 to blue"* regardless. What remains is: **a point value of 1
or 2, a warning, a timer, and pool arithmetic.** Grappling being permitted therefore costs nothing.

Both fencers can be awarded points in the same exchange, which is how afterblows and doubles are
represented — no special handling required.

---

## 3. Architecture

### Three surfaces

| Surface | Runs on | Role |
|---|---|---|
| **Server** | Organizer's PC | Source of truth; fencer registration, tournament setup, pool generation, results |
| **Table client** | Tablet or phone at the mat | The secretariat's tool. Scores one bout at a time |
| **Scoreboard** | Secondary monitor on the server PC | Live match display |

Clients join over the venue LAN via a printed URL or QR code. **The server also serves the web app
itself** — at a venue with no internet, that's how a device that has never opened it gets it.

### Sync

The network is never in the scoring path. A tap writes to the table client's own local log and
returns immediately; pushing to the server is asynchronous. The secretariat never waits on the LAN.

Each bout is an **append-only log with exactly one writer**, so there is nothing to merge — the
server orders and stores. This is log shipping, not distributed consensus. Events carry a per-bout
sequence number; reconnecting means *"here is everything after sequence N"*. The primary key is
`(bout_id, sequence)`, so retries are idempotent and need no deduplication logic.

**Push after every confirmed exchange**, not at bout end, so a lost device costs at most one
exchange.

Corrections are appended as new events rather than mutating history, which keeps the log
append-only and leaves an audit trail for disputes.

### The exchange log

Recorded per bout:

- every confirmed exchange — timestamp, points awarded to each fencer
- every warning — timestamp, fencer
- **timer events** — started, stopped at timestamp X, resumed after Y seconds

Nothing else. This is enough to reconstruct a bout completely and to produce post-event statistics
later without changing the schema.

---

## 4. The table (secretariat) view

The most important screen in the application. Large, unambiguous buttons; usable at a glance under
time pressure.

```
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│      RED                  02:47                    BLUE      │
│   Fencer name          [ START/STOP ]           Fencer name  │
│                                                              │
│        5                                            3        │
│      score                                        score      │
│                                                              │
│   ┌──────────┐                              ┌──────────┐     │
│   │    2     │                              │    2     │     │
│   └──────────┘                              └──────────┘     │
│   ┌──────────┐                              ┌──────────┐     │
│   │    1     │                              │    1     │     │
│   └──────────┘                              └──────────┘     │
│   ┌──────────┐                              ┌──────────┐     │
│   │ WARNING  │                              │ WARNING  │     │
│   └──────────┘                              └──────────┘     │
│                                                              │
│            ┌────────────────────────────┐                    │
│            │     CONFIRM EXCHANGE       │                    │
│            └────────────────────────────┘                    │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Behaviour**

- Red fencer on one side, blue on the other, mirroring the wristbands.
- Each side shows current score, a **2-point** button, a **1-point** button, and a **warning** button.
- **Points are provisional until *Confirm exchange*** — only then are they applied to the score and
  written to the log. This allows both fencers to be awarded before committing, which is how
  afterblows and doubles are entered.
- The **timer sits between them**, with start/stop.
- At **10 seconds remaining the timer turns red** (or is made equally unmissable) to signal the final
  exchange.
- **The timer keeps running past that point** — the bout ends when the final exchange is confirmed,
  not when the clock reaches zero.

---

## 5. MVP ruleset (hardcoded)

MSL's SM ruleset, confirmed as the day-one target. Longsword scoring is used for all weapons at this
stage.

| Rule | Value |
|---|---|
| Point values | 1 or 2, as announced by the ring judge |
| Point cap | 8 |
| Match time | 3 minutes |
| Final-exchange warning | 10 seconds remaining |
| Result types | Win / loss / **draw** (draws are possible in pools) |
| Pool match points | Win **9**, draw **6**, loss **3** |
| Forfeit | Recorded 8–0; winner takes 9 match points, forfeiter 0 |
| Withdrawal during pools | Treated as if the fencer never participated — results retroactively voided |

**Pool ranking**, in order, all divided by matches *completed*:

1. Match point index — match points ÷ matches completed
2. Victory index — wins ÷ matches completed
3. Score index — (points scored − points conceded) ÷ matches completed
4. Reception index — points conceded ÷ matches completed (**lowest** wins)
5. Head-to-head result
6. Random draw

Dividing by *completed* matches is what makes retroactive withdrawal work correctly.

---

## Milestone 1 — MVP: Club Event Basics

Target: run the 1 October club event.

1. **Server (organizer) + table (secretariat) clients** over the LAN.
2. **Hardcoded MSL rules** per §5.
3. **Table view** per §4.
4. **Up to 2 mats** concurrently.
5. **Up to 4 pools, up to 7 fencers each.** Fencer list view for registration; stored as a local JSON
   database file.
6. **Pools only** — no eliminations.
7. **Server views:**
   - **7.1 Fencers** — registration and status.
   - **7.2 Tournament** — number of mats, min/max fencers per pool, generate pools.
   - **7.3 Pools** — generated matches per pool with status; results as JSON.
8. **Scoreboard on a secondary monitor** — current points and warnings for red and blue, match time,
   and the winner when decided.
9. **Pool generation** honouring min/max size, with **uneven pool sizes accepted**, and bout ordering
   that minimises consecutive bouts on a best-effort basis, **reporting any remaining violations**
   rather than guaranteeing none.
10. **Export results as JSON.**
11. **Printable blank pool sheets** as a paper fallback.

**Out of scope for MVP:** eliminations, finals, handover, audience display, staff, timetable,
categories, disciplines, club crests, accessibility work, GDPR features.

---

## Milestone 2 — Club Event Stretch Goals

1. **Eliminations** — top 8 from the pools.
2. **Secondary monitor audience display.**
3. **Up to 4 mats, up to 8 pools.**
4. **Table client handover** — graceful (planned; bathroom break, shift change) and ungraceful
   (device died). Graceful flushes before releasing so nothing is lost; ungraceful increments a
   writer epoch, and any late events from the old device are quarantined and shown to the organizer
   rather than silently dropped.
5. **PDF export** alongside JSON.
6. **iOS client support.**
7. **Club balancing in pool generation** — distribute fencers from the same club as evenly as
   possible (issue #3). No effect at a club-internal event, so it needs synthetic testing.

---

## Milestone 3 — Future

Deliberately unrefined. An idea dump to be sorted later, not a commitment.

1. **Web/cloud server** — an easily deployed droplet or a web server on a PC, with clients connecting
   over the internet rather than the LAN.
2. **Participant role** — registration to an event and to individual tournaments.
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
21. **Swedish and English localisation.**

---

## 6. GitHub issues

The issues take precedence over this document where they conflict. Not all are viable as written in
an MVP context, so several are reduced or deferred.

| Issue | Milestone | Notes |
|---|---|---|
| **#1 Add a competitor** | **MVP**, reduced | Name and club only. Picture, phone number and club crest move to Future. Push notifications are out — they need internet, which a LAN-only server doesn't have |
| **#2 Add a tournament** | **MVP**, reduced | Mats, min/max pool size, generate pools. Name, logo and discipline linkage move to Future |
| **#3 Generate pools** | **MVP**, partly | Pool sizing and bout ordering in MVP. Club balancing is a stretch goal. Staff assignment is Future |
| **#4 Add a discipline** | **Future** | MVP hardcodes one ruleset; disciplines only matter once rules are data-driven |
| **#5 Add staff** | **Future** | Roles are irrelevant while the table simply records the ring judge's decision |
| **#6 Generate a timetable** | **Future** | The largest single piece of work in the issue set |

---

## 7. Risks

| Risk | Assessment |
|---|---|
| **Installability treated as a late chore** | Would forfeit the entire premise. Test the 5-minute install from the first week, on a machine that isn't the developer's |
| **28-fencer MVP ceiling** | 4 pools × 7 gives 28. The 1 October event was described as up to 40 across 1–4 disciplines, and MVP has no discipline concept — so a single tournament run must fit in 28. Confirm this works, or run the app once per discipline |
| **No correction path after confirming** | The table view has no minus button and no undo. A mis-tap, or a ring judge ordering a point deduction, currently has nowhere to go. See open questions |
| **Timer semantics past zero** | The clock continuing until the final exchange is confirmed is unusual and easy to get subtly wrong. Specify it precisely before building |
| **Club balancing untested** | Everyone shares a club at a club-internal event, so this path gets no real exercise. Cover synthetically |
| **Scope creep from Future** | Milestone 3 is an idea dump, not a queue. Nothing moves out of it without being cut down first |

---

## 8. Verification

- **Pure logic** — unit and property tests on scoring, pool generation, ranking and the four indices.
  Deterministic, no UI or network needed, and painful to debug live at an event.
- **Ranking suite** — the index chain, head-to-head fallback, 8–0 forfeits, and **retroactive voiding
  of a withdrawn fencer** with everyone else's indices recomputing correctly.
- **Installability test**, treated as an acceptance test — clean machine, not a developer's, with a
  stopwatch. **If this fails, the release fails.**
- **Simulated event** — import fencers, generate pools, score every bout, produce standings, asserting
  invariants end to end.
- **Offline test** — disconnect a table client mid-bout, score a full pool, reconnect, and assert the
  server's log matches the device's exactly.
- **Club-night trials** — put the table view in front of real fencers weekly. Worth more than any
  amount of synthetic testing.
- **LAN dress rehearsal** before the event — real tablets, real server PC, real venue wifi, a mock
  pool. This is the acceptance test for 1 October, not the unit suite.
- **Paper fallback drill** — confirm a pool can be run on printed sheets and entered afterwards.

### Fallback ladder

The MVP is built so partial completion still leaves something usable:

| Tier | Run the event on |
|---|---|
| 1 | Full system — server, table clients, scoreboard |
| 2 | Table clients alone, printed pool sheets, standings by hand |
| 3 | All paper |

Pick the tier a week out, not on the morning.
