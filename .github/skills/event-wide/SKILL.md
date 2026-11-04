---
name: event-wide
description: "Use when adding or changing event-level behavior across multiple disciplines, the HTTP coordinator, event-owned data, or the browser demo. Includes mats and plans, people, forecast, staffing, event signup, and coordinator routes."
---

# Event-Wide Feature Changes

Use this workflow when behavior belongs to the event as a whole rather than to one tournament discipline. The real server implements this boundary in `internal/http.Coordinator`; the browser demo implements it in `internal/demo.Event`.

## Ownership and Reuse

- Read the event/discipline boundary in [AGENTS.md](../../../AGENTS.md), [docs/architecture.md](../../../docs/architecture.md), and the relevant proposal or package docs before changing ownership or behavior.
- Keep event data such as `event.json`, the plan, people, staff, screens, and shared device presence at the coordinator/event level. A discipline must not write event state except through its established event interface (`EventInfo`). Preserve each discipline's separate store and folder.
- Nothing below the coordinator may assume it is the only discipline. Exercise behavior with multiple disciplines, including cases where identifiers such as match IDs repeat.
- Reuse pure domain decisions and shared HTTP view/input functions. The demo is not a second implementation: `demo.Event` should mirror the coordinator's event API and call the same domain logic wherever available. When adding a coordinator event route, add its equivalent to `demo.Event` too.
- Keep lock ownership and acquisition order consistent with nearby coordinator code. Do not hold an event registry lock while waiting for a discipline lock; check the local lock comments before changing cross-boundary calls.

## Domain Checks

- **Mats and plans:** the plan places work items from all disciplines on event mats. Build queues from the event-wide plan and all relevant snapshots, and identify a match by discipline plus match ID. Reuse the shared `Place`, `BuildMats`, and `Move` behavior; do not derive a mat queue from one discipline alone.
- **People:** a person is identified by the event registry and explicit links, not by a matching name. Reuse `internal/people` and the coordinator/demo view and decision functions; preserve merge, unmerge, and link semantics.
- **Forecast:** reuse `internal/forecast` and the shared forecast input/view/suggestion paths. A forecast warns about conflicts or gaps; it does not reject the plan.
- **Staff:** reuse `internal/staffing` and shared coordinator/demo functions. Staff are linked to event people and must not be assigned while fencing.
- **Signup:** keep the response format and all import rules in `internal/signup`; previews and imports must continue through the shared `signup.Check`. Update [docs/proposals/offline-signup.md](../../../docs/proposals/offline-signup.md) when changing the signup contract.

## Procedure

1. Trace the existing request from the coordinator route through its owner, persistence, view, stream/snapshot effects, and demo counterpart. Identify whether the change is event-owned, discipline-owned, or shared before editing.
2. Make the smallest change at the owning boundary. Prefer extending the existing pure domain function or shared view/input API over adding parallel server and demo logic.
3. Add focused tests for the coordinator and `internal/demo.Event`. Cover multi-discipline behavior and an important boundary case (for example, repeated IDs, an empty or failed discipline, or an invalid/stale plan) where relevant.
4. Update [docs/architecture.md](../../../docs/architecture.md) whenever the change affects event flow, ownership, synchronization, offline persistence, mats, forecasting, people, staffing, or tournament ranking pipelines.
5. If the change adds or alters a user-facing flow, also follow the [porta-ui skill](../porta-ui/SKILL.md) for UI consistency, localization, and browser validation.

## Verification

Run the Go suite from the repository root; it includes coordinator and portable demo tests:

```sh
go test ./...
```

For user-facing changes, run the focused web checks described by the `porta-ui` skill as well. Report any behavior that could not be exercised in both the server and demo paths.