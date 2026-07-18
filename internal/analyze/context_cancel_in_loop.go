package analyze

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/cfg"

	"github.com/gempir/strider/internal/diagnostic"
)

type contextCancelInLoopRule struct {}

func (contextCancelInLoopRule) Meta() Meta {
	return Meta{
		Code: "context-cancel-in-loop",
		Summary: "detect derived contexts whose cancellation is retained across loop iterations",
		Explanation: "Calling context.WithCancel, WithTimeout, or WithDeadline in a loop retains parent and timer resources until its cancellation function runs. A defer in the surrounding function runs too late; cancel explicitly during each iteration or move the iteration body into a helper function.",
		GoodExample: "for _, item := range items { if err := handleItem(ctx, item); err != nil { return err } } // handleItem defers its own cancel",
		BadExample: "for _, item := range items { _, cancel := context.WithCancel(ctx); defer cancel() }",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (contextCancelInLoopRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				switch loop := node.(type) {
				case *ast.ForStmt:
					reportLoopContextCancellations(pass, loop.Body)
				case *ast.RangeStmt:
					reportLoopContextCancellations(pass, loop.Body)
				}
				return true
			},
		)
	}
}

type contextCancelAcquisition struct {
	call *ast.CallExpr
	cancel types.Object
	name string
}

type contextCancelUse struct {
	position token.Pos
	deferred bool
}

func reportLoopContextCancellations(pass *Pass, body *ast.BlockStmt) {
	if body == nil {
		return
	}
	acquisitions := make([]contextCancelAcquisition, 0)
	uses := make(map[types.Object][]contextCancelUse)
	first := true
	ast.Inspect(
		body,
		func(node ast.Node) bool {
			if node == nil {
				return true
			}
			if !first {
				switch node.(type) {
				case *ast.FuncLit,
					*ast.ForStmt,
					*ast.RangeStmt:
					return false
				}
			}
			first = false
			switch node := node.(type) {
			case *ast.AssignStmt:
				acquisitions = append(
					acquisitions,
					contextAcquisitionsFromAssignment(pass, node.Lhs, node.Rhs)...,
				)
			case *ast.ValueSpec:
				left := make([]ast.Expr, 0, len(node.Names))
				for _,
				name := range node.Names {
					left = append(left, name)
				}
				acquisitions = append(
					acquisitions,
					contextAcquisitionsFromAssignment(pass, left, node.Values)...,
				)
			case *ast.ExprStmt:
				if object := calledCancelObject(pass, node.X); object != nil {
					uses[object] = append(uses[object], contextCancelUse{position:
					node.Pos()})
				}
			case *ast.DeferStmt:
				if object := calledCancelObject(pass, node.Call); object != nil {
					uses[object] = append(
						uses[object],
						contextCancelUse{position:
						node.Pos(), deferred:
						true},
					)
				}
			}
			return true
		},
	)
	for _, acquisition := range acquisitions {
		if acquisition.cancel == nil {
			pass.Report(
				acquisition.call,
				acquisition.name + " is created in a loop without retaining and calling its cancellation function during the iteration",
			)
			continue
		}
		end := token.NoPos
		for _, candidate := range acquisitions {
			if candidate.cancel == acquisition.cancel && candidate.call.Pos() > acquisition.call.Pos() && (end == token.NoPos || candidate.call.Pos() < end) {
				end = candidate.call.Pos()
			}
		}
		var deferred *contextCancelUse
		for index := range uses[acquisition.cancel] {
			use := uses[acquisition.cancel][index]
			if use.position <= acquisition.call.Pos() || end != token.NoPos && use.position >= end {
				continue
			}
			if deferred == nil {
				if use.deferred {
					deferred = &use
				}
			}
		}
		if contextCancelledOnEveryLoopPath(body, acquisition, end, uses[acquisition.cancel]) {
			continue
		}
		if deferred != nil {
			pass.Report(
				positionNode{position: deferred.position},
				"cancellation deferred inside a loop runs only when the surrounding function returns; cancel before the iteration ends",
			)
			continue
		}
		pass.Report(
			acquisition.call,
			acquisition.name + " is created in a loop but its cancellation function is not called during the iteration",
		)
	}
}

type contextCancelPathState struct {
	block *cfg.Block
	next int
	active bool
}

func contextCancelledOnEveryLoopPath(
	body *ast.BlockStmt,
	acquisition contextCancelAcquisition,
	end token.Pos,
	uses []contextCancelUse,
) bool {
	graph := cfg.New(body, func(*ast.CallExpr) bool {
		return true
	})
	queue := []contextCancelPathState{{block: graph.Blocks[0]}}
	seen := make(map[contextCancelPathState]bool)
	for len(queue) != 0 {
		state := queue[0]
		queue = queue[1:]
		if seen[state] || state.block == nil || !state.block.Live {
			continue
		}
		seen[state] = true
		if state.next >= len(state.block.Nodes) {
			if state.active && len(state.block.Succs) == 0 {
				return false
			}
			for _, successor := range state.block.Succs {
				queue = append(
					queue,
					contextCancelPathState{block: successor, active: state.active},
				)
			}
			continue
		}
		node := state.block.Nodes[state.next]
		active := state.active
		if nodeContainsPosition(node, acquisition.call.Pos()) {
			if active {
				return false
			}
			active = true
		}
		if active {
			for _, use := range uses {
				if use.deferred || use.position <= acquisition.call.Pos() || end != token.NoPos && use.position >= end || !nodeContainsPosition(
					node,
					use.position,
				) {
					continue
				}
				active = false
				break
			}
		}
		if active && end != token.NoPos && nodeContainsPosition(node, end) {
			return false
		}
		queue = append(
			queue,
			contextCancelPathState{block: state.block, next: state.next + 1, active: active},
		)
	}
	return true
}

func contextAcquisitionsFromAssignment(pass *Pass, left, right []ast.Expr) []contextCancelAcquisition {
	if len(right) != 1 || len(left) < 2 {
		return nil
	}
	call, ok := ast.Unparen(right[0]).(*ast.CallExpr)
	if !ok {
		return nil
	}
	name := contextDerivationName(pass, call)
	if name == "" {
		return nil
	}
	identifier, ok := ast.Unparen(left[1]).(*ast.Ident)
	if !ok || identifier.Name == "_" {
		return[]contextCancelAcquisition{{call: call, name: name}}
	}
	return[]contextCancelAcquisition{
		{call: call, cancel: pass.TypesInfo.ObjectOf(identifier), name: name},
	}
}

func contextDerivationName(pass *Pass, call *ast.CallExpr) string {
	for _, name := range[]string{"WithCancel", "WithTimeout", "WithDeadline"} {
		if isPackageFunction(pass.TypesInfo, call.Fun, "context", name) {
			return "context." + name
		}
	}
	return ""
}

func calledCancelObject(pass *Pass, expression ast.Expr) types.Object {
	call, ok := ast.Unparen(expression).(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil
	}
	identifier, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return nil
	}
	object := pass.TypesInfo.ObjectOf(identifier)
	if object == nil {
		return nil
	}
	signature, ok := types.Unalias(object.Type()).Underlying().(*types.Signature)
	if !ok || signature.Params().Len() != 0 || signature.Results().Len() != 0 {
		return nil
	}
	return object
}
