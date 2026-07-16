package analyze_cases

func deferredSetupClean() func() { return func() {} }

func callDeferredReturnFunction() {
	defer deferredSetupClean()()
}
