package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// write lays out files under a temporary directory and returns it, with the working
// directory moved there so the paths in the items are the relative ones a real run gives.
func write(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
}

func byName(items []Item) map[string]Item {
	out := map[string]Item{}
	for _, it := range items {
		out[it.Name] = it
	}
	return out
}

func TestGoTagsLandOnTheirDeclaration(t *testing.T) {
	write(t, map[string]string{
		"internal/m/m.go": `package m

// Replay derives the state.
// REQ: FR_MATCH_01, FR_MATCH_03 -- the note after the IDs is not one
func Replay() {}

type Server struct{}

// REQ: NFR_SEC_01
func (s *Server) guard() {}

// REQ: NFR_DATA_01
type Store struct{}

func untagged() {}
`,
		"internal/m/m_test.go": `package m

import "testing"

func TestReplay(t *testing.T) {
	// REQ: FR_MATCH_01
	// REQ: FR_MATCH_05
	Replay()
}
`,
	})
	ev, errs := Collect([]string{"internal"})
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	code := byName(ev[goCode.File])
	if len(code) != 3 {
		t.Fatalf("code items = %v, want Replay, Server.guard and Store", code)
	}
	if got := code["Replay"]; !reflect.DeepEqual(got.Refs, []string{"FR_MATCH_01", "FR_MATCH_03"}) || got.Line != 5 || got.Kind != "Function" {
		t.Errorf("Replay = %+v", got)
	}
	if got := code["Server.guard"]; got.Kind != "Method" || got.Tag != "go internal/m/m.go:Server.guard" {
		t.Errorf("method = %+v", got)
	}
	if got := code["Store"]; got.Kind != "Type" {
		t.Errorf("type = %+v", got)
	}
	tests := ev[goTests.File]
	if len(tests) != 1 || tests[0].Name != "TestReplay" || tests[0].Kind != "Test" ||
		!reflect.DeepEqual(tests[0].Refs, []string{"FR_MATCH_01", "FR_MATCH_05"}) {
		t.Errorf("tests = %+v, want one TestReplay carrying both tags", tests)
	}
}

func TestATagThatCannotBeReadOrPlacedFails(t *testing.T) {
	write(t, map[string]string{
		"a/a.go":  "package a\n\n// REQ: see the design\nfunc A() {}\n",
		"b/b.go":  "package b\n\nfunc B() {}\n\n// REQ: FR_MATCH_01\n",
		"c/c.ts":  "// REQ: lower_case\nexport function c() {}\n",
		"d/d.txt": "REQ: FR_MATCH_01 is not in a file this reads",
	})
	_, errs := Collect([]string{"a", "b", "c", "d"})
	if len(errs) != 3 {
		t.Fatalf("errors = %v, want one each for a, b and c", errs)
	}
	for i, want := range []string{"a/a.go:3", "b/b.go:5", "c/c.ts:1"} {
		if !strings.Contains(errs[i].Error(), want) {
			t.Errorf("error %d = %v, want it at %s", i, errs[i], want)
		}
	}
}

func TestScriptTagsNameTheNextDeclarationOrTheFile(t *testing.T) {
	write(t, map[string]string{
		"web/src/lib/engine.ts": `import { x } from './x';

/**
 * Replays a log.
 * REQ: FR_MATCH_01
 */
export function replay() {}

export class Session {
  // REQ: FR_MATCH_07
  togglePoint(side: Side, value: number): void {
  }

  // REQ: FR_MATCH_05
  async undo(elapsedMs: number): Promise<void> {
  }
}

// REQ: NFR_OFFLINE_02
import { y } from './y';
`,
		"web/src/routes/ScoreKeeper.svelte": `<script lang="ts">
  // REQ: US_SK_04
  async function handOver() {}
</script>

<!-- REQ: US_SK_02, US_SK_03 -->
<div></div>
`,
		"web/src/lib/rest.test.ts": `describe('the rest', () => {
  // REQ: FR_REST_01
  it('is offered for whoever fenced the match before', () => {});
});
`,
		"web/e2e/scoring.spec.ts": "// REQ: US_SK_02\ntest('a scored match reaches the server', async () => {});\n",
	})
	ev, errs := Collect([]string{"web"})
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	code := byName(ev[tsCode.File])
	for name, want := range map[string]string{
		"replay": "FR_MATCH_01", "togglePoint": "FR_MATCH_07", "undo": "FR_MATCH_05",
		"engine.ts": "NFR_OFFLINE_02", "handOver": "US_SK_04",
	} {
		if got := code[name]; len(got.Refs) != 1 || got.Refs[0] != want {
			t.Errorf("%s = %+v, want %s", name, got, want)
		}
	}
	if got := code["ScoreKeeper.svelte"]; got.Kind != "File" || len(got.Refs) != 2 {
		t.Errorf("svelte markup tag = %+v, want the file with two refs", got)
	}
	if got := ev[tsTests.File]; len(got) != 1 || got[0].Name != "is offered for whoever fenced the match before" {
		t.Errorf("vitest = %+v", got)
	}
	if got := ev[pwTests.File]; len(got) != 1 || got[0].Kind != "Test" || got[0].Line != 2 {
		t.Errorf("playwright = %+v", got)
	}
}

func TestVectorsNameTheirRequirements(t *testing.T) {
	write(t, map[string]string{
		"testdata/vectors/a.json":    `{"name":"a","requirements":["FR_MATCH_03"],"events":[]}`,
		"testdata/vectors/none.json": `{"name":"none","events":[]}`,
	})
	ev, errs := Collect([]string{"testdata"})
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	if got := ev[vectors.File]; len(got) != 1 || got[0].Tag != "vector testdata/vectors/a.json" {
		t.Errorf("vectors = %+v, want only the one that names a requirement", got)
	}
}

func TestTheFileIsLobstersInterchangeFormat(t *testing.T) {
	dir := t.TempDir()
	items := []Item{{Tag: "go a.go:A", File: "a.go", Line: 3, Name: "A", Kind: "Function", Refs: []string{"FR_MATCH_01"}}}
	for _, lvl := range []Level{goCode, goTests} {
		path := filepath.Join(dir, lvl.File)
		if err := writeLevel(path, lvl, items, "Porta"); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var f struct {
			Schema    string           `json:"schema"`
			Version   int              `json:"version"`
			Generator string           `json:"generator"`
			Data      []map[string]any `json:"data"`
		}
		if err := json.Unmarshal(b, &f); err != nil {
			t.Fatal(err)
		}
		if f.Schema != lvl.Schema || f.Version != 3 || f.Generator == "" || len(f.Data) != 1 {
			t.Fatalf("%s header = %+v", lvl.File, f)
		}
		rec := f.Data[0]
		if refs := rec["refs"].([]any); len(refs) != 1 || refs[0] != "req Porta.FR_MATCH_01" {
			t.Errorf("refs = %v, want the TRLC record lobster-trlc names", refs)
		}
		key := "framework"
		if lvl.Schema == "lobster-imp-trace" {
			key = "language"
		}
		if _, ok := rec[key]; !ok {
			t.Errorf("%s record has no %s: %v", lvl.File, key, rec)
		}
	}
}
