# Suggested Mermaid Diagrams for Porta di Ferro

This document identifies architectural parts of the codebase that would benefit from Mermaid visualizations. These diagrams would help onboarders, reviewers, and maintainers understand key flows and system components.

---

## 1. Match State Machine (High Priority)

**Location:** `internal/match/state.go`  
**Current state:** No diagram  
**Why it's needed:** The match engine has complex state transitions (Pending → Running → Ended, with undo, penalties, final exchange decision). A state machine diagram would clarify when each `Pending` type is raised and how it's resolved.

**What to show:**
```
Created
  ├─ Not Started
  │   ├─ Start → Running
  │   └─ Forfeit → Ended (0–8, NoMatchPoints)
  ├─ Running
  │   ├─ Timer expires (10s left) → Flashing (visual only)
  │   ├─ Exchange → Score/Penalty update
  │   ├─ Pending decision:
  │   │   ├─ PendingFinalExchange → End Match or Continue
  │   │   ├─ PendingPointCap → End Match or Undo
  │   │   └─ PendingPenaltyCap → End Match or Undo
  │   └─ All decisions made → Ended
  └─ Ended
      └─ (Undo last exchange → Running again)
```

**Key states to visualize:**
- `Ended` (true/false)
- `Running` (true/false)
- `Pending` (none, final_exchange, point_cap, penalty_cap)
- `Winner` (red, blue, draw)
- `Reason` (time, points, penalty, withdrawal, forfeit, disqualification, undo)
- `NoMatchPoints` (penalty loss vs normal loss)

---

## 2. Tournament Stage Pipeline (High Priority)

**Location:** `internal/tournament/` (pools.go, bracket.go, ranking.go)  
**Current state:** Partial diagram in architecture.md (mats phase 2)  
**Why it's needed:** A discipline moves through distinct stages — pools → eliminations → results. Each stage has work items, state, and transitions. The diagram should show:

**What to show:**
```
1. Pools stage
   ├─ Generate pools from competitors
   ├─ Run matches on mats (ItemsOf → work items)
   ├─ Rank after pools (Standing)
   └─ Decide elimination seeding

2. Eliminations stage
   ├─ Draw bracket (Podium computed from bracket)
   ├─ Run bracket matches (lanes, bronze match)
   └─ Results (Final standings)

3. Results stage
   └─ View final rankings
```

**Transitions:** Data flow from pools → ranking → bracket seeding → bracket results.

---

## 3. IndexedDB + Server Sync (High Priority)

**Location:** `web/src/lib/sync.svelte.ts`, `web/src/lib/db.ts`  
**Current state:** No diagram  
**Why it's needed:** The offline-first model is subtle: score keeper can work offline (IndexedDB), idempotent sync (POST with match_id + seq), server validation, reconciliation on reconnect. A sequence diagram would clarify the happy path and error cases.

**What to show:**
```
Score keeper offline:
  Local action → IndexedDB write → SSE broadcast (local only)

Reconnect:
  1. Try POST /api/d/{slug}/matches/{id}/events
  2. Server replay engine validates
  3. Server rejects duplicate seq
  4. Server broadcasts on SSE
  5. Client pulls /api/d/{slug}/matches/{id}/events (replay log)
  6. Client updates IndexedDB from server log

Key invariants:
  - Idempotency key: (match_id, sequence_number)
  - IndexedDB key: [discipline, match, seq]
  - SSE broadcast after write (even if rejected)
```

**Diagram type:** Sequence diagram (offline → online → synced states).

---

## 4. Event-Coordinator-Discipline Architecture (Medium Priority)

**Location:** `internal/http/`, `internal/event/`, `internal/demo/`  
**Current state:** Partial diagram in architecture.md (§1a)  
**Why it's needed:** The relationship between `Coordinator`, discipline `Server`s, `event.json`, and `demo.Event` needs clarity. Where does logic live? What writes the event? What is shared?

**What to show:**
```
httpapi.Coordinator (owns event.json)
├─ Routes: /api/event/*, /api/disciplines/*
├─ Event hub: /api/event/stream
├─ Reads: event.json (day, disciplines list, plan, staff)
└─ Writes: event.json (through internal/event)

httpapi.Server[0..N] (one per discipline)
├─ Routes: /api/d/{slug}/*
├─ Store: matches.json, pools.json, rankings.json
├─ SSE hub: discipline-level updates
└─ Reads: event.json (day data only, not state)

demo.Event (memory-only coordinator)
├─ Reuses: internal/tournament, internal/match, internal/hall
├─ No disk I/O
└─ localStorage for persistence

Key rule: Only Coordinator writes event.json.
Disciplines call Coordinator API to change day (plan, staff, etc).
```

**Diagram type:** Component/module diagram.

---

## 5. Hall Pure Functions (Medium Priority)

**Location:** `internal/hall/` (and `web/src/lib/match/`, `web/src/lib/organizer.ts`)  
**Current state:** No diagram  
**Why it's needed:** `hall` is the shared logic layer, but it's not obvious what functions exist or how they're used. A function/dependency map would clarify what's pure, what's side-effect-free, and how it's called.

**What to show:**
```
Pure functions in internal/hall:
├─ Snapshots
│  ├─ BuildSnapshot(source) → Snapshot
│  └─ Summarize(snapshots[]) → EventSummary
├─ Pools & Ranking
│  ├─ ViewPools() → []Pool
│  └─ ViewRanking() → []Standing
├─ Eliminations
│  ├─ ViewBracket() → Bracket
│  └─ Podium(bracket) → []Winner
├─ Mats (event-level)
│  ├─ BuildMats(plan, discipline snapshots) → []Mat
│  ├─ Place(item, mat) → updated Plan
│  └─ Move(item, mat, place) → updated Plan
├─ People
│  ├─ ViewPeople() → []Person
│  ├─ Duplicates() → [][]Person
│  └─ DecidePerson() → updated people.json
├─ Forecast
│  ├─ ForecastInput(plan, snapshots) → Input
│  └─ Run(input) → Timeline
└─ Staff
   ├─ ViewStaffing() → Assignments
   └─ SuggestStaff() → Suggestions

Callers:
├─ http handlers (read + call + write)
├─ demo.Event (call + memory)
└─ web lib (call via API)
```

**Diagram type:** Dependency/function map.

---

## 6. Offline Signup Workflow (Medium Priority)

**Location:** `internal/signup/`, `web/signup/signup.html`, `/api/d/{slug}/signup/*`  
**Current state:** No diagram  
**Why it's needed:** Signup is offline-first (HTML file on USB), but it's also integrated with the event's people registry. The flow from file → import → desk entry → person is complex.

**What to show:**
```
1. Offline phase
   ├─ web/signup/signup.html (self-contained, no server)
   ├─ Participant fills form → response file
   └─ File saved locally (CSV or JSON)

2. Import phase
   ├─ Organizer drags response folder to /api/d/{slug}/signup/import
   ├─ Server: internal/signup.Check (validation)
   ├─ Server: people.For (lookup person)
   └─ Update people.json + competitor entries

3. Desk phase
   ├─ Score keeper sees suggestion from people.For
   ├─ Accepts suggestion or picks different person
   └─ Competitor.Person = picked person ID

Idempotency: same submission id = same person
Merge: duplicates resolved in /admin · People
Aliases: discipline identifier change doesn't break old links
```

**Diagram type:** Flow / sequence diagram.

---

## 7. SSE Event Stream & Presence (Low-Medium Priority)

**Location:** `internal/store/presence.go`, `internal/http/hub.go`  
**Current state:** No diagram  
**Why it's needed:** The SSE stream carries multiple types of events (match updates, presence, discipline state changes). The presence model (heartbeat, quarantine) is not obvious.

**What to show:**
```
SSE Hub types:
├─ Match events (posted by score keeper)
│  ├─ Payload: match ID, event log entry
│  └─ Broadcast to: all clients in discipline
├─ Presence events
│  ├─ Payload: device ID, heartbeat
│  └─ Broadcast to: event-level
├─ Discipline state (summarized)
│  ├─ Payload: stage, progress, mats, podium
│  └─ Broadcast to: event-level
└─ Coalescing (150ms window)

Presence lifecycle:
├─ Heartbeat IN → Live
├─ No heartbeat (timeout) → Quarantined
├─ Heartbeat after quarantine → Live again
├─ No heartbeat (long timeout) → Gone
```

**Diagram type:** Event stream diagram.

---

## 8. Web Client State Management (Low Priority)

**Location:** `web/src/lib/event.svelte.ts`, `web/src/lib/scorekeeper.svelte.ts`, `web/src/lib/organizer.ts`  
**Current state:** No diagram  
**Why it's needed:** Multiple reactive stores drive the UI (event, scorekeeper state, organizer view). Their interactions and lifetimes are not clear.

**What to show:**
```
Central stores:
├─ event: Snapshot + metadata (reactive)
├─ scorekeeper: CurrentMatch + pending decision + undo state
├─ organizer: Pools, staff, plan (read-only)
└─ presence: Who's connected

Subscriptions:
├─ event updates → SSE
├─ scorekeeper updates → IndexedDB + POST
├─ organizer updates → REST API calls
└─ presence updates → SSE

Component tiers:
├─ Pages (routes)
├─ Containers (fetch + state)
└─ Presenters (props only)
```

**Diagram type:** State & subscription diagram.

---

## 9. File Structure & Versioning (Low Priority)

**Location:** `internal/store/`, `internal/event/`  
**Current state:** Mentioned in repo_structure.md but no diagram  
**Why it's needed:** The disk layout (event.json, disciplines/, matches.json, pools.json, deleted/) and file format evolution is important for migrations and backups.

**What to show:**
```
Event folder:
├─ event.json (day: welcome, wifi, disciplines list, plan, staff)
├─ people.json (registry of humans, one entry per person)
├─ signed-up.json (signup responses, metadata)
├─ disciplines/
│  ├─ open-steel-longsword/
│  │  ├─ summary.json (last known state: stage, progress)
│  │  ├─ matches.json (match metadata)
│  │  ├─ pools.json (pool list + entrants)
│  │  ├─ rankings.json (standings after pools)
│  │  └─ <match-id>/
│  │     └─ events (event log, immutable append)
│  └─ open-sabre/
│     └─ ...
└─ deleted/
   └─ <timestamp>-<name>/
      └─ (archived event)

Versioning: event.json has Version field for migrations
```

**Diagram type:** File tree / folder structure.

---

## 10. Forecast Calculation (Low Priority)

**Location:** `internal/forecast/forecast.go`, `docs/proposals/`  
**Current state:** Mentioned in architecture.md (phase 4) but not visualized  
**Why it's needed:** The forecast blends templates, observed pace, and the plan to predict when matches will run. The logic is complex and would benefit from a step-by-step flowchart.

**What to show:**
```
Inputs:
├─ Plan (mat count, item order, timings, not-before)
├─ Observed pace (each mat's running time)
├─ Template timings (pool size/mat → duration)
└─ Actual match logs (StartedAt, EndedAt)

Steps:
1. Calibrate template from observed matches
2. For each mat, blend template + recent pace
3. Walk the plan:
   ├─ Assign item to mat/place
   ├─ Compute start time (depends on feeders)
   └─ Predict end time (using calibrated duration)
4. Output: Timeline with predicted start/end for each match

Warnings: Item going behind plan, mat idle too long
```

**Diagram type:** Flowchart / algorithm diagram.

---

## 11. Transaction Semantics & Conflict Resolution (Low Priority)

**Location:** `internal/store/store.go` (Store lock), `internal/match/` (replay engine)  
**Current state:** No diagram  
**Why it's needed:** The lock strategy (Store-level coarse lock) and event log replay (idempotent, sequence-numbered) prevent conflicts. A diagram showing the invariants would help avoid race conditions.

**What to show:**
```
Write path:
1. POST /api/d/{slug}/matches/{id}/events
2. Server.Store.Lock()
3. Read match log (get LastSeq)
4. Validate new event (seq = LastSeq + 1)
5. Append event to log
6. Replay from scratch → State
7. Store.Unlock()
8. Broadcast SSE
9. Return State + 200

Read path:
1. GET /api/d/{slug}/matches/{id}
2. Read match log (no lock)
3. Replay from scratch → State
4. Return State

Invariants:
├─ No duplicate seq (rejected with 409)
├─ Idempotent (same req = same response)
├─ No lost updates (lock before read-modify-write)
└─ Consistent read (always replay fresh)
```

**Diagram type:** Sequence / concurrency diagram.

---

## Priority Summary

| Diagram | Priority | Effort | Impact |
|---------|----------|--------|--------|
| Match State Machine | High | Low | High |
| Tournament Stage Pipeline | High | Medium | High |
| IndexedDB + Server Sync | High | Medium | High |
| Event-Coordinator-Discipline | Medium | Medium | Medium |
| Hall Pure Functions | Medium | Low | Medium |
| Offline Signup Workflow | Medium | Medium | Medium |
| SSE Event Stream & Presence | Low-Medium | Low | Low |
| Web Client State Management | Low | Medium | Low |
| File Structure & Versioning | Low | Low | Low |
| Forecast Calculation | Low | Medium | Low |
| Transaction Semantics | Low | Low | Medium |

---

## Implementation Notes

- **Language:** Use Mermaid markdown code blocks, embedded in architecture.md or in this file (repo_structure.md).
- **Naming:** Keep type/function names matching the Go/TypeScript source (CamelCase, exact).
- **Cross-links:** From architecture.md diagrams, link to source files in comments or captions.
- **Versioning:** Update diagrams when the code changes (especially state machines, table schemas).
- **Testing:** Diagrams are documentation; they should match the actual code behavior. Run against tests when in doubt.

---

