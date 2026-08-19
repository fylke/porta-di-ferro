# Porta di Ferro — Open Design Questions

> Companion to [`design.md`](./design.md). Section references (§5, finding B, decision 11 …) point
> into that document.
>
> Most of the original questionnaire is now answered — by the decisions in §4, and by the ruleset
> findings in §5. These are what remain. Answer them **in place**, editing this file; several have a
> **Default:** line, so "defaults except Q33 and Q46" is a complete answer.
>
> Questions are grouped by the phase they block, so they can be answered just-in-time rather than
> all at once. The ones under *Urgent* genuinely gate the start of work.

## Urgent — blocks starting phase 1

- **Q13c. Confirm the §5 SM parameters are what 1 October will use**, or give the diff. SM 2022 is four
  years old; a newer club ruleset may supersede it.
- **Q13e. How many flag judges on the day?** Four per mat is a lot of people for ≤40 participants. If
  it's really two, the vote rule needs a defined two-judge form. Finding (C) means that's a config
  value, not a rewrite — but it must be decided before the resolver is written.
- **Q13f. Is grappling enabled on 1 October?** If off, finding (L) drops out and phase 1 gets
  noticeably cheaper. *Recommend off for the first outing.*
- **Q13b. Which 1–4 disciplines**, and do they all use the §5 weapon table?
- **Q8. Mat hardware** — what will the tablets actually be on the day? Gloves in use?
- **Q9. Separate spectator display** per mat? *Default for 1 Oct: no.*

## Blocks phase 2 / the 1 October build

- **Q33. Pool size** for ≤40 across 1–4 disciplines. Are uneven sizes (5,5,4) acceptable?
- **Q36. "No two bouts in a row"** — guarantee, or best-effort with a reported quality score?
  *Default: best-effort, minimise and report violations.*
- **Q2b. Do you want a final on 1 October?** If yes it's the §6 stretch item — and note finding (J)
  says a proper final is best-of-three, so decide whether a single-bout final is acceptable.
- **Q32b. How many mats** on 1 October? *Default assumption: 1–2.*
- **Q40b. How many advance to elims**, if elims happen?
- **Q65. Export formats** — CSV, JSON, printable PDF. Which do you need on the day?

## Blocks phase 3

- **Q29. Disciplines global or per-tournament?** *Default: global catalogue, copied into a tournament so
  later template edits don't rewrite history.*
- **Q31b. Swiss priority** — a real requirement per §5. Early or late in phase 3?
- **Q30. Team events?** *Default: out of scope.*
- **Q20. Non-bout events** — cutting, forms, solo? *Default: out of scope, but don't let the schema
  preclude them (§3).*
- **Q63. Persistent public results** after the event — permanent tournament pages, fencer profiles?

## Blocks phase 4

- **Q46. Duty-judge specifics** (finding N): is one discipline of duty per competitor the actual rule?
  Can people opt out by paying more, or is it mandatory? Do non-competing volunteers fill gaps first?
  **This shapes the whole assignment algorithm.**
- **Q48. Staff continuity goals** — hard constraints or soft preferences, and which wins in conflict?
- **Q51. Timetable inputs** — venue hours, mat count, expected bout duration per discipline, changeover
  time, fixed blocks. *(§5 gives 3-minute bouts plus the judging conference as a starting estimate —
  measure the rest at BotB.)*
- **Q53. Minimum rest between a competitor's bouts** — hard minimum, or just maximise?
- **Q55. Personal schedule contents** — bouts, mat, gaps, standings, next opponent, and judging duties?

## Blocks phase 5

- **Q57b. Which statistics matter most to you?** This shapes what the exchange log must capture in
  phase 1 — cheap to record now, impossible to backfill. Judge calibration is available essentially
  free; is that interesting or politically radioactive?
- **Q62. Streaming overlay contents** — names, clubs, score, time, penalties, bracket context?
- **Q66. HEMA Ratings** — export for ingestion? Import ratings for seeding?
- **Q67. Import from HEMA Scorecard** — worth building as an adoption on-ramp?

## Cross-cutting

- **Q4. MSL's events only, or a product other clubs adopt?** The §2 thesis implies others — "anyone can
  self-host" only matters if there's an *anyone*, and the natural audience is the rest of Svenska
  HEMA-förbundet. Confirm, since it raises the bar on docs, packaging and Swedish localisation.
- **Q64. GDPR** — confirm the finding (N) recommendation that the app stores **no medical data at all**.
  Beyond that: who is controller (MSL, SvHEMAF, or joint), what's retention, is there a delete path?
- ~~**Q69. Languages**~~ — **resolved**: Swedish and English both, from the first commit, with English
  identifiers in code (decision 13). Carries one deliverable: **a glossary mapping the Swedish rule
  terms to their English identifiers** (`flaggdomare → flag_judge`, `ringdomare → ring_judge`,
  `sekretariat → secretariat`, `sista utväxlingen → last_exchange`), so the implementation stays
  checkable against the Swedish source rules. Ships in phase 1.
- **Q70. Accessibility** — WCAG 2.1 AA as a stated goal? *Default: yes; note finding (R).*
- **Q72. Licence** — what's in `LICENSE`? Given §2, it should actively encourage other clubs to run and
  modify it.
- **Q73. Who's building what, and what do you each know well?** Drives the stack decision more than any
  technical merit argument — especially given the §8 tension between constraints 1 and 2.
- **Q74. Realistic hours per week?** **Directly determines whether the 1 October stretch goal is real.**
- **Q75. The Python `.gitignore`** — deliberate, or an artefact of `gh repo create`?
- **Q77. Testing appetite** — confirm heavy tests on the pure core, lighter elsewhere. *Default: yes.*
