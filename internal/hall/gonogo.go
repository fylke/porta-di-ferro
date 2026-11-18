package hall

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// Go / no-go (#167). Before a discipline runs, the tournament manager looks at one summary
// of what could go wrong -- not enough staff, a day running past the venue's close, fewer
// mats than its pools need -- and signs it off. Something major changing afterwards (its
// mats, its block or day, its start, its pool sizes) asks for a new sign-off. Like the
// forecast it warns and never refuses: a discipline not signed off still draws and runs.

// GoNoGoView is one discipline's summary and sign-off.
type GoNoGoView struct {
	Discipline string          `json:"discipline"`
	Name       string          `json:"name"`
	Warnings   []GoNoGoWarning `json:"warnings"`
	// Basis is what a sign-off now would be given on.
	Basis   map[string]string `json:"basis"`
	Signoff *store.Signoff    `json:"signoff,omitempty"`
	// Current says the sign-off was given on the basis as it stands; Changed names what
	// has changed since, when it was not.
	Current bool     `json:"current"`
	Changed []string `json:"changed,omitempty"`
}

// GoNoGoWarning is one thing the manager should know before saying go. Kind is "staff"
// (Short slots unfilled, in Roles), "overrun" (the discipline ends at End, after the venue
// closes at Close) or "mats" (Mats mats for what its numbers recommend, Recommended).
type GoNoGoWarning struct {
	Kind        string   `json:"kind"`
	NoStaff     bool     `json:"noStaff,omitempty"`
	Short       int      `json:"short,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	End         string   `json:"end,omitempty"`
	Close       string   `json:"close,omitempty"`
	Mats        int      `json:"mats,omitempty"`
	Recommended int      `json:"recommended,omitempty"`
}

// GoNoGoInput is what the summary is made from: the discipline, the plan, and the views the
// planning and staffing panels already show.
type GoNoGoInput struct {
	Slug, Name string
	Tournament store.Tournament
	HallMats   int
	Plan       store.Plan
	Slugs      []string
	Forecast   ForecastView
	Staffing   StaffingView
}

// GoNoGo is a discipline's summary as it stands.
func GoNoGo(in GoNoGoInput) GoNoGoView {
	v := GoNoGoView{Discipline: in.Slug, Name: in.Name, Warnings: []GoNoGoWarning{}, Basis: SignoffBasis(in)}
	prefix := in.Slug + "/"

	short, roles := 0, []string{}
	for _, s := range in.Staffing.Short {
		if strings.HasPrefix(s.Item, prefix) {
			short++
			if !slices.Contains(roles, s.Role) {
				roles = append(roles, s.Role)
			}
		}
	}
	if short > 0 {
		sort.Strings(roles)
		v.Warnings = append(v.Warnings, GoNoGoWarning{Kind: "staff", Short: short, Roles: roles,
			NoStaff: len(in.Staffing.Members) == 0})
	}

	if closing := in.Forecast.Timings.Close; closing != "" {
		var last time.Time
		for _, it := range in.Forecast.Items {
			if !strings.HasPrefix(it.ID, prefix) {
				continue
			}
			if end, err := time.Parse(time.RFC3339, it.End); err == nil && end.After(last) {
				last = end
			}
		}
		if !last.IsZero() {
			if shut, ok := ParseClock(last, closing); ok && last.After(shut) {
				v.Warnings = append(v.Warnings, GoNoGoWarning{Kind: "overrun", End: last.Format(time.RFC3339), Close: closing})
			}
		}
	}

	if advice, ok := in.Forecast.Mats[in.Slug]; ok {
		mats := usableMats(in)
		if advice.Recommended > mats {
			v.Warnings = append(v.Warnings, GoNoGoWarning{Kind: "mats", Mats: mats, Recommended: advice.Recommended})
		}
	}

	if s, ok := in.Plan.Signoffs[in.Slug]; ok {
		v.Signoff = &s
		for _, k := range basisKeys {
			if s.Basis[k] != v.Basis[k] {
				v.Changed = append(v.Changed, k)
			}
		}
		v.Current = len(v.Changed) == 0
	}
	return v
}

// basisKeys are what a sign-off is given on, in the order a change is reported.
var basisKeys = []string{"mats", "block", "day", "start", "pools", "elimMats"}

// SignoffBasis is the facts a sign-off is given on: what the issue calls something major.
// Entries and results are not among them -- they change all day.
func SignoffBasis(in GoNoGoInput) map[string]string {
	block := Sessions(in.Plan, in.Slugs)[in.Slug]
	return map[string]string{
		"mats":     fmt.Sprint(usableMats(in)),
		"block":    fmt.Sprint(block),
		"day":      in.Plan.Days[block],
		"start":    in.Plan.Starts[in.Slug],
		"pools":    fmt.Sprintf("%d-%d", in.Tournament.MinPoolSize, in.Tournament.MaxPoolSize),
		"elimMats": fmt.Sprint(in.Tournament.ElimMats),
	}
}

// usableMats is the mats the discipline can spread over: its own, never more than the hall
// has (EffectiveMatCount).
func usableMats(in GoNoGoInput) int { return EffectiveMatCount(in.HallMats, in.Tournament.Mats) }

// SignOff records the manager's go on the basis as it stands.
func SignOff(plan *store.Plan, in GoNoGoInput, by string, now time.Time) error {
	by = strings.TrimSpace(by)
	if by == "" {
		return errors.New("say who is signing off")
	}
	if len(by) > 80 {
		by = by[:80]
	}
	if plan.Signoffs == nil {
		plan.Signoffs = map[string]store.Signoff{}
	}
	plan.Signoffs[in.Slug] = store.Signoff{By: by, At: now.Format(time.RFC3339), Basis: SignoffBasis(in)}
	return nil
}
