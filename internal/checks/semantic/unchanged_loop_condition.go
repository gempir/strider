package semantic

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type unchangedLoopConditionRule struct{}

func (unchangedLoopConditionRule) Meta() Meta {
	return Meta{
		Code:            "unchanged-loop-condition",
		Summary:         "detect counted loops whose condition variable never changes",
		Explanation:     "A conventional three-part loop that initializes and tests one variable but never changes that variable cannot progress as intended. This often means the post statement increments the wrong counter or is unreachable.",
		GoodExample:     "for index := 0; index < limit; index++ { use(index) }",
		BadExample:      "for index := 0; index < limit; other++ { use(index) }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (unchangedLoopConditionRule) Run(pass *Pass) {
	for _, function := range pass.Functions {
		if function == nil || function.Synthetic != "" || function.Blocks == nil || function.Syntax() == nil {
			continue
		}
		inspectFunctionSyntax(
			function.Syntax(),
			func(node ast.Node) bool {
				loop,
					ok := node.(*ast.ForStmt)
				if !ok {
					return true
				}
				condition,
					ok := unchangedConditionCandidate(pass, loop)
				if !ok {
					return true
				}
				value,
					isAddress := function.ValueForExpr(condition.X)
				if value == nil || isAddress {
					return true
				}
				switch value := value.(type) {
				case *ssa.Phi:
					return true
				case *ssa.UnOp:
					if value.Op == token.MUL {
						return true
					}
				}
				pass.Report(condition, "variable in loop condition never changes")
				return true
			},
		)
	}
}

func unchangedConditionCandidate(pass *Pass, loop *ast.ForStmt) (*ast.BinaryExpr, bool) {
	if loop.Init == nil || loop.Cond == nil || loop.Post == nil {
		return nil, false
	}
	initialization, ok := loop.Init.(*ast.AssignStmt)
	if !ok || len(initialization.Lhs) != 1 || len(initialization.Rhs) != 1 {
		return nil, false
	}
	condition, ok := loop.Cond.(*ast.BinaryExpr)
	if !ok {
		return nil, false
	}
	conditionVariable, ok := condition.X.(*ast.Ident)
	if !ok {
		return nil, false
	}
	initializedVariable, ok := initialization.Lhs[0].(*ast.Ident)
	if !ok || pass.TypesInfo.ObjectOf(conditionVariable) != pass.TypesInfo.ObjectOf(initializedVariable) {
		return nil, false
	}
	if _, ok := loop.Post.(*ast.IncDecStmt); !ok {
		return nil, false
	}
	return condition, true
}

func (unchangedLoopConditionRule) Requirements() Requirements {
	return Requirements{
		Stage:       AnalysisStageSSA,
		SSAFeatures: SSAFeatureGlobalDebug,
	}
}
