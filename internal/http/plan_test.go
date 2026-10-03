package httpapi_test

import (
	"fmt"
	"reflect"
	"testing"

	httpapi "github.com/fylke/porta-di-ferro/internal/http"
	"github.com/fylke/porta-di-ferro/internal/match"
	"github.com/fylke/porta-di-ferro/internal/store"
	"github.com/fylke/porta-di-ferro/internal/tournament"
)

// The event's mats and the work items on them (proposal §9, phase 2), as pure functions.

// pool is a pool of n matches on a lane, with the statuses given ("p"ending, "r"unning,
// "c"omplete) for its first matches and pending for the rest.
type pool struct {
	number, lane, matches int
	status                string
}

// discipline builds a snapshot by hand: what BuildSnapshot would hand the planner.
func discipline(slug string, stamp string, pools []pool, bracket []store.Match, filled map[string][2]string, done map[string]bool) httpapi.MatsInput {
	snap := httpapi.Snapshot{Tournament: store.Tournament{GeneratedAt: stamp, BracketAt: stamp + "-b"}}
	snap.Competitors = []store.Competitor{{ID: "a", Name: "Astrid"}, {ID: "b", Name: "Bo"}}
	for _, p := range pools {
		sp := store.Pool{Number: p.number, Mat: p.lane, Sequence: p.number}
		pv := httpapi.PoolView{Number: p.number, Mat: p.lane, Sequence: p.number}
		for i := 0; i < p.matches; i++ {
			m := store.Match{ID: fmt.Sprintf("p%dm%d", p.number, i+1), Pool: p.number, Order: i + 1, Mat: p.lane, Red: "a", Blue: "b"}
			sp.Matches = append(sp.Matches, m)
			status := "pending"
			if i < len(p.status) {
				status = map[byte]string{'p': "pending", 'r': "running", 'c': "complete"}[p.status[i]]
			}
			pv.Matches = append(pv.Matches, httpapi.MatchView{Match: m, Status: status, State: match.State{Ended: status == "complete"}})
		}
		snap.Tournament.Pools = append(snap.Tournament.Pools, sp)
		snap.Pools = append(snap.Pools, pv)
	}
	if len(bracket) > 0 {
		snap.Tournament.Bracket = bracket
		snap.Bracket = &httpapi.BracketView{}
		for _, m := range bracket {
			if f, ok := filled[m.ID]; ok {
				m.Red, m.Blue = f[0], f[1]
			}
			status := "pending"
			if done[m.ID] {
				status = "complete"
			}
			snap.Bracket.Matches = append(snap.Bracket.Matches, httpapi.MatchView{Match: m, Status: status})
		}
	}
	return httpapi.MatsInput{Slug: slug, Name: slug, Snapshot: snap}
}

func eightBracket() []store.Match {
	return []store.Match{
		{ID: "e-qf1", Round: tournament.RoundQuarter, Slot: 1, Mat: 1, Red: "a", Blue: "b"},
		{ID: "e-qf2", Round: tournament.RoundQuarter, Slot: 2, Mat: 2, Red: "a", Blue: "b"},
		{ID: "e-qf3", Round: tournament.RoundQuarter, Slot: 3, Mat: 1, Red: "a", Blue: "b"},
		{ID: "e-qf4", Round: tournament.RoundQuarter, Slot: 4, Mat: 2, Red: "a", Blue: "b"},
		{ID: "e-sf1", Round: tournament.RoundSemi, Slot: 1, Mat: 1, FeedRed: "winner:e-qf1", FeedBlue: "winner:e-qf2"},
		{ID: "e-sf2", Round: tournament.RoundSemi, Slot: 2, Mat: 2, FeedRed: "winner:e-qf3", FeedBlue: "winner:e-qf4"},
		{ID: "e-b", Round: tournament.RoundBronze, Slot: 1, Mat: 2, FeedRed: "loser:e-sf1", FeedBlue: "loser:e-sf2"},
		{ID: "e-f", Round: tournament.RoundFinal, Slot: 1, Mat: 1, FeedRed: "winner:e-sf1", FeedBlue: "winner:e-sf2"},
	}
}

func items(in ...httpapi.MatsInput) []httpapi.WorkItem {
	var out []httpapi.WorkItem
	for _, d := range in {
		out = append(out, httpapi.ItemsOf(d.Slug, d.Snapshot.Tournament)...)
	}
	return out
}

func TestADisciplinesWorkItems(t *testing.T) {
	ls := discipline("ls", "draw1", []pool{{1, 1, 3, ""}, {2, 2, 3, ""}, {3, 1, 3, ""}}, eightBracket(), nil, nil)
	got := []string{}
	for _, it := range httpapi.ItemsOf("ls", ls.Snapshot.Tournament) {
		got = append(got, fmt.Sprintf("%s@%d%v", it.Key, it.Lane, it.Matches))
	}
	want := []string{
		"pool-1@1[p1m1 p1m2 p1m3]", "pool-3@1[p3m1 p3m2 p3m3]", "pool-2@2[p2m1 p2m2 p2m3]",
		"elim-1@1[e-qf1 e-qf3 e-sf1]", "elim-2@2[e-qf2 e-qf4 e-sf2]",
		"bronze@2[e-b]", "final@1[e-f]",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("items:\n got %v\nwant %v", got, want)
	}
}

// One discipline on its own runs exactly as it did: every item on the mat its lane names,
// in the order it ran there.
func TestADisciplineAloneKeepsItsMats(t *testing.T) {
	ls := discipline("ls", "draw1", []pool{{1, 1, 3, ""}, {2, 2, 3, ""}, {3, 1, 3, ""}}, nil, nil, nil)
	its := items(ls)
	mats := httpapi.MatCount(store.Plan{}, []store.Tournament{{Mats: 2}})
	placed, changed := httpapi.Place(its, store.Plan{}, mats)
	if !changed || mats != 2 {
		t.Fatalf("a fresh plan should place everything, on the discipline's two mats (%d)", mats)
	}
	want := map[string]store.Placement{
		"ls/pool-1": {Mat: 1, Seq: 1, Stamp: "draw1"},
		"ls/pool-3": {Mat: 1, Seq: 2, Stamp: "draw1"},
		"ls/pool-2": {Mat: 2, Seq: 1, Stamp: "draw1"},
	}
	if !reflect.DeepEqual(placed, want) {
		t.Errorf("placements:\n got %v\nwant %v", placed, want)
	}
	again, changed := httpapi.Place(its, store.Plan{Items: placed}, mats)
	if changed || !reflect.DeepEqual(again, placed) {
		t.Error("placing what is already placed should change nothing")
	}
}

func TestPlacementsSurviveUntilTheDrawChanges(t *testing.T) {
	ls := discipline("ls", "draw1", []pool{{1, 1, 3, ""}, {2, 2, 3, ""}}, nil, nil, nil)
	moved := store.Plan{Items: map[string]store.Placement{
		"ls/pool-1": {Mat: 2, Seq: 1, Stamp: "draw1"},
		"ls/pool-2": {Mat: 2, Seq: 2, Stamp: "draw1"},
	}}
	placed, _ := httpapi.Place(items(ls), moved, 2)
	if placed["ls/pool-1"].Mat != 2 {
		t.Error("a pool the organizer moved should stay moved")
	}

	redrawn := discipline("ls", "draw2", []pool{{1, 1, 3, ""}, {2, 2, 3, ""}}, nil, nil, nil)
	placed, changed := httpapi.Place(items(redrawn), moved, 2)
	if !changed || placed["ls/pool-1"].Mat != 1 {
		t.Errorf("a redraw is new pools, placed afresh: %+v", placed)
	}

	// A mat taken away: what was on it goes to the end of the mat its lane maps to.
	placed, _ = httpapi.Place(items(ls), moved, 1)
	if placed["ls/pool-1"].Mat != 1 || placed["ls/pool-2"].Mat != 1 {
		t.Errorf("with one mat everything is on it: %+v", placed)
	}
}

func TestTwoDisciplinesShareTheMats(t *testing.T) {
	ls := discipline("ls", "d", []pool{{1, 1, 2, "cc"}, {2, 2, 2, "c"}}, nil, nil, nil)
	sa := discipline("sa", "d", []pool{{1, 1, 2, ""}}, nil, nil, nil)
	placed, _ := httpapi.Place(items(ls, sa), store.Plan{}, 2)
	if p := placed["sa/pool-1"]; p.Mat != 1 || p.Seq != 2 {
		t.Fatalf("Sabre's first pool should queue behind Longsword's on mat 1: %+v", p)
	}
	none := func(int) (string, string) { return "", "" }
	view := httpapi.BuildMats([]httpapi.MatsInput{ls, sa}, placed, 2, none)

	m1 := view.Mats[0]
	if m1.Current == nil || m1.Current.Discipline != "sa" || m1.Current.Match.ID != "p1m1" {
		t.Errorf("Longsword's pool on mat 1 is done, so mat 1 moves on to Sabre: %+v", m1.Current)
	}
	if len(m1.Queue) != 4 || m1.Queue[0].Discipline != "ls" || m1.Queue[3].Discipline != "sa" {
		t.Errorf("the queue should be Longsword's matches then Sabre's: %+v", m1.Queue)
	}
	if m1.Current.Red != "Astrid" || m1.Current.DisciplineName != "sa" {
		t.Errorf("a slot carries the names and the discipline: %+v", m1.Current)
	}
	m2 := view.Mats[1]
	if m2.Current == nil || m2.Current.Discipline != "ls" || m2.Current.Match.ID != "p2m2" {
		t.Errorf("mat 2 should be on Longsword pool 2's second match: %+v", m2.Current)
	}

	// The score keeper holding a finished match keeps it on the mat until Next match.
	held := func(mat int) (string, string) {
		if mat == 1 {
			return "ls", "p1m2"
		}
		return "", ""
	}
	view = httpapi.BuildMats([]httpapi.MatsInput{ls, sa}, placed, 2, held)
	if c := view.Mats[0].Current; c == nil || c.Discipline != "ls" || c.Match.ID != "p1m2" {
		t.Errorf("a held match stays on the mat, even into the next discipline's turn: %+v", c)
	}

	status := map[string]string{}
	for _, it := range view.Items {
		status[it.ID] = it.Status
	}
	if status["ls/pool-1"] != "done" || status["ls/pool-2"] != "running" || status["sa/pool-1"] != "ready" {
		t.Errorf("item statuses: %v", status)
	}
}

func TestTheBracketWaitsForItsFeeders(t *testing.T) {
	filled := map[string][2]string{}
	done := map[string]bool{"e-qf1": true, "e-qf3": true}
	ls := discipline("ls", "d", nil, eightBracket(), filled, done)
	// The quarter-finals in this snapshot have competitors; the rest wait on results.
	placed, _ := httpapi.Place(items(ls), store.Plan{}, 2)
	view := httpapi.BuildMats([]httpapi.MatsInput{ls}, placed, 2, func(int) (string, string) { return "", "" })
	m1 := view.Mats[0]
	if m1.Current != nil {
		t.Errorf("mat 1 has fenced its quarter-finals and the semi waits on mat 2: %+v", m1.Current)
	}
	if len(m1.Waiting) != 1 || m1.Waiting[0].Match.ID != "e-sf1" {
		t.Errorf("mat 1 should say it is waiting for the semi-final: %+v", m1.Waiting)
	}
	if c := view.Mats[1].Current; c == nil || c.Match.ID != "e-qf2" {
		t.Errorf("mat 2 is still on its quarter-finals: %+v", c)
	}
}

func TestMovingItems(t *testing.T) {
	ls := discipline("ls", "d", []pool{{1, 1, 2, "r"}, {2, 1, 2, ""}, {3, 1, 2, ""}, {4, 2, 2, ""}}, nil, nil, nil)
	placed, _ := httpapi.Place(items(ls), store.Plan{}, 2)
	none := func(int) (string, string) { return "", "" }
	view := httpapi.BuildMats([]httpapi.MatsInput{ls}, placed, 2, none)

	if _, _, err := httpapi.Move(view, placed, "ls/pool-1", 2, 0, 2); err == nil {
		t.Error("a pool under way must not move")
	}
	// To the front of mat 1 means behind the running pool, never before it.
	next, changed, err := httpapi.Move(view, placed, "ls/pool-3", 1, 0, 2)
	if err != nil || !changed {
		t.Fatalf("moving pool 3 up: %v %v", changed, err)
	}
	if next["ls/pool-1"].Seq != 1 || next["ls/pool-3"].Seq != 2 || next["ls/pool-2"].Seq != 3 {
		t.Errorf("pool 3 should land right behind the running pool: %+v", next)
	}
	// A drop where it already is writes nothing.
	if _, changed, _ := httpapi.Move(view, placed, "ls/pool-2", 1, 1, 2); changed {
		t.Error("dropping a pool where it is should change nothing")
	}
	// Across mats, at a position.
	next, _, _ = httpapi.Move(view, placed, "ls/pool-2", 2, 0, 2)
	if p := next["ls/pool-2"]; p.Mat != 2 || p.Seq != 1 || next["ls/pool-4"].Seq != 2 {
		t.Errorf("pool 2 should be first on mat 2: %+v", next)
	}
	if _, _, err := httpapi.Move(view, placed, "ls/pool-2", 3, 0, 2); err == nil {
		t.Error("there is no mat 3")
	}

	// The arrows: one step, and a step past the front is no move.
	next, _, _ = httpapi.Step(view, placed, "ls/pool-3", true, 2)
	if next["ls/pool-3"].Seq >= next["ls/pool-2"].Seq {
		t.Errorf("a step up should put pool 3 before pool 2: %+v", next)
	}
	if _, changed, _ := httpapi.Step(view, placed, "ls/pool-4", true, 2); changed {
		t.Error("the first item on a mat cannot step further up")
	}
}

// A discipline's snapshot names the event's mats, so every page that prints a mat is
// right without knowing there is a plan.
func TestASnapshotSpeaksTheEventsMats(t *testing.T) {
	ls := discipline("ls", "d", []pool{{1, 1, 2, ""}, {2, 2, 2, ""}}, eightBracket(), nil, nil)
	placed := map[string]store.Placement{
		"ls/pool-1": {Mat: 3, Seq: 1}, "ls/pool-2": {Mat: 1, Seq: 4},
		"ls/elim-1": {Mat: 2, Seq: 1}, "ls/final": {Mat: 4, Seq: 9},
	}
	snap := ls.Snapshot
	httpapi.OnEventMats(&snap, "ls", placed, 4)
	if snap.Pools[0].Number != 2 || snap.Pools[0].Mat != 1 || snap.Pools[1].Mat != 3 {
		t.Errorf("the pools should be on and ordered by the event's mats: %+v", snap.Pools)
	}
	if snap.Pools[1].Matches[0].Mat != 3 {
		t.Error("a pool's matches name its event mat")
	}
	mats := map[string]int{}
	for _, m := range snap.Bracket.Matches {
		mats[m.ID] = m.Mat
	}
	if mats["e-qf1"] != 2 || mats["e-sf1"] != 2 || mats["e-f"] != 4 {
		t.Errorf("bracket matches should name their items' mats: %v", mats)
	}
	if snap.EventMats != 4 || len(snap.Mats) != 0 {
		t.Errorf("EventMats should say 4 and the discipline's own Mats be emptied: %d %v", snap.EventMats, snap.Mats)
	}
}

// The day planned before the draw is the drawn day's: a projected item's placement, which
// has no stamp, is taken over by the real item of the same name (phase 4).
func TestThePlannedDaySurvivesTheDraw(t *testing.T) {
	ls := discipline("ls", "drawn", []pool{{1, 1, 2, ""}, {2, 2, 2, ""}}, nil, nil, nil)
	plan := store.Plan{Items: map[string]store.Placement{
		"ls/pool-1": {Mat: 2, Seq: 1, Pinned: true, NotBefore: "10:00"},
		"ls/pool-2": {Mat: 1, Seq: 1},
	}}
	placed, changed := httpapi.Place(items(ls), plan, 2)
	if !changed {
		t.Error("taking over a projected placement should be written down")
	}
	if p := placed["ls/pool-1"]; p.Mat != 2 || p.Stamp != "drawn" || !p.Pinned || p.NotBefore != "10:00" {
		t.Errorf("pool 1 should stay where it was planned, pinned and held: %+v", p)
	}
}

// A card moved by hand is pinned, so a suggestion keeps it on that mat.
func TestAMoveByHandPins(t *testing.T) {
	ls := discipline("ls", "d", []pool{{1, 1, 2, ""}, {2, 1, 2, ""}}, nil, nil, nil)
	placed, _ := httpapi.Place(items(ls), store.Plan{}, 2)
	view := httpapi.BuildMats([]httpapi.MatsInput{ls}, placed, 2, func(int) (string, string) { return "", "" })
	next, _, _ := httpapi.Move(view, placed, "ls/pool-2", 2, 0, 2)
	if !next["ls/pool-2"].Pinned || next["ls/pool-1"].Pinned {
		t.Errorf("only the moved item should be pinned: %+v", next)
	}
}

// Before the draw a discipline has the work the draw would give it, under the names the
// real items will have (phase 4, planning mode).
func TestUndrawnWorkIsProjected(t *testing.T) {
	tn := store.Defaults()
	tn.Mats, tn.MinPoolSize, tn.MaxPoolSize = 2, 5, 7
	got := []string{}
	for _, it := range httpapi.ProjectedItems("sa", tn, 14) {
		if !it.Projected || it.Stamp != "" {
			t.Errorf("a projected item should say so and have no stamp: %+v", it)
		}
		got = append(got, fmt.Sprintf("%s@%d:%d", it.Key, it.Lane, len(it.Matches)))
	}
	want := []string{"pool-1@1:21", "pool-2@2:21", "elim-1@1:3", "elim-2@2:3", "bronze@2:1", "final@1:1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fourteen entrants should project two pools of seven and a bracket of eight:\n got %v\nwant %v", got, want)
	}
	if n := len(httpapi.ProjectedItems("sa", tn, 1)); n != 0 {
		t.Errorf("one entrant is nothing to plan, got %d items", n)
	}

	// Drawn pools leave only the bracket projected; a drawn bracket leaves nothing.
	ls := discipline("ls", "d", []pool{{1, 1, 3, ""}}, nil, nil, nil)
	if its := httpapi.ProjectedItems("ls", ls.Snapshot.Tournament, 2); len(its) != 1 || its[0].Key != "final" {
		t.Errorf("with the pools drawn only the bracket is still to come: %+v", its)
	}
	ls = discipline("ls", "d", []pool{{1, 1, 3, ""}}, eightBracket(), nil, nil)
	if its := httpapi.ProjectedItems("ls", ls.Snapshot.Tournament, 8); len(its) != 0 {
		t.Errorf("with everything drawn nothing is projected: %+v", its)
	}
}

// A mat never waits on work that is not drawn yet: a projected pool ahead of a drawn one
// is planned time, not the mat's current match.
func TestAMatDoesNotWaitOnProjectedWork(t *testing.T) {
	ls := discipline("ls", "d", []pool{{1, 1, 2, ""}}, eightBracket(), nil, nil)
	sa := ls
	sa.Slug, sa.Name = "sa", "sa"
	sa.Snapshot.Tournament = store.Tournament{Mats: 1, MinPoolSize: 4, MaxPoolSize: 7}
	sa.Snapshot.Pools, sa.Snapshot.Bracket = nil, nil
	sa.Expected = 6
	plan := store.Plan{Items: map[string]store.Placement{"sa/pool-1": {Mat: 1, Seq: 1}}}
	all := append(items(ls), httpapi.PlanItems("sa", sa.Snapshot.Tournament, 6)...)
	placed, _ := httpapi.Place(all, plan, 2)
	view := httpapi.BuildMats([]httpapi.MatsInput{ls, sa}, placed, 2, func(int) (string, string) { return "", "" })
	if cur := view.Mats[0].Current; cur == nil || cur.Discipline != "ls" {
		t.Errorf("mat 1 should be on the longsword's pool, not waiting for the sabre's draw: %+v", cur)
	}
	planned := 0
	for _, it := range view.Items {
		if it.Discipline == "sa" {
			planned++
			if !it.Projected || it.Status != "planned" || !it.Movable || it.Total == 0 {
				t.Errorf("a projected item is planned, movable and counts its matches: %+v", it)
			}
		}
	}
	if planned == 0 {
		t.Error("the sabre's projected work should be on the board")
	}
}

// Only what the organizer placed survives the draw; the projection's own guesses give way
// to the real draw's lanes.
func TestTheProjectionsOwnPlacementsGiveWay(t *testing.T) {
	ls := discipline("ls", "drawn", []pool{{1, 1, 2, ""}, {2, 2, 2, ""}}, nil, nil, nil)
	plan := store.Plan{Items: map[string]store.Placement{"ls/pool-2": {Mat: 1, Seq: 1}}}
	placed, _ := httpapi.Place(items(ls), plan, 2)
	if p := placed["ls/pool-2"]; p.Mat != 2 || p.Stamp != "drawn" {
		t.Errorf("pool 2 should go to its drawn lane, mat 2: %+v", p)
	}
}
