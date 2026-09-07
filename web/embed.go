// Package web carries the built Svelte bundle into the binary.
//
// //go:embed is what makes the whole distribution promise work: the binary *is* the web
// app, so there is nothing to deploy separately and a venue with no internet can still
// hand a phone the application (docs/tech-stack.md §1).
//
// dist/ is produced by `npm run build` in this directory. A committed .gitkeep keeps the
// directory present so this embed compiles on a checkout that has not built the frontend
// yet -- the server then reports that the web app is not built into the binary rather than
// failing mysteriously. `vite build` empties the directory, which would delete that
// placeholder, so the build script writes it back afterwards.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets is the bundle rooted at dist/.
func Assets() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
