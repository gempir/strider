package analyze_cases

import "context"

type requestIDKey struct{}

func namedContextKey(ctx context.Context) context.Context {
	return context.WithValue(ctx, requestIDKey{}, "value")
}
