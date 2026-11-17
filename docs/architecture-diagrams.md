# Architecture Diagrams Index

A quick guide to all Mermaid diagrams in [architecture.md](./architecture.md), organized by domain and use case.

---

## By Purpose

### Understanding the Core Design

**→ Start here if you're new to the codebase:**

1. **[Shared Pure Functions](#shared-pure-functions)** (§0)
   - Shows the core abstraction: pure functions in `internal/hall` reused by HTTP handlers and demo
   - Why: Establishes the principle that logic lives in functions, coordinated by I/O layers
   - Essential for: Understanding how to add new features without duplicating logic

2. **[Event-Coordinator-Discipline Component & Data Flow](#event-coordinator-discipline-architecture)** (§1a)
   - Shows how one event owns multiple discipline servers, and they coordinate
   - Why: Clarifies the boundary between event-level and discipline-level state
   - Essential for: Working on multi-discipline events, event-level features (plan, staff, people)

### Understanding Tournament Execution

3. **[Tournament Stages & State Transitions](#tournament-stages--state-transitions)** (§4)
   - Shows the entire tournament lifecycle: Register → Pools → Rank → Bracket → Results
   - Includes: Tie-break chain, withdrawal handling, elimination seeding
   - Why: Critical for understanding why match ids are positional (redraw retires old logs)
   - Essential for: Pool generation, ranking, bracket logic, match id semantics

### Understanding Real-Time Scoring & Offline-First

4. **[Match Engine State Machine](#match-engine-state-machine)** (§2)
   - Shows all match states and transitions, including pending decisions
   - Pending states: `PendingPointCap`, `PendingPenaltyCap`, `PendingFinalExchange`
   - Why: Clarifies when the UI must ask the head referee to decide
   - Essential for: Score keeper UI, match state handling, undo logic

5. **[IndexedDB + Server Sync](#indexeddb--server-sync)** (§3)
   - Shows offline-first flow: score keeper offline → reconnect → idempotent sync
   - Includes: Dual-engine bug detection, optimistic replay, network failure handling
   - Why: Foundation of the offline-first architecture
   - Essential for: Sync logic, network error handling, understanding IndexedDB keys

### Understanding Registration & People Management

6. **[Offline Signup Workflow](#offline-signup-workflow)** (§1d)
   - Shows participant offline phase → import preview → organizer confirmation → entry creation
   - Includes: Submission id idempotency, email-as-identity, deduplication
   - Why: Signup is completely offline but integrates with people registry
   - Essential for: Signup import, people deduplication, staff integration

---

## By Location in architecture.md

### **Shared Pure Functions** (§0)
- **Type:** Dependency map
- **What it shows:** `internal/hall` functions grouped by domain (snapshots, pools, ranking, eliminations, mats, people, forecast, staff), their inputs/outputs, and three callers (HTTP handlers, demo, web lib)
- **Key insight:** The same functions run in HTTP handlers (disk I/O on server) and demo (memory I/O in browser) — no double implementation
- **Use this to:** Understand where new logic should go (in `hall`, not in handlers); find which functions do what; trace calls across server/client boundary
- **Source:** `internal/hall/`, `web/src/lib/organizer.ts`, `internal/demo/`

### **System & Deployment Architecture** (§1)
- **Type:** Deployment diagram
- **What it shows:** One binary (porta) on organizer's PC serving embedded Svelte SPA to venue LAN clients
- **Key insight:** Everything is in one process; the Go binary includes the web app and JSON files
- **Use this to:** Understand the deployment model, single-binary design, offline LAN operation

### **Event-Coordinator-Discipline Architecture** (§1a)
- **Type:** Component diagram with data flow
- **What it shows:**
  - **Coordinator:** Owns `event.json`, `people.json`, `staff.json`, `displays.json`; routes `/api/event/*` and `/api/disciplines/*`
  - **Discipline Servers:** Mounted at `/api/d/{slug}/`; own matches, pools, rankings; have their own SSE hub
  - **Clients:** Organizer (reads/writes via REST), score keepers (POST events), displays (listen to SSE)
- **Key insight:** Event is authoritative; disciplines are independent but coordinate through the event
- **Use this to:** Understand authority boundaries; trace who writes what files; understand API routing; see when to push to event hub vs. discipline hub
- **Source:** `internal/http/coordinator.go`, `internal/http/server.go`, `internal/event/event.go`

### **Tournament Stages & State Transitions** (§4)
- **Type:** Flowchart with decision points
- **What it shows:**
  - Stages: Register → Draw Pools → Run Pools → Rank → Draw Bracket → Run Bracket → Results
  - Tie-break chain: MPI (Match Points/Matches) → VI (Wins/Matches) → SI (Points/Matches) → RI (Points Against/Matches, lower wins) → H2H (Head-to-Head) → Deterministic draw
  - Decisions: Competitor withdrawn (void all matches)? ElimMats count?
  - Notes: Match ids are positional; redraw retires old logs as `.bak`
- **Key insight:** Why match ids can't change between draws; why redraw is a restart
- **Use this to:** Understand when pools/bracket are drawn; why match logs are archived; trace the ranking pipeline; understand tie-break semantics
- **Source:** `internal/tournament/pools.go`, `internal/tournament/ranking.go`, `internal/tournament/bracket.go`

### **Match Engine State Machine** (§2)
- **Type:** State diagram
- **What it shows:**
  - **States:** `PendingMatch` → `InProgress` (Running/Paused) → `Completed`
  - **Pending decisions:** `PendingFinalExchange` (time expired, 10s left), `PendingPointCap` (score >= 8), `PendingPenaltyCap` (penalty level 3), `PendingNone` (normal play)
  - **Undo:** Revert to `InProgress` from any pending state
  - **End reasons:** Time, points, penalty, withdrawal, forfeit, disqualification
  - **Last/Undoable seq:** Tracks sequence numbers for idempotent POST and undo depth
- **Key insight:** Pending states are UI checkpoints; they block the match until a human decision is made
- **Use this to:** Understand when score keeper sees dialogs; why certain transitions are blocked; undo depth rules; why last/undoable seq matter
- **Source:** `internal/match/state.go`, `web/src/lib/match/`

### **IndexedDB + Server Sync** (§3)
- **Type:** Sequence diagram
- **What it shows:**
  - **Score keeper offline:** Record exchange → IndexedDB queue → optimistic replay on local engine
  - **Reconnect:** POST to `/api/d/{slug}/matches/{id}/events` with idempotent `(match_id, seq)`
  - **Server:** Validate via Go engine → append to disk → return 200 OK + server-derived state
  - **Client:** Compare server state with own replay (bug if different) → update IndexedDB
  - **Broadcast:** Server pushes SSE to displays → they re-render
  - **Network offline:** Keep in queue → retry on reconnect
- **Key insight:** Idempotent sync + dual-engine validation; optimistic replay lets score keeper work offline indefinitely
- **Use this to:** Understand offline-first design; trace sync failures; find where dual-engine bugs are caught; understand IndexedDB namespacing
- **Source:** `web/src/lib/sync.svelte.ts`, `web/src/lib/db.ts`, `internal/match/` (Go engine), `web/src/lib/match/` (TS engine)

### **Offline Signup Workflow** (§1d)
- **Type:** Sequence diagram + component diagram
- **What it shows:**
  - **Participant offline:** Fill `signup.html` or `signup.pdf` → save response file
  - **Transport:** USB stick or email (folder of `.json`/`.pdf` files)
  - **Organizer import:** Drag folder → preview (no changes) → review → confirm
  - **Validation:** `internal/signup.Check()` (pure function): format, fields, duplicates
  - **Dedup:** `people.For()` per entry: lookup person by submission id or email, or create new
  - **Import:** Re-check signature (prevent TOCTOU) → create `Competitor` and `Staff` entries
  - **Idempotency:** Submission id; importing same folder twice doesn't add duplicates
  - **Email identity:** Same email = same person; second response with same email is same person
  - **Staff integration:** Offers to work create `Staff` entries linked to person (prevents double-booking)
- **Key insight:** Completely offline for participants; purely functional validation; email is identity; submission id prevents duplicates
- **Use this to:** Understand signup flow; trace where duplicates are caught; why email matters; how staff signup integrates; signature checking (TOCTOU prevention)
- **Source:** `internal/signup/`, `web/signup/signup.html`, `web/src/lib/signup-pdf.ts`, `internal/people/`

---

## By Technology/Layer

### Backend (Go)
- [Shared Pure Functions](#shared-pure-functions) — `internal/hall`
- [Event-Coordinator-Discipline Architecture](#event-coordinator-discipline-architecture) — `internal/http`, `internal/event`
- [Tournament Stages & State Transitions](#tournament-stages--state-transitions) — `internal/tournament`
- [Match Engine State Machine](#match-engine-state-machine) — `internal/match` (Go engine)
- [Offline Signup Workflow](#offline-signup-workflow) — `internal/signup`, `internal/people`

### Frontend (TypeScript/Svelte)
- [Match Engine State Machine](#match-engine-state-machine) — `web/src/lib/match` (TS engine)
- [IndexedDB + Server Sync](#indexeddb--server-sync) — `web/src/lib/sync.svelte.ts`, `web/src/lib/db.ts`
- [Offline Signup Workflow](#offline-signup-workflow) — `web/signup/signup.html`, `web/src/lib/signup-pdf.ts`

### Shared/Cross-Layer
- [Shared Pure Functions](#shared-pure-functions) — reused by both backend and frontend
- [Event-Coordinator-Discipline Architecture](#event-coordinator-discipline-architecture) — API boundary between server and clients

---

## By Audience

### New Contributor
1. Start: [Shared Pure Functions](#shared-pure-functions)
   → Understand the organizing principle (logic is pure, coordinated by I/O)
2. Next: [System & Deployment Architecture](#system-and-deployment-architecture--11)
   → Understand what runs where
3. Then: [Event-Coordinator-Discipline Architecture](#event-coordinator-discipline-architecture)
   → Understand the API and state boundaries

### Score Keeper Features
- [Match Engine State Machine](#match-engine-state-machine)
- [IndexedDB + Server Sync](#indexeddb--server-sync)

### Organizer (Event Setup) Features
- [Event-Coordinator-Discipline Architecture](#event-coordinator-discipline-architecture)
- [Offline Signup Workflow](#offline-signup-workflow)
- [Tournament Stages & State Transitions](#tournament-stages--state-transitions)

### Display/Scoreboard Features
- [Event-Coordinator-Discipline Architecture](#event-coordinator-discipline-architecture) — which SSE streams to listen to
- [Match Engine State Machine](#match-engine-state-machine) — what state to render
- [Tournament Stages & State Transitions](#tournament-stages--state-transitions) — which discipline is in which stage

### Debuggers / Maintainers
- [IndexedDB + Server Sync](#indexeddb--server-sync) — trace sync failures
- [Match Engine State Machine](#match-engine-state-machine) — trace scoring bugs
- [Offline Signup Workflow](#offline-signup-workflow) — duplicate/person linking issues

---

## Cross-References

| If you're working on... | See these diagrams |
|---|---|
| **Adding a new tournament feature** | Shared Pure Functions, Tournament Stages, Match Engine State Machine |
| **Fixing offline sync bugs** | IndexedDB + Server Sync, Match Engine State Machine |
| **Multi-discipline coordination** | Event-Coordinator-Discipline, Shared Pure Functions |
| **Display rendering logic** | Match Engine State Machine, Event-Coordinator-Discipline (SSE routing) |
| **Signup deduplication** | Offline Signup Workflow, Shared Pure Functions (people.For) |
| **Score keeper UI** | Match Engine State Machine, IndexedDB + Server Sync |
| **Organizer workflows** | Event-Coordinator-Discipline, Tournament Stages, Offline Signup |
| **Demo (WASM) implementation** | Shared Pure Functions, Event-Coordinator-Discipline, Match Engine State Machine |

---

## Notes on the Mermaid Diagrams

- **Source:** All diagrams live in [docs/architecture.md](./architecture.md)
- **Language:** Mermaid 10+ (supports most flowchart, state, sequence, graph variants)
- **Naming:** Matches Go and TypeScript source code exactly (CamelCase, full names)
- **Versioning:** Update diagrams when the code changes (especially state machines, tie-break rules, API routes)
- **Testing:** Some diagrams (state machine, tie-break chain) should be verified against the actual code and test vectors

---

## Related Documentation

- [repo_structure.md](./repo_structure.md) — Where code lives and why
- [design.md](./design.md) — Product decisions and terminology
- [tech-stack.md](./tech-stack.md) — Engineering decisions (Go, Svelte, etc.)
- [demo.md](./demo.md) — How the WASM demo reuses production code
- [proposals/](./proposals/) — Early-stage ideas and decisions under discussion

