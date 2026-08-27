# Porta di Ferro — Technical Stack

> **Status:** decided. This document records the outcome of issue #8 and the discussion on it.
>
> **Companion to [`design.md`](design.md).** That document decides what the application does; this
> one decides what it is built out of. Where they overlap, design.md is the product authority and
> this document the engineering one.
>
> **Living document**, on the same terms as design.md. Everything here can be reopened, and several
> decisions carry an explicit trigger for doing so rather than a vague promise to revisit.

---

## 1. The decision

**Go on the server, an embedded Svelte SPA on the clients, shipped as one signed Windows
executable.** Everything else on this page follows from that sentence or qualifies it.

| Layer | Choice | Why, in one line |
|---|---|---|
| **Server language** | **Go** | Single static binary, no runtime, cross-compiles in one command |
| **HTTP** | `net/http` stdlib | Go 1.22 routing covers this workload; a framework buys nothing here |
| **Client** | **Svelte 5 + Vite**, built to a static bundle | Fast iteration for §4's prototyping, small payload, large contributor pool |
| **Serving the client** | `//go:embed` | The binary *is* the web app — nothing to deploy separately |
| **Local storage** | JSON files on disk | design.md decision 8; the organizer owns and can read their data |
| **Client storage** | IndexedDB + service worker app shell | Local-first writes, and a device that loaded once works offline |
| **Writes** | `POST`, idempotent on `(match_id, sequence)` | The sync model in design §3 needs nothing more |
| **Push** | **SSE** | One-directional is all a display needs, and it reconnects itself |
| **Match engine** | Written twice — Go and TypeScript | Small, pure, and mechanically verified against shared test vectors |
| **Tournament logic** | Go only | Pools and ranking never run on a client |
| **Local packaging** | Inno Setup → signed `.exe` on GitHub Releases | design §1 is an acceptance criterion, not packaging polish |
| **Cloud mirror** | Same repo, second binary, container image | Optional, Milestone 3, and a dumb presenter (§3) |
| **Mirror storage** | SQLite via `modernc.org/sqlite` | Keeps the single-file property, and pure Go keeps cgo out |
| **Tests** | `go test` + rapid; Vitest + fast-check; shared JSON vectors | Both engines run the same corpus in CI (§11) |

Two products fall out of this — a required local executable and an optional cloud mirror — and that
split is the part of the decision with the most consequences. It gets its own section (§3).

---

## 2. Why Go

The full option analysis is in issue #8 and is not repeated here. What survived the discussion:

- **Installability decides it.** A static binary with no runtime, no container, and no separate
  database server is the shortest possible path to "download one file and run it" (design §1). Every
  other candidate either adds a runtime to the download or adds a step to the install.
- **The direction of travel only works one way.** A static binary becomes a container image in about
  four lines of Dockerfile. A container-first application does not become a double-click executable
  at all. Bake a container runtime into the core and the easy local install stops being reachable;
  ship the binary first and the container is nearly free. This, rather than any performance
  argument, is why Docker sits on the deployment side of the line and not in the stack.
- **Contributor pool.** An MIT-licensed club project hoping for outside help cannot afford the
  narrowest language on the shortlist. Go is boring, stable, and widely known — good properties for
  something meant to outlive its two authors.
- **Cross-compilation is a one-liner.** `GOOS=windows GOARCH=amd64 go build` from any machine, which
  keeps the release workflow trivial and lets development happen anywhere while the target is
  Windows.

### What choosing Go costs

Stated plainly, because both costs are real and neither is fatal:

- **No desktop window.** Tauri would have given the organizer an application window; Go gives them
  "now open your browser". The mitigation is a deliberate first-run experience rather than an
  apology: the executable starts the server, opens the default browser at the organizer view, and
  that view shows the LAN URL and a QR code for the clients, large enough to read from across a
  table. A tray icon with *Open* and *Quit* is worth having and is a small library, not a project.
- **The match engine gets written twice.** See §4. The duplicated surface is smaller than the issue
  discussion assumed, and the test vectors that verify it are worth having regardless.
- **The installer is not free.** Go produces an executable, not an installer. Inno Setup covers it,
  and code signing is the real work (§9).

---

## 3. Two products, one core

|  | **Local product** | **Cloud mirror** |
|---|---|---|
| Runs on | Organizer's PC | A droplet, Fly, AWS, anywhere |
| Distribution | Signed Windows installer | Container image, plus a plain Linux binary |
| Audience | Staff, and spectators on venue wifi | Anyone, anywhere |
| Required? | Yes — this *is* the application | No — pure addition, Milestone 3 or later |
| Installed by | A volunteer, in five minutes | Someone who is good at data™ |
| Authoritative? | Yes | **Never** |

**One repository, two build targets** — `cmd/porta` and `cmd/porta-mirror`. The mirror is useless
without the local product, so separating the repositories would only add a synchronisation problem
between two halves of one thing.

### The mirror is a dumb presenter

The local product pushes both the event log and the derived results. The mirror stores what it is
given and renders it. **It never re-derives and it never decides anything.**

That has three consequences worth stating:

- **It does not need the scoring core**, which is why the duplication question in §4 is confined to
  the local product and the browser.
- **There is no conflict model to design.** Replication is one-way, from one writer, so there is
  nothing to merge and nothing to reconcile. A mirror that has fallen behind is stale, never wrong.
- **The cost of a bug moves.** An error in the local derivation propagates to the mirror silently,
  because nothing on the far side can catch it. For "who won, who is up next" that trade is
  acceptable; it would not be if the mirror were ever allowed to become a source of truth.

> [!IMPORTANT]
> **The local product is complete without the mirror.** A club that never deploys one loses nothing
> except access for people who are not in the building. This is the whole point of the split, and
> any feature that quietly makes the mirror mandatory has broken it.

### The append-only log is already a replication stream

The split is cheap because design §3 already chose an append-only exchange log with one writer per
match and a `(match_id, sequence)` primary key. That log *is* the replication stream. The mirror
receives events and stores them; resuming after an outage is "everything after sequence N";
idempotency falls out of the key. No sync model needs inventing.

Had the design stored derived state instead, this would have been a rewrite.

### The container is the easy 10%

The container image is genuinely trivial. What is not, and what needs scoping before Milestone 3 is
committed to:

- **Auth** — an organizer must be able to push to the mirror and nobody else. A per-event push token
  generated by the local product is the obvious shape; it needs deciding, not assuming.
- **Multi-tenancy** — several clubs, several events, one deployment.
- **TLS and a domain** — solvable in-process with `autocert`, or with Caddy in front. A decision
  either way.
- **Personal data on the open internet** — competitor names and clubs on a public server is a
  different legal posture from the same names on a laptop on a LAN, particularly under GDPR and
  particularly if any competitor is a minor. Explicit consent at signup, and an explicit answer to
  "what is public by default". See design §8.

---

## 4. The scoring core

### It splits in two, and only half of it is shared

The issue discussion treated "the core" as one thing. It is two, and the distinction shrinks the
duplication problem considerably:

| | Runs where | Contents |
|---|---|---|
| **Match engine** | Server **and** score keeper client | Differential exchange resolution, warning escalation and the point deduction, the point cap and the match-ending conditions, timer rules |
| **Tournament logic** | Server only | Pool generation, match ordering, colour assignment, the four ranking indices, retroactive voiding of a withdrawal |

Only the match engine is needed offline on a tablet, because that is all the score keeper view shows
(design §4). Pools and ranking belong to the organizer's screen, which is served by the machine that
has the Go implementation anyway. Realistically that is 200–400 lines duplicated rather than the
500–1500 the issue estimated.

### Duplication, verified by shared test vectors

The Go and TypeScript implementations are held together by a corpus of JSON test vectors checked
into the repository. Each vector is a sequence of log events and the expected derived state. Both
test suites consume the same corpus, and **a vector that passes in one implementation and fails in
the other fails the build.**

This is worth more than the duplication costs, for a reason that is easy to miss: the vectors are an
executable statement of the ruleset. When rules become data-driven in Milestone 3, that corpus is
already the specification a ruleset definition has to satisfy.

### Why not compile the Go engine to WebAssembly

Stock Go produces a 2–4 MB WASM module. The issue analysis treated that as disqualifying; §5 below
retracts that reasoning, so the option is genuinely open and is being declined on different grounds:

- The `syscall/js` boundary means serialising across it and hand-writing glue on both sides.
- It puts a Go toolchain inside the frontend build, which is exactly the friction §4 of design.md
  cannot afford — the score keeper view needs the *most* iteration and would get the *least* support.
- TinyGo removes the size cost but constrains reflection and parts of the standard library.

> [!NOTE]
> **Revisit if** the match engine passes roughly a thousand lines, or if a drift bug survives the
> vector corpus into a real match. The vectors make the switch cheap whenever it is wanted, which is
> precisely why this decision does not need to be right the first time.

---

## 5. The client payload argument, corrected

An earlier revision of the issue analysis called a 2–4 MB client payload "a lot to push to a tablet
over venue wifi", and used it to weigh against Blazor WebAssembly, full-stack Rust and Go's own WASM
output. **That was wrong, and it was load-bearing in three places.**

A few megabytes over a modern venue LAN is unremarkable. The correction:

- **The number to watch is tens of megabytes, not single digits.** The local install is where size
  genuinely matters, which is why a 60–110 MB packed-runtime binary remains a serious objection to
  TypeScript end-to-end and a 2–4 MB browser payload does not.
- **The scenario that actually stresses first load** is a phone joining cold, mid-event, on congested
  venue wifi with no internet fallback — not the tablet at the mat, which loaded once and has a
  service worker. Budget for that case and measure it (§11) rather than arguing about it.
- **What it changes:** Blazor's headline con shrinks, and the first of the three arguments against
  full-stack Rust weakens. Neither is revived, because both lose on the arguments in §2 that have
  nothing to do with payload size — contributor pool, install artifact, and the container asymmetry.
- **What it does not change:** the recommendation. Go was never chosen for its WASM story; it was
  chosen despite it.

---

## 6. The frontend

**Svelte 5 with Vite, built to a static bundle and embedded in the binary.** No SvelteKit: there is
no server-side rendering here, and a Node runtime at build time is acceptable while a Node runtime in
the shipped artifact is not. Client-side routing is a small dependency, not a framework.

One bundle serves every surface — the organizer views, the score keeper client, and `/display/*` —
with the route deciding what renders. The Go server serves it at all of those paths and falls
through to the SPA for anything it does not recognise.

**Offline is a service worker plus IndexedDB.** The app shell is cached, so a device that has opened
the app once keeps working through a network drop, and the score keeper's own append-only log lives
in IndexedDB and is flushed to the server asynchronously. Nothing in the scoring path waits on the
LAN (design §3).

**The score keeper view is a prototyping problem** (design §4), and the stack should serve that:
variants live behind a route so two or three can be put in front of real competitors on a club night
without a build flag or a branch. This is the strongest frontend argument in the whole decision,
because it applies weekly for months to the screen everything else depends on.

**Platform constraints to design for from the start**, per design §12:

| Constraint | Consequence |
|---|---|
| Every iOS browser is WebKit | There is no port, only compatibility. Test on real hardware early |
| Wake lock needs Safari 16.4+ | Displays and score keeper devices both need the screen kept awake |
| Element fullscreen is unavailable on iPhone Safari | iPhones are score keeper devices, not displays |
| Safari evicts unused site data after seven days | Add to Home Screen avoids it; worth knowing before a log vanishes |

---

## 7. Transports

**Writes go over `POST`. Server-to-client push is SSE. WebSocket is not used.**

| Path | Transport | Why |
|---|---|---|
| Score keeper → server | `POST` per confirmed exchange | Idempotent on `(match_id, sequence)`; retries need no deduplication |
| Server → displays | SSE | One-directional, auto-reconnecting, trivial to reason about |
| Server → score keeper | SSE | Enough for roster changes and, later, handover state |
| Server → mirror | `POST`, batched | Same events, same key, over the internet instead of the LAN |

The issue analysis assumed the score keeper's own sync "does want bidirectional". On inspection it
does not: the client writes locally and pushes, and everything it needs *from* the server is a
notification it can receive on an event stream. One transport for all push is simpler than two.

> [!NOTE]
> **Revisit if** score keeper handover (design §7, item 10) turns out to need a genuine round trip.
> That is the one candidate, and it is a Milestone 2 problem.

---

## 8. Data on disk

**The local product writes JSON files**, per design decision 8, in one directory per tournament:

| File | Contents |
|---|---|
| `competitors.json` | Registration and status |
| `tournament.json` | Mats, pool constraints, generated pools and match assignments |
| `matches/<match_id>.ndjson` | The append-only exchange log — one event per line |

Newline-delimited JSON for the log, because appending a line is the cheapest durable write there is
and a truncated final line after a crash costs one event rather than the file. The other two are
written by atomic replace, never in place. Derived state — scores, standings, rankings — is never
stored, only computed (design decision 5).

The organizer can open any of these in a text editor, which is what makes the hand-edit escape hatch
in design §6 real rather than theoretical.

**The mirror uses SQLite**, through `modernc.org/sqlite`. A shared server wants indexed queries
across events in a way a laptop running one tournament does not, and SQLite keeps the single-file
property that makes backup and migration trivial. The pure-Go driver matters: **no cgo anywhere in
this project**, because cgo is what turns one-command cross-compilation back into a toolchain
problem.

---

## 9. Build, release and signing

```
porta-di-ferro/
├── cmd/
│   ├── porta/            # the local product
│   └── porta-mirror/     # the cloud mirror
├── internal/
│   ├── match/            # match engine — mirrored in TypeScript
│   ├── tournament/       # pools, ordering, ranking
│   ├── store/            # JSON files, atomic writes, the log
│   └── http/             # routes, SSE, embedded assets
├── web/                  # Svelte + Vite; src/lib/match/ mirrors internal/match
├── testdata/vectors/     # shared JSON test vectors, consumed by both suites
├── installer/            # Inno Setup script
└── docs/
```

| Target | Produced by | Published as |
|---|---|---|
| Windows local product | `GOOS=windows go build` + Inno Setup | Signed `.exe` installer on GitHub Releases |
| Linux mirror | `GOOS=linux go build` | Plain binary on GitHub Releases |
| Mirror container | `FROM scratch` over the Linux binary | Container image |

Releases are tag-driven through GitHub Actions: build the web bundle, embed it, cross-compile, sign,
package, publish. Versions are pinnable and release notes are written for organizers rather than
developers (design §1).

### Code signing is the sharpest risk on this page

An unsigned executable downloaded from GitHub Releases triggers SmartScreen, and **that dialog is
precisely the install wall this project exists to avoid**. It is not a polish item; it sits directly
on the five-minute acceptance criterion.

Three routes, to be decided before the first public release:

| Route | Note |
|---|---|
| **OV certificate** | Cheapest. SmartScreen reputation still has to accumulate over downloads |
| **Azure Trusted Signing** | Subscription rather than a hardware token, and CI-friendly. Eligibility requires a legal entity with a verifiable history — worth checking whether MSL qualifies |
| **Accept the warning** | Free. Costs a "More info → Run anyway" step inside the five-minute test, and it is exactly the kind of step a non-technical organizer stops at |

Go binaries are also occasional Defender false-positive targets. Submit the first signed release for
analysis rather than discovering it the week before an event.

---

## 10. Ad-hoc streaming — what it implies

Milestone 3, mirror-side only, and recorded here because it is the one future feature that adds an
entire subsystem rather than a screen. The idea: point a phone at the QR code on a mat and be
streaming that mat seconds later, with none of the capture rig a normal production setup needs.

What it actually requires:

- **WebRTC ingest, not RTMP.** A browser cannot speak RTMP, so a phone streams over WebRTC — WHIP is
  the standard shape — into a media server such as MediaMTX or LiveKit, or into a hosted provider.
  That is a real dependency with real running costs, and it belongs to the mirror alone.
- **The QR code does two jobs at once**, which is the neat part of the idea: it carries the mat
  identity and a short-lived ingest token together, so joining the stream and labelling it are one
  action. The mirror already knows which match is on that mat, so the streaming overlay (design §8)
  gets its metadata for free.
- **The trust model is deliberately loose.** A phone claims a mat by scanning the code at it. Someone
  can point their camera at the wrong mat, deliberately or otherwise; the accepted mitigation is that
  tokens are short-lived and revocable, not that misuse is prevented.
- **Consent is enforceable here**, which is unusual and worth exploiting. Because the mirror knows
  which competitors are on a mat, ingest can be blocked for a mat where a competitor who has not
  consented to being filmed is fencing. That turns a GDPR problem into a scheduling rule.
- **The uplink is the constraint.** Venue upload bandwidth, not the phone, sets how many mats can
  stream at once. Two concurrent streams is a realistic starting assumption.

> [!IMPORTANT]
> None of this is allowed to influence the core stack choice. The local product gains no media
> dependency, and a club that never deploys a mirror never meets any of it.

---

## 11. Verification that belongs to the stack

Design §12 covers the product-level testing. These are the stack-specific additions:

- **Engine parity suite.** The shared vectors in `testdata/vectors/` run against both the Go and the
  TypeScript match engine in CI. Divergence fails the build. Every ruleset behaviour in design §5
  earns a vector, and a bug found in a real match earns one before it earns a fix.
- **Property tests on both sides.** `rapid` in Go, `fast-check` in TypeScript, over the same
  invariants: a derived score depends only on the log and not on when it is replayed, warning
  escalation is monotonic, and no single exchange moves a score by more than the ruleset's maximum.
- **Cold-load budget.** Measure what a phone downloads on first load over venue wifi with no internet
  fallback, on a build with a cleared cache. Record the number in the release notes so a regression
  is visible rather than discovered at an event (§5).
- **Cross-compilation smoke test.** Every CI run builds the Windows target, so a cgo dependency
  cannot creep in unnoticed.
- **End to end against the real binary.** Playwright against the built executable rather than a dev
  server, covering the simulated event and the offline test in design §12. Testing the thing that
  ships is the point.

---

## 12. What was ruled out

Recorded so it is not relitigated. Several of these are good answers to the *mirror* and poor ones to
the local product — the split is what separates them.

| Option | Why not |
|---|---|
| **TypeScript end-to-end** | The strongest challenger, and it would have dissolved the duplication in §4 entirely. A 60–110 MB packed-runtime binary, and the AV false-positive profile that comes with one, is the wrong artifact for the five-minute install |
| **Rust + Axum** | The honest answer if duplicating nothing mattered most, and `cargo-dist` neutralises much of its packaging cost. Loses on contributor pool and iteration speed, both of which matter more here than the shared crate does |
| **C# / .NET + Blazor** | The cleanest shared-core story of all, and the best Windows installer story. Loses on contributor pool for a hobbyist club project; the browser-payload objection against it is retracted (§5) |
| **Tauri 2** | Best installer ergonomics and a real desktop window, but a desktop app has no server form. The two-product split would mean maintaining the server twice |
| **Full-stack Rust (Leptos, Dioxus)** | One language, no duplication, and the option this decision came closest to taking. Loses on frontend iteration speed against design §4, and narrows the contributor pool twice over |
| **Docker as the install path** | Correct for the mirror, wrong as a core requirement, for the asymmetry in §2. A volunteer running a club tournament should never meet a container runtime |
| **Phoenix LiveView** | Server-rendered UI is structurally incompatible with scoring that survives a network drop. Genuinely strong for the mirror, though, if that ever becomes a separate codebase |
| **Python** | Not a performance objection — FastAPI would not notice this workload. It is packaging: PyInstaller artifacts are large, slow to start and heavily AV-flagged. Entirely reasonable for the mirror, which has no packaging problem |

---

## 13. Still open

- **Which signing route** (§9). Blocks the first public release, not the first line of code.
- **How the mirror authenticates a push**, and what multi-tenancy looks like (§3).
- **What is public by default** on a mirror, and how consent is captured at registration.
- **Whether the match engine stays duplicated** or moves to WASM (§4) — trigger recorded there.
- **macOS**, still unplanned. Cross-compiling the binary is free; signing and notarising it is not.
- **The media path for ad-hoc streaming** (§10) — self-hosted or provider, and who pays for it.
