package analyze_cases

import "runtime"

type finalizerResource struct{}

func finalizerRetainsObject() {
	object := &finalizerResource{}
	runtime.SetFinalizer(object, func(*finalizerResource) {
		_ = object
	})
}
