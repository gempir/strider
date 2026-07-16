package analyze_cases

func unchangedLoopCondition(limit int) {
	other := 0
	for index := 0; index < limit; other++ {
		if other > limit {
			return
		}
	}
}
