package demo

import "strings"

// The demo's routing: a table of the paths it answers, in the shape of the server's own
// ServeMux registrations (httpapi.Server.Handler, httpapi.Coordinator.Handler), so a route
// added there has an obvious place here. Not net/http's mux: a browser tab has no requests,
// only calls, and the first route that matches answers, in the order they are listed.

// call is one API call as a route sees it.
type call struct {
	method string
	// path is the call's path as it came, query and all; bare is it without the query or
	// a trailing slash, and parts its segments.
	path, bare, query string
	parts             []string
	body              []byte
	params            map[string]string
}

// param is a segment the route's pattern named.
func (c call) param(name string) string { return c.params[name] }

// route is one path: a method ("" for any) and a pattern of literal segments, {name} for
// one segment, and {name...} for one or more.
type route struct {
	method, pattern string
	answer          func(call) Response
}

// on is a route, written the way the server registers its handlers: "GET /api/state", or
// "/api/clients/{rest...}" for any method.
func on(pattern string, answer func(call) Response) route {
	method, path, found := strings.Cut(pattern, " ")
	if !found {
		method, path = "", pattern
	}
	return route{method: method, pattern: path, answer: answer}
}

func newCall(method, path string, body []byte) call {
	bare, query, _ := strings.Cut(path, "?")
	bare = strings.TrimSuffix(bare, "/")
	return call{method: method, path: path, bare: bare, query: query, body: body,
		parts: strings.Split(strings.TrimPrefix(bare, "/"), "/")}
}

// match says whether the route answers the call, and with what parameters.
func (r route) match(c call) (map[string]string, bool) {
	if r.method != "" && r.method != c.method {
		return nil, false
	}
	pattern := strings.Split(strings.TrimPrefix(r.pattern, "/"), "/")
	params := map[string]string{}
	for i, seg := range pattern {
		if name, ok := strings.CutSuffix(seg, "...}"); ok && strings.HasPrefix(name, "{") {
			if i >= len(c.parts) {
				return nil, false
			}
			params[name[1:]] = strings.Join(c.parts[i:], "/")
			return params, true
		}
		if i >= len(c.parts) {
			return nil, false
		}
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			params[seg[1:len(seg)-1]] = c.parts[i]
		} else if seg != c.parts[i] {
			return nil, false
		}
	}
	return params, len(pattern) == len(c.parts)
}

// dispatch answers a call by the first route that matches it.
func dispatch(routes []route, c call) (Response, bool) {
	for _, r := range routes {
		if params, ok := r.match(c); ok {
			c.params = params
			return r.answer(c), true
		}
	}
	return Response{}, false
}
