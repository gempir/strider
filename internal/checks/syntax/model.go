//strider:ignore-file confusing-naming
package syntax

import (
	"github.com/gempir/strider/internal/checks/catalog"
	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
)

const (
	nodeAliasDecl        NodeKind = "AliasDecl"
	nodeAssignment       NodeKind = "Assignment"
	nodeBasicLit         NodeKind = "BasicLit"
	nodeBinaryExpression NodeKind = "BinaryExpression"
	nodeBlock            NodeKind = "Block"
	nodeBreakStmt        NodeKind = "BreakStmt"
	nodeConstSpec        NodeKind = "ConstSpec"
	nodeConstSpec2       NodeKind = "ConstSpec2"
	nodeDeferStmt        NodeKind = "DeferStmt"
	nodeExprSwitchStmt   NodeKind = "ExprSwitchStmt"
	nodeFieldDecl        NodeKind = "FieldDecl"
	nodeForStmt          NodeKind = "ForStmt"
	nodeFunctionDecl     NodeKind = "FunctionDecl"
	nodeIfElseStmt       NodeKind = "IfElseStmt"
	nodeIfStmt           NodeKind = "IfStmt"
	nodeImportSpec       NodeKind = "ImportSpec"
	nodeInterfaceType    NodeKind = "InterfaceType"
	nodeMethodDecl       NodeKind = "MethodDecl"
	nodeParameterDecl    NodeKind = "ParameterDecl"
	nodePrimaryExpr      NodeKind = "PrimaryExpr"
	nodeReturnStmt       NodeKind = "ReturnStmt"
	nodeSelectStmt       NodeKind = "SelectStmt"
	nodeShortVarDecl     NodeKind = "ShortVarDecl"
	nodeSourceFile       NodeKind = "SourceFile"
	nodeStructType       NodeKind = "StructType"
	nodeTypeAssertion    NodeKind = "TypeAssertion"
	nodeTypeDef          NodeKind = "TypeDef"
	nodeTypeSwitchStmt   NodeKind = "TypeSwitchStmt"
	nodeUnaryExpr        NodeKind = "UnaryExpr"
	nodeVarDecl          NodeKind = "VarDecl"
	nodeVarSpec          NodeKind = "VarSpec"
	nodeVarSpec2         NodeKind = "VarSpec2"
)

// Meta describes one built-in syntax check.
type Meta = catalog.Meta

// NodeKind identifies a CST shape a syntax check consumes. The native engine
// keeps a single traversal and dispatches only the selected interests.
type NodeKind string

// Check is a concrete-syntax check selected by the registry. The
// traversal owns walking the CST; checks declare their metadata here and are
// the only source of enabled syntax work.
type Check interface {
	catalog.Check
	Interests() []NodeKind
	Start(*Pass)
	Inspect(*Pass, cst.Node)
	Finish(*Pass)
}

type definition struct {
	meta     Meta
	behavior syntaxBehavior
}

type syntaxBehavior struct {
	interests []NodeKind
	start     func(*Pass)
	inspect   func(*Pass, cst.Node)
	finish    func(*Pass)
}

// Finding is a check result before the syntax package converts source positions
// and applies suppression directives.
type Finding struct {
	Node     cst.Node
	Start    int
	End      int
	HasRange bool
	Code     string
	Message  string
	Fixes    []diagnostic.Fix
}

// CSTInput contains everything needed for the concrete-syntax lint pass.
type CSTInput struct {
	Filename string
	Tree     *cst.Tree
	Checks   []Check
	Options  map[string]catalog.ResolvedOptions
	Report   func(Finding)
}

func (check definition) Meta() Meta {
	return catalog.CloneMeta(check.meta)
}

func (check definition) Interests() []NodeKind {
	return append([]NodeKind(nil), check.behavior.interests...)
}

func (check definition) Inspect(pass *Pass, node cst.Node) {
	if check.behavior.inspect != nil {
		check.behavior.inspect(pass, node)
	}
}

func (check definition) Start(pass *Pass) {
	if check.behavior.start != nil {
		check.behavior.start(pass)
	}
}

func (check definition) Finish(pass *Pass) {
	if check.behavior.finish != nil {
		check.behavior.finish(pass)
	}
}
