package hall

// What the organizer marks on the plan: a match not to learn from, and a work item pinned,
// held to a time, or let run beside its final.

import (
	"errors"
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/store"
)

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

// ItemFlags are what the card menu sets on a work item: a pin, a time to hold it to, and
// for a bronze match whether it may run at the same time as the final (#149).
type ItemFlags struct {
	Pinned    *bool   `json:"pinned"`
	NotBefore *string `json:"notBefore"`
	WithFinal *bool   `json:"withFinal"`
}

// Any says the request sets a flag.
func (f ItemFlags) Any() bool { return f.Pinned != nil || f.NotBefore != nil || f.WithFinal != nil }

// SetItemFlags pins or unpins a work item, holds it to a time, or lets a bronze match run
// with its final, in the placements.
func SetItemFlags(placed map[string]store.Placement, id string, f ItemFlags) (map[string]store.Placement, error) {
	p, ok := placed[id]
	if !ok {
		return placed, errors.New("no work item " + id)
	}
	if f.NotBefore != nil {
		if _, ok := ParseClock(time.Now(), *f.NotBefore); *f.NotBefore != "" && !ok {
			return placed, errors.New("write the time as 16:30")
		}
		p.NotBefore = *f.NotBefore
	}
	if f.Pinned != nil {
		p.Pinned = *f.Pinned
	}
	if f.WithFinal != nil {
		if !strings.HasSuffix(id, "/"+ItemBronze) {
			return placed, errors.New("only a bronze match can run with the final")
		}
		p.WithFinal = *f.WithFinal
	}
	next := make(map[string]store.Placement, len(placed))
	for k, v := range placed {
		next[k] = v
	}
	next[id] = p
	return next, nil
}
