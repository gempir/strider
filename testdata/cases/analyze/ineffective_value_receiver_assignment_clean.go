package analyze_cases

type observedCounter struct {
	value int
}

func (counter observedCounter) withValue(value int) int {
	counter.value = value
	return counter.value
}

func (counter *observedCounter) set(value int) {
	counter.value = value
}
