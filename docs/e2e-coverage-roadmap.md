# E2E Test Coverage Roadmap

**Goal**: Extend E2E test suite to cover all documented use cases across all personas, prioritized by impact and effort.

**Current Coverage**: ~50-60% of use cases have explicit E2E tests.

---

## Prioritized Test Gaps

### Phase 1: Critical (Score Keeper) — Highest Priority
*Impact: Scoring is core to tournament operation. These are user-facing features with complex state transitions.*

#### 1.1 `test-score-keeper-timer-control` — Timer Start/Pause/Reset
- **Why**: Timer is essential for match timing. No E2E test verifies timer works at all.
- **Scenario**: 
  - Start match → Click timer start button → Verify timer increments
  - Pause timer → Verify timer stops incrementing  
  - Resume timer → Verify timer continues from pause point
  - Reset timer → Verify timer returns to 00:00
  - Verify timer affects match duration and final time
- **Files to test**: `web/src/routes/score/[mat]/+page.svelte`, timer component
- **Effort**: Medium (requires Playwright timer mocking/assertions)
- **Backend dependency**: Existing match scoring works, needs UI verification

#### 1.2 `test-score-keeper-points-and-warnings` — Record Points and Issue Warnings
- **Why**: Core scoring actions. No E2E test verifies points and warnings are recorded.
- **Scenario**:
  - Click "+1" for Red → Verify 1 point appears and is synced
  - Click "+1" for Blue → Verify 1 point appears for Blue  
  - Issue warning to Red → Verify warning indicator appears
  - Issue second warning to Red → Verify match ends (penalty cap)
  - Issue warning to Blue → Verify independent from Red's warnings
  - Undo last warning → Verify warning removed
  - Offline: Record points offline → Go online → Verify sync to server
- **Files to test**: Score keeper form component, point recording, warning display
- **Effort**: High (complex state, multiple interactions, offline sync)
- **Backend dependency**: Backend scoring already tested; UI/sync needs coverage

#### 1.3 `test-score-keeper-final-exchange-decision` — Decide Exchange Outcome
- **Why**: Final exchanges determine who wins close matches. No test verifies decision UI works.
- **Scenario**:
  - Match timer expires → Exchange button appears
  - Click "Red wins exchange" → Red gets +1 point
  - Click "Blue wins exchange" → Blue gets +1 point
  - Click "Draw (no points)" → Both stay tied
  - Verify decision is recorded in match log
  - Verify match proceeds/ends based on decision
- **Files to test**: Exchange decision dialog/form
- **Effort**: Medium (single decision event, but state-dependent)
- **Backend dependency**: Backend handles decision logic; UI needs test

#### 1.4 `test-score-keeper-point-cap-and-penalty-cap` — Cap Decisions
- **Why**: These are hard stops that end matches. No test verifies UI prevents invalid states.
- **Scenario**:
  - Red reaches point cap (12 pts) → Cannot add more points
  - Blue at 2 warnings → Next warning auto-ends match
  - Verify cap warning message appears
  - Verify match cannot proceed beyond cap
  - Verify undo removes the cap limit
- **Files to test**: Score keeper score display, cap enforcement UI
- **Effort**: Medium (mostly state display, existing backend logic)
- **Backend dependency**: Backend already enforces; UI needs to show state correctly

#### 1.5 `test-score-keeper-forfeit-and-disqualification` — Record Forfeit/DQ
- **Why**: Ending matches early. No test verifies UI for these critical outcomes.
- **Scenario**:
  - Click Forfeit button → Confirm dialog appears
  - Confirm forfeit → Match ends, opponent marked winner
  - Click Disqualification → Confirm dialog
  - Confirm DQ → Match ends, opponent marked winner
  - Verify decision recorded in match log
  - Verify forfeit/DQ shown in standings
- **Files to test**: Match end actions dialog
- **Effort**: Medium (dialog interaction + verification)
- **Backend dependency**: Backend already handles; UI needs test

#### 1.6 `test-score-keeper-undo-history` — Undo Full Workflow
- **Why**: Backend already tested (TestUndoIsAppendedNotMutated), but UI workflow untested.
- **Scenario**:
  - Score Red +3 points → Click Undo → Points removed
  - Issue warning → Undo → Warning removed
  - Multiple undos in sequence → Verify order is correct
  - Offline: Score offline → Undo offline → Sync up with undo included
  - Verify undo button disabled when at start of match
  - Verify undo button re-enabled after each action
- **Files to test**: Undo button/list UI
- **Effort**: Low (mostly assertion on existing backend behavior)
- **Backend dependency**: Backend tested; UI just needs assertion

---

### Phase 2: High Impact (Event Organizer) — High Priority
*Impact: Event setup is performed once per tournament, but mistakes cascade. Forecast is used by all roles.*

#### 2.1 `test-organizer-draw-bracket` — Draw Bracket UI
- **Why**: Backend tested (eliminations), but bracket drawing UI not tested.
- **Scenario**:
  - Pools finished, standings ready
  - Click "Draw Bracket" button
  - Select bracket size/configuration if applicable
  - Verify bracket appears correctly
  - Verify bracket structure matches pool results
  - Verify bracket persists on reload
- **Files to test**: Bracket drawing UI/page
- **Effort**: High (new bracket UI needs full workflow test)
- **Backend dependency**: Backend already works; UI needs test

#### 2.2 `test-organizer-forecast-and-timings` — View & Apply Forecast Suggestions
- **Why**: Forecasting is critical for schedule accuracy. No test verifies UI suggestions work.
- **Scenario**:
  - Run some matches → Forecasted times appear
  - Forecast shows mat availability, expected match duration
  - View forecast for each mat
  - Verify suggested times appear
  - Apply forecast suggestion → Mat reassigned if needed
  - Verify staff assignments account for new schedule
  - Check if forecast warnings appear when schedule is tight
- **Files to test**: Forecast UI/suggestion apply
- **Effort**: High (complex state, dependent on match progress)
- **Backend dependency**: Backend forecasting tested; UI for suggestions needs test

#### 2.3 `test-organizer-staff-assignment` — Assign Staff to Roles
- **Why**: Staff are assigned to referees/scorekeepers. Currently only tested via signup import.
- **Scenario**:
  - View staff list
  - Assign staff member to "referee" role
  - Assign staff member to "scorekeeper" role
  - Verify assignment persists
  - Verify staff cannot be assigned to conflicting roles/times
  - Remove staff assignment
  - Verify UI prevents double-booking (conflicts)
- **Files to test**: Staff assignment UI
- **Effort**: High (new UI workflow)
- **Backend dependency**: Backend staff tracking exists; UI workflow needs test

#### 2.4 `test-organizer-mat-movement` — Move Matches Between Mats
- **Why**: Backend tested (TestOrganizerOverridesTheMatAssignment), but full movement UI untested.
- **Scenario**:
  - Match assigned to Mat 1
  - Click "Move to Mat 2" or drag match
  - Verify match moves in real-time display
  - Score keeper on old mat sees match gone
  - Score keeper on new mat sees match appear
  - Verify move is tracked in history
  - Verify offline score keeper syncs and finds match on new mat
- **Files to test**: Mat management/assignment UI
- **Effort**: Medium (backend works; UI/real-time sync needs test)
- **Backend dependency**: Backend handles move; UI update verification needed

#### 2.5 `test-organizer-event-setup-creation` — Create & Configure Event
- **Why**: Event setup is crucial. TestSimulatedEvent covers it programmatically, not UI.
- **Scenario**:
  - Start from scratch
  - Enter event name and date
  - Create new discipline
  - Add mats (count, timings)
  - Verify event JSON created with correct structure
  - Reload → Event persists with same config
  - Rename event → Verify name updates everywhere
- **Files to test**: Event creation/setup pages
- **Effort**: High (new E2E for UI-driven setup)
- **Backend dependency**: Backend already works; UI workflow verification needed

---

### Phase 3: High Impact (Competitor) — High Priority
*Impact: Competitors should see accurate info. Zero current coverage means broken competitor flow wouldn't be caught.*

#### 3.1 `test-competitor-view-schedule` — View My Schedule
- **Why**: Competitors need to know when they compete. No E2E coverage for competitor views.
- **Scenario**:
  - Log in as competitor (or view /competitor/{id} page)
  - Verify "My Schedule" shows all assigned matches
  - Verify schedule updates after draw
  - Verify schedule updates after mat moves
  - Verify forecast times are shown
  - Verify schedule shows in correct order (chronological)
- **Files to test**: Competitor schedule page
- **Effort**: Medium (read-only view, backend already serves data)
- **Backend dependency**: Backend endpoint exists; UI rendering needs test

#### 3.2 `test-competitor-view-pool-standings` — View My Pool & Standings
- **Why**: Competitors check standings during tournament. No test verifies they see correct data.
- **Scenario**:
  - In pool phase → Competitor sees their pool standings
  - Verify their position and points are correct
  - Verify standings update after each match in their pool
  - Verify tie-break rules applied correctly (shown in UI)
  - Verify standings show before/after pool ends
- **Files to test**: Competitor pool standings page
- **Effort**: Medium (read-only, backend tested; UI needs assertion)
- **Backend dependency**: Standing calculation tested; display needs test

#### 3.3 `test-competitor-view-results` — View My Results & Opponents
- **Why**: Competitors want to know match outcomes. No test verifies result display.
- **Scenario**:
  - After match completes → Competitor sees result (W/L/D)
  - See opponent's name and their club
  - See final score
  - See if result affected ranking
  - Verify results persist and are consistent across page reloads
- **Files to test**: Competitor results page
- **Effort**: Low (read-only view, mostly assertions)
- **Backend dependency**: Backend serves correct data; UI needs test

#### 3.4 `test-competitor-track-bracket-position` — Bracket Position (Eliminations)
- **Why**: Competitors want to track their path through bracket. Zero coverage.
- **Scenario**:
  - Bracket draws → Competitor sees their starting position
  - Win first match → Competitor advances in bracket UI
  - Lose → Competitor placed correctly (consolation/out)
  - Verify bracket UI matches actual standings
  - Verify advancement/placement is correct per rules
- **Files to test**: Competitor bracket view page
- **Effort**: Medium (read-only, but complex bracket rendering)
- **Backend dependency**: Backend bracket logic tested; UI rendering needs test

#### 3.5 `test-competitor-forecast-times` — Check Forecasted Match Times
- **Why**: Competitors plan around predicted match times. No test verifies they see forecast.
- **Scenario**:
  - Forecast available (matches running)
  - Competitor views their next match
  - Sees forecasted start time
  - Forecast updates as schedule progresses
  - Verify forecast accuracy improves as matches complete
- **Files to test**: Competitor match detail / forecast display
- **Effort**: Medium (requires forecast state; backend tested)
- **Backend dependency**: Backend forecast works; UI display needs test

---

### Phase 4: Medium Impact (Audience & Display) — Medium Priority
*Impact: Display is used by everyone at venue, but failures are less critical than scoring.*

#### 4.1 `test-audience-current-scoreboard` — View & Update Scoreboard Live
- **Why**: Live scoreboard is what audience sees. No explicit test verifies real-time updates.
- **Scenario**:
  - Display page open showing current scoreboard
  - Score update on scorekeeper device
  - Display updates in real-time (verify via WebSocket/SSE)
  - Multiple updates (3-4 points) in sequence
  - Verify updates are correct and in order
  - Verify display shows correct match (mat + teams)
  - Offline display → Goes back online → Catches up
- **Files to test**: Audience scoreboard page, real-time update verification
- **Effort**: High (requires real-time event synchronization test)
- **Backend dependency**: Backend SSE/real-time works; display update needs assertion

#### 4.2 `test-audience-programme-and-schedule` — View Programme & Schedule
- **Why**: Audience needs to know when matches happen. No test verifies schedule display.
- **Scenario**:
  - Display page showing programme
  - Verify all mats are listed
  - Verify schedule is chronological
  - Verify current match is highlighted/marked
  - Verify next matches preview
  - Verify schedule updates after mat moves
  - Verify forecast times shown
- **Files to test**: Audience programme/schedule display page
- **Effort**: Medium (mostly read-only state assertion)
- **Backend dependency**: Backend schedule works; display needs test

#### 4.3 `test-audience-podium-and-rankings` — View Final Podium & Rankings
- **Why**: Audience sees results at end. No test verifies podium display.
- **Scenario**:
  - Tournament complete → Podium page available
  - Verify 1st, 2nd, 3rd place shown correctly
  - Verify tie-break rules applied (podium reflects correct ranking)
  - Verify full standings shown below podium
  - Verify names/clubs displayed correctly
  - Verify trophy/medal icons for top 3
  - Verify podium matches final tournament standings
- **Files to test**: Podium/results display page
- **Effort**: Low (read-only state assertion, backend tested)
- **Backend dependency**: Backend ranking tested; display needs assertion

---

## Recommended Phasing Strategy

### **Sprint 1: Score Keeper Critical Path** (Weeks 1-2)
Focus on scoring workflow—the most critical user path.
1. ✅ Timer Control (1.1) — _Medium effort, enables all other scoring tests_
2. ✅ Points & Warnings (1.2) — _High effort, highest value_
3. ✅ Undo History (1.6) — _Low effort, quick win_
4. ✅ Final Exchange (1.3) — _Medium effort_
5. ✅ Cap Decisions (1.4) — _Medium effort_
6. ✅ Forfeit/DQ (1.5) — _Medium effort_

**Expected Result**: Complete score keeper workflow tested end-to-end.

### **Sprint 2: Organizer Event Setup** (Weeks 3-4)
Focus on event configuration—needed before scoring can happen.
1. ✅ Event Setup Creation (2.5) — _High effort, foundational_
2. ✅ Bracket Drawing (2.1) — _High effort_
3. ✅ Mat Movement (2.4) — _Medium effort_
4. ✅ Forecast & Timings (2.2) — _High effort, complex_
5. ✅ Staff Assignment (2.3) — _High effort_

**Expected Result**: Event organizer full workflow tested.

### **Sprint 3: Competitor Visibility** (Weeks 5-6)
Focus on competitor-facing pages—currently zero coverage.
1. ✅ View Schedule (3.1) — _Medium effort_
2. ✅ Pool Standings (3.2) — _Medium effort_
3. ✅ View Results (3.3) — _Low effort, quick win_
4. ✅ Bracket Position (3.4) — _Medium effort_
5. ✅ Forecast Times (3.5) — _Medium effort_

**Expected Result**: Competitor views fully tested.

### **Sprint 4: Audience Display** (Week 7+)
Focus on public-facing displays—important but lower priority than core workflows.
1. ✅ Current Scoreboard (4.1) — _High effort, requires real-time test harness_
2. ✅ Programme & Schedule (4.2) — _Medium effort_
3. ✅ Podium & Rankings (4.3) — _Low effort, quick win_

**Expected Result**: Audience views fully tested; real-time updates verified.

---

## Implementation Guidelines

### Test Structure
Each test should follow this pattern:
```typescript
test.describe('Feature Name', () => {
  test('complete user workflow', async ({ page, request, context }) => {
    // 1. Setup: Create event, draw pools, etc. via API calls
    // 2. Navigate: Go to feature page
    // 3. Interact: Perform user actions
    // 4. Assert: Verify UI shows correct state
    // 5. Sync: If offline capable, test offline → online sync
    // 6. Persist: Reload page, verify state persists
  });
});
```

### Offline Testing
For score keeper tests: each should include offline scenario:
```typescript
// Offline phase
await context.setOffline(true);
// ... perform actions ...
await context.setOffline(false);
// Verify sync succeeds and UI updates
```

### Real-Time Updates
For display/audience tests: mock/verify SSE or WebSocket updates:
```typescript
// Wait for real-time update
const scoreUpdated = page.waitForEvent('response', 
  r => r.url().includes('/api/matches') && r.ok()
);
// Trigger score update from another context
// Verify UI updated
await scoreUpdated;
```

### Cross-Device Verification
For multi-device scenarios (score keeper + display):
```typescript
// Use separate browser contexts
const scoreKeeperContext = await browser.newContext();
const displayContext = await browser.newContext();
// Open both
// Update in scorekeeper context
// Assert change in display context
```

---

## Coverage Target

| Persona | Use Cases | Current | Target | Gap |
|---------|-----------|---------|--------|-----|
| Event Organizer | 11 | 6 | 11 | 5 |
| Score Keeper | 12 | 4 | 12 | 8 |
| Competitor | 5 | 0 | 5 | 5 |
| Audience | 6 | 3 | 6 | 3 |
| **Total** | **34** | **13** | **34** | **21** |

**After roadmap**: All 34 use cases will have explicit E2E tests.

---

## Estimated Effort

| Phase | Effort | Duration | Tests |
|-------|--------|----------|-------|
| Phase 1: Score Keeper | 60 hours | 2 weeks | 6 tests |
| Phase 2: Organizer Setup | 70 hours | 2 weeks | 5 tests |
| Phase 3: Competitor | 40 hours | 2 weeks | 5 tests |
| Phase 4: Audience | 35 hours | 1 week | 3 tests |
| **Total** | **205 hours** | **~7 weeks** | **19 tests** |

---

## Success Criteria

- ✅ All 34 persona use cases covered by at least 1 explicit E2E test
- ✅ Score keeper workflow (scoring, timer, decisions) testable end-to-end
- ✅ Competitor views show correct data under various tournament states
- ✅ Offline → online sync tested for score keeper path
- ✅ Real-time updates (scoreboard) verified
- ✅ All tests pass on main branch continuously
- ✅ Test coverage dashboard shows >80% persona coverage

---

## Notes

- **Backend scoring is well-tested**: Most gaps are UI/integration tests, not business logic.
- **Offline capability is critical**: Most gaps should include offline scenarios.
- **Real-time is complex**: Scoreboard real-time updates need special test harness (may require new infrastructure).
- **Competitor views are low-risk**: Backend already tested, UI mostly assertion.
- **Consider test maintenance**: Each test should be ~100-200 lines, well-commented, using page objects for reusability.
