package analyze_cases

import "context"

func loadWithContext(context.Context) {}

func passNilContext() {
	loadWithContext(nil)
}
