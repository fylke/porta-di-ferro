# Porta di Ferro Repository Structure

This is a **HEMA tournament application** — a system for managing fencing competitions. The repo is organized around a critical principle: **one Go binary serving a Svelte 5 SPA, with shared logic between backend and frontend**.

## High-Level Architecture

```
┌─────────────────────────────────────────┐
│   Organizer's PC (Windows .exe)         │
│  ┌────────────────────────────────────┐ │
│  │  porta (Go binary + embedded web)  │ │
│  └────────────────────────────────────┘ │
└─────────────────────────────────────────┘
              ↓
   Serves to venue LAN clients
   (score keepers, displays, organizer)
```

---

## Root-Level Organization

| Folder | Purpose |
|--------|---------|
| **`cmd/`** | Go binaries to build: `porta` (main app) and `demo-wasm` (browser demo) |
| **`internal/`** | Shared Go logic (backend only, not API) |
| **`web/`** | Svelte 5 SPA — the client UI for all roles |
| **`e2e/`** | End-to-end tests (Playwright, runs against running server) |
| **`testdata/`** | Shared test vectors (JSON) verified by both Go & TypeScript engines |
| **`docs/`** | Architecture diagrams, design decisions, tech stack, proposals |
| **`installer/`** | Inno Setup configuration for Windows .exe packaging |
| **`.github/`** | CI workflows (tests, e2e, nightly) |

---

## `internal/` — Go Backend Logic

The backend is **not layered vertically by feature** (api, models, services). Instead, it's organized by **architectural concern**:

| Package | Responsibility |
|---------|-----------------|
| **`http/`** | HTTP server + routing. Mounts disciplines, owns the event-level coordinator |
| **`hall/`** | **Pure functions** — the event and tournament logic every part shares (is `Coordinator` calls it too) |
| **`event/`** | Reads/writes `event.json`, manages the day's metadata |
| **`store/`** | Disk I/O for JSON files (matches, pools, rankings, plans, staff) |
| **`demo/`** | **In-memory event simulator** — reuses `internal/tournament`, `internal/match`, `internal/hall` exactly as the HTTP server does |
| **`match/`** | Go match engine: scoring, state transitions, replay from event log |
| **`tournament/`** | Pool generation, ranking, elimination bracket logic |
| **`people/`** | Competitor registration, withdrawal, signup import |
| **`forecast/`** | Timeline prediction for the day's schedule |
| **`staffing/`** | Duty assignment for volunteers |
| **`signup/`** | Offline signup file formats and validation |
| **`lan/`** | LAN device discovery (mDNS) |
| **`qr/`** | QR code generation |
| **`watchdog/`** | Restart loop on Windows — restarts porta if it crashes |

**Key architectural rule:** Code that decides logic belongs in `internal/hall`, called by both `http` handlers and `demo` state machine. A handler is for **reading the disk and serving**, not for **deciding**.

---

## `web/` — Svelte 5 Client (SPA)

The embedded frontend, which runs on:
- Organizer's browser (event setup, results, management)
- Score keeper's tablet (match scoring at the mat)
- Displays (scoreboards, rosters, results)
- Participants (signing up, viewing results)

### `web/src/`

| File/Folder | Purpose |
|-------------|---------|
| **`main.ts`** | App entry point |
| **`App.svelte`** | Root component |
| **`router.svelte.ts`** | Client-side routing (SvelteKit-less) |
| **`api.ts`** | HTTP client for the backend API |
| **`api-types.ts`** | TypeScript types matching Go struct JSON |
| **`app.css`** | Global styles |

### `web/src/lib/` — Reusable Logic

| File/Folder | Purpose |
|-------------|---------|
| **`match/`** | TypeScript match engine (mirrors Go `internal/match/`), test vectors |
| **`i18n.svelte.ts`** | Internationalization (English → Swedish via `t()`) |
| **`db.ts`** | IndexedDB client storage (match logs, scores, state) |
| **`paths.ts`** | LocalStorage/IndexedDB key namespacing per discipline |
| **`sync.svelte.ts`** | Bidirectional sync: local ↔ server (idempotent POST) |
| **`event.svelte.ts`** | Reactive store for the current event |
| **`lan.svelte.ts`** | LAN URL detection |
| **`presence.svelte.ts`** | Who's connected (WebSocket presence) |
| **`scorekeeper.svelte.ts`** | Score keeper state machine |
| **`organizer.ts`** | Organizer's view logic (pools, staff, planning) |
| **`stage.ts`** | Tournament stage transitions |
| **`outcome.ts`** | Result interpretation (win/loss/draw) |
| **`rank.ts`** | Pool ranking (mirror of Go logic) |
| **`board.ts`** | Scoreboard rendering (which match, which mat, points) |
| **`qr.ts`** | QR code for venue WiFi/LAN URL |
| **`sv.ts`** | Swedish translations (must keep in sync with `i18n.svelte.ts` keys) |

### `web/src/routes/` — Pages

| Component | URL | Role |
|-----------|-----|------|
| **`EventLanding.svelte`** | `/` | Shows event status, discipline list, QR code |
| **`Welcome.svelte`** | `/welcome` | First-run setup |
| **`EventAdmin.svelte`** | `/admin` | Organizer's main dashboard |
| **`PlanEvent.svelte`** | `/admin/plan` | Set up disciplines, timeline, mats |
| **`DisciplineSetup.svelte`** | `/d/{slug}/setup` | Per-discipline configuration |
| **`Pools.svelte`** | `/d/{slug}/pools` | View & manage pools |
| **`ScoreKeeper.svelte`** | `/score/:mat` | At-the-mat score entry |
| **`DisplayMat.svelte`** | `/display/mat/:n` | Single-mat scoreboard |
| **`DisplayRoster.svelte`** | `/display/roster` | List all competitors |
| **`Signup.svelte`** | `/signup` | Participant registration UI |

---

## Critical Architectural Principles

### 1. One Event, Many Disciplines (#102)

- **One `porta` process** = one event (all disciplines, one day)
- Each discipline gets its own `httpapi.Server`, mounted at `/api/d/{slug}/`
- Event-level data (`event.json`) is shared; disciplines can't write it except through the API
- **Clients namespace everything per discipline** (`paths.ts`): a device keeps separate match logs for each

```
/api/event/*           ← event-level (organizer)
/api/disciplines/*     ← list disciplines
/api/d/open-steel/api/matches/*   ← discipline 1 matches
/api/d/open-sabre/api/matches/*    ← discipline 2 matches
```

### 2. Pure Functions in `internal/hall`

- Core logic (pools, ranking, timeline, staff allocation) is **pure functions in `hall`**
- Called by **both** HTTP handlers (`http/`) and **the demo** (`demo/`)
- Handlers read disk and serve; they don't decide
- Example: `hall.ViewPeople()` + `hall.DecidePerson()` are shared, `demo.Event` calls them too

### 3. Dual Match Engines

- Match scoring logic **written twice**: Go (`internal/match/`) + TypeScript (`web/src/lib/match/`)
- Both verified against **shared JSON test vectors** in `testdata/vectors/`
- Why? Scoring must be identical on server and score-keeper tablet
- CI runs the same vectors through both engines

### 4. Event Log = State

- All state is derived by **replaying** the match event log
- Nothing is mutated in place
- Scores, decisions, and results are immutable events
- Undo works by rewriting the log file

### 5. Local-First Offline

- Score keeper works offline: scores saved to IndexedDB, synced when reconnected
- SSE (Server-Sent Events) is **one-directional**: clients push with idempotent POST, pull with SSE
- Idempotency key: `(match_id, sequence_number)` — safe to retry

---

## Data Flow

```
Event organizer (browser)
├─ Calls: internal/hall functions via HTTP
├─ Examples: EditPlan(), AddCompetitor(), MoveDiscipline()
├─ Reads: event.json, plan, staff (disk via HTTP)
└─ Sees: SSE event stream (/api/event/stream)

Score keeper (tablet at mat)
├─ Offline-first: saves to IndexedDB
├─ Calls: idempotent POST /api/d/{slug}/matches/{id}/events
├─ Uses: TypeScript match engine (web/src/lib/match)
└─ Sees: SSE match events

Displays (read-only)
├─ No input, only SSE streams
├─ Parse scoreboard data from hall.Summarize()
└─ Show current match, times, podium

Backend (porta)
├─ Disk: JSON files (event.json, matches, pools, staff)
├─ Memory: Store lock, SSE hub, presence
├─ Uses: Go match engine for validation/replaying
└─ Coordinator: owns event.json + mounts discipline servers
```

---

## Testing Strategy

| Type | Location | Run | Scope |
|------|----------|-----|-------|
| **Unit** | `*_test.go` (Go), `*.test.ts` (TS) | `go test ./...` or `npm test` in `web/` | Individual functions |
| **Shared vectors** | `testdata/vectors/` | Both engines process same JSON | Match scoring correctness |
| **E2E** | `e2e/` (Playwright) | `npm run test:e2e` in `web/` | Full workflow (browser → server) |
| **Nightly** | `.github/workflows/nightly-e2e.yml` | Daily 02:00 UTC | Full suite on schedule |

---

## Key Files to Know

| File | Purpose |
|------|---------|
| [docs/architecture.md](./architecture.md) | Mermaid diagrams of system, state machines, event flow |
| [docs/design.md](./design.md) | Product decisions (what it does, terminology) |
| [docs/tech-stack.md](./tech-stack.md) | Engineering decisions (why Go, why embedded, why dual engines) |
| `internal/event/event.go` | Event JSON structure (day metadata, discipline list) |
| `internal/store/` | JSON file reading/writing (matches, pools, rankings) |
| `internal/tournament/` | Pool generation, ranking logic |
| `internal/hall/` | Pure functions shared with demo |
| `web/src/lib/match/` | TypeScript match engine |
| `web/src/lib/paths.ts` | Key namespacing per discipline |
| `web/src/lib/i18n.svelte.ts` | Translation keys (English) |
| `web/src/lib/sv.ts` | Swedish translations (must stay in sync) |

---

## Why It's Organized This Way

| Decision | Benefit |
|----------|---------|
| **Go backend** | Single static binary, no runtime, offline-ready |
| **Embedded Svelte SPA** | No separate deployment; binary *is* the app |
| **Pure functions in `hall`** | Reused by both HTTP handlers and browser demo (no double implementation) |
| **Dual match engines** | Identical scoring on server and tablet; verified by shared vectors |
| **IndexedDB + SSE** | Works offline, syncs when reconnected |
| **Event log** | Immutable audit trail; undo = rewrite log |
| **One event, many disciplines** | Club events often mix weapons; added in phase 1 |
| **i18n from the start** | Swedish translations mandatory at every step (not retrofit) |

---

The structure prioritizes **reusability** (shared `hall` functions), **offline resilience** (IndexedDB + idempotent sync), and **correctness** (dual engines with vector tests) over strict separation of concerns. Everything is designed to run on a single USB drive at a club venue with no internet.
