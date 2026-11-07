package hall

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/forecast"
	"github.com/fylke/porta-di-ferro/internal/store"
)

// The plan's settings as the planning panel writes them: the template, the breaks, each
// discipline's preparation and start, and the blocks of the day.

// DefaultTimings is the built-in template, in the plan's terms.
var DefaultTimings = store.Timings{
	Match:       int(forecast.Defaults.Match / time.Second),
	Changeover:  int(forecast.Defaults.Changeover / time.Second),
	BeforeElims: int(forecast.Defaults.BeforeElims / time.Second),
	Start:       "09:00",
}

// TimingsOr fills what t leaves out from fallback: the event's own timings, over the
// organizer's default for new events, over the built-in template.
func TimingsOr(t, fallback store.Timings) store.Timings {
	if t.Match <= 0 {
		t.Match = fallback.Match
	}
	if t.Changeover <= 0 {
		t.Changeover = fallback.Changeover
	}
	if t.BeforeElims <= 0 {
		t.BeforeElims = fallback.BeforeElims
	}
	if t.Start == "" {
		t.Start = fallback.Start
	}
	if t.Close == "" {
		t.Close = fallback.Close
	}
	return t
}

// CleanTimings checks what the planning panel sent.
func CleanTimings(t store.Timings) (store.Timings, error) {
	t.Start, t.Close = strings.TrimSpace(t.Start), strings.TrimSpace(t.Close)
	for _, s := range []string{t.Start, t.Close} {
		if _, ok := ParseClock(time.Now(), s); s != "" && !ok {
			return t, fmt.Errorf("%q is not a time of day; write it as 09:30", s)
		}
	}
	if t.Match < 0 || t.Match > 3600 || t.Changeover < 0 || t.Changeover > 3600 || t.BeforeElims < 0 || t.BeforeElims > 4*3600 {
		return t, fmt.Errorf("those timings are out of range")
	}
	return t, nil
}

// ParseClock is a time of day, "09:30", on the given day. Anything else -- "after the
// pools" is a legitimate programme row -- is not a time.
func ParseClock(day time.Time, s string) (time.Time, bool) {
	var h, m int
	if n, err := fmt.Sscanf(strings.TrimSpace(s), "%d:%d", &h, &m); err != nil || n != 2 || h < 0 || h > 23 || m < 0 || m > 59 {
		return time.Time{}, false
	}
	y, mo, d := day.Date()
	return time.Date(y, mo, d, h, m, 0, 0, day.Location()), true
}

// BreaksOf is the hall's breaks: the plan's (#149), or -- for an event planned before the
// plan held them -- the break rows of the programme the organizer used to type.
func BreaksOf(plan store.Plan, ev store.Event) []store.Break {
	if len(plan.Breaks) > 0 {
		return plan.Breaks
	}
	var out []store.Break
	for _, row := range ev.Schedule {
		if row.Kind == "break" && row.Ends != "" {
			out = append(out, store.Break{Label: row.Label, From: row.At, To: row.Ends})
		}
	}
	return out
}

// maxBreaks caps the hall's breaks: a day with more stops than this is not a plan.
const maxBreaks = 10

// CleanBreaks checks the hall's breaks: times of day, each ending after it starts, in the
// order of the day. A break with no name is lunch, the one every day has.
func CleanBreaks(in []store.Break) ([]store.Break, error) {
	if len(in) > maxBreaks {
		return nil, fmt.Errorf("a day with more than %d breaks is not one anybody plans", maxBreaks)
	}
	now := time.Now()
	out := make([]store.Break, 0, len(in))
	for _, b := range in {
		b.Label, b.From, b.To = strings.TrimSpace(b.Label), strings.TrimSpace(b.From), strings.TrimSpace(b.To)
		from, ok1 := ParseClock(now, b.From)
		to, ok2 := ParseClock(now, b.To)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("a break needs a start and an end, written as 12:00")
		}
		if !to.After(from) {
			return nil, fmt.Errorf("the break from %s should end after it starts", b.From)
		}
		if b.Label == "" {
			b.Label = "Lunch"
		}
		if len(b.Label) > 60 {
			return nil, fmt.Errorf("a break's name is a word or two, not %d characters", len(b.Label))
		}
		out = append(out, b)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].From < out[j].From })
	return out, nil
}

// DefaultPrep is a discipline's gear check and staff briefing until the organizer says
// otherwise (#153): a quarter of an hour each, one after the other.
var DefaultPrep = store.Prep{GearCheck: 15 * 60, Briefing: 15 * 60}

// PrepOf is a discipline's gear check and staff briefing: the plan's, or the default.
func PrepOf(plan store.Plan, slug string) store.Prep {
	if p, ok := plan.Prep[slug]; ok {
		return p
	}
	return DefaultPrep
}

// CleanPrep checks a discipline's gear check and staff briefing: up to four hours each.
func CleanPrep(p store.Prep) (store.Prep, error) {
	if p.GearCheck < 0 || p.GearCheck > 4*3600 || p.Briefing < 0 || p.Briefing > 4*3600 {
		return p, fmt.Errorf("a gear check or a briefing takes between nothing and four hours")
	}
	return p, nil
}

// CleanStart checks when a discipline starts at the earliest: a time of day, or blank for
// whenever the plan reaches it.
func CleanStart(s string) (string, error) {
	s = strings.TrimSpace(s)
	if _, ok := ParseClock(time.Now(), s); s != "" && !ok {
		return s, fmt.Errorf("%q is not a time of day; write it as 09:30", s)
	}
	return s, nil
}

// Sessions is the block of the day each discipline runs in (#136): the organizer's, or,
// for a discipline not given one, a block of its own in the event's order -- one
// discipline after another.
func Sessions(plan store.Plan, slugs []string) map[string]int {
	out := map[string]int{}
	for i, slug := range slugs {
		out[slug] = i + 1
		if n := plan.Sessions[slug]; n > 0 {
			out[slug] = n
		}
	}
	return out
}

// OrderOf is the order of the day for placing work (#136).
func OrderOf(plan store.Plan, slugs []string, started func(WorkItem) bool) PlaceOrder {
	sessions := Sessions(plan, slugs)
	position := map[string]int{}
	for i, s := range slugs {
		position[s] = i
	}
	return PlaceOrder{
		Session:    func(slug string) int { return sessions[slug] },
		Position:   func(slug string) int { return position[slug] },
		FinalsLast: plan.FinalsLast,
		Started:    started,
	}
}

// CleanMat checks one mat's setting from the planning panel.
func CleanMat(m store.MatSetting) (store.MatSetting, error) {
	m.Name = strings.TrimSpace(m.Name)
	if len([]rune(m.Name)) > 30 {
		return m, fmt.Errorf("a mat's name is at most 30 characters")
	}
	day := time.Now()
	for i, a := range m.Away {
		from, ok1 := ParseClock(day, a.From)
		to, ok2 := ParseClock(day, a.To)
		if !ok1 || !ok2 || !to.After(from) {
			return m, fmt.Errorf("away from %q to %q: two times of day, the second later", a.From, a.To)
		}
		m.Away[i] = store.Away{From: strings.TrimSpace(a.From), To: strings.TrimSpace(a.To)}
	}
	return m, nil
}

// SetMat sets one mat's name and away times in the plan.
func SetMat(plan *store.Plan, mat int, m store.MatSetting) {
	for len(plan.MatSettings) < mat {
		plan.MatSettings = append(plan.MatSettings, store.MatSetting{})
	}
	plan.MatSettings[mat-1] = m
	// Nothing set past the last named mat need be kept.
	for len(plan.MatSettings) > 0 {
		last := plan.MatSettings[len(plan.MatSettings)-1]
		if last.Name != "" || len(last.Away) > 0 {
			break
		}
		plan.MatSettings = plan.MatSettings[:len(plan.MatSettings)-1]
	}
}

// SetPrep writes the gear check and staff briefing of the disciplines named, of those the
// event has. Shared with the browser demo.
func SetPrep(plan *store.Plan, in map[string]store.Prep, slugs []string) error {
	for slug, p := range in {
		if !slices.Contains(slugs, slug) {
			return fmt.Errorf("this event has no discipline %q", slug)
		}
		p, err := CleanPrep(p)
		if err != nil {
			return err
		}
		if plan.Prep == nil {
			plan.Prep = map[string]store.Prep{}
		}
		plan.Prep[slug] = p
	}
	return nil
}

// SetStarts writes when the disciplines named start at the earliest; blank takes the time
// off. Shared with the browser demo.
func SetStarts(plan *store.Plan, in map[string]string, slugs []string) error {
	for slug, s := range in {
		if !slices.Contains(slugs, slug) {
			return fmt.Errorf("this event has no discipline %q", slug)
		}
		s, err := CleanStart(s)
		if err != nil {
			return err
		}
		if s == "" {
			delete(plan.Starts, slug)
			continue
		}
		if plan.Starts == nil {
			plan.Starts = map[string]string{}
		}
		plan.Starts[slug] = s
	}
	return nil
}

// SetAnomaly marks or unmarks one match in the plan's list.
func SetAnomaly(list []string, key string, on bool) []string {
	out := []string{}
	for _, k := range list {
		if k != key {
			out = append(out, k)
		}
	}
	if on {
		out = append(out, key)
	}
	return out
}

// SetExpected writes how many each discipline named expects; 0 takes the number off.
// Shared with the browser demo.
func SetExpected(plan *store.Plan, in map[string]int) error {
	for _, n := range in {
		if n < 0 || n > 200 {
			return errors.New("expect between 0 and 200")
		}
	}
	if plan.Expected == nil {
		plan.Expected = map[string]int{}
	}
	for slug, n := range in {
		if n == 0 {
			delete(plan.Expected, slug)
		} else {
			plan.Expected[slug] = n
		}
	}
	return nil
}

// SetSessions writes the block of the day each discipline named runs in (#136), of those
// the event has. Shared with the browser demo.
func SetSessions(plan *store.Plan, in map[string]int, slugs []string) error {
	for slug, n := range in {
		if !slices.Contains(slugs, slug) || n < 1 || n > 20 {
			return errors.New("a block is a discipline of this event and a number from 1 to 20")
		}
	}
	if plan.Sessions == nil {
		plan.Sessions = map[string]int{}
	}
	for slug, n := range in {
		plan.Sessions[slug] = n
	}
	return nil
}

// LearnTimings writes what the day measured into the template (#64). The changeover is
// kept when nothing measured one.
func LearnTimings(t *store.Timings, rep forecast.Report) {
	t.Match = int(rep.Match / time.Second)
	if rep.Changeover > 0 {
		t.Changeover = int(rep.Changeover / time.Second)
	}
}
