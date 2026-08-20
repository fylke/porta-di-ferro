# Porta di Ferro — Open Questions

> Companion to [`design.md`](./design.md). Answer in place by editing this file.
>
> Most of the original questionnaire is now settled — see §3 for the record. What follows is what
> genuinely remains.

---

## 1. Blocks starting the MVP

**Q1. How is a mistake corrected after *Confirm exchange*?**
The table view as specified has a 2-point button, a 1-point button and a warning button — no minus,
no undo. Two situations have nowhere to go today:

- a mis-tap that is only noticed after confirming
- the ring judge ordering a **point deduction** as a penalty

Options: an undo of the last confirmed exchange; an edit mode over the bout history; a dedicated
deduction control; or a supervisor override on the server. *Suggestion: undo-last-exchange in MVP,
full history editing later — the append-only log supports both, so this is purely a UI decision.*

**Q2. Exact timer semantics.**
The clock turns red at 10 seconds and *keeps running until the final exchange is confirmed*. Needs
pinning down before it's built:

- Does it count **down to 0:00 and stop**, count **negative**, or count **up** once past zero?
- Does the 10-second warning fire again if the clock is stopped and restarted?
- What ends the bout — confirming the final exchange, or an explicit "end bout" action?
- Does reaching the **8-point cap** end the bout immediately on confirm, mid-exchange?

**Q3. Do warnings affect the score in MVP?**
MSL's rules escalate warning → point deduction → loss of match → disqualification, but the ring judge
applies that at their discretion. *Suggestion: MVP records and displays warnings only, and any point
consequence arrives through the normal scoring buttons as the ring judge directs.* Confirm.

**Q4. Who is red and who is blue?**
Assigned automatically when pools are generated, or chosen at the table before the bout starts?

---

## 2. Blocks finishing the MVP

**Q5. Does 1 October actually fit in 28 fencers?**
MVP caps at 4 pools × 7 = **28**, but the event was described as up to 40 participants across 1–4
disciplines, and MVP has no discipline concept. Either a single tournament run stays under 28, or the
app gets run once per discipline. Which is it in practice?

**Q6. How are pools assigned to the 2 mats?**
One pool per mat running concurrently, or bouts from any pool dealt to whichever mat is free? The
second is better for throughput and considerably more complex.

**Q7. Is localisation still wanted, and in which milestone?**
Swedish and English were agreed earlier, but the restructure left localisation unplaced — it now sits
in Future. Worth deciding deliberately: **scaffolding i18n costs almost nothing at the start and is
tedious to retrofit**, even if the Swedish strings arrive much later. *Suggestion: scaffold in MVP,
translate whenever.*

**Q8. Which milestone owns the cloud/web server?**
It was described as a stretch goal in discussion but appears under Future in the milestone list.
Currently placed in **Future**. Move it if that was the intent.

---

## 3. Settled

Recorded so it doesn't get re-litigated.

| | Decision |
|---|---|
| Ruleset | MSL SM rules confirmed; **longsword scoring for all weapons** at this stage |
| Judges | **Irrelevant to the app** — the table waits for the ring judge's final decision |
| Grappling | Permitted, and costs the app nothing under the above |
| Client platform | **Android for MVP**, iOS a stretch goal. Gloves not a factor |
| Scoreboard | Stretch: secondary monitor from the server PC. Future: per-mat clients |
| Pools | Uneven sizes acceptable; best-effort bout ordering that **reports** remaining violations |
| Eliminations | MVP none; stretch **top 8 from pools**; future configurable |
| Mats | MVP 1–2, stretch up to 4 |
| Export | MVP **JSON**, stretch **+ PDF** |
| Exchange log | Timestamps, scoring, warnings, and **timer start/stop events** (stopped at X, resumed after Y seconds). Nothing else |
| Rest between bouts | Maximise, but not a hard constraint |
| Scope | Club-agnostic setup |
| GDPR | Not an app concern for MVP or stretch. Future: organizer chooses indefinite storage or HEMA Ratings push, with participant consent at signup |
| Licence | **MIT** — already in place |
| Testing | Heavy tests on the pure logic core |
| Scorecard import | Not worth the effort |
| Staff, timetable, categories, team and non-bout events, public results, streaming overlay, HEMA Ratings, accessibility | **Future** |

---

## 4. Next decision — the stack

Not a question for this document, but the next thing to settle, and it is now unblocked.

Two constraints dominate:

1. **Installability.** A single self-contained artifact the organizer starts on a PC. No separate
   database server, no container runtime, no pre-installed language runtime. This is the whole
   premise, and it eliminates entire families of otherwise reasonable choices.
2. **Shared scoring logic.** The table client needs it offline to show a live score; the server needs
   it as the authority. That means one language on both sides, a shared spec implemented twice, or a
   core compiled to WebAssembly.

These pull against each other — the easy answer to (2) is one language everywhere, while the easy
answer to (1) is a compiled binary. That tension is the substance of the decision.

Two answers change the usual calculus:

- The Python `.gitignore` was an artefact, so **it carries no signal** and constrains nothing.
- The stack will be **built with Claude Code rather than chosen for existing familiarity**, so it
  should be optimised for the two constraints above and for long-term maintainability — not for what
  either of you has written most of before.
