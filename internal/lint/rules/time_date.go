package rules

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
)

func (a *analyzer) checkTimeDate(call *ast.CallExpr) {
	limits := []struct {
		index, minimum, maximum int
		label                   string
	}{
		{1, 1, 12, "month"},
		{2, 1, 31, "day"},
		{3, 0, 23, "hour"},
		{4, 0, 59, "minute"},
		{5, 0, 59, "second"},
		{6, 0, 999999999, "nanosecond"},
	}
	for _, limit := range limits {
		if limit.index >= len(call.Args) {
			continue
		}
		literal, ok := call.Args[limit.index].(*ast.BasicLit)
		if !ok || literal.Kind != token.INT {
			continue
		}
		value, err := strconv.Atoi(literal.Value)
		if err == nil && (value < limit.minimum || value > limit.maximum) {
			a.report(
				"time-date",
				literal,
				fmt.Sprintf(
					"time.Date %s argument %d is outside %d..%d",
					limit.label,
					value,
					limit.minimum,
					limit.maximum,
				),
			)
		}
	}
}
