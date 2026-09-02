package match

import (
	"bytes"
	"io"
)

// newTrimReader strips a UTF-8 byte order mark, which Windows editors add and the JSON
// decoder rejects. Worth handling: the organizer is expected to open these files.
func newTrimReader(b []byte) io.Reader {
	return bytes.NewReader(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")))
}
