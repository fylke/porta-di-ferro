# Porta di Ferro

A HEMA tournament application: score matches at the mat, run pools, show results.

**A club can actually run it themselves.** One file to download, one to run, no external
service, no account, no internet. Built by two members of
[MSL — Medeltida Stridsteknik Linköping IF](https://msl.nu).

## For organizers

1. Download the installer from [Releases](https://github.com/fylke/porta-di-ferro/releases)
   and run it.
2. It opens your browser at the organizer page. That page shows a web address and a QR
   code.
3. Enter the competitors, pick the number of mats and the pool size, and draw the pools.
4. At each mat, open the address on a tablet or phone and pick the mat. That is the score
   keeper client.
5. Put a spare screen on `/display/mats` for the scoreboard, or `/display/roster` for the
   match list. Any browser on the venue wifi can open them — a spectator's phone included.

Everything is stored as plain JSON in a folder you own, so you can read it, back it up, and
in a pinch fix it in a text editor.

**Print the pool sheets before the event** (`/print/pools`). If anything goes wrong on the
day, a pool can be run on paper and entered afterwards.

### The addresses

| Address | Shows |
|---|---|
| `/` | Organizer: competitors, setup, pools and standings |
| `/score` | Score keeper — pick a mat |
| `/display/mat/1`, `/display/mat/2` | One mat's scoreboard |
| `/display/mats` | Every mat on one screen; `?ids=1,2` for a subset |
| `/display/roster` | The match roster |
| `/print/pools` | Printable pool sheets |
| `/api/export.json` | The whole tournament as JSON |

## What it does today

This is Milestone 1, scoped to run MSL's club event on 15 November 2026:

- Up to 2 mats, up to 4 pools of up to 7 competitors — 28 per run
- Pools only; eliminations are Milestone 2
- MSL's ruleset, hardcoded: 8 points or 3 minutes, differential scoring, the three-step
  warning ladder
- Undo of the last confirmed exchange. Deeper correction is Milestone 2, and until then the
  escape hatch is the JSON on disk
- English only
- Several disciplines are run one after another, as separate runs of the application

The full picture is in [`docs/design.md`](docs/design.md); the engineering decisions and
what was ruled out are in [`docs/tech-stack.md`](docs/tech-stack.md).

## For developers

Go on the server with an embedded Svelte SPA, shipped as one Windows executable.

```
go test ./...              # engine, ranking, store, and end to end against the real binary
cd web && npm ci && npm test   # the TypeScript engine against the same shared vectors
cd web && npm run build    # the bundle //go:embed picks up
go run ./cmd/porta         # http://localhost:8080
```

The match engine exists twice, in Go and in TypeScript, because the score keeper client
scores offline. The two are held together by the shared vectors in
[`testdata/vectors/`](testdata/vectors), which both test suites run: **a vector that passes
in one implementation and fails in the other fails the build.** A bug found in a real match
earns a vector before it earns a fix.

```
cmd/porta/          the local product
internal/match/     the match engine — mirrored in web/src/lib/match/
internal/tournament/  pools, ordering, colours, ranking
internal/store/     JSON files, atomic writes, the append-only log
internal/http/      routes, SSE, the embedded bundle
web/                Svelte 5 + Vite
testdata/vectors/   the shared corpus
installer/          Inno Setup
e2e/                drives the built binary over HTTP
```

## Licence

MIT. See [LICENSE](LICENSE).
