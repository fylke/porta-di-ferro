# Plan: Requirements-as-Code Integration (TRLC + Lobster)

## Executive Summary

This plan proposes integrating **TRLC** (The Requirements Language Compiler, by BMW Software Engineering) and **Lobster** (Lightweight Open BMW Software Traceability Evidence Report) into the Porta di Ferro tournament application. This enables managing requirements as executable code, with built-in traceability between requirements, design decisions, code implementation, and tests.

---

## 1. What TRLC and Lobster Do

### TRLC (The Requirements Language Compiler)
- **Purpose**: Treat requirements as code, not prose
- **Features**:
  - Define requirements in a structured, human-readable format (`.trlc` files)
  - Compile requirements into validated, queryable specifications
  - Support for formal attributes (priority, status, owner, rationale)
  - Type checking and cross-reference validation
  - Integrates with CI/CD pipelines
  - Export to various formats (HTML, JSON, traceability matrices)

### Lobster (Lightweight Open BMW Traceability)
- **Purpose**: Link requirements to implementation and test evidence
- **Features**:
  - Annotation-based traceability (tags like `lobster-req: REQ-123`)
  - Scans codebase for requirement references
  - Generates traceability matrices and coverage reports
  - Reports gaps and orphaned requirements
  - Integrates with CI to enforce coverage
  - Produces visual traceability reports

---

## 2. Current State Analysis

### Porta di Ferro's Existing Approach
- **Requirements**: Captured in markdown documentation (`docs/design.md`, `docs/architecture.md`)
- **Use Cases**: Defined as Mermaid diagrams in design docs
- **Process**: Living document approach; design decisions recorded ad-hoc
- **Traceability**: Manual, implicit—no machine-enforced link between requirements, code, and tests
- **Testing**: Go unit tests + e2e Playwright tests, but no formal requirement coverage tracking

### Key Artifacts to Manage as Code
1. **User Stories** (from use case diagrams):
   - Event Organizer setup and configuration flows
   - Score Keeper match scoring operations
   - Competitor/Spectator viewing roles
   - Staff assignment and scheduling

2. **Functional Requirements** (from architecture + design):
   - Tournament pool generation and ranking
   - Match state machine and scoring engine
   - Offline sync and data persistence
   - Multi-discipline event coordination
   - Handover of mat tablets

3. **Non-Functional Requirements**:
   - Offline-first capability
   - Graceful degradation (e.g., score keeper crash recovery)
   - Cross-platform browser compatibility
   - Secure context restrictions (HTTP vs HTTPS)
   - <1 second UI responsiveness for score entry

4. **Constraints** (design decisions):
   - Single responsibility (event owns event.json; discipline owns its folder)
   - Shared pure functions in `internal/hall`
   - Append-only event stream for match state
   - No mutable derived state storage

---

## 3. Benefits & Rationale

### Why TRLC + Lobster for This Project?

| Benefit | Why It Matters for Porta di Ferro |
|---------|-----------------------------------|
| **Executable Requirements** | Turn design docs into validated, queryable specs; catch ambiguity early |
| **Built-in Traceability** | Link each requirement to code (via annotations) and tests—automatic coverage reports |
| **CI Integration** | Fail builds if a requirement is not traced to implementation or test |
| **Design Documentation** | Auto-generate traceability matrices and coverage dashboards from code |
| **Onboarding** | New contributors see which requirements affect which files |
| **Compliance** | Easier to demonstrate that design rules (like "no event.json writes outside EventInfo") are enforced |
| **Change Impact** | When a requirement changes, see all affected code and tests immediately |
| **Living Document** | Markdown + code annotations stay in sync; no separate documentation drift |

---

## 4. Phased Implementation Plan

### Phase 0: Preparation (Week 1-2)
**Goal**: Set up tooling infrastructure and define TRLC schema

#### Tasks:
1. **Create TRLC schema file** (`req/schema.trlc`)
   - Define requirement types: `UserStory`, `FunctionalReq`, `NonFunctionalReq`, `DesignConstraint`, `TestCase`
   - Define attributes: `id`, `title`, `description`, `rationale`, `owner`, `priority`, `status`, `parent`, `links`
   - Set up revision tracking: version, date, author

2. **Add tools to dev container** (`.devcontainer/devcontainer.json`)
   - Install `python3-trlc` (or via pip)
   - Install `python3-lobster` (or via pip)
   - Add shell aliases for `trlc` and `lobster` commands

3. **Create CI/CD pipeline** (`.github/workflows/requirements.yml`)
   - Compile TRLC requirements on each commit
   - Run Lobster traceability checks
   - Fail if coverage < 80% (adjustable)
   - Generate traceability report as artifact

4. **Document convention** (`docs/REQUIREMENTS_AS_CODE.md`)
   - How to write TRLC requirement files
   - Lobster annotation syntax for Go and TypeScript
   - Traceability workflow and coverage expectations

---

### Phase 1: Core Requirements (Week 3-4)
**Goal**: Migrate existing design/architecture into TRLC requirements

#### Requirements to Define:

**1. User Stories** (`req/stories.trlc`)
- `US-ORG-01`: Event Organizer—Set up event (mats, pools, timings)
- `US-ORG-02`: Event Organizer—Add competitors (manual or via signup file)
- `US-ORG-03`: Event Organizer—Generate pool draws
- `US-ORG-04`: Event Organizer—View standings and rankings
- `US-SK-01`: Score Keeper—Select mat and match
- `US-SK-02`: Score Keeper—Record exchange (points, warnings)
- `US-SK-03`: Score Keeper—Confirm and finish match
- `US-SK-04`: Score Keeper—Undo last exchange
- `US-COMP-01`: Competitor—View their schedule
- `US-COMP-02`: Competitor—View pool standings
- `US-SPEC-01`: Spectator—View scoreboard
- `US-SPEC-02`: Spectator—View overall standings

**2. Functional Requirements** (`req/functional.trlc`)
- `FR-POOLS-01`: Pool generation algorithm (round-robin, balanced by seed)
- `FR-POOLS-02`: Pool ranking (SOS, D, V)
- `FR-MATCH-01`: Match state machine (not started → in progress → finished)
- `FR-MATCH-02`: Scoring: 8 points or 3 minutes
- `FR-MATCH-03`: Differential scoring (marginal advantage)
- `FR-MATCH-04`: Warning ladder (yellow, red, disqualification)
- `FR-MATCH-05`: Undo of last confirmed exchange
- `FR-ELIM-01`: Single-elimination bracket from pools
- `FR-ELIM-02`: Bronze match (3rd place)
- `FR-ELIM-03`: Sudden death (no draws)
- `FR-DISCIPLINE-01`: Multiple disciplines per event
- `FR-DISCIPLINE-02`: Each discipline owns its own folder
- `FR-TABLET-01`: Tablet handover (graceful and ungraceful)
- `FR-TABLET-02`: Tablet crash recovery (epoch-based conflict resolution)
- `FR-OFFLINE-01`: Offline sync when connection restored
- `FR-OFFLINE-02`: Graceful degradation (scorer can continue on paper)

**3. Non-Functional Requirements** (`req/non-functional.trlc`)
- `NFR-PERF-01`: Score entry UI responds in <1 second
- `NFR-PERF-02`: Pool rankings calculate in <2 seconds
- `NFR-OFFLINE-01`: Full functionality without internet (desktop app)
- `NFR-OFFLINE-02`: No secure APIs used on insecure origins (HTTP/tablet)
- `NFR-BROWSER-01`: Chrome, Safari, Firefox support
- `NFR-BROWSER-02`: Mobile browser support (tablets)
- `NFR-DATA-01`: All data as plain JSON (human-readable, editable)
- `NFR-DATA-02`: Append-only event log (no mutable derived state)

**4. Design Constraints** (`req/constraints.trlc`)
- `DC-ARCH-01`: Single event per run; disciplines own their folders
- `DC-ARCH-02`: Shared pure functions in `internal/hall`
- `DC-ARCH-03`: No event.json writes outside `EventInfo`
- `DC-DATA-01`: Append-only event stream for match state
- `DC-DATA-02`: No mutable derived state in JSON
- `DC-API-01`: Each discipline mounted at `/api/d/{slug}/`
- `DC-API-02`: Demo reuses `internal/tournament` and `internal/match`
- `DC-SIGNUP-01`: Offline signup file is single HTML (no build, works from `file://`)

#### Tasks:
1. Extract requirements from:
   - `docs/design.md` (use cases, scoring rules, definitions)
   - `docs/architecture.md` (component boundaries, state machines)
   - `docs/proposals/one-event-many-disciplines.md` (architecture rules)
   - `docs/proposals/offline-signup.md` (file formats, import rules)

2. Write TRLC files for each category

3. Run `trlc compile` to validate; fix any schema violations

4. Generate HTML report and spot-check

---

### Phase 2: Code Traceability (Week 5-6)
**Goal**: Annotate existing code with Lobster references

#### Go Code Annotations
Add comments linking requirements to implementation:

```go
// REQ: FR-POOLS-01, FR-POOLS-02 - Pool ranking algorithm
func ViewRanking(ctx context.Context, snapshot *Snapshot) ([]Standing, error) {
    // ...
}

// REQ: FR-MATCH-01, FR-MATCH-02 - Match state transitions
type Match struct {
    Status  MatchStatus
    Red, Blue *Competitor
    // ...
}

// REQ: DC-ARCH-02 - Shared pure function in internal/hall
func BuildMats(plan *Plan, snapshots map[string]*Snapshot) []Mat {
    // ...
}
```

#### TypeScript Code Annotations
```typescript
// REQ: FR-MATCH-03, FR-MATCH-04 - Differential scoring & warning ladder
export function scoreExchange(
  red: number,
  blue: number,
  warnings: Warning[]
): ScoreResult {
  // ...
}

// REQ: NFR-PERF-01 - <1 second UI response
export const ScoreEntry = /* Svelte component */
```

#### Test Annotations
Link tests to requirements they verify:

```go
// REQ: FR-POOLS-02 (verify ranking algorithm)
func TestPoolRankingTiebreaker(t *testing.T) {
    // ...
}

// REQ: FR-MATCH-03 (verify differential scoring)
func TestDifferentialScoring(t *testing.T) {
    // ...
}
```

#### Playwright E2E Tests
```typescript
test('REQ: US-SK-02, FR-MATCH-02 - Score keeper records exchange', async ({ page }) => {
    // ...
});

test('REQ: FR-TABLET-02 - Tablet crash recovery', async ({ page }) => {
    // ...
});
```

#### Tasks:
1. **Go Backend** (`internal/`):
   - `internal/hall` (snapshot, ranking, bracket, mats, people, forecast, staffing)
   - `internal/match` (scoring engine)
   - `internal/tournament` (pools, eliminations)
   - `cmd/porta` (main server and handlers)

2. **TypeScript Frontend** (`web/src/`):
   - Match scoring UI (`lib/pages/score.svelte`)
   - Pool standings (`lib/pages/standings.svelte`)
   - Display screens (`lib/pages/display/*.svelte`)
   - Match engine (`lib/match/`)

3. **Tests**:
   - Go unit tests (`*_test.go` in `internal/`)
   - TypeScript tests (`web/src/**/*.test.ts`)
   - Playwright E2E tests (`e2e/tests/`)

4. Run Lobster traceability scan:
   ```bash
   lobster-collect --output=lobster.json \
     req/schemas.trlc req/*.trlc \
     internal/*.go cmd/*.go \
     web/src/**/*.ts web/src/**/*.svelte \
     e2e/tests/*.ts
   lobster-html lobster.json
   ```

5. Fix coverage gaps (requirements with no implementation or tests)

---

### Phase 3: CI/CD Integration (Week 7)
**Goal**: Automate traceability checks in GitHub Actions

#### Workflow File (`.github/workflows/requirements.yml`):
```yaml
name: Requirements Validation

on: [push, pull_request]

jobs:
  requirements:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Python
        uses: actions/setup-python@v5
        with:
          python-version: '3.11'
      
      - name: Install TRLC and Lobster
        run: pip install trlc lobster
      
      - name: Compile TRLC Requirements
        run: trlc compile req/*.trlc --output=req/compiled.json
      
      - name: Collect Traceability Evidence
        run: |
          lobster-collect \
            --req-pattern='REQ:\s*([A-Z]+-[0-9]+)' \
            --req-pattern='lobster-req:\s*([A-Z]+-[0-9]+)' \
            --output=evidence.json \
            req/compiled.json \
            internal/ cmd/ web/src/ e2e/
      
      - name: Generate Coverage Report
        run: lobster-html evidence.json --output=coverage.html
      
      - name: Check Coverage
        run: |
          COVERAGE=$(python3 -c "import json; \
            data=json.load(open('evidence.json')); \
            covered = sum(1 for r in data.get('requirements', []) if r.get('traced')); \
            total = len(data.get('requirements', [])); \
            print(int(100 * covered / total))")
          echo "Requirement coverage: $COVERAGE%"
          if [ $COVERAGE -lt 80 ]; then
            echo "ERROR: Coverage below 80%"
            exit 1
          fi
      
      - name: Upload Coverage Report
        if: failure() || success()
        uses: actions/upload-artifact@v4
        with:
          name: requirements-coverage
          path: coverage.html
```

#### Tasks:
1. Create workflow file
2. Update PR template to link to coverage report
3. Document: "Coverage must be ≥80%; add annotations before merging"
4. Test workflow locally (act or similar)

---

### Phase 4: Documentation & Governance (Week 8)
**Goal**: Make requirements-as-code the source of truth

#### Tasks:
1. **Update README** (`README.md`):
   - Link to requirements documentation
   - Explain how to read the traceability matrix

2. **Create REQUIREMENTS.md**:
   - Quick reference: how many requirements, by type
   - Link to auto-generated coverage report
   - Glossary of requirement IDs

3. **Update CONTRIBUTING.md**:
   - Requirement annotation workflow
   - How to add a new requirement (edit TRLC, annotate code, write test, cite in PR)

4. **Archive old docs** (or repurpose):
   - `docs/design.md` becomes a summary with link to TRLC sources
   - `docs/architecture.md` links to `DC-*` constraints
   - Keep as reference but mark as "see also requirements/"

5. **Create onboarding guide**:
   - For new contributors: "Start with `req/` folder to understand what this component does"
   - Recommended reading order: schema → stories → functional → constraints

---

## 5. Maintenance & Governance

### Requirement Lifecycle

| Phase | Action | Owner | Tool |
|-------|--------|-------|------|
| **1. Propose** | Add `REQ-* draft` to `req/backlog.trlc` | Team discussion | TRLC |
| **2. Review** | Discussion in issue/PR | Maintainers | GitHub |
| **3. Approve** | Move to `req/*.trlc` and set `status: "ACTIVE"` | Maintainers | TRLC |
| **4. Implement** | Add code/test annotations | Developer | Lobster |
| **5. Verify** | CI passes coverage check | CI | Lobster |
| **6. Close** | Mark `status: "IMPLEMENTED"` after release | Maintainers | TRLC |

### Traceability Matrix Updates

**After each release:**
1. Run Lobster report
2. Identify any untested requirements
3. Add tests or close as N/A
4. Publish report to GitHub Pages or project wiki

**Before major changes:**
1. Query TRLC for affected requirements
2. List all touched code (via Lobster)
3. Plan test cases to verify unchanged behavior

---

## 6. File Structure (Post-Integration)

```
porta-di-ferro/
├── req/                           # Requirements directory
│   ├── schema.trlc                # TRLC schema definition
│   ├── stories.trlc               # User stories (US-*)
│   ├── functional.trlc            # Functional requirements (FR-*)
│   ├── non-functional.trlc        # Non-functional requirements (NFR-*)
│   ├── constraints.trlc           # Design constraints (DC-*)
│   ├── backlog.trlc               # Proposed/future requirements
│   ├── compiled.json              # Generated: compiled requirements
│   └── README.md                  # Guide to requirements structure
│
├── .github/
│   └── workflows/
│       └── requirements.yml       # TRLC + Lobster CI/CD
│
├── docs/
│   ├── REQUIREMENTS_AS_CODE.md   # How to use TRLC+Lobster
│   ├── design.md                 # (Updated: links to req/)
│   ├── architecture.md           # (Updated: links to req/)
│   └── CONTRIBUTING.md           # (Updated: annotation workflow)
│
├── .devcontainer/
│   └── devcontainer.json         # (Updated: add trlc, lobster)
│
└── [rest of repo unchanged]
```

---

## 7. Quick Reference: Annotation Syntax

### TRLC Requirement (in `req/functional.trlc`):
```trlc
REQUIREMENT FR-MATCH-02 "Match ends at 8 points or 3 minutes" {
  RATIONALE "Core scoring rule for HEMA tournaments"
  OWNER "Fencing Rules Committee"
  PRIORITY HIGH
  STATUS ACTIVE
  COMMENT "Part of MVP: implemented in both Go and TypeScript engines"
}
```

### Go Code Annotation:
```go
// REQ: FR-MATCH-02
func (m *Match) IsFinished() bool {
    return m.RedPoints >= 8 || m.BluePoints >= 8 || m.ElapsedTime >= 180*time.Second
}
```

### TypeScript Annotation:
```typescript
// REQ: FR-MATCH-02
export const isMatchFinished = (red: number, blue: number, elapsed: number) => {
  return red >= 8 || blue >= 8 || elapsed >= 180_000;
};
```

### Test Annotation:
```go
func TestMatchFinishesAt8Points(t *testing.T) {
    // REQ: FR-MATCH-02
    m := &Match{RedPoints: 8}
    assert.True(t, m.IsFinished())
}
```

---

## 8. Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|-----------|
| **Initial overhead**: Writing TRLC files takes time | Delays MVP | Start with Phase 0+1 in parallel with current work; don't block shipping |
| **Annotation drift**: Code and annotations diverge | Stale traceability | Lobster CI check catches gaps; add to definition of done |
| **Learning curve**: Team learns new syntax | Adoption slow | Start with examples; pair on first few PRs |
| **Tool maintenance**: TRLC/Lobster bugs or incompatibility | Broken CI | Monitor upstream; keep vendored copies if needed |
| **Over-requirement**: Adding too many low-value reqs | Noise | Be selective; focus on design rules and risky features |

---

## 9. Success Criteria

✅ **Phase 0 Complete**:
- TRLC schema defined
- Dev container updated
- CI/CD skeleton in place
- Team docs published

✅ **Phase 1 Complete**:
- 50+ requirements written (stories, functional, NFR, constraints)
- All requirements compile without errors
- Initial coverage report generated

✅ **Phase 2 Complete**:
- 80%+ of requirements traced to code
- 70%+ of requirements traced to tests
- Zero high-priority untested requirements

✅ **Phase 3 Complete**:
- CI pipeline enforces 80% coverage on every PR
- Build fails if requirements not annotated
- Coverage report auto-generated and published

✅ **Phase 4 Complete**:
- Traceability-driven development workflow documented
- New contributors use req/ folder as onboarding entry point
- Architecture docs link to requirements

✅ **Operational**:
- Requirements-as-code is source of truth
- Change impact visible via Lobster
- Release checklist includes requirement review

---

## 10. Next Steps

**Immediate (this week):**
1. ✅ Confirm team interest and assign phase owners
2. Create a GitHub issue to track the initiative
3. Set up discussion forum (GitHub Discussions or similar) for requirements questions

**Week 1-2:**
1. Create `req/` directory and schema
2. Update dev container
3. Create CI workflow skeleton

**Week 3-4:**
1. Extract requirements from docs
2. Write TRLC requirement files
3. Validate compilation

**Week 5-6:**
1. Annotate code (prioritize core scoring engine and pool ranking)
2. Annotate tests
3. Run Lobster and fix gaps

**Week 7-8:**
1. Activate CI checks
2. Document governance and workflow
3. Celebrate! 🎉

---

## References

- [TRLC GitHub](https://github.com/bmw-software-engineering/trlc)
- [Lobster GitHub](https://github.com/bmw-software-engineering/lobster)
- [Porta di Ferro Design](../docs/design.md)
- [Porta di Ferro Architecture](../docs/architecture.md)

---

**Document Version**: 1.0  
**Date**: 2026-11-19  
**Author**: Copilot (AI Assistant)  
**Status**: Proposal for review
