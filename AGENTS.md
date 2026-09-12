# Agent Guidelines

## Architecture & Documentation Maintenance

- **Keep [docs/architecture.md](docs/architecture.md) in sync**: Whenever modifying system components, match scoring rules, state machine transitions, event flow, synchronization logic, offline persistence, or tournament ranking pipelines, update the corresponding Mermaid diagrams and documentation in [docs/architecture.md](docs/architecture.md).
- **Product and Stack References**: Refer to [docs/design.md](docs/design.md) for product rules and [docs/tech-stack.md](docs/tech-stack.md) for architectural constraints.

## Build and Test

- **Go Backend & Engine Tests**: Run `go test ./...`
- **Frontend / Client Tests**: Run `npm test` inside `web/`
- **Shared Test Vectors**: Verify scoring and rules changes against vector tests in `testdata/vectors/` and both engine implementations (`internal/match/` and `web/src/lib/match/`).

## Key Conventions

- **Dual Match Engine**: The scoring match engine is implemented in both Go (`internal/match/`) and TypeScript (`web/src/lib/match/`), verified against shared JSON vectors.
- **Every string a person reads goes through `t()`** (`web/src/lib/i18n.svelte.ts`), with the English as the key and the Swedish in `web/src/lib/sv.ts`. `npm test` fails if a key in the source has no Swedish entry or an entry is unused, so add the Swedish in the same change. Internal identifiers (`red`, `exchange`, `quarter`) are never translated on the wire.
- **Append-Only Event Stream**: Match state is derived by replaying the event log. Do not store mutable derived state directly. The organizer's history editor is the one deliberate exception: it rewrites a match log wholesale via `PUT /api/matches/{id}/events` and always keeps the previous version as a `.bak` beside it.
