package semantic

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"

	"golang.org/x/tools/go/cfg"

	"github.com/gempir/strider/internal/diagnostic"
)

const (
	httpResponseResource acquiredResourceKind = iota + 1
	sqlRowsResource
	sqlStmtResource
	sqlResource // selector used by the rule to include both SQL kinds
)

type unclosedHTTPResponseBodyRule struct{}

type unclosedSQLResourceRule struct{}

type acquiredResourceKind uint8

type acquiredResource struct {
	kind             acquiredResourceKind
	object           types.Object
	acquisitionError types.Object
	node             ast.Node
	start            token.Pos
}

type resourceUse struct {
	object          types.Object
	kind            acquiredResourceKind
	pos             token.Pos
	deferredClosure bool
}

type resourceAssignment struct {
	target      types.Object
	source      types.Object
	kind        acquiredResourceKind
	pos         token.Pos
	acquisition token.Pos
}

type resourcePathState struct {
	block  *cfg.Block
	next   int
	active bool
	owners map[types.Object]bool
}

type resourcePathKey struct {
	block  *cfg.Block
	next   int
	active bool
	owners string
}

type literalPathState struct {
	block   *cfg.Block
	next    int
	reached bool
}

func (unclosedHTTPResponseBodyRule) Meta() Meta {
	return Meta{
		Code:            "unclosed-http-response-body",
		Summary:         "detect locally acquired HTTP response bodies that are not closed",
		Explanation:     "An HTTP response body owns a connection until Body.Close is called. Failing to close a locally acquired response can leak file descriptors and prevent connection reuse.",
		GoodExample:     "response, err := http.Get(url); if err != nil { return err }; defer response.Body.Close()",
		BadExample:      "response, err := http.Get(url); if err != nil { return err }; return decode(response.Body)",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (unclosedHTTPResponseBodyRule) Run(pass *Pass) {
	runUnclosedResourceRule(pass, httpResponseResource)
}

func (unclosedSQLResourceRule) Meta() Meta {
	return Meta{
		Code:            "unclosed-sql-resource",
		Summary:         "detect locally acquired sql.Rows and sql.Stmt values that are not closed",
		Explanation:     "Rows and prepared statements retain database resources until Close is called. Close every locally acquired value, normally with a defer immediately after checking the acquisition error.",
		GoodExample:     "rows, err := db.Query(query); if err != nil { return err }; defer rows.Close()",
		BadExample:      "rows, err := db.Query(query); if err != nil { return err }; return scan(rows)",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (unclosedSQLResourceRule) Run(pass *Pass) {
	runUnclosedResourceRule(pass, sqlResource)
}

func runUnclosedResourceRule(pass *Pass, wanted acquiredResourceKind) {
	forEachAnalysisFunction(
		pass,
		func(body *ast.BlockStmt, signature *types.Signature) {
			if body == nil {
				return
			}
			resources,
				assignments := collectAcquiredResources(pass, body, wanted)
			if len(resources) == 0 {
				return
			}
			closes,
				transfers := collectResourceUses(pass, body, signature, assignments)
			for _, resource := range resources {
				if resourceClosedOnEveryExit(pass, resource, body, closes, transfers, assignments) {
					continue
				}
				message := "locally acquired HTTP response body is not closed on every path before the function exits"
				if resource.kind == sqlRowsResource {
					message = "locally acquired sql.Rows is not closed on every path before the function exits"
				} else if resource.kind == sqlStmtResource {
					message = "locally acquired sql.Stmt is not closed on every path before the function exits"
				}
				pass.Report(resource.node, message)
			}
		},
	)
}

func resourceClosedOnEveryExit(pass *Pass, resource acquiredResource, body *ast.BlockStmt, closes, transfers []resourceUse, assignments []resourceAssignment) bool {
	graph := cfg.New(body, func(*ast.CallExpr) bool {
		return true
	})
	queue := []resourcePathState{
		{
			block: graph.Blocks[0],
		},
	}
	ownerIDs := resourceOwnerIDs(resource, closes, transfers, assignments)
	seen := make(map[resourcePathKey]bool)
	for len(queue) != 0 {
		state := queue[0]
		queue = queue[1:]
		if state.block == nil || !state.block.Live {
			continue
		}
		key := resourcePathKey{
			block:  state.block,
			next:   state.next,
			active: state.active,
			owners: resourceOwnersKey(state.owners, ownerIDs),
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		if state.next >= len(state.block.Nodes) {
			if state.active && len(state.block.Succs) == 0 {
				return false
			}
			for _, successor := range state.block.Succs {
				queue = append(queue, resourcePathState{
					block:  successor,
					active: state.active,
					owners: state.owners,
				})
			}
			continue
		}

		node := state.block.Nodes[state.next]
		active := state.active
		owners := cloneResourceOwners(state.owners)
		if active && (nodeObservesOwnedResource(resource, node, closes, owners, assignments) || nodeObservesOwnedResource(resource, node, transfers, owners, assignments)) {
			active = false
			owners = nil
		}
		var leaked bool
		active, owners, leaked = applyResourceAssignments(resource, node, assignments, active, owners)
		if leaked {
			return false
		}
		if active {
			if returning, ok := node.(*ast.ReturnStmt); ok {
				if resourceUnavailableOnReturn(pass, body, resource, returning) {
					active = false
					owners = nil
				} else {
					return false
				}
			}
		}
		queue = append(queue, resourcePathState{
			block:  state.block,
			next:   state.next + 1,
			active: active,
			owners: owners,
		})
	}
	return true
}

func resourceUnavailableOnReturn(pass *Pass, body *ast.BlockStmt, resource acquiredResource, returning *ast.ReturnStmt) bool {
	if resource.acquisitionError == nil || returning == nil {
		return false
	}
	unavailable := false
	inspectFunctionBody(
		body,
		func(node ast.Node) bool {
			statement,
				ok := node.(*ast.IfStmt)
			if !ok {
				return true
			}
			if objectAssignedBetween(pass, body, resource.acquisitionError, resource.start, statement.Cond.Pos()) {
				return true
			}
			branch := ast.Node(nil)
			if nonNilErrorComparison(pass, statement.Cond, token.NEQ) == resource.acquisitionError {
				branch = statement.Body
			} else if nonNilErrorComparison(pass, statement.Cond, token.EQL) == resource.acquisitionError {
				branch = statement.Else
			}
			if branch != nil && branch.Pos() <= returning.Pos() && returning.End() <= branch.End() {
				unavailable = true
				return false
			}
			return true
		},
	)
	return unavailable
}

func objectAssignedBetween(pass *Pass, body *ast.BlockStmt, object types.Object, start, end token.Pos) bool {
	assigned := false
	inspectFunctionBody(
		body,
		func(node ast.Node) bool {
			if assigned || node == nil || node.Pos() <= start || node.Pos() >= end {
				return !assigned
			}
			assignment,
				ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, expression := range assignment.Lhs {
				identifier,
					ok := ast.Unparen(expression).(*ast.Ident)
				if ok && pass.TypesInfo.ObjectOf(identifier) == object {
					assigned = true
					return false
				}
			}
			return true
		},
	)
	return assigned
}

func nodeContainsPosition(node ast.Node, position token.Pos) bool {
	return node != nil && position.IsValid() && node.Pos() <= position && position < node.End()
}

func nodeObservesOwnedResource(resource acquiredResource, node ast.Node, uses []resourceUse, owners map[types.Object]bool, assignments []resourceAssignment) bool {
	if node == nil {
		return false
	}
	for _, use := range uses {
		if use.pos < node.Pos() || use.pos >= node.End() || use.kind != resource.kind || !owners[use.object] {
			continue
		}
		if use.deferredClosure && resourceObjectAssignedAfter(assignments, use.object, use.kind, use.pos) {
			continue
		}
		return true
	}
	return false
}

func resourceObjectAssignedAfter(assignments []resourceAssignment, object types.Object, kind acquiredResourceKind, position token.Pos) bool {
	for _, assignment := range assignments {
		if assignment.kind == kind && assignment.target == object && assignment.pos > position {
			return true
		}
	}
	return false
}

func applyResourceAssignments(resource acquiredResource, node ast.Node, assignments []resourceAssignment, active bool, owners map[types.Object]bool) (
	bool,
	map[types.Object]bool,
	bool,
) {
	contained := make([]resourceAssignment, 0)
	for _, assignment := range assignments {
		if assignment.kind == resource.kind && nodeContainsPosition(node, assignment.pos) {
			contained = append(contained, assignment)
		}
	}
	if len(contained) == 0 {
		return active, owners, false
	}
	sort.SliceStable(contained, func(left, right int) bool {
		return contained[left].pos < contained[right].pos
	})
	for first := 0; first < len(contained); {
		last := first + 1
		for last < len(contained) && contained[last].pos == contained[first].pos {
			last++
		}
		before := cloneResourceOwners(owners)
		next := cloneResourceOwners(owners)
		for _, assignment := range contained[first:last] {
			if !assignment.acquisition.IsValid() {
				continue
			}
			if assignment.acquisition == resource.start {
				if active {
					// Reaching the same acquisition again while its prior value is
					// still live loses that resource on the next loop iteration.
					return active, owners, true
				}
				active = true
				next = map[types.Object]bool{
					assignment.target: true,
				}
				continue
			}
			if active {
				delete(next, assignment.target)
			}
		}
		for _, assignment := range contained[first:last] {
			if assignment.acquisition.IsValid() || !active {
				continue
			}
			sourceOwned := assignment.source != nil && before[assignment.source]
			delete(next, assignment.target)
			if sourceOwned {
				next[assignment.target] = true
			}
		}
		owners = next
		first = last
	}
	return active, owners, false
}

func cloneResourceOwners(owners map[types.Object]bool) map[types.Object]bool {
	if len(owners) == 0 {
		return nil
	}
	clone := make(map[types.Object]bool, len(owners))
	for object := range owners {
		clone[object] = true
	}
	return clone
}

func resourceOwnerIDs(resource acquiredResource, closes, transfers []resourceUse, assignments []resourceAssignment) map[types.Object]int {
	identifiers := make(map[types.Object]int)
	add := func(object types.Object) {
		if object == nil {
			return
		}
		if _, exists := identifiers[object]; !exists {
			identifiers[object] = len(identifiers)
		}
	}
	add(resource.object)
	for _, use := range append(closes, transfers...) {
		add(use.object)
	}
	for _, assignment := range assignments {
		add(assignment.target)
		add(assignment.source)
	}
	return identifiers
}

func resourceOwnersKey(owners map[types.Object]bool, identifiers map[types.Object]int) string {
	bits := make([]byte, (len(identifiers)+7)/8)
	for object := range owners {
		identifier, exists := identifiers[object]
		if !exists {
			continue
		}
		bits[identifier/8] |= 1 << (identifier % 8)
	}
	return string(bits)
}

func collectAcquiredResources(pass *Pass, body *ast.BlockStmt, wanted acquiredResourceKind) ([]acquiredResource, []resourceAssignment) {
	resources := make([]acquiredResource, 0)
	assignments := make([]resourceAssignment, 0)
	httpBodies := make(map[types.Object]bool)
	inspectFunctionBody(
		body,
		func(node ast.Node) bool {
			var left,
				right []ast.Expr
			switch statement := node.(type) {
			case *ast.AssignStmt:
				left,
					right = statement.Lhs,
					statement.Rhs
			case *ast.ValueSpec:
				left = make([]ast.Expr, 0, len(statement.Names))
				for _, name := range statement.Names {
					left = append(left, name)
				}
				right = statement.Values
			default:
				return true
			}
			collectResourceAssignments(pass, left, right, node, httpBodies, &assignments)
			acquired := resourcesFromAssignment(pass, left, right, node, wanted)
			resources = append(resources, acquired...)
			for _, resource := range acquired {
				assignments = append(assignments, resourceAssignment{
					target:      resource.object,
					kind:        resource.kind,
					pos:         resource.start,
					acquisition: resource.start,
				})
			}
			return true
		},
	)
	return resources, assignments
}

func resourcesFromAssignment(pass *Pass, left, right []ast.Expr, node ast.Node, wanted acquiredResourceKind) []acquiredResource {
	result := make([]acquiredResource, 0)
	if len(right) == 1 {
		call, ok := ast.Unparen(right[0]).(*ast.CallExpr)
		if !ok {
			return result
		}
		valueType := pass.TypesInfo.TypeOf(call)
		if tuple, ok := valueType.(*types.Tuple); ok {
			var acquisitionError types.Object
			for index, expression := range left {
				if index >= tuple.Len() || !isErrorType(tuple.At(index).Type()) {
					continue
				}
				identifier, _ := ast.Unparen(expression).(*ast.Ident)
				if identifier != nil {
					acquisitionError = pass.TypesInfo.ObjectOf(identifier)
				}
			}
			for index, expression := range left {
				if index >= tuple.Len() {
					break
				}
				result = appendAcquiredResource(pass, result, expression, tuple.At(index).Type(), call, call, wanted, acquisitionError)
			}
			return result
		}
		if len(left) == 1 {
			result = appendAcquiredResource(pass, result, left[0], valueType, call, call, wanted, nil)
		}
		return result
	}
	for index, expression := range left {
		if index >= len(right) {
			break
		}
		call, ok := ast.Unparen(right[index]).(*ast.CallExpr)
		if !ok {
			continue
		}
		result = appendAcquiredResource(pass, result, expression, pass.TypesInfo.TypeOf(call), call, node, wanted, nil)
	}
	return result
}

func appendAcquiredResource(
	pass *Pass,
	resources []acquiredResource,
	expression ast.Expr,
	valueType types.Type,
	call *ast.CallExpr,
	node ast.Node,
	wanted acquiredResourceKind,
	acquisitionError types.Object,
) []acquiredResource {
	identifier, ok := ast.Unparen(expression).(*ast.Ident)
	if !ok || identifier.Name == "_" {
		return resources
	}
	kind := acquiredResourceKindForType(valueType)
	if kind == 0 || (wanted == httpResponseResource && kind != wanted) || (wanted == sqlResource && kind != sqlRowsResource && kind != sqlStmtResource) {
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
		kind:             kind,
		object:           object,
		acquisitionError: acquisitionError,
		node:             node,
		start:            node.Pos(),
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
		if function.Pkg().Path() != "database/sql" {
			return false
		}
		switch function.Name() {
		case "Query", "QueryContext":
			return true
		}
	case sqlStmtResource:
		if function.Pkg().Path() != "database/sql" {
			return false
		}
		switch function.Name() {
		case "Prepare", "PrepareContext":
			return true
		}
	}
	return false
}

func acquiredResourceKindForType(valueType types.Type) acquiredResourceKind {
	pointer, ok := types.Unalias(valueType).(*types.Pointer)
	if !ok {
		return 0
	}
	named, ok := types.Unalias(pointer.Elem()).(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
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

func collectResourceAssignments(pass *Pass, left, right []ast.Expr, node ast.Node, httpBodies map[types.Object]bool, assignments *[]resourceAssignment) {
	if len(left) != len(right) {
		return
	}
	for index, leftExpression := range left {
		rightExpression := right[index]
		if selector, ok := ast.Unparen(leftExpression).(*ast.SelectorExpr); ok && selector.Sel.Name == "Body" && acquiredResourceKindForType(
			pass.TypesInfo.TypeOf(selector.X),
		) == httpResponseResource {
			response, _ := ast.Unparen(selector.X).(*ast.Ident)
			target := types.Object(nil)
			if response != nil {
				target = pass.TypesInfo.ObjectOf(response)
			}
			if target != nil {
				*assignments = append(
					*assignments,
					resourceAssignment{
						target: target,
						source: httpBodyAssignmentSource(pass, rightExpression, httpBodies),
						kind:   httpResponseResource,
						pos:    node.Pos(),
					},
				)
			}
			continue
		}

		leftIdentifier, ok := ast.Unparen(leftExpression).(*ast.Ident)
		if !ok || leftIdentifier.Name == "_" {
			continue
		}
		leftObject := pass.TypesInfo.ObjectOf(leftIdentifier)
		if leftObject == nil {
			continue
		}
		leftKind := acquiredResourceKindForType(pass.TypesInfo.TypeOf(leftIdentifier))
		if leftKind != 0 {
			var source types.Object
			if rightIdentifier, rightOK := ast.Unparen(rightExpression).(*ast.Ident); rightOK && acquiredResourceKindForType(pass.TypesInfo.TypeOf(rightIdentifier)) == leftKind {
				source = pass.TypesInfo.ObjectOf(rightIdentifier)
			}
			*assignments = append(*assignments, resourceAssignment{
				target: leftObject,
				source: source,
				kind:   leftKind,
				pos:    node.Pos(),
			})
		}

		bodySource := httpBodyAssignmentSource(pass, rightExpression, httpBodies)
		if bodySource != nil {
			httpBodies[leftObject] = true
			*assignments = append(*assignments, resourceAssignment{
				target: leftObject,
				source: bodySource,
				kind:   httpResponseResource,
				pos:    node.Pos(),
			})
		} else if httpBodies[leftObject] {
			*assignments = append(*assignments, resourceAssignment{
				target: leftObject,
				kind:   httpResponseResource,
				pos:    node.Pos(),
			})
		}
	}
}

func httpBodyAssignmentSource(pass *Pass, expression ast.Expr, httpBodies map[types.Object]bool) types.Object {
	switch expression := ast.Unparen(expression).(type) {
	case *ast.SelectorExpr:
		if expression.Sel.Name != "Body" || acquiredResourceKindForType(pass.TypesInfo.TypeOf(expression.X)) != httpResponseResource {
			return nil
		}
		identifier, _ := ast.Unparen(expression.X).(*ast.Ident)
		if identifier != nil {
			return pass.TypesInfo.ObjectOf(identifier)
		}
	case *ast.Ident:
		object := pass.TypesInfo.ObjectOf(expression)
		if httpBodies[object] {
			return object
		}
	}
	return nil
}

func collectResourceUses(pass *Pass, body *ast.BlockStmt, signature *types.Signature, assignments []resourceAssignment) ([]resourceUse, []resourceUse) {
	closes := make([]resourceUse, 0)
	transfers := make([]resourceUse, 0)
	parents := resourceASTParents(body)
	inspectFunctionBody(
		body,
		func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.CallExpr:
				literal,
					literalCall := ast.Unparen(node.Fun).(*ast.FuncLit)
				_,
					deferred := parents[node].(*ast.DeferStmt)
				_,
					immediate := parents[node].(*ast.ExprStmt)
				if literalCall && (deferred || immediate) {
					collectLiteralResourceCloses(pass, literal.Body, assignments, deferred, &closes)
				}
				if selector,
					ok := ast.Unparen(node.Fun).(*ast.SelectorExpr); ok && selector.Sel.Name == "Close" {
					if object,
						kind := closedResourceObject(pass, selector.X, node.Pos(), assignments); object != nil {
						closes = append(closes, resourceUse{
							object: object,
							kind:   kind,
							pos:    node.Pos(),
						})
					}
				}
			case *ast.ReturnStmt:
				if len(node.Results) == 0 {
					transfers = append(transfers, namedResultResourceUses(signature, assignments, node.Pos())...)
				}
				for _, expression := range node.Results {
					if object,
						kind := transferredResourceObject(pass, expression, assignments); object != nil {
						transfers = append(transfers, resourceUse{
							object: object,
							kind:   kind,
							pos:    expression.Pos(),
						})
					}
				}
			case *ast.AssignStmt:
				if len(node.Lhs) != len(node.Rhs) {
					break
				}
				for index, expression := range node.Rhs {
					if _,
						local := ast.Unparen(node.Lhs[index]).(*ast.Ident); local {
						continue
					}
					if isHTTPResponseBodySelector(pass, node.Lhs[index]) {
						continue
					}
					if object,
						kind := transferredResourceObject(pass, expression, assignments); object != nil {
						transfers = append(transfers, resourceUse{
							object: object,
							kind:   kind,
							pos:    expression.Pos(),
						})
					}
				}
			case *ast.SendStmt:
				if object,
					kind := transferredResourceObject(pass, node.Value, assignments); object != nil {
					transfers = append(transfers, resourceUse{
						object: object,
						kind:   kind,
						pos:    node.Value.Pos(),
					})
				}
			}
			return true
		},
	)
	return closes, transfers
}

func namedResultResourceUses(signature *types.Signature, assignments []resourceAssignment, position token.Pos) []resourceUse {
	if signature == nil || signature.Results() == nil {
		return nil
	}
	uses := make([]resourceUse, 0)
	for index := range signature.Results().Len() {
		result := signature.Results().At(index)
		if result == nil || result.Name() == "" {
			continue
		}
		kind := acquiredResourceKindForType(result.Type())
		if kind == 0 && resourceObjectHasKindBefore(assignments, result, httpResponseResource, position) {
			kind = httpResponseResource
		}
		if kind != 0 {
			uses = append(uses, resourceUse{
				object: result,
				kind:   kind,
				pos:    position,
			})
		}
	}
	return uses
}

func isHTTPResponseBodySelector(pass *Pass, expression ast.Expr) bool {
	selector, ok := ast.Unparen(expression).(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Body" && acquiredResourceKindForType(pass.TypesInfo.TypeOf(selector.X)) == httpResponseResource
}

func resourceASTParents(root ast.Node) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node)
	stack := make([]ast.Node, 0)
	ast.Inspect(
		root,
		func(node ast.Node) bool {
			if node == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if len(stack) != 0 {
				parents[node] = stack[len(stack)-1]
			}
			stack = append(stack, node)
			return true
		},
	)
	return parents
}

func collectLiteralResourceCloses(pass *Pass, body *ast.BlockStmt, assignments []resourceAssignment, deferred bool, closes *[]resourceUse) {
	if body == nil {
		return
	}
	inspectFunctionBody(
		body,
		func(node ast.Node) bool {
			call,
				ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector,
				ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Close" || !literalPositionReachedOnEveryExit(body, call.Pos()) {
				return true
			}
			object,
				kind := closedResourceObject(pass, selector.X, call.Pos(), assignments)
			if object != nil {
				*closes = append(*closes, resourceUse{
					object:          object,
					kind:            kind,
					pos:             call.Pos(),
					deferredClosure: deferred,
				})
			}
			return true
		},
	)
}

func literalPositionReachedOnEveryExit(body *ast.BlockStmt, position token.Pos) bool {
	graph := cfg.New(body, func(*ast.CallExpr) bool {
		return true
	})
	queue := []literalPathState{
		{
			block: graph.Blocks[0],
		},
	}
	seen := make(map[literalPathState]bool)
	for len(queue) != 0 {
		state := queue[0]
		queue = queue[1:]
		if state.block == nil || !state.block.Live || seen[state] {
			continue
		}
		seen[state] = true
		if state.next >= len(state.block.Nodes) {
			if len(state.block.Succs) == 0 && !state.reached {
				return false
			}
			for _, successor := range state.block.Succs {
				queue = append(queue, literalPathState{
					block:   successor,
					reached: state.reached,
				})
			}
			continue
		}
		node := state.block.Nodes[state.next]
		queue = append(queue, literalPathState{
			block:   state.block,
			next:    state.next + 1,
			reached: state.reached || nodeContainsPosition(node, position),
		})
	}
	return true
}

func closedResourceObject(pass *Pass, receiver ast.Expr, position token.Pos, assignments []resourceAssignment) (types.Object, acquiredResourceKind) {
	if identifier, ok := ast.Unparen(receiver).(*ast.Ident); ok {
		object := pass.TypesInfo.ObjectOf(identifier)
		kind := acquiredResourceKindForType(pass.TypesInfo.TypeOf(identifier))
		if kind == sqlRowsResource || kind == sqlStmtResource {
			return object, kind
		}
		if resourceObjectHasKindBefore(assignments, object, httpResponseResource, position) {
			return object, httpResponseResource
		}
	}
	selector, ok := ast.Unparen(receiver).(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Body" || acquiredResourceKindForType(pass.TypesInfo.TypeOf(selector.X)) != httpResponseResource {
		return nil, 0
	}
	identifier, ok := ast.Unparen(selector.X).(*ast.Ident)
	if !ok {
		return nil, 0
	}
	return pass.TypesInfo.ObjectOf(identifier), httpResponseResource
}

func resourceObjectHasKindBefore(assignments []resourceAssignment, object types.Object, kind acquiredResourceKind, position token.Pos) bool {
	for _, assignment := range assignments {
		if assignment.kind == kind && assignment.target == object && assignment.pos < position {
			return true
		}
	}
	return false
}

func transferredResourceObject(pass *Pass, expression ast.Expr, assignments []resourceAssignment) (types.Object, acquiredResourceKind) {
	if identifier, ok := ast.Unparen(expression).(*ast.Ident); ok {
		object := pass.TypesInfo.ObjectOf(identifier)
		kind := acquiredResourceKindForType(pass.TypesInfo.TypeOf(identifier))
		if kind != 0 {
			return object, kind
		}
		if resourceObjectHasKindBefore(assignments, object, httpResponseResource, expression.Pos()) {
			return object, httpResponseResource
		}
	}
	selector, ok := ast.Unparen(expression).(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Body" || acquiredResourceKindForType(pass.TypesInfo.TypeOf(selector.X)) != httpResponseResource {
		return nil, 0
	}
	identifier, ok := ast.Unparen(selector.X).(*ast.Ident)
	if !ok {
		return nil, 0
	}
	return pass.TypesInfo.ObjectOf(identifier), httpResponseResource
}
