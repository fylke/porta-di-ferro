# Contributing

How to build and test, and the conventions the code keeps, are in [AGENTS.md](AGENTS.md).
They apply to people as much as to agents.

**A change to what the app does is a change to a requirement.** Edit or add the record in
[`req/`](req), tag the code that implements it and the test that verifies it
(`// REQ: FR_MATCH_03`), and run `sh req/trace.sh` before pushing. Name the requirement in
the PR. CI fails if a requirement has no code or no test tracing to it, or if a tag names
a requirement that does not exist. The how and why are in
[docs/requirements-as-code.md](docs/requirements-as-code.md).
