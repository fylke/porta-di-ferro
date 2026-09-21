package match

// Ruleset is the hardcoded MSL ruleset (design §5). It is a struct rather than a set of
// constants because Milestone 3 makes rulesets data-driven, and the shared test vectors
// are already the specification such a definition will have to satisfy.
type Ruleset struct {
	// MaxValue is the highest value a single hit can be assessed at.
	MaxValue int `json:"maxValue"`
	// PointCap ends the match once a competitor reaches it. Overshoot is recorded as it
	// happened rather than clamped: point difference feeds two of the four ranking
	// indices, so clamping would quietly distort the standings.
	PointCap int `json:"pointCap"`
	// MatchTimeMS is the nominal match length, and what the final-exchange threshold is
	// derived from. The engine never compares against it: a match does not end when the
	// clock passes it, it ends when the final exchange is confirmed, and the call for
	// that comes ten seconds earlier.
	MatchTimeMS int64 `json:"matchTimeMs"`
	// FinalExchangeMS is when the clock starts flashing -- and, because the flash means
	// "this is the last one", from where a confirmed exchange raises the final-exchange
	// dialog. The two were separate once, the flash a cue with no effect on scoring and
	// the dialog held back until 03:00, which left a ten-second stretch where the clock
	// said the match was ending and confirming an exchange did not offer to end it.
	FinalExchangeMS int64 `json:"finalExchangeMs"`
	// PenaltyLoss is the level at which a competitor loses the match.
	PenaltyLoss int `json:"penaltyLoss"`
	// PenaltyDeduction is the level at which a competitor loses a point.
	PenaltyDeduction int `json:"penaltyDeduction"`
	// ForfeitScore is the score a forfeited or penalty-lost match is recorded at.
	ForfeitScore int `json:"forfeitScore"`
	// Match points for the pool standings. Not the same as points scored.
	WinPoints  int `json:"winPoints"`
	DrawPoints int `json:"drawPoints"`
	LossPoints int `json:"lossPoints"`
}

// MSL is the ruleset for the MVP: MSL's SM ruleset, longsword scoring for all weapons.
func MSL() Ruleset {
	return Ruleset{
		MaxValue:         2,
		PointCap:         8,
		MatchTimeMS:      3 * 60 * 1000,
		FinalExchangeMS:  2*60*1000 + 50*1000,
		PenaltyLoss:      3,
		PenaltyDeduction: 2,
		ForfeitScore:     8,
		WinPoints:        9,
		DrawPoints:       6,
		LossPoints:       3,
	}
}
