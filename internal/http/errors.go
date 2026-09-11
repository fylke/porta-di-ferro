package httpapi

import "errors"

var errNoOverride = errors.New(`send {"mat": N} to move a pool, or {"move": "up"|"down"} to reorder it`)

var errMissingURL = errors.New("a url query parameter is required")
