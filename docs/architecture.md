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
        OrgUI["Organizer Web Client\n(/organizer)"]
        ScoreUI["Score Keeper Client\n(/match/:id)"]
        DisplaySingle["Mat Display\n(/display/mat/:id)"]
        DisplayMulti["Multi-Mat / Roster Display\n(/display/mats, /display/roster)"]
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

Matches are driven by an append-only event log. State is pure and recomputed by replaying events. Points are differential (e.g., scoring $2$ vs $1$ awards $1$ net point to the higher scorer), capped at $8$ points or $3$ minutes ($180\,000\text{ ms}$).

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
        Server-->>LocalEngine: 200 OK (Event Log)
        Server-)Displays: Broadcast SSE (match_updated)
        Displays->>Displays: Re-render Scoreboard / Roster
    else Network offline or delayed
        Server--xLocalEngine: Network Failure / Timeout
        Note over LocalEngine: Keep in IndexedDB queue
        LocalEngine->>Server: Retry on reconnect / next action
    end
```

---

## 4. Tournament Lifecycle & Ranking Pipeline

Tournaments progress through competitor intake, pool generation, schedule assignment across mats, match execution, and multi-tier index ranking.

```mermaid
flowchart TD
    A[Add / Register Competitors] --> B[Generate Pools]
    B --> C[Generate Schedule & Assign Mats]
    C --> D[Execute Matches at Mats]
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

Scorekeeper devices stay operational even during transient venue Wi-Fi dropouts by leveraging a Service Worker app shell and IndexedDB event spooling.

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
        App->>IDB: Clear Synced Events
    end
```
