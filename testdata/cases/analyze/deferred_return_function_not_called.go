package analyze_cases

func deferredSetup() func() { return func() {} }

func missingDeferredReturnCall() {
	defer deferredSetup()
}
