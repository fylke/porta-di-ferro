# Architecture & System Design Visualizations

This document provides architectural diagrams for Porta di Ferro, illustrating component boundaries, state lifecycles, event synchronization, tournament ranking pipelines, and offline-first capabilities.

---

## 1. System & Deployment Architecture

Porta di Ferro runs as a single Go binary on the organizer's PC, embedding the Svelte 5 SPA via `//go:embed`. Devices connect locally across the venue LAN without requiring an external internet connection.

```mermaid
flowchart TB
    subgraph Host["Organizer PC (Go Server Process)"]
        Binary["porta (Go Binary)"]
        StaticFS["Embedded Assets (Svelte 5 SPA)"]
        Store["Disk Storage (JSON Files)"]
        EngineGo["Go Match & Tournament Engine"]
        HTTPRouter["net/http Server + SSE Hub"]

        Binary --> StaticFS
        Binary --> EngineGo
        Binary --> HTTPRouter
        EngineGo <--> Store
    end

    subgraph Clients["Venue LAN Clients (Browsers)"]
        OrgUI["Organizer Web Client\n(/)"]
        ScoreUI["Score Keeper Client\n(/score/:mat)"]
        DisplaySingle["Mat / Audience Display\n(/display/mat/:n, /display/audience/:n)"]
        DisplayMulti["Multi-Mat / Roster / Assigned\n(/display/mats, /display/roster, /display)"]
    end

    subgraph CloudTarget["Milestone 3 (Optional)"]
        Mirror["Cloud Mirror (porta-mirror)"]
    end

    HTTPRouter -- "Static Assets (HTTP GET)" --> Clients
    ScoreUI -- "Idempotent POST /api/matches/:id/events" --> HTTPRouter
    OrgUI -- "REST API (Competitors, Pools, Setup)" --> HTTPRouter
    HTTPRouter -- "SSE Event Stream (/api/events)" --> ScoreUI
    HTTPRouter -- "SSE Event Stream (/api/events)" --> OrgUI
    HTTPRouter -- "SSE Event Stream (/api/events)" --> DisplaySingle
    HTTPRouter -- "SSE Event Stream (/api/events)" --> DisplayMulti
    Store -. "One-way Replication Stream" .-> Mirror
```

---

## 2. Match Engine State Machine & Scoring Logic

Matches are driven by an append-only event log. State is pure and recomputed by replaying events. Event types are `exchange`, `timer` (start / stop / resume / reset), `undo`, `end`, and `options` — the last carries the competitors' colours and the display side order, is ignored by replay, and is read separately by `OptionsOf` / `optionsOf` so presentation can never fail a scoring vector. Points are differential (e.g., scoring $2$ vs $1$ awards $1$ net point to the higher scorer), capped at $8$ points or $3$ minutes ($180\,000\text{ ms}$).

```mermaid
stateDiagram-v2
    [*] --> PendingMatch: Match Created

    PendingMatch --> InProgress_Paused: First Interaction / Timer Start
    InProgress_Paused --> InProgress_Running: Timer Start / Resume
    InProgress_Running --> InProgress_Paused: Timer Stop / Pause
    InProgress_Running --> InProgress_Paused: Timer Reset (clock to 00:00, scores kept)
    InProgress_Paused --> InProgress_Paused: Timer Reset (clock to 00:00, scores kept)

    state InProgress_Running {
        [*] --> ScoreCheck
        ScoreCheck --> ExchangeConfirmed: Record Exchange (Differential 1-2 pts)
        ExchangeConfirmed --> WarningIssued: Penalty +1 (Warning) / +2 (Double, -1 pt) / +3 (Triple)
        WarningIssued --> ScoreCheck
        ExchangeConfirmed --> ScoreCheck
    }

    InProgress_Running --> PendingPointCap: Score >= 8 (Point Cap)
    InProgress_Running --> PendingPenaltyCap: Penalty Level 3 Reached
    InProgress_Running --> PendingFinalExchange: Elapsed Time >= 180s on Exchange

    PendingPointCap --> Completed: Confirm End Match
    PendingPointCap --> InProgress_Paused: Undo Last Exchange

    PendingPenaltyCap --> Completed: Confirm Match Loss (0-8, 0 pts)
    PendingPenaltyCap --> InProgress_Paused: Undo Last Penalty

    PendingFinalExchange --> Completed: Confirm End Match
    PendingFinalExchange --> InProgress_Paused: Continue One More Exchange

    InProgress_Paused --> Completed: Manual End / Forfeit / Disqualification
    InProgress_Running --> Completed: Manual End / Forfeit / Disqualification

    Completed --> [*]
```

---

## 3. Data Flow, Sync & Event Logging

The score keeper client applies updates optimistically to its local TypeScript engine while queueing the event to IndexedDB. The event is posted with `(match_id, sequence)` for server validation, JSON disk persistence, and SSE broadcast.

```mermaid
sequenceDiagram
    autonumber
    actor SK as Score Keeper (Browser)
    participant LocalEngine as Local TS Engine / IndexedDB
    participant Server as Go HTTP Server & Store
    participant Displays as Mat Displays & Organizer UI

    SK->>LocalEngine: Record Exchange / Timer Toggle
    LocalEngine->>LocalEngine: Update Optimistic State
    LocalEngine->>LocalEngine: Persist to Queue (IndexedDB)
    LocalEngine->>Server: POST /api/matches/:id/events (seq, type, payload)

    alt Server accepts event
        Server->>Server: Replay & Validate via Go Engine
        Server->>Server: Append to Match JSON on Disk
        Server-->>LocalEngine: 200 OK (written, server-derived state, lastSeq)
        LocalEngine->>LocalEngine: Compare server state with own replay of the same log
        Note over LocalEngine: A difference is a dual-engine bug: banner + console report.<br/>Score keeper may adopt the server's state for the rest of the match.
        Server-)Displays: Broadcast SSE (match_updated)
        Displays->>Displays: Re-render Scoreboard / Roster
    else Network offline or delayed
        Server--xLocalEngine: Network Failure / Timeout
        Note over LocalEngine: Keep in IndexedDB queue
        LocalEngine->>Server: Retry on reconnect / next action
    end
```

---

## 3b. Connected Clients, Handover and Server-Assigned Displays

Every score keeper client and every `/display` screen announces itself with a stable id and heartbeats every five seconds (`POST /api/clients/{id}`); the registry lives in memory and is pushed to the organizer over SSE as `presence`. A score keeper claims its match before writing (`POST /api/matches/{id}/claim`) and stamps every push with the granted epoch; the claims live in `writers.json` so they survive a restart. The mat's own idea of which match is up follows the live score keeper on it, so displays show a finished match's result for exactly as long as the score keeper holds it.

```mermaid
sequenceDiagram
    autonumber
    participant A as Device A (score keeper)
    participant S as Go Server
    participant B as Device B (score keeper)
    participant O as Organizer UI

    A->>S: POST /api/clients/A (heartbeat: mat 1, match M)
    A->>S: POST /api/matches/M/claim {client A}
    S-->>A: {epoch 1}
    A->>S: POST /api/matches/M/events (X-Porta-Epoch: 1)
    S-->>A: 200

    Note over A: A dies, or B wants the mat while A is alive
    B->>S: POST /api/matches/M/claim {client B}
    alt A alive
        S-->>B: 409 {holder: A}
        B->>S: POST claim {client B, force: true}
    else A silent for 15 s, or A released
        Note over S: no contest
    end
    S-->>B: {epoch 2, tookOverFrom A}

    A->>S: POST events (X-Porta-Epoch: 1)
    S->>S: quarantine to matches/M.quarantine.ndjson
    S-->>A: 409 {quarantined: n}
    S-)O: SSE presence (set aside: n events from A on M)
    O->>S: DELETE /api/quarantine/M (after looking)
```

Displays: a screen opens `/display`, heartbeats with role `display`, and renders whatever `target` comes back (`mat/1`, `audience/2`, `mats`, `mats/1,2`, `roster`). The organizer sets it with `PUT /api/clients/{id}/target`; assignments are kept in `displays.json`.

---

## 4. Tournament Lifecycle & Ranking Pipeline

Tournaments progress through competitor intake, pool generation, schedule assignment across mats, match execution, and multi-tier index ranking.

```mermaid
flowchart TD
    A[Add / Register Competitors] --> B[Generate Pools: sizes within one, clubs spread by a greedy deal plus a swap pass, remainder reported]
    B --> C[Generate Schedule & Assign Mats]
    C --> C2{Organizer override?}
    C2 -- Move / reorder pool --> C3[Pool.Sequence updated; snapshot serves pools in run order]
    C2 -- No --> D
    C3 --> D[Execute Matches at Mats]
    D --> E{Competitor Withdrawn?}

    E -- Yes --> F[Void All Matches for Withdrawn Competitor]
    E -- No --> G[Keep Match Results]
    F --> H[Replay Match Log for Active Competitors]
    G --> H

    H --> I[Calculate Standing Metrics per Pool]

    subgraph RankingChain["Tie-break Chain (Order of Application)"]
        I --> J1["1. Match Point Index (MPI = Match Points / Matches)"]
        J1 --> J2["2. Victory Index (VI = Wins / Matches)"]
        J2 --> J3["3. Score Index (SI = Points Scored / Matches)"]
        J3 --> J4["4. Reception Index (RI = Points Conceded / Matches, lowest wins)"]
        J4 --> J5["5. Head-to-Head Comparison"]
        J5 --> J6["6. Deterministic Random Draw (FNV Seed)"]
    end

    J6 --> K[Final Pool Standings & Promotion]
```

---

## 5. Offline & Local-First Resilience

Scorekeeper devices stay operational even during transient venue Wi-Fi dropouts by leveraging a Service Worker app shell and IndexedDB event spooling. The last snapshot is kept in `localStorage` too, so a client opens with the pool's schedule and names when the server is unreachable; the score keeper client keeps its own record of which matches it has finished, so it can move through a whole pool offline. On reconnect, every match log on the device that the server is missing is pushed — not only the match on screen.

```mermaid
sequenceDiagram
    autonumber
    actor Scorekeeper
    participant SW as Service Worker / App Shell
    participant App as Scorekeeper Web App
    participant IDB as IndexedDB (Outbox Queue)
    participant LAN as Venue LAN / Go Server

    Scorekeeper->>SW: Load Scorekeeper URL
    alt Online Initial Load
        SW->>LAN: Fetch App Bundle
        LAN-->>SW: HTML, JS, CSS Assets
        SW->>SW: Cache in CacheStorage
    else Offline Load / Reload
        SW-->>App: Serve from CacheStorage
    end

    Scorekeeper->>App: Record Points / Warnings
    App->>IDB: Enqueue Event (matchId, seq, payload)
    App->>App: Update UI Instantly (Optimistic Replay)

    alt LAN Connection Available
        App->>LAN: Flush Queue (POST events)
        LAN-->>App: 200 OK
        App->>IDB: Mark Event Synced / Clear Outbox
    else LAN Connection Lost
        App->xLAN: Connection Dropped
        Note over App,IDB: Work continues offline.<br/>Matches scored without interruption.
        Note over App,LAN: Wi-Fi Restored
        App->>LAN: Flush Pending Queue in Sequence
        LAN-->>App: 200 OK
        App->>LAN: GET /api/matches/:id/events for every other match on the device
        App->>LAN: POST whatever the server is missing (finished offline earlier)
        App->>IDB: Clear Synced Events
    end
```
