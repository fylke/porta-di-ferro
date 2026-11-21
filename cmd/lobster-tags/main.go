// Command lobster-tags collects the requirement tags in the code and the tests and writes
// them as LOBSTER evidence, so lobster-report can check every requirement in req/ against
// what implements it and what verifies it (docs/requirements-as-code.md).
//
// LOBSTER reads Python, C++ and JSON on its own, but not Go, TypeScript or Svelte, and
// those are what this repository is written in. A tag is a comment:
//
//	// REQ: FR_MATCH_03, FR_MATCH_04
//
// above a declaration (Go, TypeScript) or a test, inside a Go function, or as an HTML
// comment in a Svelte or HTML file, where it stands for the whole file. A shared match
// vector names its requirements in a "requirements" list.
//
// Usage:
//
//	go run ./cmd/lobster-tags -out build/req internal cmd e2e web/src web/e2e web/signup testdata/vectors
//
// It writes one file per tracing level, always all of them so the tracing policy can name
// each one, and fails on a tag it cannot read or place.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	out := flag.String("out", "build/req", "directory to write the .lobster files into")
	pkg := flag.String("package", "Porta", "TRLC package the requirements are declared in")
	flag.Parse()
	roots := flag.Args()
	if len(roots) == 0 {
		fmt.Fprintln(os.Stderr, "usage: lobster-tags [-out dir] [-package name] root...")
		os.Exit(2)
	}

	ev, errs := Collect(roots)
	for _, err := range errs {
		fmt.Fprintln(os.Stderr, err)
	}
	if len(errs) > 0 {
		os.Exit(1)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, lvl := range Levels {
		path := filepath.Join(*out, lvl.File)
		if err := writeLevel(path, lvl, ev[lvl.File], *pkg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("lobster-tags: wrote %d items to %s\n", len(ev[lvl.File]), filepath.ToSlash(path))
	}
}
