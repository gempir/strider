package analyze_cases

import "context"

type contextService struct{}

func (contextService) Run(ctx context.Context) {
	_ = ctx
}
