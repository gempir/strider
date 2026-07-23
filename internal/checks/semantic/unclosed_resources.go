package semantic

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/gempir/strider/internal/diagnostic"
)

const (
	httpResponseResource acquiredResourceKind = iota + 1
	sqlRowsResource
	sqlStmtResource
	sqlResource
)

type unclosedHTTPResponseBodyCheck struct{}

type unclosedSQLResourceCheck struct{}

type acquiredResourceKind uint8

type acquiredResource struct {
	kind   acquiredResourceKind
	object types.Object
	node   ast.Node
	start  token.Pos
}

type resourceEvent struct {
	object   types.Object
	kind     acquiredResourceKind
	pos      token.Pos
	released bool
}

func (unclosedHTTPResponseBodyCheck) Meta() Meta {
	return Meta{
		Code:            "unclosed-http-response-body",
		Summary:         "detect locally acquired HTTP response bodies that are not closed",
		Explanation:     "An HTTP response body owns a connection until Body.Close is called. Close or return each locally acquired response before replacing its variable or leaving the function.",
		GoodExample:     "response, err := http.Get(url); if err != nil { return err }; defer response.Body.Close()",
		BadExample:      "response, err := http.Get(url); if err != nil { return err }; return decode(response.Body)",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (unclosedHTTPResponseBodyCheck) Run(pass *Pass) {
	runUnclosedResourceCheck(pass, httpResponseResource)
}

func (unclosedSQLResourceCheck) Meta() Meta {
	return Meta{
		Code:            "unclosed-sql-resource",
		Summary:         "detect locally acquired sql.Rows and sql.Stmt values that are not closed",
		Explanation:     "Rows and prepared statements retain database resources until Close is called. Close or return each directly owned local value before replacing it or leaving the function.",
		GoodExample:     "rows, err := db.Query(query); if err != nil { return err }; defer rows.Close()",
		BadExample:      "rows, err := db.Query(query); if err != nil { return err }; return scan(rows)",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (unclosedSQLResourceCheck) Run(pass *Pass) {
	runUnclosedResourceCheck(pass, sqlResource)
}

// The check deliberately follows direct local owners only. It catches the
// common missing-defer mistake without embedding a path-sensitive alias and
// ownership interpreter in one rule. Aliased or conditionally managed
// resources require a dedicated ownership analysis and are left unreported.
func runUnclosedResourceCheck(pass *Pass, wanted acquiredResourceKind) {
	forEachAnalysisFunction(
		pass,
		func(body *ast.BlockStmt, signature *types.Signature) {
			if body == nil {
				return
			}
			resources := collectDirectResourceAcquisitions(pass, body, wanted)
			if len(resources) == 0 {
				return
			}
			events := collectDirectResourceEvents(pass, body, signature)
			for index, resource := range resources {
				if directResourceReleased(resource, resources[index+1:], events) {
					continue
				}
				message := "locally acquired HTTP response body is not directly closed or transferred before replacement or function exit"
				if resource.kind == sqlRowsResource {
					message = "locally acquired sql.Rows is not directly closed or transferred before replacement or function exit"
				} else if resource.kind == sqlStmtResource {
					message = "locally acquired sql.Stmt is not directly closed or transferred before replacement or function exit"
				}
				pass.Report(resource.node, message)
			}
		},
	)
}

func directResourceReleased(resource acquiredResource, later []acquiredResource, events []resourceEvent) bool {
	boundary := token.NoPos
	for _, candidate := range later {
		if candidate.object == resource.object && candidate.kind == resource.kind {
			boundary = candidate.start
			break
		}
	}
	for _, event := range events {
		if event.object != resource.object || event.kind != resource.kind || event.pos <= resource.start {
			continue
		}
		if boundary.IsValid() && event.pos >= boundary {
			break
		}
		return event.released
	}
	return false
}

func collectDirectResourceAcquisitions(pass *Pass, body *ast.BlockStmt, wanted acquiredResourceKind) []acquiredResource {
	result := []acquiredResource{}
	inspectFunctionBody(
		body,
		func(node ast.Node) bool {
			var left, right []ast.Expr
			switch current := node.(type) {
			case *ast.AssignStmt:
				left, right = current.Lhs, current.Rhs
			case *ast.ValueSpec:
				left = make([]ast.Expr, 0, len(current.Names))
				for _, name := range current.Names {
					left = append(left, name)
				}
				right = current.Values
			default:
				return true
			}
			result = append(result, directResourcesFromAssignment(pass, left, right, node, wanted)...)
			return true
		},
	)
	return result
}

func directResourcesFromAssignment(pass *Pass, left, right []ast.Expr, node ast.Node, wanted acquiredResourceKind) []acquiredResource {
	result := []acquiredResource{}
	if len(right) == 1 {
		call, ok := ast.Unparen(right[0]).(*ast.CallExpr)
		if !ok {
			return result
		}
		valueType := pass.TypesInfo.TypeOf(call)
		if tuple, ok := valueType.(*types.Tuple); ok {
			for index, expression := range left {
				if index >= tuple.Len() {
					break
				}
				result = appendDirectResource(pass, result, expression, tuple.At(index).Type(), call, node, wanted)
			}
			return result
		}
		if len(left) == 1 {
			result = appendDirectResource(pass, result, left[0], valueType, call, node, wanted)
		}
		return result
	}
	for index, expression := range left {
		if index >= len(right) {
			break
		}
		call, ok := ast.Unparen(right[index]).(*ast.CallExpr)
		if ok {
			result = appendDirectResource(pass, result, expression, pass.TypesInfo.TypeOf(call), call, node, wanted)
		}
	}
	return result
}

func appendDirectResource(pass *Pass, resources []acquiredResource, expression ast.Expr, valueType types.Type, call *ast.CallExpr, node ast.Node, wanted acquiredResourceKind) []acquiredResource {
	identifier, ok := ast.Unparen(expression).(*ast.Ident)
	if !ok || identifier.Name == "_" {
		return resources
	}
	kind := acquiredResourceKindForType(valueType)
	if kind == 0 || wanted == httpResponseResource && kind != wanted || wanted == sqlResource && kind != sqlRowsResource && kind != sqlStmtResource {
		return resources
	}
	if !knownResourceAcquisition(pass, call, kind) {
		return resources
	}
	object := pass.TypesInfo.ObjectOf(identifier)
	if object == nil || object.Parent() == pass.Types.Scope() {
		return resources
	}
	return append(resources, acquiredResource{
		kind:   kind,
		object: object,
		node:   node,
		start:  node.Pos(),
	})
}

func knownResourceAcquisition(pass *Pass, call *ast.CallExpr, kind acquiredResourceKind) bool {
	if call == nil {
		return false
	}
	function := calledFunction(pass.TypesInfo, call.Fun)
	if function == nil || function.Pkg() == nil {
		return false
	}
	switch kind {
	case httpResponseResource:
		if function.Pkg().Path() != "net/http" {
			return false
		}
		switch function.Name() {
		case "Do", "Get", "Head", "Post", "PostForm", "RoundTrip":
			return true
		}
	case sqlRowsResource:
		return function.Pkg().Path() == "database/sql" && (function.Name() == "Query" || function.Name() == "QueryContext")
	case sqlStmtResource:
		return function.Pkg().Path() == "database/sql" && (function.Name() == "Prepare" || function.Name() == "PrepareContext")
	}
	return false
}

func acquiredResourceKindForType(valueType types.Type) acquiredResourceKind {
	if valueType == nil {
		return 0
	}
	if _, ok := types.Unalias(valueType).(*types.Pointer); !ok {
		return 0
	}
	named := namedType(valueType)
	if named == nil || named.Obj().Pkg() == nil {
		return 0
	}
	switch named.Obj().Pkg().Path() + "." + named.Obj().Name() {
	case "net/http.Response":
		return httpResponseResource
	case "database/sql.Rows":
		return sqlRowsResource
	case "database/sql.Stmt":
		return sqlStmtResource
	default:
		return 0
	}
}

func collectDirectResourceEvents(pass *Pass, body *ast.BlockStmt, signature *types.Signature) []resourceEvent {
	result := []resourceEvent{}
	inspectFunctionBody(
		body,
		func(node ast.Node) bool {
			switch current := node.(type) {
			case *ast.CallExpr:
				selector, ok := ast.Unparen(current.Fun).(*ast.SelectorExpr)
				if ok && selector.Sel.Name == "Close" {
					if object, kind := directResourceObject(pass, selector.X); object != nil {
						result = append(result, resourceEvent{
							object:   object,
							kind:     kind,
							pos:      current.Pos(),
							released: true,
						})
					}
				}
			case *ast.ReturnStmt:
				if len(current.Results) == 0 {
					result = append(result, namedResultEvents(signature, current.Pos())...)
				}
				for _, expression := range current.Results {
					result = appendDirectTransfer(pass, result, expression)
				}
			case *ast.SendStmt:
				result = appendDirectTransfer(pass, result, current.Value)
			case *ast.AssignStmt:
				for index, expression := range current.Lhs {
					if selector, ok := ast.Unparen(expression).(*ast.SelectorExpr); ok && selector.Sel.Name == "Body" {
						if object, kind := directResourceObject(pass, selector); object != nil {
							result = append(result, resourceEvent{
								object: object,
								kind:   kind,
								pos:    current.Pos(),
							})
						}
					}
					if index < len(current.Rhs) {
						result = appendEscapingResource(pass, result, expression, current.Rhs[index])
					}
				}
			}
			return true
		},
	)
	return result
}

func appendEscapingResource(pass *Pass, events []resourceEvent, target, source ast.Expr) []resourceEvent {
	object, _ := directResourceObject(pass, source)
	if object == nil {
		return events
	}
	identifier, local := ast.Unparen(target).(*ast.Ident)
	if local && identifier.Name == "_" {
		return events
	}
	if local && pass.TypesInfo.ObjectOf(identifier) == object {
		return events
	}
	return appendDirectTransfer(pass, events, source)
}

func appendDirectTransfer(pass *Pass, events []resourceEvent, expression ast.Expr) []resourceEvent {
	object, kind := directResourceObject(pass, expression)
	if object == nil {
		return events
	}
	return append(events, resourceEvent{
		object:   object,
		kind:     kind,
		pos:      expression.Pos(),
		released: true,
	})
}

func directResourceObject(pass *Pass, expression ast.Expr) (types.Object, acquiredResourceKind) {
	switch current := ast.Unparen(expression).(type) {
	case *ast.Ident:
		return pass.TypesInfo.ObjectOf(current), acquiredResourceKindForType(pass.TypesInfo.TypeOf(current))
	case *ast.SelectorExpr:
		if current.Sel.Name != "Body" || acquiredResourceKindForType(pass.TypesInfo.TypeOf(current.X)) != httpResponseResource {
			return nil, 0
		}
		identifier, ok := ast.Unparen(current.X).(*ast.Ident)
		if !ok {
			return nil, 0
		}
		return pass.TypesInfo.ObjectOf(identifier), httpResponseResource
	default:
		return nil, 0
	}
}

func namedResultEvents(signature *types.Signature, position token.Pos) []resourceEvent {
	if signature == nil || signature.Results() == nil {
		return nil
	}
	result := []resourceEvent{}
	for index := range signature.Results().Len() {
		variable := signature.Results().At(index)
		kind := acquiredResourceKindForType(variable.Type())
		if variable.Name() != "" && kind != 0 {
			result = append(result, resourceEvent{
				object:   variable,
				kind:     kind,
				pos:      position,
				released: true,
			})
		}
	}
	return result
}
