package demo_test

import (
	"encoding/json"
	"testing"

	"github.com/fylke/porta-di-ferro/internal/demo"
	httpapi "github.com/fylke/porta-di-ferro/internal/http"
)

// The demo's people answer as the coordinator's do (phase 3).

func peopleOf(t *testing.T, e *demo.Event) httpapi.PeopleView {
	t.Helper()
	res := e.Request("GET", "/api/people", nil)
	if res.Status != 200 {
		t.Fatalf("GET /api/people returned %d: %s", res.Status, res.Body)
	}
	var v httpapi.PeopleView
	if err := json.Unmarshal([]byte(res.Body), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func personIn(t *testing.T, e *demo.Event, slug, name string) string {
	t.Helper()
	for _, c := range snapIn(t, e, slug).Competitors {
		if c.Name == name {
			return c.Person
		}
	}
	t.Fatalf("%s is not in %s", name, slug)
	return ""
}

func TestTheDemoKnowsItsPeople(t *testing.T) {
	e := demo.NewEvent()
	ls, sa := "open-steel-longsword", "open-sabre"
	if a, b := personIn(t, e, ls, "Astrid Lindqvist"), personIn(t, e, sa, "Astrid Lindqvist"); a == "" || a != b {
		t.Errorf("Astrid signed up for both on one response, so she is one person: %q %q", a, b)
	}
	bo1, bo2 := personIn(t, e, ls, "Bo Kjellberg"), personIn(t, e, sa, "Bo Kjellberg")
	if bo1 == bo2 {
		t.Fatal("Bo was typed in at two desks, so he is two people until the organizer says otherwise")
	}
	v := peopleOf(t, e)
	if len(v.Duplicates) != 1 {
		t.Fatalf("the two Bos should be offered as one: %+v", v.Duplicates)
	}

	res := e.Request("POST", "/api/people/"+bo2+"/merge", []byte(`{"into":"`+bo1+`"}`))
	if res.Status != 200 || !res.Changed {
		t.Fatalf("merging returned %d: %s", res.Status, res.Body)
	}
	if got := personIn(t, e, sa, "Bo Kjellberg"); got != bo1 {
		t.Errorf("after the merge Sabre's Bo should be the longsword's: %q", got)
	}
	res = e.Request("GET", "/api/people/"+bo2, nil)
	var one httpapi.PersonView
	_ = json.Unmarshal([]byte(res.Body), &one)
	if one.ID != bo1 || len(one.Entries) != 2 {
		t.Errorf("the old link should find Bo, with both entries: %+v", one)
	}

	// It survives the tab being closed.
	saved, err := e.Save()
	if err != nil {
		t.Fatal(err)
	}
	again := demo.NewEvent()
	if err := again.Load(saved); err != nil {
		t.Fatal(err)
	}
	if got := personIn(t, again, sa, "Bo Kjellberg"); got != bo1 || len(peopleOf(t, again).Duplicates) != 0 {
		t.Errorf("a reloaded demo should keep the merge: %q", got)
	}

	// The desk names the person it picked.
	res = e.Request("POST", "/api/d/"+sa+"/competitors", []byte(`{"name":"Clara Wikström","person":"`+
		personIn(t, e, ls, "Clara Wikström")+`"}`))
	if res.Status != 200 {
		t.Fatalf("adding returned %d: %s", res.Status, res.Body)
	}
	if personIn(t, e, sa, "Clara Wikström") != personIn(t, e, ls, "Clara Wikström") {
		t.Error("an entry the desk linked should be that person")
	}
}

func TestTheDemoImportsForTheWholeEvent(t *testing.T) {
	e := demo.NewEvent()
	res := e.Request("GET", "/api/event/signup/ready", nil)
	var ready struct {
		Disciplines []httpapi.SignupShare    `json:"disciplines"`
		Unclaimed   []struct{ Label string } `json:"unclaimed"`
	}
	if err := json.Unmarshal([]byte(res.Body), &ready); err != nil || len(ready.Disciplines) != 2 {
		t.Fatalf("ready should list both disciplines: %v %s", err, res.Body)
	}
	if ready.Disciplines[0].Tournament == "" || ready.Disciplines[1].Tournament == "" {
		t.Errorf("each discipline should take a programme row: %+v", ready.Disciplines)
	}
	if len(ready.Unclaimed) != 0 {
		t.Errorf("the demo's signup should offer nothing that no discipline takes: %+v", ready.Unclaimed)
	}
	res = e.Request("GET", "/api/event/signup/app.html", nil)
	if res.Status != 200 || res.ContentType != "text/html; charset=utf-8" {
		t.Errorf("the signup app should be served: %d %s", res.Status, res.ContentType)
	}
}
