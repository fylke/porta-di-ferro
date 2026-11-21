package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Level is one LOBSTER tracing level this tool writes, and the file it goes in. The names
// of the files are what lobster.yaml reads.
type Level struct {
	File     string
	Schema   string // lobster-imp-trace or lobster-act-trace
	Language string // for code
	Frame    string // for tests: the framework that runs them
}

var (
	goCode    = Level{File: "go-code.lobster", Schema: "lobster-imp-trace", Language: "Go"}
	goTests   = Level{File: "go-tests.lobster", Schema: "lobster-act-trace", Frame: "go test"}
	tsCode    = Level{File: "ts-code.lobster", Schema: "lobster-imp-trace", Language: "TypeScript"}
	tsTests   = Level{File: "ts-tests.lobster", Schema: "lobster-act-trace", Frame: "Vitest"}
	pwTests   = Level{File: "browser-tests.lobster", Schema: "lobster-act-trace", Frame: "Playwright"}
	vectors   = Level{File: "vectors.lobster", Schema: "lobster-act-trace", Frame: "shared vectors"}
	Levels    = []Level{goCode, goTests, tsCode, tsTests, pwTests, vectors}
	skipDirs  = map[string]bool{"node_modules": true, "dist": true, ".git": true, "build": true, "test-results": true, "playwright-report": true}
	idPattern = regexp.MustCompile(`^[A-Z]+(_[A-Z0-9]+)+$`)
)

// Item is one traced thing: a function, a type, a test, a file, a vector.
type Item struct {
	Tag  string
	File string
	Line int
	Name string
	Kind string
	Refs []string // requirement IDs, as written
}

// Collect walks roots and returns the traced items by level file, and every tag it could
// not read or place.
func Collect(roots []string) (map[string][]Item, []error) {
	ev := map[string][]Item{}
	var errs []error
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipDirs[d.Name()] && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			rel := filepath.ToSlash(path)
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var items []Item
			var lvl Level
			var ferrs []error
			switch {
			case strings.HasSuffix(rel, ".go"):
				items, ferrs = scanGo(rel, src)
				lvl = goCode
				if strings.HasSuffix(rel, "_test.go") {
					lvl = goTests
				}
			case strings.HasSuffix(rel, ".test.ts"):
				items, ferrs = scanScript(rel, src)
				lvl = tsTests
			case strings.HasSuffix(rel, ".spec.ts"):
				items, ferrs = scanScript(rel, src)
				lvl = pwTests
			case strings.HasSuffix(rel, ".ts"), strings.HasSuffix(rel, ".svelte"), strings.HasSuffix(rel, ".html"):
				items, ferrs = scanScript(rel, src)
				lvl = tsCode
			case strings.HasSuffix(rel, ".json") && strings.Contains(rel, "testdata/vectors/"):
				items, ferrs = scanVector(rel, src)
				lvl = vectors
			default:
				return nil
			}
			errs = append(errs, ferrs...)
			ev[lvl.File] = append(ev[lvl.File], items...)
			return nil
		})
		if err != nil {
			errs = append(errs, err)
		}
	}
	for f, items := range ev {
		ev[f] = merge(items)
	}
	return ev, errs
}

// merge folds the tags that land on one item into it, and keeps the output in file order
// so a diff of two runs reads.
func merge(items []Item) []Item {
	byTag := map[string]*Item{}
	var order []string
	for _, it := range items {
		if have, ok := byTag[it.Tag]; ok {
			have.Refs = append(have.Refs, it.Refs...)
			continue
		}
		c := it
		byTag[it.Tag] = &c
		order = append(order, it.Tag)
	}
	out := make([]Item, 0, len(order))
	for _, tag := range order {
		it := *byTag[tag]
		it.Refs = unique(it.Refs)
		out = append(out, it)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

func unique(s []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// tagLine matches a comment line that carries a tag, in any of the comment forms the
// repository's languages use, and captures what follows "REQ:".
var tagLine = regexp.MustCompile(`^\s*(?://+|/\*+|\*|<!--)?\s*REQ:(.*)$`)

// parseIDs reads the IDs after "REQ:": separated by commas or spaces, and ending at the
// first word that is not one, so a tag can carry a note after it.
func parseIDs(rest string) []string {
	rest = strings.TrimSuffix(strings.TrimSpace(rest), "-->")
	rest = strings.TrimSuffix(strings.TrimSpace(rest), "*/")
	var ids []string
	for _, w := range strings.FieldsFunc(rest, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		if !idPattern.MatchString(w) {
			break
		}
		ids = append(ids, w)
	}
	return ids
}

// scanGo reads the tags in a Go file's comments and puts each on the function it sits in,
// or else on the declaration that follows it.
func scanGo(rel string, src []byte) ([]Item, []error) {
	if !bytes.Contains(src, []byte("REQ:")) {
		return nil, nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, parser.ParseComments)
	if err != nil {
		return nil, []error{err}
	}
	test := strings.HasSuffix(rel, "_test.go")
	var items []Item
	var errs []error
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			for i, line := range strings.Split(c.Text, "\n") {
				m := tagLine.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				pos := fset.Position(c.Pos())
				pos.Line += i
				ids := parseIDs(m[1])
				if len(ids) == 0 {
					errs = append(errs, fmt.Errorf("%s:%d: REQ: names no requirement", rel, pos.Line))
					continue
				}
				decl := goDeclFor(f, c.Pos())
				if decl == nil {
					errs = append(errs, fmt.Errorf("%s:%d: REQ: is not above or inside a declaration", rel, pos.Line))
					continue
				}
				name, kind := goName(decl)
				if test {
					kind = "Test"
				}
				items = append(items, Item{
					Tag:  "go " + rel + ":" + name,
					File: rel,
					Line: fset.Position(decl.Pos()).Line,
					Name: name,
					Kind: kind,
					Refs: ids,
				})
			}
		}
	}
	return items, errs
}

// goDeclFor is the function a comment is inside, or else the first declaration after it.
func goDeclFor(f *ast.File, p token.Pos) ast.Decl {
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil && p > fd.Body.Lbrace && p < fd.Body.Rbrace {
			return d
		}
	}
	for _, d := range f.Decls {
		if d.Pos() > p {
			return d
		}
	}
	return nil
}

func goName(d ast.Decl) (name, kind string) {
	switch d := d.(type) {
	case *ast.FuncDecl:
		if d.Recv == nil || len(d.Recv.List) == 0 {
			return d.Name.Name, "Function"
		}
		t := d.Recv.List[0].Type
		if s, ok := t.(*ast.StarExpr); ok {
			t = s.X
		}
		if ix, ok := t.(*ast.IndexExpr); ok {
			t = ix.X
		}
		if id, ok := t.(*ast.Ident); ok {
			return id.Name + "." + d.Name.Name, "Method"
		}
		return d.Name.Name, "Method"
	case *ast.GenDecl:
		for _, s := range d.Specs {
			switch s := s.(type) {
			case *ast.TypeSpec:
				return s.Name.Name, "Type"
			case *ast.ValueSpec:
				if len(s.Names) > 0 {
					return s.Names[0].Name, "Variable"
				}
			}
		}
	}
	return "?", "Declaration"
}

// The declarations a TypeScript or Svelte tag can sit above, and the tests.
var (
	tsDecl = []*regexp.Regexp{
		regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\*?\s+([A-Za-z_$][\w$]*)`),
		regexp.MustCompile(`^\s*(?:export\s+)?(?:abstract\s+)?class\s+([A-Za-z_$][\w$]*)`),
		regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)`),
		regexp.MustCompile(`^\s*(?:export\s+)?(?:interface|type|enum)\s+([A-Za-z_$][\w$]*)`),
		regexp.MustCompile(`^\s*(?:(?:public|private|protected|static|async|get|set)\s+)*([A-Za-z_$][\w$]*)\s*\(.*\)\s*(?::\s*[^={]+)?\{\s*$`),
	}
	tsTest     = regexp.MustCompile("^\\s*(?:test|it|describe)(?:\\.\\w+)?\\(\\s*(['\"`])(.*?)['\"`]")
	tsKeywords = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "function": true, "return": true}
	tsComment  = regexp.MustCompile(`^\s*(//|/\*|\*|<!--|-->)`)
)

// scanScript reads the tags in a TypeScript, Svelte or HTML file. A tag names the
// declaration or test on the next line of code, or, with none there, the file itself.
func scanScript(rel string, src []byte) ([]Item, []error) {
	if !bytes.Contains(src, []byte("REQ:")) {
		return nil, nil
	}
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	file := filepath.Base(rel)
	var items []Item
	var errs []error
	for i, line := range lines {
		m := tagLine.FindStringSubmatch(line)
		if m == nil || !tsComment.MatchString(line) {
			continue
		}
		ids := parseIDs(m[1])
		if len(ids) == 0 {
			errs = append(errs, fmt.Errorf("%s:%d: REQ: names no requirement", rel, i+1))
			continue
		}
		name, kind, at := file, "File", 1
		if !strings.Contains(line, "<!--") {
			if n, k, l, ok := scriptDeclAfter(lines, i+1); ok {
				name, kind, at = n, k, l
			}
		}
		if strings.HasSuffix(rel, ".test.ts") || strings.HasSuffix(rel, ".spec.ts") {
			kind = "Test"
		}
		items = append(items, Item{
			Tag:  "ts " + rel + ":" + strings.ReplaceAll(name, "@", "at"),
			File: rel,
			Line: at,
			Name: name,
			Kind: kind,
			Refs: ids,
		})
	}
	return items, errs
}

// scriptDeclAfter finds the first line of code from line i on and names what it declares.
func scriptDeclAfter(lines []string, i int) (name, kind string, line int, ok bool) {
	for ; i < len(lines); i++ {
		l := lines[i]
		if strings.TrimSpace(l) == "" || tsComment.MatchString(l) {
			continue
		}
		if m := tsTest.FindStringSubmatch(l); m != nil {
			return m[2], "Test", i + 1, true
		}
		for _, re := range tsDecl {
			if m := re.FindStringSubmatch(l); m != nil && !tsKeywords[m[1]] {
				return m[1], "Function", i + 1, true
			}
		}
		return "", "", 0, false
	}
	return "", "", 0, false
}

// scanVector reads a shared match vector's requirements list.
func scanVector(rel string, src []byte) ([]Item, []error) {
	var v struct {
		Name         string   `json:"name"`
		Requirements []string `json:"requirements"`
	}
	if err := json.Unmarshal(bytes.TrimPrefix(src, []byte("\xef\xbb\xbf")), &v); err != nil {
		return nil, []error{fmt.Errorf("%s: %w", rel, err)}
	}
	if len(v.Requirements) == 0 {
		return nil, nil
	}
	for _, id := range v.Requirements {
		if !idPattern.MatchString(id) {
			return nil, []error{fmt.Errorf("%s: %q is not a requirement ID", rel, id)}
		}
	}
	name := v.Name
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(rel), ".json")
	}
	return []Item{{Tag: "vector " + rel, File: rel, Line: 1, Name: name, Kind: "Vector", Refs: v.Requirements}}, nil
}

// writeLevel writes one level's items in LOBSTER's interchange format, with each
// requirement ID as a reference to the TRLC record lobster-trlc names "req <pkg>.<ID>".
func writeLevel(path string, lvl Level, items []Item, pkg string) error {
	data := make([]map[string]any, 0, len(items))
	for _, it := range items {
		refs := make([]string, len(it.Refs))
		for i, id := range it.Refs {
			refs[i] = "req " + pkg + "." + id
		}
		rec := map[string]any{
			"tag":         it.Tag,
			"location":    map[string]any{"kind": "file", "file": it.File, "line": it.Line, "column": nil},
			"name":        it.Name,
			"messages":    []string{},
			"just_up":     []string{},
			"just_down":   []string{},
			"just_global": []string{},
			"refs":        refs,
			"kind":        it.Kind,
		}
		if lvl.Schema == "lobster-imp-trace" {
			rec["language"] = lvl.Language
		} else {
			rec["framework"] = lvl.Frame
			rec["status"] = nil
		}
		data = append(data, rec)
	}
	b, err := json.MarshalIndent(map[string]any{
		"data":      data,
		"generator": "lobster-tags",
		"schema":    lvl.Schema,
		"version":   3,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
