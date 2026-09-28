// Package signup is the offline registration exchange: the file an organizer publishes,
// the file a participant sends back, and the rules for deciding what to do with it
// (issue #91, docs/proposals/offline-signup.md).
//
// The premise of the project is that a club can run an event with no internet-facing
// server and no accounts. Registration was the part that still made the organizer type
// forty names in by hand off a spreadsheet. So it goes out as a file and comes back as a
// file, by email or on a memory stick, and nothing in between needs a network.
//
// Everything here is pure: formats, validation and a preview. It reads no files and
// writes none, which is what lets the whole of it be tested as data in and data out.
package signup

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/fylke/porta-di-ferro/internal/store"
)

// The format markers. They are in the files rather than implied, so a participant who
// sends back the wrong JSON is told which wrong JSON it is instead of having it parsed
// halfway and rejected on a missing field.
const (
	DefinitionFormat = "porta.signup"
	ResponseFormat   = "porta.signup.response"
	// Version is the format version this build writes and the highest it reads.
	Version = 1
)

// Definition is what the organizer publishes: the event, when things run, and what can
// be entered. It carries stable tournament identifiers rather than ports or paths, so it
// survives the organizer restarting the application on a different port.
type Definition struct {
	Format       string           `json:"format"`
	Version      int              `json:"version"`
	DefinitionID string           `json:"definitionId"`
	Event        DefinitionEvent  `json:"event"`
	Schedule     []ScheduleEntry  `json:"schedule,omitempty"`
	Tournaments  []TournamentInfo `json:"tournaments"`
	// Fields are what the participant is asked for. "name" and "club" always; "contact"
	// when the organizer turned it on.
	Fields []string `json:"fields"`
	// StaffRoles are the jobs a participant can offer to do in the disciplines they are
	// not fencing in (issue #5). Published rather than built into the app, so the list
	// can change without every copy of the app already sent out going stale.
	StaffRoles []StaffRole `json:"staffRoles"`
}

// StaffRole is one job on the floor.
type StaffRole struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Offered is whether the app ticks it for them. Everyone is down for every role
	// unless they untick it -- asking people to volunteer finds fewer staff than asking
	// them to opt out -- except the one that is a qualification rather than a willingness.
	Offered bool `json:"offered"`
}

// Roles are the staff roles of issue #5, in the order the app lists them.
var Roles = []StaffRole{
	{ID: "head-ref", Label: "Head referee", Offered: true},
	{ID: "assistant-ref", Label: "Assistant referee", Offered: true},
	{ID: "score-keeper", Label: "Score keeper", Offered: true},
	{ID: "physician", Label: "Physician", Offered: false},
}

func isRole(id string) bool {
	for _, r := range Roles {
		if r.ID == id {
			return true
		}
	}
	return false
}

type DefinitionEvent struct {
	Name  string `json:"name"`
	Venue string `json:"venue,omitempty"`
	Date  string `json:"date,omitempty"`
}

// ScheduleEntry is one row of the published programme. Times are the organizer's text.
type ScheduleEntry struct {
	// Type is "discipline" or "break".
	Type string `json:"type"`
	// Tournament is set on a discipline row and names the tournament it runs.
	Tournament string `json:"tournamentId,omitempty"`
	Label      string `json:"label"`
	StartsAt   string `json:"startsAt,omitempty"`
	EndsAt     string `json:"endsAt,omitempty"`
}

// TournamentInfo is one thing a participant can enter.
type TournamentInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Capacity is 0 for no limit.
	Capacity int `json:"capacity,omitempty"`
}

// Response is what comes back: one participant, what they enter and what they will work.
type Response struct {
	Format       string `json:"format"`
	Version      int    `json:"version"`
	DefinitionID string `json:"definitionId"`
	// SubmissionID is made by the participant app and is what makes importing twice a
	// no-op. Name and club cannot do that job: two people of the same name at the same
	// club is a thing that happens, and a participant who corrects their entry and sends
	// it again should replace themselves rather than arrive twice.
	SubmissionID string      `json:"submissionId"`
	SubmittedAt  string      `json:"submittedAt,omitempty"`
	Participant  Participant `json:"participant"`
	Entries      []string    `json:"entries"`
	// Staff is what they will work, in the disciplines they are not entering. Absent
	// for somebody who unticked all of it.
	Staff *StaffOffer `json:"staff,omitempty"`
}

// StaffOffer is one participant's offer to work: which disciplines, and in which roles.
// One set of roles across all of them: a grid of disciplines by roles is a form nobody
// finishes.
type StaffOffer struct {
	Tournaments []string `json:"tournaments"`
	Roles       []string `json:"roles"`
}

type Participant struct {
	Name    string `json:"name"`
	Club    string `json:"club,omitempty"`
	Contact string `json:"contact,omitempty"`
}

// BuildDefinition turns the organizer's own event settings into the file that goes out.
//
// The programme is the source: rows marked as a discipline are what can be entered, and
// a row with no identifier of its own gets one derived from its label, so an organizer
// who never thinks about identifiers still publishes stable ones.
func BuildDefinition(t store.Tournament) Definition {
	ev := t.Event
	def := Definition{
		Format:       DefinitionFormat,
		Version:      Version,
		DefinitionID: strings.TrimSpace(ev.Signup.DefinitionID),
		Event: DefinitionEvent{
			Name:  strings.TrimSpace(ev.Signup.Name),
			Venue: strings.TrimSpace(ev.Signup.Venue),
			Date:  strings.TrimSpace(ev.Signup.Date),
		},
		Fields:     []string{"name", "club"},
		StaffRoles: Roles,
	}
	if def.Event.Name == "" {
		def.Event.Name = strings.TrimSpace(t.Discipline)
	}
	if ev.Signup.Contact {
		def.Fields = append(def.Fields, "contact")
	}

	seen := map[string]bool{}
	for _, item := range ev.Schedule {
		entry := ScheduleEntry{
			Type:     "break",
			Label:    item.Label,
			StartsAt: item.At,
			EndsAt:   item.Ends,
		}
		if item.Kind == "discipline" {
			id := TournamentID(item)
			entry.Type = "discipline"
			entry.Tournament = id
			if id != "" && !seen[id] {
				seen[id] = true
				def.Tournaments = append(def.Tournaments, TournamentInfo{
					ID: id, Label: item.Label, Capacity: item.Capacity,
				})
			}
		} else if item.Kind != "break" {
			// Anything that is neither is still worth publishing -- gear check, the
			// prize giving -- but it is not a break with an end time either.
			entry.Type = "item"
		}
		def.Schedule = append(def.Schedule, entry)
	}
	if def.Tournaments == nil {
		def.Tournaments = []TournamentInfo{}
	}
	return def
}

// TournamentID is the stable identifier of a discipline row: the organizer's own if they
// set one, and a slug of the label otherwise.
func TournamentID(item store.ScheduleItem) string {
	if id := strings.TrimSpace(item.Tournament); id != "" {
		return Slug(id)
	}
	return Slug(item.Label)
}

// Slug makes an identifier out of a label: lowercase, words joined by hyphens, and
// nothing in it that needs escaping in a file name or a JSON key.
func Slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			// Fold the Nordic letters rather than dropping them: "långsvärd" and
			// "langsvard" should not be two different disciplines.
			b.WriteString(fold(r))
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func fold(r rune) string {
	switch r {
	case 'å', 'ä', 'æ':
		return "a"
	case 'ö', 'ø':
		return "o"
	case 'é', 'è':
		return "e"
	case 'ü':
		return "u"
	}
	return string(r)
}

// --- the preview ---------------------------------------------------------------------

// Verdict is what the organizer is told about one response file.
type Verdict string

const (
	// New: a competitor this import would add.
	New Verdict = "new"
	// Staff: somebody this import would add to the staff, because they offered to work
	// this discipline rather than fence in it (issue #5).
	Staff Verdict = "staff"
	// Already: this submission has been imported before. Re-importing is a no-op, which
	// is what lets an organizer point at the same folder twice without thinking.
	Already Verdict = "already"
	// Repeat: the same submission id twice in one import -- the same file in the folder
	// under two names. Counted once.
	Repeat Verdict = "repeat"
	// OtherEvent: a response for a different event.
	OtherEvent Verdict = "other-event"
	// NotHere: valid, but for a discipline this run of the application is not.
	NotHere Verdict = "not-here"
	// Unknown: names a discipline the definition does not have.
	Unknown Verdict = "unknown"
	// Invalid: not a response file, or missing what it needs.
	Invalid Verdict = "invalid"
)

// Row is one response as the organizer sees it before deciding.
type Row struct {
	// Source is the file the response came from, for an organizer looking at a folder.
	Source       string  `json:"source"`
	Verdict      Verdict `json:"verdict"`
	Name         string  `json:"name,omitempty"`
	Club         string  `json:"club,omitempty"`
	Contact      string  `json:"contact,omitempty"`
	SubmissionID string  `json:"submissionId,omitempty"`
	// Entries are every discipline the response asked for, whether or not this run is one.
	Entries []string `json:"entries,omitempty"`
	// Staffing and Roles are the offer to work, likewise whole.
	Staffing []string `json:"staffing,omitempty"`
	Roles    []string `json:"roles,omitempty"`
	// Problem says what is wrong, for the verdicts that are a refusal.
	Problem string `json:"problem,omitempty"`
}

// Preview is the whole answer: what would happen, and nothing having happened.
type Preview struct {
	Rows []Row `json:"rows"`
	// Adding is how many competitors a confirmed import would create.
	Adding int `json:"adding"`
	// AddingStaff is how many people it would add to the staff.
	AddingStaff int `json:"addingStaff"`
	// Capacity is the warning for a discipline that would end up over its limit. A
	// warning, not a refusal: the organizer decides.
	Capacity []string `json:"capacity,omitempty"`
	// Tournament is the discipline this run imports for, so the screen can say so.
	Tournament string `json:"tournament,omitempty"`
}

// File is one response as it was read off disk, with the name it had.
type File struct {
	Source string `json:"source"`
	// Body is the file's bytes. Invalid JSON is a row in the preview, not an error.
	Body []byte `json:"body"`
}

// Check works out what an import would do, without doing any of it.
//
// It is the whole of the decision-making: the handler that confirms an import runs this
// again and adds exactly the rows it returns as New or Staff, so what the organizer
// approved and what gets written cannot come apart.
//
// mine is the tournament identifier this run of the application is, or "" for a single
// discipline event that never set one. A response naming only other disciplines is
// somebody else's to import, and says so rather than being an error.
func Check(def Definition, mine string, files []File, existing []store.Competitor, staff []store.StaffMember) Preview {
	known := map[string]TournamentInfo{}
	for _, t := range def.Tournaments {
		known[t.ID] = t
	}
	mine = strings.TrimSpace(mine)

	// One submission is one person in one run: a competitor here or staff here, never
	// both, so either list having it means it is done.
	imported := map[string]bool{}
	entered := 0
	for _, c := range existing {
		if c.Signup != "" {
			imported[c.Signup] = true
		}
		if !c.Withdrawn {
			entered++
		}
	}
	for _, s := range staff {
		if s.Signup != "" {
			imported[s.Signup] = true
		}
	}

	out := Preview{Rows: []Row{}, Tournament: mine}
	inThisImport := map[string]bool{}

	for _, f := range files {
		row := Row{Source: f.Source}
		res, err := ParseResponse(f.Body)
		if err != nil {
			row.Verdict, row.Problem = Invalid, err.Error()
			out.Rows = append(out.Rows, row)
			continue
		}

		row.Name, row.Club, row.Contact = res.Participant.Name, res.Participant.Club, res.Participant.Contact
		row.SubmissionID, row.Entries = res.SubmissionID, res.Entries
		if res.Staff != nil {
			row.Staffing, row.Roles = res.Staff.Tournaments, res.Staff.Roles
		}

		switch {
		case def.DefinitionID != "" && res.DefinitionID != def.DefinitionID:
			row.Verdict = OtherEvent
			row.Problem = fmt.Sprintf("for %q, not %q", res.DefinitionID, def.DefinitionID)
		case inThisImport[res.SubmissionID]:
			row.Verdict = Repeat
		case imported[res.SubmissionID]:
			row.Verdict = Already
		default:
			row.Verdict = verdictFor(res, known, mine)
			if row.Verdict == Unknown {
				row.Problem = unknownEntries(res, known)
			}
		}

		switch row.Verdict {
		case New, Staff, NotHere:
			inThisImport[res.SubmissionID] = true
		}
		switch row.Verdict {
		case New:
			out.Adding++
		case Staff:
			out.AddingStaff++
		}
		out.Rows = append(out.Rows, row)
	}

	if info, ok := known[mine]; ok && info.Capacity > 0 {
		if total := entered + out.Adding; total > info.Capacity {
			out.Capacity = append(out.Capacity, fmt.Sprintf(
				"%s would have %d entered, over its capacity of %d", info.Label, total, info.Capacity))
		}
	}
	return out
}

// verdictFor places a valid response. Fencing here beats working here: the app never
// offers to staff a discipline the participant has entered, and ParseResponse drops such
// an offer from a hand-edited file, so the two never meet in one run.
func verdictFor(res Response, known map[string]TournamentInfo, mine string) Verdict {
	var staffing []string
	if res.Staff != nil {
		staffing = res.Staff.Tournaments
	}
	enters, entersMine := names(res.Entries, known, mine)
	staffs, staffsMine := names(staffing, known, mine)
	switch {
	case !enters && !staffs:
		return Unknown
	case mine == "":
		// This run has not been told which discipline it is. Everything valid is
		// importable, which is the sensible answer for a one-discipline event.
		if enters {
			return New
		}
		return Staff
	case entersMine:
		return New
	case staffsMine:
		return Staff
	default:
		return NotHere
	}
}

// names reports whether ids has any discipline the definition knows, and whether one of
// them is mine.
func names(ids []string, known map[string]TournamentInfo, mine string) (found, isMine bool) {
	for _, id := range ids {
		if _, ok := known[id]; ok {
			found = true
			if id == mine {
				isMine = true
			}
		}
	}
	return found, isMine
}

func unknownEntries(res Response, known map[string]TournamentInfo) string {
	ids := res.Entries
	if res.Staff != nil {
		ids = append(append([]string(nil), ids...), res.Staff.Tournaments...)
	}
	var bad []string
	for _, id := range ids {
		if _, ok := known[id]; !ok {
			bad = append(bad, id)
		}
	}
	sort.Strings(bad)
	if len(bad) == 0 {
		return "no disciplines chosen"
	}
	return "no such discipline: " + strings.Join(bad, ", ")
}

// Import turns the rows a preview marked New into competitors, numbered by the caller's
// own scheme. Only New: everything else the organizer saw as a refusal stays refused.
func Import(p Preview, nextID func([]store.Competitor) string, existing []store.Competitor) []store.Competitor {
	out := append([]store.Competitor(nil), existing...)
	for _, row := range p.Rows {
		if row.Verdict != New {
			continue
		}
		out = append(out, store.Competitor{
			ID:     nextID(out),
			Name:   row.Name,
			Club:   row.Club,
			Signup: row.SubmissionID,
		})
	}
	return out
}

// ImportStaff is Import for the rows a preview marked Staff.
func ImportStaff(p Preview, existing []store.StaffMember) []store.StaffMember {
	out := append([]store.StaffMember(nil), existing...)
	for _, row := range p.Rows {
		if row.Verdict != Staff {
			continue
		}
		out = append(out, store.StaffMember{
			ID:     NextStaffID(out),
			Name:   row.Name,
			Club:   row.Club,
			Roles:  append([]string(nil), row.Roles...),
			Signup: row.SubmissionID,
		})
	}
	return out
}

// NextStaffID numbers staff the way competitors are numbered, with their own prefix so
// the two can never be mistaken for each other in a file.
func NextStaffID(existing []store.StaffMember) string {
	highest := 0
	for _, s := range existing {
		if n, err := strconv.Atoi(strings.TrimPrefix(s.ID, "s")); err == nil && n > highest {
			highest = n
		}
	}
	return fmt.Sprintf("s%d", highest+1)
}
