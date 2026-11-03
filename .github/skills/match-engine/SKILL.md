---
name: match-engine
description: "Use when changing or debugging match scoring, penalties, timer behavior, undo, match-ending rules, replayed state, or the Go and TypeScript match engines and shared test vectors."
---

# Match Engine Changes

Use this workflow for scoring and match-state behavior in `internal/match/` and `web/src/lib/match/`. The match engine is deliberately implemented twice; `testdata/vectors/` is the executable contract that keeps the two implementations aligned.

## Invariants

- Match state is derived by replaying the append-only event log. Do not store or mutate derived state as a substitute for replay; corrections are appended as undo events.
- Keep Go and TypeScript behavior equivalent. Changes to event fields or their JSON representation must be made consistently in both engines and treated as a persisted-data/API contract change.
- Event values and identifiers on the wire remain language-neutral English identifiers such as `red`, `exchange`, and `point_cap`.
- Match score and pool match points are different concepts. Scoring rules belong in the match engine; tournament ranking belongs elsewhere.
- Treat [docs/design.md](../../../docs/design.md) as the product rules authority and [AGENTS.md](../../../AGENTS.md) as the repository workflow authority. Do not resolve an ambiguous rule by making the two engines agree on an undocumented guess.

## Procedure

1. Trace the behavior through the Go implementation, its TypeScript mirror, and the relevant vectors before editing. Check the existing edge-case tests and identify the exact expected state transition.
2. State the rule and its boundary cases from the product documentation. Keep the change limited to the engine contract; do not pull in tournament ranking or UI behavior unless the request requires it.
3. Implement the same behavior in both engines. Preserve the existing event schema and public types unless a contract change is necessary.
4. Add or update a focused JSON vector under `testdata/vectors/`. Give it a descriptive filename, `name`, and `description`; include the event sequence and the complete expected derived state. Set the expected result from the rule, not by copying one engine's output as the oracle.
5. Cover relevant interactions and precedence, not just the happy path. Depending on the change, consider undo/voided events, penalty thresholds and deductions, score floors, point-cap overshoot, timer thresholds, simultaneous end conditions, forfeits, and events after the match has ended.
6. If scoring rules or state transitions change, update the corresponding section/diagram in [docs/architecture.md](../../../docs/architecture.md); update [docs/design.md](../../../docs/design.md) when the documented product rules themselves change.

## Verification

Run both implementations against the shared corpus and their local tests:

```sh
go test ./internal/match
cd web && npm test -- src/lib/match
```

If the change affects other packages that consume match state or changes a persisted/API contract, broaden verification to the relevant Go and web suites. Do not claim parity based on tests for only one engine.