package semantic

import (
	"go/ast"
	"go/token"
	"go/types"
	"sync"

	"golang.org/x/tools/go/ssa"
)

const syntaxFacts = FactCallArguments | FactParents

const ssaFacts = FactStaticCalls

type packageFactData struct {
	arguments      map[token.Pos][]ast.Node
	firstArguments map[token.Pos]ast.Node
	parents        map[ast.Node]ast.Node
}

type packageFactBuilder func([]*ast.File, FactSet) packageFactData

type packageSSAFactData struct {
	staticCallsByPackage map[string][]ssa.CallInstruction
}

type packageSSAFactBuilder func([]*ssa.Function, FactSet) packageSSAFactData

type packageFacts struct {
	required           FactSet
	staticCallPackages map[string]bool
	syntaxOnce         sync.Once
	ssaOnce            sync.Once
	builder            packageFactBuilder
	ssaBuilder         packageSSAFactBuilder
	data               packageFactData
	ssaData            packageSSAFactData
	deprecatedObjects  map[types.Object]string
	deprecatedPackages map[*types.Package]string
}

func newPackageFacts(required FactSet, staticCallPackages ...map[string]bool) *packageFacts {
	required &= syntaxFacts | ssaFacts
	if required == 0 {
		return nil
	}
	facts := &packageFacts{
		required: required,
	}
	if len(staticCallPackages) != 0 {
		facts.staticCallPackages = staticCallPackages[0]
	}
	return facts
}

func (facts *packageFacts) require(files []*ast.File, wanted FactSet) {
	if facts == nil || !facts.required.Has(wanted) {
		return
	}
	facts.syntaxOnce.Do(func() {
		builder := facts.builder
		if builder == nil {
			builder = buildPackageFacts
		}
		facts.data = builder(files, facts.required)
	})
}

func (facts *packageFacts) requireSSA(functions []*ssa.Function, wanted FactSet) {
	if facts == nil || !facts.required.Has(wanted) {
		return
	}
	facts.ssaOnce.Do(
		func() {
			builder := facts.ssaBuilder
			if builder == nil {
				facts.ssaData = buildPackageSSAFacts(functions, facts.required, facts.staticCallPackages)
				return
			}
			facts.ssaData = builder(functions, facts.required)
		},
	)
}

// buildPackageFacts is the shared typed-AST dispatch hook. Adding another
// syntax fact should extend this visitor instead of introducing another full
// package traversal.
func buildPackageFacts(files []*ast.File, required FactSet) packageFactData {
	result := packageFactData{}
	wantArguments := required.Has(FactCallArguments)
	wantFirstArgument := required.Has(FactCallArguments)
	wantParents := required.Has(FactParents)
	if wantArguments {
		result.arguments = make(map[token.Pos][]ast.Node)
	}
	if wantFirstArgument {
		result.firstArguments = make(map[token.Pos]ast.Node)
	}
	if wantParents {
		result.parents = make(map[ast.Node]ast.Node)
	}
	for _, file := range files {
		stack := make([]ast.Node, 0)
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				if node == nil {
					if wantParents && len(stack) != 0 {
						stack = stack[:len(stack)-1]
					}
					return true
				}
				if wantParents {
					if len(stack) != 0 {
						result.parents[node] = stack[len(stack)-1]
					}
					stack = append(stack, node)
				}
				if !wantArguments && !wantFirstArgument {
					return true
				}
				call,
					ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if wantArguments {
					arguments := make([]ast.Node, len(call.Args))
					for index, argument := range call.Args {
						arguments[index] = argument
					}
					result.arguments[call.Pos()] = arguments
					result.arguments[call.Lparen] = arguments
				}
				if wantFirstArgument && len(call.Args) != 0 {
					result.firstArguments[call.Pos()] = call.Args[0]
					result.firstArguments[call.Lparen] = call.Args[0]
				}
				return true
			},
		)
	}
	return result
}

// buildPackageSSAFacts is the shared SSA dispatch index. It deliberately
// indexes only statically resolved calls: every consumer already rejects
// dynamic calls, and grouping by package keeps each check's candidate set
// small without changing its matching logic.
func buildPackageSSAFacts(functions []*ssa.Function, required FactSet, staticCallPackages ...map[string]bool) packageSSAFactData {
	result := packageSSAFactData{}
	if !required.Has(FactStaticCalls) {
		return result
	}
	packageCount := 0
	if len(staticCallPackages) != 0 {
		packageCount = len(staticCallPackages[0])
	}
	counts := make(map[string]int, packageCount)
	for _, function := range functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}
				callee := call.Common().StaticCallee()
				if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil {
					continue
				}
				packagePath := callee.Object().Pkg().Path()
				if len(staticCallPackages) != 0 && staticCallPackages[0] != nil && !staticCallPackages[0][packagePath] {
					continue
				}
				counts[packagePath]++
			}
		}
	}
	result.staticCallsByPackage = make(map[string][]ssa.CallInstruction, len(counts))
	for packagePath, count := range counts {
		result.staticCallsByPackage[packagePath] = make([]ssa.CallInstruction, 0, count)
	}
	for _, function := range functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}
				callee := call.Common().StaticCallee()
				if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil {
					continue
				}
				packagePath := callee.Object().Pkg().Path()
				if counts[packagePath] == 0 {
					continue
				}
				result.staticCallsByPackage[packagePath] = append(result.staticCallsByPackage[packagePath], call)
			}
		}
	}
	return result
}

func (pass *Pass) argumentsByCallPosition() map[token.Pos][]ast.Node {
	if pass.facts == nil {
		return nil
	}
	pass.facts.require(pass.Files, FactCallArguments)
	return pass.facts.data.arguments
}

func (pass *Pass) firstArgumentsByCallPosition() map[token.Pos]ast.Node {
	if pass.facts == nil {
		return nil
	}
	pass.facts.require(pass.Files, FactCallArguments)
	return pass.facts.data.firstArguments
}

func (pass *Pass) analysisParents() map[ast.Node]ast.Node {
	if pass.facts == nil {
		return nil
	}
	pass.facts.require(pass.Files, FactParents)
	return pass.facts.data.parents
}

func (pass *Pass) staticCallsInPackage(packagePath string) []ssa.CallInstruction {
	if pass.facts == nil || pass.facts.staticCallPackages != nil && !pass.facts.staticCallPackages[packagePath] {
		return nil
	}
	pass.facts.requireSSA(pass.Functions, FactStaticCalls)
	return pass.facts.ssaData.staticCallsByPackage[packagePath]
}
