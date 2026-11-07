package hall

import (
	"strings"
	"time"

	"github.com/fylke/porta-di-ferro/internal/forecast"
)

// The measured day, as the planning panel reads it.

// ReportRow is one fenced match in the measured report, with what the organizer needs to
// recognise it.
type ReportRow struct {
	Key            string `json:"key"`
	Discipline     string `json:"discipline"`
	DisciplineName string `json:"disciplineName"`
	Match          string `json:"match"`
	Red            string `json:"red"`
	Blue           string `json:"blue"`
	Mat            int    `json:"mat"`
	Started        string `json:"started"`
	Seconds        int    `json:"seconds"`
	Changeover     int    `json:"changeover"`
	Anomaly        bool   `json:"anomaly"`
}

// ReportView is the measured day: every fenced match, and the averages a template learned
// from it would have.
type ReportView struct {
	Matches    []ReportRow `json:"matches"`
	Match      int         `json:"match"`
	Changeover int         `json:"changeover"`
	Samples    int         `json:"samples"`
}

// ViewReport is the measured report as the planning panel reads it.
func ViewReport(rep forecast.Report, inputs []MatsInput) ReportView {
	type label struct{ name, red, blue string }
	labels := map[string]label{}
	for _, d := range inputs {
		names := map[string]string{}
		for _, c := range d.Snapshot.Competitors {
			names[c.ID] = c.Name
		}
		d.Snapshot.eachMatch(func(m *MatchView) {
			labels[d.Slug+"/"+m.ID] = label{d.Name, names[m.Red], names[m.Blue]}
		})
	}
	out := ReportView{Matches: []ReportRow{}, Match: int(rep.Match / time.Second), Changeover: int(rep.Changeover / time.Second), Samples: rep.Samples}
	for _, m := range rep.Matches {
		disc, id, _ := strings.Cut(m.Key, "/")
		l := labels[m.Key]
		out.Matches = append(out.Matches, ReportRow{Key: m.Key, Discipline: disc, DisciplineName: l.name, Match: id,
			Red: l.red, Blue: l.blue, Mat: m.Mat, Started: stamp(m.Started),
			Seconds: int(m.Ended.Sub(m.Started) / time.Second), Changeover: int(m.Changeover / time.Second), Anomaly: m.Anomaly})
	}
	return out
}
