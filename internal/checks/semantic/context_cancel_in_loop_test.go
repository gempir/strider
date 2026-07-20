package semantic

import "testing"

func TestContextCancelInLoopRequiresIterationBoundedCancel(t *testing.T) {
	reports := runResearchCorrectnessCheck(
		t,
		contextCancelInLoopCheck{},
		`package fixture

import (
	"context"
	"time"
)

func badDeferred(ctx context.Context, items []int) {
	for range items {
		_, cancel := context.WithCancel(ctx)
		defer cancel()
	}
}

func badOmitted(ctx context.Context) {
	for i := 0; i < 2; i++ {
		_, cancel := context.WithTimeout(ctx, time.Second)
		_ = cancel
	}
}

func badIgnored(ctx context.Context, items []int) {
	for range items {
		_, _ = context.WithDeadline(ctx, time.Now())
	}
}

func badConditional(ctx context.Context, items []int, ok bool) {
	for range items {
		_, cancel := context.WithCancel(ctx)
		if ok {
			cancel()
		}
	}
}

func badEarlyContinue(ctx context.Context, items []int, skip bool) {
	for range items {
		_, cancel := context.WithCancel(ctx)
		if skip {
			continue
		}
		cancel()
	}
}

func good(ctx context.Context, items []int) {
	for range items {
		_, cancel := context.WithCancel(ctx)
		cancel()
	}
}

func goodHelper(ctx context.Context, items []int) {
	for range items {
		func() {
			_, cancel := context.WithCancel(ctx)
			defer cancel()
		}()
	}
}
`,
	)
	assertResearchReportCount(t, reports, 5)
}
