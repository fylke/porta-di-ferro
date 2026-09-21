package match

// DefaultOptions is what a match looks like until somebody says otherwise: the red side
// in red, the blue side in blue, red on the left of the displays.
func DefaultOptions() Options {
	return Options{Red: "red", Blue: "blue"}
}

// OptionsOf reads the presentation options out of a log: the last options record wins,
// and a log with none has the defaults. Mirrored in web/src/lib/match/options.ts.
//
// It is separate from Replay on purpose. State is what the vectors pin, field for field,
// in both engines; colours are not a scoring matter and must not be able to make a vector
// fail.
func OptionsOf(events []Event) Options {
	out := DefaultOptions()
	for _, e := range events {
		if e.Type != TypeOptions || e.Options == nil {
			continue
		}
		if e.Options.Red != "" {
			out.Red = e.Options.Red
		}
		if e.Options.Blue != "" {
			out.Blue = e.Options.Blue
		}
		out.SwapDisplay = e.Options.SwapDisplay
	}
	return out
}
