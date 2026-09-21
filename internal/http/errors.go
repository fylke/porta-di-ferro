package httpapi

import "errors"

var errNoOverride = errors.New(`send {"mat": N} to move a pool, or {"move": "up"|"down"} to reorder it`)

var errPoolsNotDone = errors.New("the eliminations are drawn from finished pools; some pool matches are still to be scored")

var errMissingURL = errors.New("a url query parameter is required")
