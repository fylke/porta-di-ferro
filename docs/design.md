# Porta di Ferro — Phased Feature Plan

> **Scope:** the *feature* plan, in phases. Tech stack is deliberately deferred — §8 records what
> these decisions have already constrained about it.
>
> **Companion document:** [`open-questions.md`](./open-questions.md) holds the design questions that
> are still open, tagged by which phase each one blocks. Answer them there, in place.
>
> **Status:** proposed. Not yet agreed.

---

## 1. Context

`fylke/porta-di-ferro` is greenfield — one commit, README + LICENSE + a Python `.gitignore`. Six open
issues, no code. The goal is a HEMA tournament app better than the incumbents
([HEMA Scorecard](https://github.com/SeanFranklin/hemaScorecard), [HEMAGON](https://hemagon.com/about)).

**The tension this plan resolves.** The original brief and the GitHub issues describe two different
products. The brief is bottom-up (v0.1 is a client-side scoring clicker, no backend). The issues are
top-down (tournament/discipline/competitor CRUD, and #2/#4 explicitly say *"Should be an API
endpoint"*). The bottom-up path won, so the issues become phase 3–4 material rather than phase 1.

**Who's building it.** Two members of **MSL — Medeltida Stridsteknik Linköping IF**, a HEMA club in
Linköping affiliated with Svenska HEMA-förbundet. MSL hosted **SM (Svenska Mästerskapen) 2022** at
Vasahallen, so this is a club that already runs championship-scale events and has a documented
ruleset of its own (§5). The 1 October event is MSL's own club-internal tournament. SvHEMAF
membership also identifies the natural adoption audience if this outgrows one club: other Swedish
federation clubs, who face the same self-hosting problem (§2) and operate under the same
competition-licence regime.

---

## 2. Product thesis

Asked what specifically makes HEMA Scorecard bad, the answer was: *nothing much — except that the
open-source variant has a complex installation and basically only works against Sean Franklin's own
server.*

That is not a minor gripe. It is the whole product.

> **Porta di Ferro's differentiator is that you can actually run it yourself.**
> Any club, on any laptop, with no external service, no account, and no dependency on anyone else's
> infrastructure, in minutes.

Installability is therefore not a packaging chore at the end — it is **the primary acceptance
criterion**, and it outranks features. A missing bracket type is a disappointment; a fiddly install
is a failure of the entire premise.

This is evidence-backed. Checked while writing:

- **HEMA Scorecard** is PHP + MySQL, installed via Docker Compose. The wall is a container runtime
  *and* a database server — precisely where a club organizer gives up.
- **HEMAGON** has no public repositories at all and a closed SPA site, operated from Spain.
  Proprietary and hosted-only; your event data lives on someone else's infrastructure.

The open one can't practically be self-hosted; the self-hostable one doesn't exist. That's the gap.

Concrete, testable form of the promise:

- **From download to a running tournament in under 5 minutes**, by someone who is not a programmer.
- **No external dependencies** — no separate database server, no Docker requirement, no runtime to
  install first, no internet connection.
- **Works on Windows, macOS and Linux**, because club organizers use all three.
- **The organizer owns their data** — a file on their laptop they can copy, back up and read.

This also reinforces LAN-only: no cloud isn't a limitation here, it's the point.

---

## 3. Inspiration: what to borrow, what to avoid

Take their **features**, explicitly not their **stack or distribution model**.

### From HEMA Scorecard

| Borrow | Why |
|---|---|
| **Transparent tiebreak resolution** — shows not just who advanced but *who fell on tiebreaks and why* | Standings disputes are a live event's worst time-sink. Make the tiebreak chain visible in the UI, not merely correct internally. **Adopted into phase 2.** |
| **Permanent public per-event page showing the ruleset in force** (its `infoRules` / `infoSummary` pattern) | Competitors genuinely read these. Maps onto our phase 3 static export. |
| **Genuine ruleset breadth** — deductive *and* full afterblows, doubles, one-hit formats, multiple pool rounds, solo/cutting events | Validates the data-driven ruleset decision. We won't build cutting events, but **the schema shouldn't preclude non-bout scoring later.** |
| **Free, open, community-supported** | The trust model to match. See the licence question (Q72). |

**Avoid:** the Docker + MySQL install wall; being effectively one hosted instance; sparse docs.

### From HEMAGON

| Borrow | Why |
|---|---|
| **Persistent fighter profiles with achievement history** | Identity surviving across events makes results feel like a career, not a spreadsheet row. Natural home for HEMA Ratings integration. **Phase 5.** |
| **Post-event statistics as a first-class feature** | Both incumbents do it and competitors love it. See below — we can beat both. |
| **Low-friction registration ("a couple of clicks")** | Out of scope by decision 4, but sets the bar for whenever registration is revisited. |
| **Full-lifecycle thinking** — pre-event, event day, post-event | A check on our phase boundaries: don't ship an event-day tool with no "after". |

**Avoid:** closed source; hosted-only; the organizer's data living in another country.

### Where we can beat both

**Statistics.** Both incumbents store *scores*. The exchange-log decision means we store *every
exchange* — every judge signal, target tier, grappling action and timing. That yields per-fencer
high-vs-low distributions, double rates, afterblow rates, time-to-first-point, and something neither
incumbent could ever produce: **judge agreement rates and per-judge calibration**, straight out of
the four-flag voting data (finding B). It's the analysis HEMA people currently do by hand-mining
video (cf. Sword STEM), and for us it's nearly free — a read-only view over data phase 1 already
collects.

**This is the strongest argument for the exchange log**, recorded here as a deliberate long-term
payoff rather than something discovered later. **Phase 5.**

---

## 4. Decisions locked in

| # | Decision | Choice |
|---|---|---|
| 1 | MVP scope | **Mat app + pools** |
| 2 | Offline | **Fully offline-first** mat app |
| 3 | Rulesets | **Data-driven config**, generic engine, shipped presets |
| 4 | Registration | **CSV/spreadsheet import**; no native signup, no payments |
| 5 | Scoring model | **Exchange log** — record judge signals, derive the score |
| 6 | Concurrency | **One writer per bout** — the secretariat (§5) |
| 7 | Sync target | **Venue LAN only** — organizer's laptop is the source of truth |
| 8 | Day-one ruleset | **MSL's own SM ruleset** (§5) |
| 9 | Target event | **1 October 2026**, club-internal, 1–4 disciplines, ≤40 participants |
| 10 | Core promise | **Trivial self-hosting** (§2) |
| 11 | Clients | **Web / PWA everywhere** — phones and tablets as mat devices, nothing to install |
| 12 | Host | **A computer runs the server**; phones are clients only. Kept host-agnostic so a phone host stays possible later |
| 13 | Localization | **Swedish + English** from the first commit; **English identifiers** in code, both languages as display locales |

### Consequences of LAN-only

- **Personal schedules work only on venue wifi.** Fine via QR at the venue. **Operational risk:** many
  venue APs enable client isolation, which would block mat↔server traffic entirely, not just
  schedules. Verify at the venue in advance; fallback is the laptop running its own hotspot.
- **Push and SMS are out** (both need internet from the server). Writes off issue #1's Textbee
  suggestion unless the laptop is allowed online.
- **No remote public results.** Mitigated in phase 3 by a static export published afterwards.
- **Upside:** GDPR obligations shrink to nearly nothing — no cloud processor, no accounts, no payment
  data, data on a laptop the club already controls. Further helped by 18+ only, and by the medical-data
  boundary in finding (N).

### Consequence of exchange-log + single-writer

> Each bout is an **append-only log of exchanges with exactly one writer**. The mat device appends
> locally and pushes when it can. With one writer per bout there are no concurrent edits to merge —
> the server just orders and stores. **No CRDTs, no operational transform.**

The uncovered case is a dead or lost mat device mid-bout: handle with an **explicit organizer-triggered
writer handover**, not by weakening the single-writer rule.

### The sync protocol

Spelled out, because it's the part most likely to be misjudged as harder than it is.

**The network is never in the scoring path.** A tap writes to the device's own local log and returns
immediately. Pushing to the server is separate and asynchronous. The secretariat never waits on the
LAN, never sees a spinner, and a bad network cannot slow down entry between exchanges.

Each bout's log carries a per-bout sequence number. The client tracks the highest sequence the server
has acknowledged. Reconnection is then just:

> *"Server, here is everything after sequence N."*

No merge, no conflict resolution, no prompting anyone to choose a version — with one writer per bout
there is never a second divergent history to reconcile. The server is receiving a log from the only
device entitled to produce it. **This is log shipping, not distributed consensus**, which is why
offline-first fits inside a six-week budget at all.

Two properties make it robust:

- **Retries are free.** The primary key is `(bout_id, sequence)`, so re-delivering events the server
  already holds is a no-op. Clients retry blindly; no deduplication logic on either side, and a push
  that died mid-flight needs no special handling.
- **Edits stay append-only.** Undo and history correction are **correction events appended to the log**,
  like an accounting reversal — never in-place mutation of a past entry. This preserves the property
  that makes sync trivial, and yields an audit trail of what was changed and when, which is exactly
  what a disputed result needs.

**Push cadence is a requirement, not a tuning parameter.** Push after **every exchange**, never at bout
end. If a device is destroyed or lost, anything not yet pushed goes with it, so per-exchange pushing
bounds the worst case to a single exchange — which the judges can simply restate.

### Handover as a first-class feature

Any staff client can **take over a bout and continue where the previous device left off** — pulling
the bout's log from the server and resuming. Single-writer is preserved (one writer *at a time*), but
the transfer is cheap, local, and available at the mat without involving the organizer.

**This is a routine operation, not an emergency one.** Bathroom breaks, shift changes, a battery
running low, someone being called to fence. And it matters more here than in most systems: the
**pliktdomarsystem** (finding N) means competitors are obliged to staff disciplines they aren't
fencing in, so **staff churn is designed into the event**. People rotate off to fence and rotate back.
Handover will happen dozens of times a day, so it has to be a polished everyday flow.

**Simplification worth noting:** the writer token is **per bout**, so a shift change *between* bouts
isn't a handover at all — the next bout is simply assigned to whoever is sitting there. Only mid-bout
switches need the protocol below, which is the rarer case even under heavy rotation.

#### Two paths

**Graceful handover (the common case).** The outgoing device is alive and connected:

1. Device A flushes all pending events and waits for the server to confirm receipt.
2. The server increments the bout's writer epoch and assigns the token to device B.
3. Device A drops to read-only.

**No data can be lost**, because A only releases the token after the server has confirmed everything.
No epoch conflict is possible, so no quarantine is needed. Initiable from either end — A hands off, or
B requests and A confirms. Since the server knows every connected client, the UI is a simple picker
rather than codes to type.

**Ungraceful takeover (the crash case).** Device A is gone — dead battery, dropped in a water bottle,
walked out of the building. B claims the token, the epoch increments without A's participation, and B
resumes at most one exchange behind thanks to per-exchange pushing.

> **The one real correctness trap** lives here. If A later reconnects holding unsent exchanges, its
> late arrivals would interleave with B's and corrupt the log — precisely where the single-writer
> guarantee silently breaks.
>
> **Fix: the writer epoch.** The server **rejects appends from a stale epoch.**
>
> **Rejected events must be quarantined and surfaced to the organizer, never silently discarded** —
> they are real scoring data, and dropping them at a disputed bout would be indefensible. Show
> "device A holds 2 unsent exchanges from before handover" and let a human decide.

**Honest limit:** a graceful handover needs connectivity, since A must flush before releasing. If the
network is down, only the ungraceful path is available — with its quarantine possibility. Worth
surfacing in the UI so staff know which kind of handover they just performed.

The epoch is a few lines of code, but only if designed in now; discovering it at an event means
discovering it as a corrupted bout log.

**The server's underrated job: serving the app itself.** At a venue with no internet, a mat device
that has never opened the app has no other way to obtain it. The host serving the PWA over the LAN is
what makes decision 11 workable — and it's the main practical argument against a pure peer-to-peer
design, which has no answer for an uncached device.

---

## 5. Ruleset findings

Primary source: **MSL's own SM 2022 rules** (`SMArtiklar.json`), cross-checked against the Battle of
the Bridge rules (Örebro HEMA IF) where they differ.

> **On reuse — resolved.** The SM ruleset is MSL's own document from an event MSL hosted, so there is
> no permission question at all. Ship it as the day-one preset. The BotB comparison below is used only
> to prove which parameters *vary between events* — which is exactly what the config schema must
> absorb. If a BotB preset is ever shipped, ask Örebro first as a courtesy.

### 5.1 Findings that change the design

**A. The exchange model is symmetric.** *"Efterslag och slag som träffar samtidigt behandlas på samma
sätt, och båda kan ge poäng oavsett vem som slog först. Båda slag från fäktarna poängsätts oberoende
av varandra."* Afterblows and simultaneous hits are treated identically; both fencers may score; each
fencer's hit is assessed independently. There is no attacker/defender asymmetry.

> **Model correction:** an exchange holds **one independent assessment per fencer**, not
> "attacker + optional afterblow". "Double" and "afterblow" cease to be flags and become *emergent* —
> simply both fencers scoring in the same exchange.

**B. Scoring is a four-judge vote, not a single assessment.** This is the most consequential finding
in the document, and it replaces the simpler model derived from BotB.

Signals a flag judge can show — **five, not three**:

| Signal | Meaning |
|---|---|
| Flag vertical, up | 2 points |
| Flag horizontal | 1 point |
| Flag straight down | No point |
| **Flag behind the head** | Opponent turned their back / back of head during the exchange → **2 points to the fencer who could have attacked** |
| **Flags crossed toward the floor** | *Saw nothing / cannot judge* — an **abstention**, distinct from "no point" |

Resolution rule, from four judges:

- A point is awarded if **at least 2 of 4 judges agree** there was a hit.
- If two judges agree on the hit but **disagree on its value, the lower value is awarded**.
- **Except:** if **two judges show 2 points, 2 is awarded regardless of the others** — their worked
  example is `2-2-1-1 → 2`.

> **UI consequence:** the mat app's primary control records **four judge signals per fencer per
> exchange**, and the engine computes the award. Not a single HIGH/LOW/NONE tap, and emphatically not
> an anatomical body diagram.
>
> **This is a real value-add, not data entry.** The precedence between "lower value wins on
> disagreement" and "two 2s override" is exactly the fiddly logic humans get wrong under time pressure.
> The computer should do it, every time, identically.
>
> **Abstention must be modelled separately from "no point"** — a judge who couldn't see it is not
> voting against.

**C. The judging model is itself a ruleset parameter.** SM uses four flag judges with the vote above;
BotB uses a **judging pair with equal authority**, with the secretariat computing the exchange total.
Same scene, same year, different judging machinery.

> **Schema consequence:** judge count, signal vocabulary, and vote-resolution rule are **config, not
> constants**. Alongside per-weapon scoring tables, this is the schema's real load-bearing work.

**D. Point values are per weapon**, and they differ between events:

| Action | SM 2022 | BotB (for contrast) |
|---|---|---|
| Cut to head above the chin | 2 in longsword / sabre / S&B; 1 in rapier & dagger | same |
| Thrust to torso or head | **2, all weapons** | **3 in rapier & dagger** |
| Pommel strike to the face | **2 (longsword only)** | 1, and only against mask mesh |
| All other legal targets (incl. all dagger hits in R&D) | 1 | 1 |
| One-handed cut outside grappling | 1 regardless of target (longsword only) | same |
| **Maximum for any single hit** | **2** | — |

> **Schema consequence:** issue #4 defines a discipline as *ruleset + weapon*, treating them as
> orthogonal. They aren't — **the weapon parameterises the scoring table inside the ruleset.** Model
> weapon as an input to the ruleset config, not a peer of it. The SM/BotB divergence on the thrust
> value proves this must be data, not code.

**E. Point cap is 8; bouts are 3 minutes.** Time is **not paused** for scoring; the ring judge calls an
explicit time-out for longer interruptions. At **10 seconds remaining** the secretariat calls *"sista
utväxlingen"* and the ring judge echoes it; that exchange runs to its end while fencers are active,
and the ring judge ends it if it turns passive. On reaching the cap the secretariat calls *"match"*.

> **Features:** a 10-second warning cue for the secretariat, and an explicit time-out control distinct
> from the running clock.

**F. There is an explicit afterblow window in the bout procedure.** A flag judge calls *"träff"* on the
first hit; *after time for the afterblow*, the ring judge calls *"bryt"*; then *"domare"*, at which
point the flags are shown. The bout state machine should model this sequence — it's when the app
prompts for judge signals.

**G. Draws exist, and match points aren't 3/1/0.** Tied at time = a draw in pools; sudden death (one
exchange at a time until someone leads) in eliminations. Pool match points: **win 9, draw 6, loss 3.**
The bout result type must include `draw`.

**H. Tiebreakers are indices, divided by matches *completed*** — not totals:

1. Match point index — match points ÷ matches completed
2. Victory index — wins ÷ matches completed
3. Score index — (points scored − points conceded) ÷ matches completed
4. Reception index — points conceded ÷ matches completed (**lowest** wins)
5. Head-to-head result
6. Random draw

Dividing by *completed* matches is deliberate: it makes incomplete pools rank correctly, which is what
makes finding (I) workable.

**I. Withdrawal handling is fully specified:**

- **Forfeit a single match** → recorded as **8–0** to the opponent regardless of the score at the time;
  winner takes 9 match points, the forfeiting fencer 0.
- **Withdraw or DQ during pools** → *the pool is handled as if that fencer never participated*,
  regardless of how many bouts they'd already fenced. **Their results are retroactively voided.**
- **Withdraw after qualifying for elims** → walk-over win for the opponent.

> **Data-model consequence:** the standings engine must support **retroactively voiding a competitor
> and all their bouts**, with everyone else's indices recomputing correctly. Design in from the start;
> miserable to bolt on.

**J. Finals are best-of-three.** *"Matcherna om tredje- och förstaplatserna avgörs av de bästa av tre
matcher med 1 minuts paus emellan."* First and third place are decided over up to three bouts with
1-minute breaks. Won by taking two bouts, **or one win and two draws**. Still level after three → sudden
death, first to score at least one point.

> **Model consequence:** finals need a **best-of-N series** concept sitting above the individual bout.
> The bout cannot be the top-level unit. Small to add now, structural to add later.

**K. Penalty escalation is discretionary, not automatic.** The normal ladder is warning → point
deduction → loss of match → disqualification. But the ring judge **may jump straight to any severity**
for serious intentional violations, and **may issue several warnings without escalating** when
offences are minor and unrelated.

> **UX consequence:** the app must **propose** the next escalation step and let the ring judge
> override it. Do not auto-apply penalties. *(Note this differs from BotB, which escalates strictly —
> so the escalation policy is also ruleset config.)*

Penalties also attach to **coaches**, with the resulting warning landing on their competitor. Each
competitor is entitled to a coach, who must stand and stay ≥2 m from the mat during an exchange.

**L. Grappling is a second scoring source**, judged by the ring judge alone (flag judges score only
weapon hits), enabled per tournament: mat exit 1, disarm 2 (in rapier & dagger only if the *rapier* is
lost, not merely the dagger), controlled takedown 2, dominant position when neither stands 1, opponent
falls out of grappling range 2, **arm lock or weapon control while able to mark a head hit 2**,
**controlled lift with both feet off the ground 2**. Kicks and strikes to legal targets are permitted
but score nothing. Deadlock guideline ~5 seconds.

**M. An injury time-out stops every mat**, not just the one involved: *"Inga matcher på några mattor
ska genomföras under sådan time-out."*

> **Feature:** a tournament-wide pause state affecting the organizer dashboard and, later, the
> timetable projection — not merely a per-bout timer stop.

**N. Registration, staffing and data — three findings at once:**

- **Registration is a Google Form.** MSL's actual current workflow. **Decision 4 (CSV import) matches
  reality exactly** and needs no defending.
- **Pliktdomarsystem (duty-judge system).** Competitors are *expected* to serve as staff in at least
  one discipline — head judge, flag judge, or secretariat. **This substantially rewrites issue #5.**
  Staffing isn't voluntary availability; it's an obligation attached to competing, with a hard
  constraint that each competitor is rostered to a discipline they aren't fencing in. That's a far more
  interesting assignment problem than issue #5 describes, and it's the real one. **Phase 4.**
- **Health data.** SVHEMAF and MSL document injuries requiring care and conditions affecting care
  (concussion, medication allergies, diabetes). That is **special-category data under GDPR Article 9**,
  a materially higher bar than ordinary personal data.
  > **Recommendation: Porta di Ferro should not store medical data at all.** Leave it in the existing
  > organizer/medical process. A clean, defensible scope boundary that costs nothing and removes the
  > single heaviest compliance burden. Record it as an explicit non-goal.

**O. Categories are real.** SM 2022 ran *långsvärd (herr och dam)*, sword & buckler, sabre, and rapier
& dagger — longsword split into men's and women's divisions sharing weapon *and* ruleset. **Confirms
the category axis is separate from discipline**, as suspected against issue #4.

**P. Roles**: tournament manager (final authority, present throughout), ring judge, flag judges (a
group), secretariat (announces matches, keeps time, records score, issues wristbands). The penalties
text also references timekeeper, score-counter and administrator as distinct functions.

> **This confirms decision 6.** The secretariat is a single, well-defined person who already owns score
> entry — our one writer per bout. **The mat app is the secretariat's tool.**

**Q. Illegal targets:** back of head, nape, spine, achilles, groin, back of knee, feet. **Illegal
actions** (a separate list): striking the throat, throwing by the neck or against a joint's natural
direction, joint techniques against natural direction, grabbing the mask, striking an unprotected head
if the mask comes off, hitting the floor with the weapon, turning the back, weapon techniques without
at least one hand on the grip (e.g. Mordschlag), striking with the crossguard.

**R. Red/blue wristbands** are the identification convention — keep red/blue, but carry redundant
name/position cues for colour vision deficiency.

### 5.2 Still needed

- Confirmation that 1 October uses the **SM parameters** above, or a diff. *(SM 2022 is four years old;
  a newer club ruleset may exist.)*
- **Judge count on the day.** Four flag judges per mat is a lot of people for a club-internal event with
  ≤40 participants. If you'll actually run two, the vote rule needs a defined two-judge form —
  **and finding (C) means that's config, not a rewrite.**
- Whether **grappling is enabled** on 1 October. If off, finding (L) drops out of the 1 Oct build
  entirely and phase 1 gets noticeably cheaper.
- **Pool size** policy and how many advance to elims (SM defers both to participant count).

---

## 6. The 1 October constraint

**Today is 19 August 2026. The event is 1 October 2026 — roughly six weeks, for two people working
evenings, on a greenfield codebase.**

Tight but achievable, *provided* scope is cut ruthlessly and the fallback ladder is real. Event
parameters are forgiving: ≤40 participants across 1–4 disciplines means small pools, probably one or
two mats, club-internal audience.

### The calendar

| Date | Event |
|---|---|
| 19 Aug | Plan approved; stack decision immediately after |
| 25 Aug | MSL autumn term begins — club nights become available for live testing |
| ~15 Sep | Checkpoint: are phases 1–2 solid? Stretch bracket lives or dies here |
| **18–20 Sep** | **Battle of the Bridge 2026, Kristinehamn Arena (Örebro HEMA IF)** |
| 24 Sep | **Go/no-go: pick the fallback tier** |
| 1 Oct | The event |

**BotB lands 13 days out, directly on the stretch-goal checkpoint.** If either of you is attending, a
weekend plus travel and recovery disappears from an already-tight six weeks — plan around it rather
than discovering it in mid-September.

But it is also **the single best research opportunity available**, and it happens before any of this
is locked in code:

- Watch a real secretariat work, and validate the four-signal judge-vote capture model (finding B)
  against live judging — including how fast signals actually need to be entered between exchanges.
- **Time real bouts, changeovers and the judging conference** — the real input to phase 4's timetable
  estimates, otherwise pure guesswork.
- See how withdrawals, penalties and tiebreak disputes play out in practice.
- Compare BotB's judging pair against SM's four-judge vote in the flesh, which directly tests whether
  finding (C)'s config-driven judging model is general enough.

Treat attending as a project activity, not a distraction — and bring a stopwatch and a notebook.

**Club nights from 25 August are the other free asset**: a standing weekly opportunity to put phase 1
in front of real fencers, worth more than any amount of synthetic testing.

### Hard cut line

**Committed for 1 October** — pools only, no eliminations:

- SM ruleset preset per §5, scoring engine incl. the judge-vote resolver, exchange-log mat app
  (phase 1 entire)
- Laptop server on the LAN, joined via printed URL/QR
- CSV import of ≤40 competitors
- **Minimal multi-discipline** — 1–4 disciplines per tournament, each with its own roster and pools.
  *(Pulled forward from phase 3 because the event needs it. Cheap: a field on competitors and pools,
  not the full discipline/category model.)*
- Pool generation with club balancing + bout ordering, manual override
- Live standings with the §5 index tiebreakers, and **withdrawal handling per finding (I)**
- Printable pool sheets, continuous file backup

**Stretch, cut without ceremony if phases 1–2 aren't solid by ~15 September:**

- A minimal top-4 or top-8 single-elimination bracket with §5 seeding (highest vs lowest qualifier),
  sudden death, and a 3rd-place match. **Note finding (J):** a proper final is best-of-three, so the
  cheap version is a single-bout final with the series concept deferred — say so out loud rather than
  quietly shipping the wrong format.

**Explicitly deferred:** staff and the duty-judge system, timetable, personal schedules, categories,
Swiss, best-of-N series, stream overlay, crests, registration, HEMA Ratings export, statistics — all
of phases 4–5.

### Fallback ladder

What makes the timeline safe. Each tier independently runs the event, and **phase 1 needs no server**,
so even total failure of phase 2 leaves a working tool:

| Tier | If… | Run the event on… |
|---|---|---|
| 1 | Everything works | Full system — mat app + laptop server + live standings |
| 2 | Server/sync not ready | **Mat app alone** on tablets, printed pool sheets, standings by hand |
| 3 | Nothing is ready | All paper, as always |

**Decide the tier on 24 September**, one week out — not on the morning.

---

## 7. The phases

### Phase 1 — The mat  *(committed for 1 Oct)*

*Goal: usable at a club sparring night with zero infrastructure. No server exists yet.*

**Workstream A — the pure logic core** (build first; it's the heart of everything):
- Ruleset config schema: **weapon-keyed** scoring tables (D), **judging model** — judge count, signal
  vocabulary, vote-resolution rule (B, C), point cap, match time, penalty ladder and whether it
  escalates strictly or by discretion (K), grappling toggle and actions (L), victory conditions.
- **Symmetric exchange model** per finding (A) — per-fencer assessment, no attacker/defender.
- **The judge-vote resolver** (B) — the 2-of-4 threshold, lower-value-on-disagreement, the two-2s
  override, and abstentions excluded from the count. Small, subtle, and the highest-value unit tests
  in the project.
- **Scoring engine**: a pure fold, `(ruleset config, exchange log) → bout state`. Deterministic, no
  I/O, exhaustively unit- and property-tested.
- **The SM preset** — the only ruleset needed day one, which substantially de-risks phase 1. Build the
  schema generically as decided, validate and ship exactly one preset.

**Workstream B — the mat UI** (the secretariat's tool, per finding P):
- **Judge signal capture**: four judges × five signals per fencer per exchange (B), laid out for speed
  between exchanges. Large tap targets. Score is *derived*, never stored directly. **Prototype this on
  an actual phone first** — it's the densest screen in the app and decision 11 means it must work at
  phone size, not just on a tablet.
- **i18n scaffolding and the glossary** (decision 13): every string externalised from the first commit,
  Swedish and English locales both populated, plus the Swedish-rule-term → English-identifier glossary
  that keeps the code checkable against the source rules.
- Bout procedure per finding (F): hit called → afterblow window → *"bryt"* → *"domare"* → signals
  entered.
- Timer per finding (E): 3 min, never paused for scoring, explicit time-out control, **10-second "last
  exchange" cue**, point cap at 8.
- Penalties per finding (K): **propose** the next escalation step, always overridable; coach penalties
  attach to the competitor.
- Grappling actions if enabled (L).
- Undo and full history editing — free consequence of the exchange log.
- Bout state machine: `not started → running → halted → finished → confirmed`, with **draw** as a valid
  result (G). The confirmation step exists because disputes happen.
- Local persistence surviving reload and crash.
- Red/blue per finding (R), plus redundant non-colour cues.
- Printable blank pool sheet — the tier-2/3 fallback.

**Out:** accounts, tournaments, network code.

---

### Phase 2 — The pool  *(committed for 1 Oct)*

*Goal: run the 1 October event on one laptop plus a handful of tablets.*

- **The server app.** Single artifact, started by double-click or one command, per §2. Serves the LAN;
  mats join via printed URL/QR or mDNS. **This component embodies the product thesis — if it isn't
  trivially installable, the project has failed at its main goal.**
- Competitor and club records; **CSV/spreadsheet import** matching the existing Google Form export (N).
- **Minimal multi-discipline**: 1–4 disciplines per tournament, own roster and pools each.
- **Pool generation** (issue #3): club balancing, max pool size, max pool count. Snake-draft by club
  size handles club separation without a solver. *(At a club-internal event everyone shares a club, so
  this path gets no real exercise on 1 Oct — cover it synthetically.)*
- **Bout ordering** minimising back-to-back bouts. For some pool sizes a strict guarantee is
  impossible; define a fallback and report remaining violations.
- **Manual override of everything** — drag fencers between pools, reorder bouts. Non-negotiable.
- Mat assignment, and the secretariat's match-announcement queue (P).
- **Sync**: single-writer append-only log per bout, mat → server, pushed every exchange.
- **Handover UI** — a routine flow, not an emergency one, given the staff churn the duty-judge system
  creates. Graceful handoff via a picker of connected clients (either party can initiate), plus
  ungraceful takeover when a device is gone, with the epoch and quarantine handling from §4.
- **Standings** with §5 index tiebreakers, **retroactive voiding of withdrawn competitors** (I), and
  **visible tiebreak resolution** (borrowed from Scorecard, §3).
- Organizer dashboard: every mat at a glance, bouts done vs pending, plus the **tournament-wide injury
  pause** (M).
- Export: CSV/JSON snapshot, printable *filled* pool sheets.
- **Continuous backup to a downloadable file** — event-day disaster recovery.

---

### Phase 3 — The tournament

- Full discipline + **category** model — confirmed necessary by finding (O)'s longsword herr/dam split
  — extending phase 2's minimal version rather than rewriting it.
- **Eliminations** (brief v0.4): cut size, §5 seeding, top-seed byes, sudden death, walk-overs for
  withdrawals.
- **Best-of-N series** for finals and 3rd place per finding (J), including the win-one-draw-two
  condition and sudden death after three.
- **Swiss system** — SM notes it may replace pools before eliminations depending on entry numbers, and
  BotB says the same, so this is a real requirement rather than speculation.
- Multiple mats across concurrent disciplines.
- Public read-only results view on the LAN (QR posters at the venue).
- **Static post-event export** publishable anywhere once back online — the LAN-only mitigation.
- Additional ruleset presets beyond SM — the BotB variant is the obvious second, and finding (D)'s
  divergences make it a good schema stress test.

---

### Phase 4 — The day  *(the differentiator vs the incumbents)*

*Delivers brief v0.5.*

- Person model with per-tournament roles from finding (P) — tournament manager, ring judge, flag
  judges, secretariat, timekeeper, medical. One entity that is both competitor and staff, which issue
  #5 correctly anticipates.
- **The duty-judge system (pliktdomarsystem)** per finding (N). This is the substantive correction to
  issue #5: staffing is an **obligation attached to competing**, not voluntary availability. The
  assignment problem is *"every competitor must be rostered into at least one discipline they are not
  fencing in, while keeping people in one role and one pool where possible"* — harder and more useful
  than what issue #5 describes.
- Staff availability by discipline *and* time of day.
- **Timetable** (issue #6): heuristic generation plus a drag-and-drop editor. Fixed blocks for lunch,
  equipment check, briefings, ceremonies. **Not a constraint solver** — see §9.
- **Live re-projection as the day slips**, accounting for tournament-wide injury pauses (M). In neither
  source document, but the truest reading of the brief's intent about "informed decisions about lunch".
- Personal schedule per competitor via QR badge on the venue LAN: bouts, mat, time to next, gaps — and,
  given the duty-judge system, **their judging assignments too**, which is arguably the more useful half.

---

### Phase 5 — Polish

- **Statistics** — the §3 differentiator, nearly free over the exchange log, including judge agreement
  and calibration analysis that no incumbent can produce.
- **Streaming overlay** (brief v14.0) — OBS browser source on the LAN, transparent background. Cheap
  once phases 1–3 exist: a read-only view of existing data.
- Persistent fighter profiles with history (borrowed from HEMAGON).
- Club crests and competitor photos (issue #1).
- **HEMA Ratings export** — a concrete adoption lever.
- Native registration, if ever (out of scope by decision 4).
- Notifications — **blocked by LAN-only**; revisit only if the server gets internet.

**Explicit non-goal:** storing medical or health data, per finding (N).

---

## 8. What these decisions constrain about the stack

Not choosing a stack here, per instruction — but five constraints are fixed, ordered by how binding:

1. **Trivial installability, per §2.** A single self-contained binary or equivalent on the host. No
   separate database server, no Docker requirement, no pre-installed runtime, cross-platform. **The
   dominant constraint** — it eliminates whole families of otherwise reasonable choices, and it's
   exactly what HEMA Scorecard got wrong. Note that only the *host* installs anything; per decision 11
   the clients install nothing at all.
2. **The scoring engine must run identically on mat client and server.** The client needs it offline
   for live score; the server needs it as authority. One language both sides, a rigorous shared spec
   with two implementations, or compiling the core to WASM.
3. **Clients are web/PWA only** (decision 11) — offline-capable, reliable local persistence, wake-lock,
   and usable on a phone-sized screen. **The host must also serve the PWA over the LAN**, since that's
   how an uncached device obtains the app at a venue with no internet.
4. **Keep the host portable** (decision 12). Phones are clients only for now, but don't foreclose an
   ARM/Android host later — a server that can only ever target x86 desktops closes that door
   permanently. Worth weighing, not worth paying much for.
5. **LAN discovery** — mDNS, or accept a printed URL/QR as the joining mechanism.
6. **Strong property-based testing** for the pure core. The judge-vote resolver, rulesets, pool
   generation, standings and tiebreakers are deterministic, high-value, and horrible to debug live at
   an event.
7. **Mature i18n support** (decision 13) — Swedish and English from the first commit, with English
   identifiers in code and both languages as display locales.

Constraints 1 and 2 pull in tension — the easiest way to satisfy 2 is one language everywhere; the
easiest way to satisfy 1 is a compiled single binary. **That tension is the substance of the stack
conversation**, which should happen immediately after this plan is approved, before any code.

Also unresolved: the Python `.gitignore` — deliberate signal, or an artefact of `gh repo create`?

---

## 9. Risks

| Risk | Assessment |
|---|---|
| **Six weeks is short** | Dominant risk. Mitigated by the §6 cut line and three-tier fallback. Phase 1 needs no server, so partial completion still yields a usable tool. Tier decision **24 Sept**. |
| **BotB 18–20 Sept eats the stretch window** | A weekend plus travel lands on the 15 Sept checkpoint. If attending, assume the stretch bracket is *already* cut and treat it as a bonus. Offset by the research value in §6. |
| **Installability treated as a late chore** | Would forfeit the entire product thesis. Package and test the 5-minute install from the *first* week of phase 2, on a machine that isn't yours. |
| **The judging model is more variable than expected** | Findings (B) and (C): SM votes across four judges, BotB uses a pair. Getting the judging model into config rather than code is the difference between a two-event tool and a general one. **Highest-risk part of the schema.** |
| **Judge-vote resolver correctness** | Subtle precedence, easy to get wrong, and wrong answers are visible and disputed live. Property-test it hard; it is the single highest-value test target in the project. |
| **Judge capture on a phone screen** | Decision 11 means the secretariat may be on a phone, and finding (B) needs up to 4 judges × 5 signals × 2 fencers per exchange. That is a lot of state on a small screen, entered under time pressure between exchanges. **Prototype this layout early** — it's the riskiest single screen in the app, and validate it at BotB. |
| **Resurrected writer device** | The one place single-writer breaks. Mitigated by the writer epoch in §4, but only if built in from the start rather than discovered as a corrupted bout log at an event. |
| **Localization drift** | Decision 13 puts English identifiers over Swedish source rules. Mitigated by the glossary deliverable in phase 1 — without it, the code and the rules it implements diverge quietly. |
| **Retroactive withdrawal voiding** | Finding (I) is easy to build in now and horrible to retrofit. Don't defer it as an edge case. |
| **Best-of-N finals discovered late** | Finding (J) means bout ≠ top-level unit. Cheap to anticipate in the schema now even though series play is phase 3. |
| **Timetable as constraint solver** | How this project dies later. Organizers override everything anyway. Heuristic + editor first. Not a phase 1–2 concern. |
| **Offline-first cost** | Real but defused by single-writer. Design in from day one; retrofitting is what's expensive. |
| **Venue AP client isolation** | Could break mat↔server entirely. **Test at the venue beforehand.** Fallback: laptop hotspot. |
| **Club balancing untested on 1 Oct** | Everyone shares a club at a club-internal event. Cover synthetically. |
| **Scope creep from the issues** | The six issues describe phases 3–4. Resist pulling them into phases 1–2. |

---

## 10. Verification

No code exists yet, so this is the test strategy the plan commits to:

- **Judge-vote resolver suite** — the 2-of-4 threshold, lower-value-on-disagreement, the two-2s
  override (`2-2-1-1 → 2`), abstentions excluded from the count, back-turned signals awarding to the
  opponent. Exhaustive over the signal space; it's small enough to enumerate completely.
- **Pure core**: unit + property tests on the scoring engine, pool generation, standings and
  tiebreakers. Near-total coverage; runs without UI or network.
- **Ruleset conformance suite**: known bout scenarios → expected score and winner, covering the §5
  weapon-keyed tables, symmetric afterblow/double assessment, grappling actions, discretionary penalty
  escalation, and draws. Written directly from §5. **Add the BotB variant as a second preset purely as
  a schema stress test**, even before shipping it.
- **Standings suite**: the four indices, head-to-head fallback, 8–0 forfeits, and **retroactive voiding
  of a withdrawn competitor** with everyone's indices recomputing correctly.
- **Installability test — a first-class acceptance test.** On a clean machine that is not a
  developer's, with a stopwatch: download → running tournament in under 5 minutes, no prior tooling.
  Ideally performed by someone who didn't write it. **If this fails, the release fails.**
- **Simulated event harness**: replay a synthetic tournament — import → pools → all bouts → standings —
  asserting invariants end to end.
- **Offline testing**: airplane-mode a mat device mid-bout, score a full pool, reconnect, assert the
  server log matches the device's exactly.
- **Handover testing**, both paths:
  - *Graceful*: hand off mid-bout from a live device, initiated from each end in turn, and assert zero
    loss and no quarantine. This is the everyday path and should be boringly reliable.
  - *Ungraceful*: kill the writer device mid-bout, take over from a second client, confirm no lost or
    duplicated exchanges. **Then bring the original device back online holding unsent exchanges** and
    assert its stale-epoch appends are rejected, quarantined, and surfaced to the organizer rather
    than silently dropped or interleaved. The sync model's one genuine correctness trap — test it
    deliberately.
  - *Repeated rotation*: hand a single bout through three or four devices in sequence, mimicking real
    duty-judge churn, and assert the log stays intact and correctly ordered throughout.
- **Localization pass**: every user-facing string resolves in both Swedish and English, with no
  hardcoded text and no untranslated keys. Cheap to check continuously, painful to fix in bulk later.
- **Field observation at BotB, 18–20 September**: validate signal-capture speed against live judging,
  and time real bouts, changeovers and judging conferences (§6).
- **Club-night trials from 25 August**: put phase 1 in front of real fencers weekly. Worth more than any
  amount of synthetic testing, and already in the calendar.
- **Full LAN dress rehearsal before 24 September**: real tablets, real laptop, real venue wifi, a mock
  pool with volunteers. The acceptance test for 1 October and the input to the fallback-tier decision.
- **Paper fallback drill**: confirm a pool can be run on printed sheets and typed back in afterwards.
