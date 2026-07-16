package analyze_cases

type localCounter struct {
	value int
}

func (counter localCounter) set(value int) {
	counter.value = value
}
