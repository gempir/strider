package analyze_cases

import "context"

func passNonNilContext() {
	loadWithContext(context.TODO())
}
