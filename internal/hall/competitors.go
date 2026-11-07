package hall

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// NextCompetitorID keeps ids short and readable -- c1, c2 -- because the organizer is
// expected to open competitors.json in a text editor, and that escape hatch is only real
// if what they find in there is legible. Numbering continues past the highest id ever
// used, so removing someone never hands their id to the next entrant.
func NextCompetitorID(existing []store.Competitor) string {
	highest := 0
	for _, c := range existing {
		if n, err := strconv.Atoi(strings.TrimPrefix(c.ID, "c")); err == nil && n > highest {
			highest = n
		}
	}
	return fmt.Sprintf("c%d", highest+1)
}
