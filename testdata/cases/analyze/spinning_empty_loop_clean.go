package analyze_cases

func dynamicEmptyLoop(ready func() bool) {
	for !ready() {
	}
}

func disabledEmptyLoop() {
	for false {
	}
}
