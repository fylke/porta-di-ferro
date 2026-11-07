package httpapi

import "github.com/fylke/porta-di-ferro/internal/hall"

// snapshot builds this server's picture. The work is in BuildSnapshot; what the server
// adds is where the state comes from and who is connected.
//
// In an event, Mats is read off the hall: for every event mat, the match it is on when that
// match is this discipline's. Every client that read Mats before there were several
// disciplines still finds it where it was.
func (s *Server) snapshot() (hall.Snapshot, error) {
	snap, err := s.PlacedSnapshot()
	if err == nil && s.mats != nil {
		var etas map[string]string
		snap.Mats, etas = s.mats.Hall(s.self().Slug)
		hall.ApplyEtas(&snap, etas)
	}
	return snap, err
}
