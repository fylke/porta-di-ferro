package forecast

import "time"

func calibratedPace(it Item, tpl Timings, p Pace, scales map[string]float64) Pace {
	if it.Kind == KindPool {
		return poolPace(it, tpl, p)
	}
	if scale, ok := scales[it.Discipline]; ok {
		p.Match += time.Duration(float64(tpl.Match) * (scale - 1) * p.templateWeight)
		p.Changeover += time.Duration(float64(tpl.Changeover) * (scale - 1) * p.changeoverWeight)
	}
	return p
}

func eliminationScales(in Input) map[string]float64 {
	if in.Timings.Match != Defaults.Match || in.Timings.Changeover != Defaults.Changeover {
		return nil
	}
	byDiscipline := map[string][]Item{}
	for _, it := range in.Items {
		switch it.Kind {
		case KindEliminations, KindBronze, KindFinal:
			it.Held, it.WithFinal, it.Session = false, false, 0
			it.NotBefore = time.Time{}
			byDiscipline[it.Discipline] = append(byDiscipline[it.Discipline], it)
		}
	}
	totals := map[int]map[int]time.Duration{
		8:  {1: 45 * time.Minute, 2: 40 * time.Minute},
		16: {1: 85 * time.Minute, 2: 55 * time.Minute, 3: 50 * time.Minute, 4: 45 * time.Minute},
		32: {1: 160 * time.Minute, 2: 80 * time.Minute, 3: 65 * time.Minute, 4: 55 * time.Minute},
	}
	scales := map[string]float64{}
	for discipline, items := range byDiscipline {
		count, bronze, final := 0, 0, 0
		mats := map[int]bool{}
		for _, it := range items {
			count += len(it.Matches)
			switch it.Kind {
			case KindEliminations:
				if len(it.Matches) > 0 {
					mats[it.Mat] = true
				}
			case KindBronze:
				bronze += len(it.Matches)
			case KindFinal:
				final += len(it.Matches)
			}
		}
		target := totals[count][len(mats)]
		if target == 0 || bronze != 1 || final != 1 {
			continue
		}
		stage := Input{Items: items, Timings: in.Timings}
		baseline, _ := simulateCalibrated(stage, nil, false, nil)
		end := in.Timings.Start
		for _, span := range baseline.items {
			if span.end.After(end) {
				end = span.end
			}
		}
		if duration := end.Sub(in.Timings.Start); duration > 0 {
			scales[discipline] = float64(target) / float64(duration)
		}
	}
	return scales
}
