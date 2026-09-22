//go:build js && wasm

// Command demo-wasm is the demo tournament, compiled for a browser (issue #88).
//
// It is deliberately thin. Everything that decides anything lives in internal/demo,
// which is ordinary portable Go under `go test ./...`; this is the glue that lets a
// browser call it, and glue is the one part of a program that cannot be unit tested, so
// there should be as little of it as possible.
//
// Built by .github/workflows/pages.yml. Locally:
//
//	GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o web/public/demo.wasm ./cmd/demo-wasm
//	cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" web/public/wasm_exec.js
package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/fylke/porta-di-ferro/internal/demo"
)

func main() {
	d := demo.New()

	api := js.Global().Get("Object").New()
	api.Set("request", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return request(d, args)
	}))
	js.Global().Set("portaDemo", api)

	// The loader waits on this rather than polling for the object to appear: Go's main
	// runs after instantiation, so there is a window in which the module is live and
	// this has not been set.
	if ready := js.Global().Get("__portaDemoReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}

	// Go's main returning would tear the runtime down and take every exported function
	// with it. The tab holds this open for as long as it is open.
	select {}
}

// request answers one API call and hands back the response as JSON text.
//
// A panic here would kill the Go runtime and leave the page with an adapter that answers
// nothing, which looks to a visitor like the whole application is broken. So it is
// recovered into a 500, and the demo carries on.
func request(d *demo.Demo, args []js.Value) (out string) {
	defer func() {
		if r := recover(); r != nil {
			b, _ := json.Marshal(demo.Response{
				Status:      500,
				ContentType: "application/json; charset=utf-8",
				Body:        `{"error":"the demo hit a bug answering that. Reset it and it will come back."}`,
			})
			out = string(b)
		}
	}()

	method, path := args[0].String(), args[1].String()
	var body []byte
	if len(args) > 2 && args[2].Type() == js.TypeString {
		body = []byte(args[2].String())
	}

	b, err := json.Marshal(d.Request(method, path, body))
	if err != nil {
		return `{"status":500,"contentType":"application/json","body":"{}"}`
	}
	return string(b)
}
