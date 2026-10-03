# The public demo

<https://fylke.github.io/porta-di-ferro/> — the whole application, running in a browser
tab, with an event of two disciplines, one of them already half fenced (issues #88, #102).

It exists to lower the bar. Somebody who might help with usability testing, or run an
event on this, or write some of it, should be able to see what it is without installing
anything or being walked through it.

## What it is not

It is not a hosted version of the product. There is no server, nothing leaves the
browser, and no other device can see it — leave it alone for half an hour and it is gone.
Every screen says so.

The real deployment is unchanged and is the only one that runs an event: one binary on
the organizer's PC, serving JSON files it owns, with the score keepers' tablets and the
hall screens on the venue LAN. Nothing about the demo is a step towards hosting that.

## How it works

```text
A real event      Browser  →  Go server  →  JSON files, SSE, the venue LAN
The demo          Browser  →  demo adapter  →  the same Go code, as WebAssembly
```

The rule it is built to is that **the application must not know it is in a demo**. The
Svelte bundle is the same one an organizer's PC serves; what changes is only what is on
the other end of `fetch` and `EventSource`.

| Piece | What it does |
| --- | --- |
| `internal/demo` | The event in memory — `Event` over two `Demo` disciplines — and a router answering the same API paths. Ordinary portable Go, tested by `go test ./...`. |
| `cmd/demo-wasm` | Forty lines of `syscall/js` glue. The only part with a `js && wasm` build tag. |
| `web/src/demo/adapter.ts` | Replaces `window.fetch` and `window.EventSource` before the app mounts. |
| `web/src/demo/DemoBanner.svelte` | The strip that says what this is, with Play the rest and Start over. |
| `.github/workflows/pages.yml` | Builds the module and the bundle, deploys to Pages from `main`. |

### Why WebAssembly rather than fixtures or a TypeScript port

The standings, the tie-break chain and the bracket are the parts people ask hard
questions about. Three ways to compute them without a Go server:

- **Fixture JSON.** Smallest, and inert: scoring a match would leave the pool table
  frozen, which reads as broken on the first click.
- **Port `ranking.go` and `bracket.go` to TypeScript.** Fully interactive, and a second
  implementation of logic `docs/tech-stack.md` §4 says never runs on a client, with no
  shared-vector harness holding it to the Go. It would drift, and a demo that quietly
  disagrees with a real event is worse than no demo.
- **Compile the Go.** No second implementation at all. `internal/tournament`,
  `internal/match` and `httpapi.BuildSnapshot` are the same code in both places, so the
  demo cannot say something a real event would not.

The third costs about 1.6 MB gzipped on the demo page, and nothing at all on the real
one: `VITE_DEMO` is substituted at build time, so a normal build drops the adapter, the
banner and the loader. The application bundle grew by a fifth of a kilobyte, which is the
router learning about base paths.

### What the demo cannot do, and says so

- **Taking a discipline out keeps nothing.** At an event its folder is kept under
  `retired/`; in a browser tab there is nowhere to keep it, and the answer says so.
- **Joining from another device** needs a LAN. The organizer view says "one tab, every
  screen" instead of reporting no network, and links to the score keeper and the displays
  in the same tab.
- **Nothing persists beyond the browser,** and not for long in it. See below.

### Between tabs

Several of the organizer's links open a new tab — the displays, the roster, the info
sheet — and every tab is its own copy of the module. Without help, a visitor who edited
the welcome message and opened the landing page found the demo as it was before they
touched it (issue #108).

So the adapter keeps the event in `localStorage`. After every request the module
reports as a change it writes the whole state out (`GET /api/demo/save`); a tab that
opens loads it (`POST /api/demo/load`); and the tabs already open follow the `storage`
event, so the score keeper in one tab and the organizer's view in another are the same
event. **Start over** clears it, which resets every tab.

It lasts until the visitor has done nothing for half an hour. After that the next tab
starts from the fixture, so somebody coming back next week, or the next person at the
same computer, sees what every first visitor sees. A save the module will not take, such
as one written by an older demo (`eventSaveFormat` in `internal/demo`), is dropped the same
way.

The event editor's call, `PUT /api/event`, was missing from the demo's router and is
answered now, through the same `httpapi.CleanEvent` the server uses.

### One concession

`BuildSnapshot` was a method on `Server`; it is now a function over a `Source`
interface that `*store.Store` already satisfied. That is the only change the demo asked
of production code, and it is a better shape regardless: the snapshot builder no longer
needs a filesystem to be exercised.

`Organizer.svelte` has one `if (demo)` branch, for the LAN panel described above. Every
other difference is behind `fetch`.

## The event

Stångebroslaget, with two disciplines, at the addresses a real event gives them
(`demo.Event`, the demo's stand-in for the server's coordinator).

**Open steel Longsword**: thirty-two entrants across six Nordic clubs, three mats, pools of five and six, about
halfway through the pools. Mat 1 is two thirds into its first pool with a match under
way, mat 2 is halfway into its second, and mat 3 is between its two, so many fencers have
two matches left. The eliminations are not drawn yet.

None of that is a stored snapshot. `internal/demo/fixture.go` holds a list of names, and
everything after it — the pools, the club spread, the running order, the colours, the mat
assignment — comes out of the same `tournament.Generate` a real event uses, with results
that are real match logs the real engine replays. A fixture written out as JSON would
have gone stale the first time the draw changed. The seed is fixed, so every visitor sees
the same tournament.

**Open Sabre**: fourteen entrants in two pools of seven on two mats, drawn and waiting
for the 15:00 slot in the programme. Astrid, Bo and Greta are in both, which is the
normal case at a club open and what the landing page's name search is for. Astrid and
Greta signed up for both on one response, so each is one person with one page for the day;
Bo was typed in at both desks, so the event admin's People panel asks whether the two Bos
are one person. The people are part of what the demo keeps between tabs. A visitor can
add a discipline of their own from the event admin; it starts empty.

The mats are the event's, as at a real event: three of them, the longsword on all three
and the sabre's pools queued behind on the first two. The event admin's mat board moves
any card by drag or menu, and the score keeper and the screens follow their mat from one
discipline to the next. The plan is part of what the demo keeps between tabs.

The day has a past: the fixture's fenced matches carry the times they were fenced at,
counted back from the moment the demo opened, so the mat board forecasts the rest of the
day from each mat's real pace, the landing page lists the fencing with its times, and a
person's page says about when each match still to come is expected. The planned day starts
when the first match did, and the wasm module tells times in the visitor's own zone.

The event has ten volunteers, already put to work by the staff suggestion: referees, a
short supply of assistants, score keepers, a physician on call, and Clara, who fences the
longsword and referees the sabre, so her page shows her duties beside her matches and the
suggestion never puts her on a mat while she fences. The staff are part of what the demo
keeps between tabs.

**Play the rest** finishes every open match in every discipline, so the brackets and the
podiums can be reached without scoring by hand. **Start over** rebuilds the event. Neither
exists in the application.

## Working on it locally

```sh
# The module and the Go runtime shim that loads it, both gitignored build artifacts.
GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o web/public/demo.wasm ./cmd/demo-wasm
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" web/public/wasm_exec.js   # lib/wasm on Go 1.24+

cd web && npm run build:demo
```

`dist/` then wants serving from a `/porta-di-ferro/` path with `.wasm` as
`application/wasm` and unknown paths falling back to `404.html`, which is what GitHub
Pages does. Anything less will mislead you: a static server that serves `.js` as
`text/plain` fails in a way that looks like a bundling problem and is not.

Most of it needs no browser at all. `internal/demo` is plain Go:

```sh
go test ./internal/demo/...
```
