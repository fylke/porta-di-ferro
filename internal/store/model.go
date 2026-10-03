// Package store is the on-disk database: local JSON files the organizer owns and can
// read (design decision 8). One directory per tournament.
//
//	competitors.json          registration and status
//	tournament.json           mats, pool constraints, generated pools and assignments, staff
//	matches/<match_id>.ndjson the append-only exchange log, one event per line
//
// The two JSON files are written by atomic replace, never in place. The log is
// newline-delimited because appending a line is the cheapest durable write there is, and
// a truncated final line after a crash costs one event rather than the file.
//
// Derived state -- scores, standings, rankings -- is never stored here, only computed.
package store

// Competitor is a person entered in the tournament. Name and club only: picture, phone
// number and club crest are Milestone 3 (design §9, issue #1).
type Competitor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Club string `json:"club"`
	// Signup is the submission identifier of the offline signup file this competitor was
	// imported from, or empty when the organizer typed them in (issue #91). It is what
	// makes importing the same folder twice a no-op: name and club are not an identity --
	// two Anna Nilssons from the same club is a real thing at a club open -- so the file
	// carries an id and the competitor remembers it.
	Signup string `json:"signup,omitempty"`
	// Withdrawn voids this competitor's results as though they never entered. The
	// ranking divides by matches completed, which is what makes that work retroactively.
	Withdrawn bool `json:"withdrawn"`
	// Person is who this entry is across the event: one person can be entered in several
	// disciplines, and each entry points at the same Person (phase 3 of
	// docs/proposals/one-event-many-disciplines.md). Empty for a discipline on its own.
	Person string `json:"person,omitempty"`
}

// Match is one competitor against another, fixed at pool creation along with the colours
// -- or, in the eliminations, fixed by the results that feed it.
type Match struct {
	ID string `json:"id"`
	// Pool is 0 for a bracket match.
	Pool int `json:"pool"`
	// Order is the position within the pool's running order, or the bracket's.
	Order int    `json:"order"`
	Mat   int    `json:"mat"`
	Red   string `json:"red"`
	Blue  string `json:"blue"`
	// Round and Slot place a bracket match: "quarter", "semi", "bronze" or "final", and
	// the match's number within its round. FeedRed and FeedBlue name the matches whose
	// results fill Red and Blue -- "winner:e-qf1", "loser:e-sf2" -- so a later round's
	// competitors are derived from the log rather than stored, and a corrected
	// quarter-final corrects the semi-final for free.
	Round    string `json:"round,omitempty"`
	Slot     int    `json:"slot,omitempty"`
	FeedRed  string `json:"feedRed,omitempty"`
	FeedBlue string `json:"feedBlue,omitempty"`
}

// Pool is a group of competitors who each fence all the others once.
type Pool struct {
	Number int `json:"number"`
	Mat    int `json:"mat"`
	// Sequence is the pool's place in its mat's queue. The generator sets it to the pool
	// number, so the default order is by number; the organizer's override moves it. A
	// file from before the field existed has zeros throughout, which reads the same way.
	Sequence    int      `json:"sequence,omitempty"`
	Competitors []string `json:"competitors"`
	Matches     []Match  `json:"matches"`
}

// Event is the day the tournament sits inside, as the people who turn up experience it
// (issue #98). None of it affects a single result: it is what the participant view, the
// printed info sheet and the hall put around the fencing.
//
// It lives with the tournament because that is the file an organizer already owns and
// can hand-edit. An event running several disciplines at once is several runs of the
// application, and each carries its own copy for now -- one welcome message typed twice
// is a smaller problem than a shared file two processes both write.
type Event struct {
	// Welcome is what the info sheet and the landing page open with: a few lines about
	// the event in general terms, written by the organizer.
	Welcome  string         `json:"welcome,omitempty"`
	Schedule []ScheduleItem `json:"schedule,omitempty"`
	Wifi     Wifi           `json:"wifi,omitempty"`
	Signup   Signup         `json:"signup,omitempty"`
}

// Signup is what the offline signup files carry about the event (issue #91,
// docs/proposals/offline-signup.md).
//
// The programme above doubles as the signup definition's schedule: the organizer types
// the day once, marks which rows are disciplines, and those rows are what a participant
// can enter. One list to keep right rather than two that can disagree.
type Signup struct {
	// DefinitionID identifies the event across the files that go out and the files that
	// come back, so a response from last year's open cannot be imported into this one.
	DefinitionID string `json:"definitionId,omitempty"`
	Name         string `json:"name,omitempty"`
	Venue        string `json:"venue,omitempty"`
	// Date as the organizer wrote it. Shown, never computed with.
	Date string `json:"date,omitempty"`
	// Tournament is which discipline *this* run of the application is, matching the
	// Tournament of one of the schedule rows. An event with three disciplines is three
	// runs, and each imports the entries naming its own: the organizer points all three
	// at the same folder of responses and each takes its share.
	Tournament string `json:"tournament,omitempty"`
	// Contact turns on the optional contact field in the participant app. Name and club
	// are always asked for; anything more is the organizer's choice to collect.
	Contact bool `json:"contact,omitempty"`
}

// ScheduleItem is one line of the day's agenda: gear check, the pools, lunch, the
// eliminations. Deliberately free text with a time beside it rather than anything the
// application derives -- the application does not know when lunch is, and an organizer
// who has to make the schedule fit a data model will keep the real one on paper.
type ScheduleItem struct {
	// At is a time of day as the organizer wrote it, "09:00". Not parsed: a schedule
	// that says "after the pools" is a legitimate schedule.
	At string `json:"at,omitempty"`
	// Ends is when a break finishes, for the rows where that matters. Same rules.
	Ends  string `json:"ends,omitempty"`
	Label string `json:"label"`
	// Kind is "discipline", "break" or "" -- used to style the row, and to decide which
	// rows are things a participant can sign up for.
	Kind string `json:"kind,omitempty"`
	// Tournament is the stable identifier of the discipline this row runs, on a row of
	// kind "discipline". It is what a signup file refers to, so it must not change once
	// definitions have gone out: the response that comes back names it (issue #91).
	//
	// Stable identifiers rather than ports or paths is the point. A discipline is a
	// separate run of the application on whichever port was free that morning, and a
	// participant's file must survive the organizer restarting it.
	Tournament string `json:"tournament,omitempty"`
	// Capacity is how many the discipline can take, or 0 for no limit. Used to warn on
	// import rather than to refuse it -- an organizer who wants a twenty-ninth in a pool
	// of twenty-eight is allowed to have one, and should be told.
	Capacity int `json:"capacity,omitempty"`
}

// Wifi is the venue network, for the QR code on the printed info sheet. A spectator who
// cannot get on the wifi cannot reach any of this.
//
// The password is stored in the clear in the organizer's own file, which is the same
// place it would be on the poster they would otherwise print. It is a guest network
// password written on a wall, not a credential.
type Wifi struct {
	SSID     string `json:"ssid,omitempty"`
	Password string `json:"password,omitempty"`
	// Security is "WPA", "WEP" or "nopass", as the Wi-Fi QR format spells it.
	Security string `json:"security,omitempty"`
	Hidden   bool   `json:"hidden,omitempty"`
}

// Tournament is the setup and the generated draw. Name, logo and discipline linkage are
// Milestone 3 (design §9, issue #2): one run of the application is one tournament under
// one hardcoded ruleset.
type Tournament struct {
	// Discipline is what this run is called -- "Open steel Longsword", "Open Sabre" --
	// shown on every page so a hall with two runs going can tell them apart. Kept here
	// rather than only in the -name flag, so renaming it from the organizer page sticks
	// across a restart (issue #80).
	Discipline  string `json:"discipline,omitempty"`
	Mats        int    `json:"mats"`
	MinPoolSize int    `json:"minPoolSize"`
	MaxPoolSize int    `json:"maxPoolSize"`
	// Event is the day around the tournament: the welcome message, the agenda and the
	// venue wifi (issue #98). Nothing here reaches a result.
	Event Event `json:"event,omitempty"`
	// ElimMats is how many mats the organizer wants the eliminations run on, or 0 to
	// take the suggestion. The pools spread over everything available because they run
	// for hours; a bracket of seven matches often finishes no sooner on three mats than
	// on two, and the third is a referee and a score keeper staffed for nothing
	// (issue #94). tournament.EliminationMatsFor resolves the two.
	ElimMats int `json:"elimMats,omitempty"`
	// Seed makes the random-draw tie-break reproducible, so a standing can be explained
	// after the fact rather than being a fresh coin toss on every page load.
	Seed        int64  `json:"seed"`
	Pools       []Pool `json:"pools"`
	GeneratedAt string `json:"generatedAt,omitempty"`
	// Violations are the ordering constraints the generator could not satisfy. Reported
	// rather than guaranteed away (design §6 item 9).
	Violations []string `json:"violations,omitempty"`
	// Bracket is the eliminations, in playing order, drawn once the pools are done.
	// First-round competitors are stored; later rounds are filled from results.
	Bracket   []Match `json:"bracket,omitempty"`
	BracketAt string  `json:"bracketAt,omitempty"`
	// Staff is who has offered to work this discipline rather than fence in it: the
	// people a head referee, an assistant or a score keeper is found among (issue #5).
	// Offered, not assigned -- who stands where is a later step.
	Staff []StaffMember `json:"staff,omitempty"`
}

// StaffMember is one person available to staff this discipline, and in which roles.
type StaffMember struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Club string `json:"club,omitempty"`
	// Roles are what they are willing to do: "head-ref", "assistant-ref", "score-keeper",
	// "physician". Everything they did not untick, so an empty list is not a volunteer.
	Roles []string `json:"roles"`
	// Signup is the submission identifier they came in on, the same as a competitor's:
	// importing the folder twice must not offer the same person twice.
	Signup string `json:"signup,omitempty"`
	// Person is who they are across the event (phase 5): the same person as their
	// competitor entries, so they are never put on a mat while they are fencing.
	Person string `json:"person,omitempty"`
	// Disciplines are the disciplines, by slug, they will work. Empty is any discipline.
	Disciplines []string `json:"disciplines,omitempty"`
}

// Staff is the event's staff and who works where (docs/proposals/one-event-many-
// disciplines.md, phase 5; #5), kept in staff.json in the event folder: one person cannot
// referee two mats at once, so staff belong to the event rather than to a discipline.
type Staff struct {
	Members []StaffMember `json:"members"`
	// Crew is how many of each role a mat needs while it runs an item, by role id. A role
	// left out takes the default (see staffing.DefaultCrew).
	Crew map[string]int `json:"crew,omitempty"`
	// Assignments are who works each item in each role.
	Assignments []Assignment `json:"assignments,omitempty"`
}

// Assignment is one member working one role on one work item.
type Assignment struct {
	// Item is the work item, "open-sabre/pool-2".
	Item string `json:"item"`
	Role string `json:"role"`
	// Slot tells apart two of the same role on one item: the two assistant referees.
	Slot  int    `json:"slot"`
	Staff string `json:"staff"`
	// Pinned says the organizer chose this one; a suggestion keeps it.
	Pinned bool `json:"pinned,omitempty"`
}

// Person is one human across an event: the competitor entries in every discipline that
// are them point here (proposal §8). Ids are random, never sequential, so they cannot
// collide between disciplines, devices or events the way competitor ids do.
//
// Two people are never made one by their names: two Anna Nilssons from the same club are a
// real thing. A signup's submission id is exact and links without asking; everything else
// is the organizer's to merge -- and a merge can be undone.
type Person struct {
	ID string `json:"id"`
	// Name and Club are as the person was first entered. Pages show the entries' own
	// names, which the organizer corrects in the discipline; these are for when there are
	// none.
	Name string `json:"name"`
	Club string `json:"club,omitempty"`
	// Signup is the submission id the person came in on, so a response that enters
	// several disciplines is one person in all of them.
	Signup string `json:"signup,omitempty"`
	// MergedInto says this person turned out to be another; their entries were moved
	// there. Moved lists those entries, "discipline/competitor", so the merge can be
	// undone exactly.
	MergedInto string   `json:"mergedInto,omitempty"`
	Moved      []string `json:"moved,omitempty"`
	// Apart lists people the organizer has said are somebody else, despite the name, so
	// the pair stops being offered as a possible duplicate.
	Apart []string `json:"apart,omitempty"`
}

// Plan is where every discipline's work runs: the event's physical mats, and each work
// item's place in one mat's queue (docs/proposals/one-event-many-disciplines.md §9). A
// work item is a pool, a discipline's eliminations on one of its lanes, the bronze match
// or the final. The disciplines say what their items are; the plan says which mat runs
// each and in what order.
type Plan struct {
	// Mats is how many mats the hall has, or 0 for as many as the disciplines ask for --
	// which, with one discipline, keeps a one-discipline event exactly as it always was.
	Mats int `json:"mats,omitempty"`
	// Items is each work item's placement, by "slug/key": "open-sabre/pool-3".
	Items map[string]Placement `json:"items,omitempty"`
	// Timings are how long things take, for the forecast (phase 4). Zero fields take the
	// defaults: see Timings.
	Timings Timings `json:"timings,omitempty"`
	// Expected is how many each discipline expects to enter, by slug, for planning the day
	// before the entries are in (#64). The forecast uses it while it is more than are
	// entered and the pools are not drawn.
	Expected map[string]int `json:"expected,omitempty"`
	// Anomalies are matches whose times are not to be learned from -- a long injury
	// break, a score keeper who forgot to end the match -- by "slug/match" (#64).
	Anomalies []string `json:"anomalies,omitempty"`
}

// Timings are how long fencing takes, in seconds, and when the day starts and ends, as
// "HH:MM". The template the forecast starts from, until the day's own pace replaces it,
// and what the measured times can be written back into for the next event (#64).
type Timings struct {
	// Match is one match, from its first event to its last.
	Match int `json:"match,omitempty"`
	// Changeover is the gap between one match ending and the next starting on a mat.
	Changeover int `json:"changeover,omitempty"`
	// BeforeElims is the pause between a discipline's last pool and its eliminations:
	// ranking, the draw, the call to the mats.
	BeforeElims int `json:"beforeElims,omitempty"`
	// Start is when the first match of the day starts; Close is when the venue closes.
	Start string `json:"start,omitempty"`
	Close string `json:"close,omitempty"`
}

// Screens is how the hall's mat screens look (#110), set by the organizer for every screen
// at once.
type Screens struct {
	// Upcoming is what a mat screen shows of the matches after the current one: "bottom"
	// (the next match along the foot), "list" (the next few down the right-hand side) or
	// "none". Empty is "bottom".
	Upcoming string `json:"upcoming,omitempty"`
}

// UpcomingChoices are the values Screens.Upcoming takes.
var UpcomingChoices = []string{"bottom", "list", "none"}

// UpcomingOf is the choice in force.
func UpcomingOf(s Screens) string {
	for _, c := range UpcomingChoices {
		if s.Upcoming == c {
			return c
		}
	}
	return "bottom"
}

// Placement is one work item's mat and its place in that mat's queue.
type Placement struct {
	Mat int `json:"mat"`
	Seq int `json:"seq"`
	// Stamp is the draw the item belongs to: the pools' GeneratedAt or the bracket's
	// BracketAt. A redraw makes new items of the same names, which are placed afresh
	// rather than inheriting where the old ones had been moved.
	Stamp string `json:"stamp,omitempty"`
	// Pinned says the organizer put the item here, so a suggested plan keeps it on this
	// mat (phase 4).
	Pinned bool `json:"pinned,omitempty"`
	// NotBefore is the earliest the item may start, "HH:MM": a final held for 16:30.
	NotBefore string `json:"notBefore,omitempty"`
	// Planned says the organizer settled this place by applying a suggestion. Like a pin it
	// keeps a projected item where it is and hands the place to the drawn item; unlike a
	// pin, the next suggestion may move it.
	Planned bool `json:"planned,omitempty"`
}

// Defaults returns the MVP tournament setup: two mats, pools of four to seven.
func Defaults() Tournament {
	return Tournament{Mats: 2, MinPoolSize: 4, MaxPoolSize: 7}
}
