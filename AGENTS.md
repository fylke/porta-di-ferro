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
- **Append-Only Event Stream**: Match state is derived by replaying the event log. Do not store mutable derived state directly.
