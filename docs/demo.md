# The public demo

<https://fylke.github.io/porta-di-ferro/> — the whole application, running in a browser
tab, with a tournament already half fenced (issue #88).

It exists to lower the bar. Somebody who might help with usability testing, or run an
event on this, or write some of it, should be able to see what it is without installing
anything or being walked through it.

## What it is not

It is not a hosted version of the product. There is no server, nothing is saved, and no
other device can see it — close the tab and it is gone. Every screen says so.

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
| `internal/demo` | The tournament in memory, and a router answering the same API paths. Ordinary portable Go, tested by `go test ./...`. |
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

- **Starting a second discipline** is a second copy of the application running beside the
  first. There is no process to start, so the API answers 400 with that sentence.
- **Joining from another device** needs a LAN. The organizer view says "one tab, every
  screen" instead of reporting no network, and links to the score keeper and the displays
  in the same tab.
- **Nothing persists.** Not to a disk, not to `localStorage`, not between tabs.

### One concession

`BuildSnapshot` was a method on `Server`; it is now a function over a `Source`
interface that `*store.Store` already satisfied. That is the only change the demo asked
of production code, and it is a better shape regardless: the snapshot builder no longer
needs a filesystem to be exercised.

`Organizer.svelte` has one `if (demo)` branch, for the LAN panel described above. Every
other difference is behind `fetch`.

## The tournament

Thirty-two entrants across six Nordic clubs, three mats, pools of five and six. Mats 2
and 3 have finished; mat 1 has a match under way and two more to come; the eliminations
are not drawn yet.

None of that is a stored snapshot. `internal/demo/fixture.go` holds a list of names, and
everything after it — the pools, the club spread, the running order, the colours, the mat
assignment — comes out of the same `tournament.Generate` a real event uses, with results
that are real match logs the real engine replays. A fixture written out as JSON would
have gone stale the first time the draw changed. The seed is fixed, so every visitor sees
the same tournament.

**Play the rest** finishes every open match, so the bracket and the podium can be reached
without scoring by hand. **Start over** rebuilds the fixture. Neither exists in the
application.

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
