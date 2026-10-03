package staffing_test

import (
	"reflect"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/signup"
	"github.com/fylke/porta-di-ferro/internal/staffing"
	"github.com/fylke/porta-di-ferro/internal/store"
)

func nobody(string, string, string) string { return "pr-x" }

// One response offering to work two disciplines is one member working both, however
// many imports it comes through. A name is never enough.
func TestOffersFromSignupsAreOneMember(t *testing.T) {
	row := signup.Row{Verdict: signup.Staff, Name: "Dag", SubmissionID: "sub-1", Roles: []string{"head-ref"}}
	members, n := staffing.FromSignup(nil, "ls", []signup.Row{row}, nobody)
	if n != 1 || len(members) != 1 || members[0].Person != "pr-x" {
		t.Fatalf("a new offer should be a new member with a person: %+v", members)
	}
	row.Roles = []string{"score-keeper"}
	members, n = staffing.FromSignup(members, "sa", []signup.Row{row}, nobody)
	if n != 1 || len(members) != 1 || !reflect.DeepEqual(members[0].Disciplines, []string{"ls", "sa"}) || len(members[0].Roles) != 2 {
		t.Errorf("the same response for another discipline should add it to the member: %+v", members)
	}
	if _, n := staffing.FromSignup(members, "sa", []signup.Row{row}, nobody); n != 0 {
		t.Error("importing the same offer twice should take nothing")
	}
	twin := signup.Row{Verdict: signup.Staff, Name: "Dag", SubmissionID: "sub-2"}
	if members, _ := staffing.FromSignup(members, "ls", []signup.Row{twin}, nobody); len(members) != 2 {
		t.Error("another Dag on another response is somebody else")
	}
}

func TestLiftingEachDisciplinesStaff(t *testing.T) {
	got := staffing.Lift(map[string][]store.StaffMember{
		"ls": {{ID: "s1", Name: "Dag", Signup: "sub-1", Roles: []string{"head-ref"}}, {ID: "s2", Name: "Eva"}},
		"sa": {{ID: "s1", Name: "Dag", Signup: "sub-1", Roles: []string{"score-keeper"}}},
	}, nobody)
	if len(got) != 2 || !reflect.DeepEqual(got[0].Disciplines, []string{"ls", "sa"}) || got[0].ID == "s1" || got[0].ID == got[1].ID {
		t.Errorf("Dag on one response is one member in both, with an id of the event's: %+v", got)
	}
}

func TestRemovingFromADiscipline(t *testing.T) {
	members := []store.StaffMember{{ID: "a", Disciplines: []string{"ls", "sa"}}, {ID: "b"}, {ID: "c", Disciplines: []string{"ls"}}}
	all := []string{"ls", "sa", "rp"}
	out, changed := staffing.RemoveFrom(members, "ls", "a", all)
	if !changed || !reflect.DeepEqual(out[0].Disciplines, []string{"sa"}) {
		t.Errorf("a keeps sabre: %+v", out)
	}
	out, _ = staffing.RemoveFrom(out, "ls", "b", all)
	if !reflect.DeepEqual(out[1].Disciplines, []string{"sa", "rp"}) {
		t.Errorf("b worked anything, and now works everything but longsword: %+v", out[1])
	}
	out, _ = staffing.RemoveFrom(out, "ls", "c", all)
	if len(out) != 2 {
		t.Errorf("c worked only longsword, and is off the staff: %+v", out)
	}
	if _, changed := staffing.RemoveFrom(out, "ls", "a", nil); changed {
		t.Error("a no longer works longsword; nothing to remove")
	}
}

func TestCleanKeepsKnownRoles(t *testing.T) {
	m, err := staffing.Clean(store.StaffMember{Name: "  Eva ", Roles: []string{"head-ref", "juggler", "head-ref"}})
	if err != nil || m.Name != "Eva" || !reflect.DeepEqual(m.Roles, []string{"head-ref"}) {
		t.Errorf("trimmed, known roles once: %+v %v", m, err)
	}
	if _, err := staffing.Clean(store.StaffMember{Name: " "}); err == nil {
		t.Error("a member needs a name")
	}
}
