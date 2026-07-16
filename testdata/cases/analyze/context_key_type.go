package analyze_cases

import "context"

func contextKeyType(ctx context.Context) context.Context {
	return context.WithValue(ctx, "request-id", "value")
}
