package analyze_cases

import "runtime"

type cleanFinalizerResource struct{}

func finalizerUsesParameter() {
	object := &cleanFinalizerResource{}
	runtime.SetFinalizer(object, func(object *cleanFinalizerResource) {
		_ = object
	})
}
