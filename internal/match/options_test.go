package match_test

import (
	"testing"

	"github.com/fylke/porta-di-ferro/internal/match"
)

func TestOptionsOfLastRecordWinsAndDefaultsFillIn(t *testing.T) {
	events := []match.Event{
		{Seq: 1, Type: match.TypeTimer, Timer: &match.Timer{Action: match.TimerStart}},
		{Seq: 2, Type: match.TypeOptions, Options: &match.Options{Red: "green", Blue: "yellow"}},
		{Seq: 3, Type: match.TypeOptions, Options: &match.Options{Blue: "white", SwapDisplay: true}},
	}
	got := match.OptionsOf(events)
	want := match.Options{Red: "green", Blue: "white", SwapDisplay: true}
	if got != want {
		t.Errorf("OptionsOf = %+v, want %+v", got, want)
	}
	if def := match.OptionsOf(nil); def != match.DefaultOptions() {
		t.Errorf("an empty log should have the defaults, got %+v", def)
	}
}

// TestOptionsDoNotTouchTheScore is the property the shared vector states for both engines,
// checked here directly as well so a change to Replay cannot quietly start reading them.
func TestOptionsDoNotTouchTheScore(t *testing.T) {
	base := []match.Event{
		{Seq: 1, Type: match.TypeExchange, ElapsedMS: 1000,
			Exchange: &match.Exchange{Red: match.Assessment{Value: 2}}},
	}
	with := append(append([]match.Event{}, base...),
		match.Event{Seq: 2, Type: match.TypeOptions, Options: &match.Options{Red: "green", SwapDisplay: true}})
	a, b := match.Replay(match.MSL(), base), match.Replay(match.MSL(), with)
	b.LastSeq = a.LastSeq
	if a != b {
		t.Errorf("an options record changed the derived state:\nwithout %+v\nwith    %+v", a, b)
	}
}
