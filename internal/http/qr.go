package httpapi

import (
	"net/http"

	qrcode "github.com/skip2/go-qrcode"
)

// qr renders a QR code for whatever address the organizer view was told to advertise.
//
// It exists so the first screen an organizer sees carries the client address in a form a
// score keeper can join from across a table, rather than one they have to read out loud
// and have typed in wrong.
func (s *Server) qr(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("url")
	if target == "" {
		writeErr(w, http.StatusBadRequest, errMissingURL)
		return
	}
	png, err := qrcode.Encode(target, qrcode.Medium, 512)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(png)
}
