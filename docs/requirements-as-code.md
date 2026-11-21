# Requirements as code

What Porta di Ferro must do is written in [`req/`](../req) as
[TRLC](https://github.com/bmw-software-engineering/trlc) records, and every record is
traced to the code that does it and a test that proves it. A requirement nobody
implemented or nobody tests fails the build, and so does a tag in the code that names a
requirement that does not exist. [LOBSTER](https://github.com/bmw-software-engineering/lobster)
does the checking and draws the report.

[`docs/design.md`](design.md) is still where the product is argued out. A requirement
says briefly what was decided and cites the section or issue where it was decided. When
a decision changes, the requirement changes in the same PR as the code.

## What is where

| Path | What it is |
|---|---|
| `req/porta.rsl` | The schema: the four kinds of requirement and the fields each has, with the checks TRLC runs on them |
| `req/stories.trlc` | `US_*`: what each person does with the app, by role |
| `req/functional.trlc` | `FR_*`: rules of behaviour, such as scoring, pools, the bracket, mats and sync |
| `req/non-functional.trlc` | `NFR_*`: qualities the whole app keeps, such as offline, secure, installable and readable data |
| `req/constraints.trlc` | `DC_*`: rules for the shape of the code that keep the rest true, from `AGENTS.md` |
| `req/lobster.yaml` | The tracing policy: what has to trace to what |
| `req/lobster-trlc.yaml` | How the TRLC records become LOBSTER requirements |
| `req/trace.sh` | The whole check, the same locally and in CI |
| `cmd/lobster-tags` | Reads the tags in Go, TypeScript, Svelte, HTML and the shared vectors |
| `.github/workflows/requirements.yml` | Runs `req/trace.sh` on every push and PR |

## Running it

```
python -m venv .venv && . .venv/bin/activate     # Windows: .venv\Scripts\activate
pip install -r req/requirements.txt
sh req/trace.sh
```

It prints the coverage of each level and writes `build/req/report.html`. Open the report
to see every requirement, its text, and what traces to it, each linked to its file and
line. The dev container installs the tools for you. On Windows, run the script from Git
Bash. It sets `PYTHONUTF8=1` itself, so the requirement texts are read as UTF-8.

The script runs these steps, and any of them can fail it:

1. `trlc` checks the records against the schema, including the schema's own checks. For
   example, a draft must say why it has no trace.
2. `lobster-trlc` turns the records into `build/req/requirements.lobster`.
3. `go run ./cmd/lobster-tags` collects the tags into one `.lobster` file per level. It
   fails on a tag it cannot read or place.
4. `lobster-report` joins them under `req/lobster.yaml`.
5. `lobster-html-report` writes the report. Graphviz is optional, and only draws the
   policy diagram.
6. `lobster-ci-report` fails on any requirement without its traces and on any tag
   pointing nowhere.

## The policy

Every requirement needs at least one trace from **code** (Go or TypeScript, Svelte
included) and one from a **test** (a Go test, a Vitest test, a Playwright test or a
shared match vector). Each tag in the code or a test must name a requirement in `req/`.
That is all of `req/lobster.yaml`.

There is no percentage to stay above. The check counts what is missing, so it is either
complete or failing. When a requirement really cannot be traced, it says why in
`untraced`, and LOBSTER counts it as justified. Some reasons it might not be traceable:
it is checked by hand on devices, or done by a workflow rather than code, or not built
yet. Five requirements do this today; each one's reason is in its record.

## Tagging code and tests

A tag is a comment of `REQ:` and one or more IDs, separated by commas:

```go
// Replay derives a match's state from its log.
// REQ: FR_MATCH_01, FR_MATCH_05
func Replay(r Ruleset, events []Event) State {
```

```ts
// REQ: FR_MATCH_07
togglePoint(side: Side, value: number): void {
```

```svelte
<!-- REQ: US_SK_02, US_SK_03 -->
<script lang="ts">
```

```json
"requirements": ["FR_MATCH_03"],
```

- **Go**: above a function, method or type, or anywhere inside a function body. The tag
  goes to that declaration: `go internal/match/engine.go:Replay` in the report. A
  `Test…` function in a `_test.go` file is a test, and anything else is code.
- **TypeScript**: directly above a function, class, method or `const`, or above a
  `test(`, `it(` or `describe(`, which then names the test by its title. Other comment
  lines may sit in between. A tag with no declaration under it stands for the whole file.
- **Svelte and HTML**: an HTML comment stands for the whole component or page. A `//` tag
  inside `<script>` works as in TypeScript.
- **Shared vectors**: a `requirements` list in the vector file. Every vector must have
  one (`TestEveryVectorNamesItsRequirements`).
- `*.test.ts` files are Vitest tests, `web/e2e/*.spec.ts` files are Playwright tests, and
  every other `.ts` and `.svelte` file is code.

Tag the place that decides, not every file that passes the value along. One tag per
requirement on the function that implements the rule is enough, plus one on the test
that would fail if the rule broke.

The ID is the TRLC record's name. TRLC names cannot contain hyphens, so the IDs are
written `FR_MATCH_03` everywhere. Then a single search finds the requirement, its code
and its tests.

## Changing what the app does

1. **Change the requirement first.** Edit the record in `req/`, or add one with the
   next free number in its group, with a `source` saying where it was decided. If it is
   agreed but not built yet, use `status = Status.Draft` with an `untraced` reason. TRLC
   refuses a draft without one.
2. **Build it, and tag it.** Tag the code that implements it and the test that verifies
   it, and remove the `untraced` and the draft status in the same change.
3. **Run `sh req/trace.sh`** with the other checks before pushing.
4. **Name the requirement in the PR** when the change is to behaviour, so a reviewer can
   read the record next to the diff.

Do not renumber. When a requirement is withdrawn, delete its record and its tags
together; the check will find any tag you missed.

## Reading the report in CI

The **Requirements** workflow puts each level's coverage, and any LOBSTER errors, in the
run summary. The full HTML report is the `requirements-report` artifact, kept for
seven days.
