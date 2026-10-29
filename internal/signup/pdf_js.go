//go:build js

package signup

import "fmt"

const MaxPDFResponse = 2 << 20

func BuildPDF(def Definition) ([]byte, error) {
	return nil, fmt.Errorf("PDF signup forms are not available in the browser demo; use the installed server")
}

func parsePDF(body []byte) (Response, error) {
	return Response{}, fmt.Errorf("PDF signup import is not available in the browser demo; import the filled PDF on the installed server")
}
