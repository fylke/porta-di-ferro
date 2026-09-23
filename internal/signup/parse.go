package signup

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Reading the files.
//
// Everything a participant sends back has been through a mail client, a memory stick and
// possibly a spreadsheet, so none of it is trusted. The errors are written for the
// organizer standing over a folder of forty files wondering which one is the problem:
// they say what the file is, not what the parser expected.

// maxResponse is a limit on one response file. A signup is a few hundred bytes; anything
// past this is not one, and an import screen is not a place to find that out by waiting.
const maxResponse = 64 << 10

// ParseResponse reads one participant's file and checks it is usable.
func ParseResponse(body []byte) (Response, error) {
	if len(body) == 0 {
		return Response{}, fmt.Errorf("the file is empty")
	}
	if len(body) > maxResponse {
		return Response{}, fmt.Errorf("the file is %d kB, which is not a signup", len(body)>>10)
	}

	var res Response
	if err := json.Unmarshal(body, &res); err != nil {
		// A participant who sends the definition back instead of their response is a
		// thing that will happen, and it is worth naming rather than reporting as
		// malformed JSON.
		var probe struct {
			Format string `json:"format"`
		}
		if json.Unmarshal(body, &probe) == nil && probe.Format == DefinitionFormat {
			return Response{}, fmt.Errorf("this is the signup definition, not a filled-in response")
		}
		return Response{}, fmt.Errorf("not readable as JSON")
	}

	switch {
	case res.Format == DefinitionFormat:
		return Response{}, fmt.Errorf("this is the signup definition, not a filled-in response")
	case res.Format == "":
		return Response{}, fmt.Errorf("not a signup file: it has no format marker")
	case res.Format != ResponseFormat:
		return Response{}, fmt.Errorf("not a signup response: it says it is %q", res.Format)
	case res.Version == 0:
		return Response{}, fmt.Errorf("the file does not say which version of the format it is")
	case res.Version > Version:
		return Response{}, fmt.Errorf(
			"written by a newer signup app (version %d; this reads up to %d)", res.Version, Version)
	}

	res.Participant.Name = strings.TrimSpace(res.Participant.Name)
	res.Participant.Club = strings.TrimSpace(res.Participant.Club)
	res.Participant.Contact = strings.TrimSpace(res.Participant.Contact)
	res.SubmissionID = strings.TrimSpace(res.SubmissionID)
	res.DefinitionID = strings.TrimSpace(res.DefinitionID)

	if res.Participant.Name == "" {
		return Response{}, fmt.Errorf("no name in it")
	}
	if len([]rune(res.Participant.Name)) > 80 {
		return Response{}, fmt.Errorf("the name is too long to be one")
	}
	if res.SubmissionID == "" {
		// Without it, importing the same folder twice would add everybody twice. Better
		// to refuse the file than to silently lose the guarantee.
		return Response{}, fmt.Errorf("no submission id, so it cannot be told apart from a second copy")
	}

	// Entries are identifiers, and they reach a comparison against the definition's.
	// Normalising here means a hand-edited file with "Open Sabre" in it still matches.
	cleaned := make([]string, 0, len(res.Entries))
	seen := map[string]bool{}
	for _, e := range res.Entries {
		id := Slug(e)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		cleaned = append(cleaned, id)
	}
	res.Entries = cleaned
	if len(res.Entries) == 0 {
		return Response{}, fmt.Errorf("no disciplines chosen")
	}
	return res, nil
}

// ParseDefinition reads a definition, for the participant app's sake and for a test.
func ParseDefinition(body []byte) (Definition, error) {
	var def Definition
	if err := json.Unmarshal(body, &def); err != nil {
		return Definition{}, fmt.Errorf("not readable as JSON")
	}
	if def.Format != DefinitionFormat {
		return Definition{}, fmt.Errorf("not a signup definition: it says it is %q", def.Format)
	}
	if def.Version > Version {
		return Definition{}, fmt.Errorf(
			"a newer definition (version %d; this reads up to %d)", def.Version, Version)
	}
	return def, nil
}

// Ready reports what still has to be filled in before a definition can go out, so the
// organizer is told on the page rather than by a participant who cannot use the file.
func Ready(def Definition) []string {
	// Empty rather than nil: this crosses the wire, and a nil slice arrives as null,
	// which is a different thing to read a length off.
	missing := []string{}
	if def.DefinitionID == "" {
		missing = append(missing, "an event identifier")
	}
	if def.Event.Name == "" {
		missing = append(missing, "the event's name")
	}
	if len(def.Tournaments) == 0 {
		missing = append(missing, "at least one row of the programme marked as a discipline")
	}
	return missing
}
