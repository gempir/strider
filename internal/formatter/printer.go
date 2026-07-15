package formatter

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type astBuilder struct {
	filename string
	fset *token.FileSet
	before map[ast.Node][]*ast.CommentGroup
	after map[ast.Node][]*ast.CommentGroup
	headerAfter map[ast.Node][]*ast.CommentGroup
	module string
}

func newASTBuilder(filename string, fset *token.FileSet, file *ast.File) (*astBuilder, error) {
	builder := &astBuilder{
		filename: filename,
		fset: fset,
		before: make(map[ast.Node][]*ast.CommentGroup),
		after: make(map[ast.Node][]*ast.CommentGroup),
		headerAfter: make(map[ast.Node][]*ast.CommentGroup),
		module: findModulePath(filename),
	}
	claimed := claimCaseHeaderComments(builder, file)
	commentMap := ast.NewCommentMap(fset, file, file.Comments)
	for node, groups := range commentMap {
		groups = unclaimedComments(groups, claimed)
		if len(groups) == 0 {
			continue
		}
		if err := builder.attachComments(node, groups); err != nil {
			return nil, err
		}
	}
	return builder, nil
}

func claimCaseHeaderComments(builder *astBuilder, file *ast.File) map[*ast.CommentGroup]bool {
	claimed := make(map[*ast.CommentGroup]bool)
	ast.Inspect(
		file,
		func(node ast.Node) bool {
			colon, ok := clauseColon(node)
			if !ok {
				return true
			}
			colonLine := builder.fset.Position(colon).Line
			for _, group := range file.Comments {
				if builder.fset.Position(group.Pos()).Line == colonLine && group.Pos() > colon {
					builder.headerAfter[node] = append(builder.headerAfter[node], group)
					claimed[group] = true
				}
			}
			return true
		},
	)
	return claimed
}

func clauseColon(node ast.Node) (token.Pos, bool) {
	switch clause := node.(type) {
	case *ast.CaseClause:
		return clause.Colon, true
	case *ast.CommClause:
		return clause.Colon, true
	default:
		return token.NoPos, false
	}
}

func unclaimedComments(groups []*ast.CommentGroup, claimed map[*ast.CommentGroup]bool) []*ast.CommentGroup {
	filtered := groups[:0]
	for _, group := range groups {
		if !claimed[group] {
			filtered = append(filtered, group)
		}
	}
	return filtered
}

func (b *astBuilder) attachComments(node ast.Node, groups []*ast.CommentGroup) error {
	if !commentTarget(node) {
		position := b.fset.Position(groups[0].Pos())
		return &UnsupportedError{
			Filename: b.filename,
			Line: position.Line,
			Column: position.Column,
			Feature: fmt.Sprintf("comments attached to %T", node),
		}
	}
	for _, group := range groups {
		groupLine := b.fset.Position(group.Pos()).Line
		nodeEndLine := b.fset.Position(node.End()).Line
		switch {
		case group.End() <= node.Pos():
			b.before[node] = append(b.before[node], group)
		case groupLine == nodeEndLine:
			b.after[node] = append(b.after[node], group)
		default:
			position := b.fset.Position(group.Pos())
			return &UnsupportedError{
				Filename: b.filename,
				Line: position.Line,
				Column: position.Column,
				Feature: "free-floating comments after syntax nodes",
			}
		}
	}
	return nil
}

func commentTarget(node ast.Node) bool {
	switch node.(type) {
	case *ast.File, ast.Decl, ast.Spec, ast.Stmt, *ast.Field, *ast.KeyValueExpr, *ast.BasicLit:
		return true
	default:
		return false
	}
}

func (b *astBuilder) file(file *ast.File) Doc {
	parts := make([]Doc, 0, len(file.Decls) + 6)
	for _, group := range b.before[file] {
		parts = append(parts, b.comment(group), hard())
		if isBuildConstraint(group) {
			parts = append(parts, hard())
		}
	}
	parts = append(parts, text("package " + file.Name.Name))
	declarations := b.fileDeclarations(file)
	for _, declaration := range declarations {
		parts = append(parts, hard(), hard(), declaration)
	}
	return concat(parts...)
}

func (b *astBuilder) fileDeclarations(file *ast.File) []Doc {
	importSpecs, canConsolidate := b.importsForConsolidation(file)
	return b.emitDeclarations(file, importSpecs, canConsolidate)
}

func (b *astBuilder) importsForConsolidation(file *ast.File) ([]ast.Spec, bool) {
	importSpecs := []ast.Spec{}
	canConsolidate := true
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.IMPORT {
			continue
		}
		importSpecs = append(importSpecs, generic.Specs...)
		if len(b.before[generic]) != 0 || len(b.after[generic]) != 0 ||
		b.specsHaveComments(generic.Specs) {
			canConsolidate = false
		}
	}
	if len(importSpecs) <= 1 {
		canConsolidate = false
	}
	return importSpecs, canConsolidate
}

func (b *astBuilder) emitDeclarations(file *ast.File, importSpecs []ast.Spec, consolidate bool) []Doc {
	docs := make([]Doc, 0, len(file.Decls))
	importsEmitted := false
	for _, declaration := range file.Decls {
		generic, isImport := declaration.(*ast.GenDecl)
		isImport = isImport && generic.Tok == token.IMPORT
		if !consolidate || !isImport {
			docs = append(docs, b.decl(declaration))
			continue
		}
		if importsEmitted {
			continue
		}
		importsEmitted = true
		docs = append(
			docs,
			b.genDecl(&ast.GenDecl{Tok: token.IMPORT, Lparen: token.Pos(1), Specs: importSpecs}),
		)
	}
	return docs
}

func (b *astBuilder) withComments(node ast.Node, core Doc) Doc {
	return b.withCommentsAndSuffix(node, core, nil)
}

func (b *astBuilder) withCommentsAndSuffix(node ast.Node, core, suffix Doc) Doc {
	parts := make([]Doc, 0, len(b.before[node]) * 2 + 2)
	for _, group := range b.before[node] {
		parts = append(parts, b.comment(group), hard())
		groupEndLine := b.fset.Position(group.End()).Line
		nodeStartLine := b.fset.Position(node.Pos()).Line
		if nodeStartLine - groupEndLine > 1 {
			parts = append(parts, hard())
		}
	}
	parts = append(parts, core, suffix)
	for _, group := range b.after[node] {
		parts = append(parts, text(" "), b.comment(group))
	}
	return concat(parts...)
}

func (b *astBuilder) comment(group *ast.CommentGroup) Doc {
	parts := make([]Doc, 0, len(group.List) * 2)
	for index, comment := range group.List {
		if index != 0 {
			parts = append(parts, hard())
		}
		lines := strings.Split(strings.ReplaceAll(comment.Text, "\r\n", "\n"), "\n")
		for lineIndex, line := range lines {
			if lineIndex != 0 {
				parts = append(parts, hard())
			}
			parts = append(parts, text(line))
		}
	}
	return concat(parts...)
}

func (b *astBuilder) decl(node ast.Decl) Doc {
	var core Doc
	switch current := node.(type) {
	case *ast.GenDecl:
		core = b.genDecl(current)
	case *ast.FuncDecl:
		core = b.funcDecl(current)
	default:
		core = text("/* unsupported declaration */")
	}
	return b.withComments(node, core)
}

func (b *astBuilder) genDecl(decl *ast.GenDecl) Doc {
	specs := append([]ast.Spec(nil), decl.Specs...)
	if decl.Tok == token.IMPORT && !b.specsHaveComments(specs) {
		sort.SliceStable(
			specs,
			func(i, j int) bool {
				leftSpec := specs[i].(*ast.ImportSpec)
				rightSpec := specs[j].(*ast.ImportSpec)
				leftCategory := b.importCategory(leftSpec)
				rightCategory := b.importCategory(rightSpec)
				if leftCategory != rightCategory {
					return leftCategory < rightCategory
				}
				left := leftSpec.Path.Value
				right := rightSpec.Path.Value
				return left < right
			},
		)
	}
	if len(specs) == 1 && !decl.Lparen.IsValid() {
		return Group{Doc: concat(text(decl.Tok.String() + " "), b.spec(specs[0]))}
	}
	items := make([]Doc, 0, len(specs) * 2)
	for index, spec := range specs {
		if index != 0 {
			items = append(items, hard())
			if decl.Tok == token.IMPORT &&
			b.importCategory(spec.(*ast.ImportSpec)) !=
			b.importCategory(specs[index - 1].(*ast.ImportSpec)) {
				items = append(items, hard())
			}
		}
		items = append(items, b.spec(spec))
	}
	return concat(
		text(decl.Tok.String() + " ("),
		Indent{Doc: concat(hard(), concat(items...))},
		hard(),
		text(")"),
	)
}

func (b *astBuilder) importCategory(spec *ast.ImportSpec) int {
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		path = spec.Path.Value
	}
	if b.module != "" && (path == b.module || strings.HasPrefix(path, b.module + "/")) {
		return 2
	}
	first := path
	if slash := strings.IndexByte(first, '/'); slash >= 0 {
		first = first[:slash]
	}
	if !strings.Contains(first, ".") {
		return 0
	}
	return 1
}

func findModulePath(filename string) string {
	if filename == "" || strings.HasPrefix(filename, "<") {
		return ""
	}
	directory, err := filepath.Abs(filepath.Dir(filename))
	if err != nil {
		return ""
	}
	for {
		file, err := os.Open(filepath.Join(directory, "go.mod"))
		if err == nil {
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				fields := strings.Fields(scanner.Text())
				if len(fields) == 2 && fields[0] == "module" {
					_ = file.Close()
					return fields[1]
				}
			}
			_ = file.Close()
			return ""
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return ""
		}
		directory = parent
	}
}

func isBuildConstraint(group *ast.CommentGroup) bool {
	for _, comment := range group.List {
		if strings.HasPrefix(comment.Text, "//go:build ") ||
		strings.HasPrefix(comment.Text, "// +build ") {
			return true
		}
	}
	return false
}

func (b *astBuilder) specsHaveComments(specs []ast.Spec) bool {
	for _, spec := range specs {
		if len(b.before[spec]) != 0 || len(b.after[spec]) != 0 {
			return true
		}
	}
	return false
}

func (b *astBuilder) spec(node ast.Spec) Doc {
	var core Doc
	switch current := node.(type) {
	case *ast.ImportSpec:
		parts := []Doc{}
		if current.Name != nil {
			parts = append(parts, text(current.Name.Name + " "))
		}
		parts = append(parts, text(current.Path.Value))
		core = concat(parts...)
	case *ast.ValueSpec:
		names := make([]Doc, 0, len(current.Names))
		for _, name := range current.Names {
			names = append(names, text(name.Name))
		}
		core = join(text(", "), names)
		if current.Type != nil {
			core = concat(core, text(" "), b.expr(current.Type))
		}
		if len(current.Values) != 0 {
			values := b.exprs(current.Values)
			core = concat(core, text(" = "), join(concat(text(","), soft()), values))
		}
	case *ast.TypeSpec:
		core = text(current.Name.Name + " ")
		if current.Assign.IsValid() {
			core = concat(core, text("= "))
		}
		core = concat(core, b.expr(current.Type))
	}
	return b.withComments(node, Group{Doc: core})
}

func (b *astBuilder) funcDecl(decl *ast.FuncDecl) Doc {
	parts := []Doc{text("func")}
	if decl.Recv != nil {
		parts = append(parts, text(" "), b.fieldList(decl.Recv, true))
	}
	parts = append(parts, text(" " + decl.Name.Name), b.signature(decl.Type))
	if decl.Body != nil {
		parts = append(parts, text(" "), b.block(decl.Body))
	}
	return Group{Doc: concat(parts...)}
}

func (b *astBuilder) signature(function *ast.FuncType) Doc {
	result := b.fieldList(function.Params, true)
	if function.Results == nil || len(function.Results.List) == 0 {
		return result
	}
	if len(function.Results.List) == 1 && len(function.Results.List[0].Names) == 0 {
		return concat(result, text(" "), b.expr(function.Results.List[0].Type))
	}
	return concat(result, text(" "), b.fieldList(function.Results, true))
}

func (b *astBuilder) fieldList(list *ast.FieldList, delimit bool) Doc {
	if list == nil || len(list.List) == 0 {
		if delimit {
			return text("()")
		}
		return Text{}
	}
	fields := make([]Doc, 0, len(list.List))
	for _, field := range list.List {
		fields = append(fields, b.field(field))
	}
	if !delimit {
		return join(concat(text(","), soft()), fields)
	}
	return Group{
		Doc: concat(
			text("("),
			Indent{Doc: concat(softBreak(), join(concat(text(","), soft()), fields))},
			IfBreak{Broken: text(",")},
			softBreak(),
			text(")"),
		),
	}
}

func (b *astBuilder) field(field *ast.Field) Doc {
	parts := []Doc{}
	if len(field.Names) != 0 {
		names := make([]Doc, 0, len(field.Names))
		for _, name := range field.Names {
			names = append(names, text(name.Name))
		}
		parts = append(parts, join(text(", "), names), text(" "))
	}
	parts = append(parts, b.expr(field.Type))
	if field.Tag != nil {
		parts = append(parts, text(" " + field.Tag.Value))
	}
	return b.withComments(field, Group{Doc: concat(parts...)})
}

func (b *astBuilder) block(block *ast.BlockStmt) Doc {
	if len(block.List) == 0 {
		return text("{}")
	}
	statements := make([]Doc, 0, len(block.List))
	for _, statement := range block.List {
		statements = append(statements, b.stmt(statement))
	}
	return concat(
		text("{"),
		Indent{Doc: concat(hard(), join(hard(), statements))},
		hard(),
		text("}"),
	)
}

func (b *astBuilder) stmt(node ast.Stmt) Doc {
	core, ok := b.basicStmt(node)
	if !ok {
		core, ok = b.controlStmt(node)
	}
	if !ok {
		core = b.flowStmt(node)
	}
	return b.withComments(node, Group{Doc: core})
}

func (b *astBuilder) basicStmt(node ast.Stmt) (Doc, bool) {
	switch current := node.(type) {
	case *ast.BlockStmt:
		return b.block(current), true
	case *ast.ExprStmt:
		return b.expr(current.X), true
	case *ast.AssignStmt:
		return concat(
			join(text(", "), b.exprs(current.Lhs)),
			text(" " + current.Tok.String() + " "),
			join(text(", "), b.exprs(current.Rhs)),
		), true
	case *ast.ReturnStmt:
		core := text("return")
		if len(current.Results) != 0 {
			core = concat(core, text(" "), join(text(", "), b.exprs(current.Results)))
		}
		return core, true
	case *ast.DeclStmt:
		return b.decl(current.Decl), true
	case *ast.GoStmt:
		return concat(text("go "), b.expr(current.Call)), true
	case *ast.DeferStmt:
		return concat(text("defer "), b.expr(current.Call)), true
	default:
		return nil, false
	}
}

func (b *astBuilder) controlStmt(node ast.Stmt) (Doc, bool) {
	switch current := node.(type) {
	case *ast.IfStmt:
		return b.ifStmt(current), true
	case *ast.ForStmt:
		return b.forStmt(current), true
	case *ast.RangeStmt:
		return b.rangeStmt(current), true
	case *ast.SwitchStmt:
		return b.switchStmt(current), true
	case *ast.TypeSwitchStmt:
		return b.typeSwitchStmt(current), true
	case *ast.SelectStmt:
		return b.selectStmt(current), true
	default:
		return nil, false
	}
}

func (b *astBuilder) flowStmt(node ast.Stmt) Doc {
	switch current := node.(type) {
	case *ast.BranchStmt:
		core := text(current.Tok.String())
		if current.Label != nil {
			core = concat(core, text(" " + current.Label.Name))
		}
		return core
	case *ast.IncDecStmt:
		return concat(b.expr(current.X), text(current.Tok.String()))
	case *ast.SendStmt:
		return concat(b.expr(current.Chan), text(" <- "), b.expr(current.Value))
	case *ast.LabeledStmt:
		return concat(text(current.Label.Name + ":"), hard(), b.stmt(current.Stmt))
	case *ast.EmptyStmt:
		return Text{}
	default:
		return text("/* unsupported statement */")
	}
}

func (b *astBuilder) simpleStmt(node ast.Stmt) Doc {
	if node == nil {
		return Text{}
	}
	return b.stmt(node)
}

func (b *astBuilder) ifStmt(statement *ast.IfStmt) Doc {
	parts := []Doc{text("if ")}
	if statement.Init != nil {
		parts = append(parts, b.simpleStmt(statement.Init), text("; "))
	}
	parts = append(parts, b.expr(statement.Cond), text(" "), b.block(statement.Body))
	if statement.Else != nil {
		parts = append(parts, text(" else "), b.stmt(statement.Else))
	}
	return Group{Doc: concat(parts...)}
}

func (b *astBuilder) forStmt(statement *ast.ForStmt) Doc {
	parts := []Doc{text("for")}
	if statement.Init != nil || statement.Post != nil {
		parts = append(parts, text(" "), b.simpleStmt(statement.Init), text("; "))
		if statement.Cond != nil {
			parts = append(parts, b.expr(statement.Cond))
		}
		parts = append(parts, text("; "), b.simpleStmt(statement.Post))
	} else if statement.Cond != nil {
		parts = append(parts, text(" "), b.expr(statement.Cond))
	}
	parts = append(parts, text(" "), b.block(statement.Body))
	return Group{Doc: concat(parts...)}
}

func (b *astBuilder) rangeStmt(statement *ast.RangeStmt) Doc {
	parts := []Doc{text("for ")}
	if statement.Key != nil {
		parts = append(parts, b.expr(statement.Key))
		if statement.Value != nil {
			parts = append(parts, text(", "), b.expr(statement.Value))
		}
		parts = append(parts, text(" " + statement.Tok.String() + " "))
	}
	parts = append(parts, text("range "), b.expr(statement.X), text(" "), b.block(statement.Body))
	return Group{Doc: concat(parts...)}
}

func (b *astBuilder) switchStmt(statement *ast.SwitchStmt) Doc {
	parts := []Doc{text("switch")}
	if statement.Init != nil {
		parts = append(parts, text(" "), b.simpleStmt(statement.Init), text(";"))
	}
	if statement.Tag != nil {
		parts = append(parts, text(" "), b.expr(statement.Tag))
	}
	clauses := make([]Doc, 0, len(statement.Body.List))
	for _, item := range statement.Body.List {
		clause := item.(*ast.CaseClause)
		header := text("default:")
		if len(clause.List) != 0 {
			header = concat(text("case "), join(text(", "), b.exprs(clause.List)), text(":"))
		}
		for _, group := range b.headerAfter[clause] {
			header = concat(header, text(" "), b.comment(group))
		}
		if len(clause.Body) == 0 {
			clauses = append(clauses, b.withComments(clause, header))
			continue
		}
		body := make([]Doc, 0, len(clause.Body))
		for _, bodyStmt := range clause.Body {
			body = append(body, b.stmt(bodyStmt))
		}
		clauses = append(
			clauses,
			b.withComments(clause, concat(header, Indent{Doc: concat(hard(), join(hard(), body))})),
		)
	}
	parts = append(parts, text(" {"))
	if len(clauses) != 0 {
		parts = append(parts, hard(), join(hard(), clauses), hard())
	}
	parts = append(parts, text("}"))
	return concat(parts...)
}

func (b *astBuilder) typeSwitchStmt(statement *ast.TypeSwitchStmt) Doc {
	parts := []Doc{text("switch")}
	if statement.Init != nil {
		parts = append(parts, text(" "), b.simpleStmt(statement.Init), text(";"))
	}
	parts = append(parts, text(" "), b.simpleStmt(statement.Assign))
	clauses := make([]Doc, 0, len(statement.Body.List))
	for _, item := range statement.Body.List {
		clause := item.(*ast.CaseClause)
		header := text("default:")
		if len(clause.List) != 0 {
			header = concat(text("case "), join(text(", "), b.exprs(clause.List)), text(":"))
		}
		for _, group := range b.headerAfter[clause] {
			header = concat(header, text(" "), b.comment(group))
		}
		if len(clause.Body) == 0 {
			clauses = append(clauses, b.withComments(clause, header))
			continue
		}
		body := make([]Doc, 0, len(clause.Body))
		for _, bodyStmt := range clause.Body {
			body = append(body, b.stmt(bodyStmt))
		}
		clauses = append(
			clauses,
			b.withComments(clause, concat(header, Indent{Doc: concat(hard(), join(hard(), body))})),
		)
	}
	parts = append(parts, text(" {"))
	if len(clauses) != 0 {
		parts = append(parts, hard(), join(hard(), clauses), hard())
	}
	parts = append(parts, text("}"))
	return concat(parts...)
}

func (b *astBuilder) selectStmt(statement *ast.SelectStmt) Doc {
	clauses := make([]Doc, 0, len(statement.Body.List))
	for _, item := range statement.Body.List {
		clause := item.(*ast.CommClause)
		header := text("default:")
		if clause.Comm != nil {
			header = concat(text("case "), b.simpleStmt(clause.Comm), text(":"))
		}
		for _, group := range b.headerAfter[clause] {
			header = concat(header, text(" "), b.comment(group))
		}
		body := make([]Doc, 0, len(clause.Body))
		for _, bodyStmt := range clause.Body {
			body = append(body, b.stmt(bodyStmt))
		}
		if len(body) != 0 {
			header = concat(header, Indent{Doc: concat(hard(), join(hard(), body))})
		}
		clauses = append(clauses, b.withComments(clause, header))
	}
	parts := []Doc{text("select {")}
	if len(clauses) != 0 {
		parts = append(parts, hard(), join(hard(), clauses), hard())
	}
	return concat(append(parts, text("}"))...)
}

func (b *astBuilder) exprs(expressions []ast.Expr) []Doc {
	docs := make([]Doc, 0, len(expressions))
	for _, expression := range expressions {
		docs = append(docs, b.expr(expression))
	}
	return docs
}

func (b *astBuilder) expr(node ast.Expr) Doc {
	if node == nil {
		return Text{}
	}
	if doc, ok := b.primaryExpr(node); ok {
		return doc
	}
	if doc, ok := b.valueExpr(node); ok {
		return doc
	}
	if doc, ok := b.typeExpr(node); ok {
		return doc
	}
	return text("/* unsupported expression " + strconv.Quote(fmt.Sprintf("%T", node)) + " */")
}

func (b *astBuilder) primaryExpr(node ast.Expr) (Doc, bool) {
	switch current := node.(type) {
	case *ast.Ident:
		return text(current.Name), true
	case *ast.BasicLit:
		return b.withComments(current, text(current.Value)), true
	case *ast.SelectorExpr:
		return concat(b.expr(current.X), text("." + current.Sel.Name)), true
	case *ast.ParenExpr:
		return concat(text("("), b.expr(current.X), text(")")), true
	case *ast.UnaryExpr:
		return concat(text(current.Op.String()), b.expr(current.X)), true
	case *ast.BinaryExpr:
		return Group{
			Doc: concat(
				b.expr(current.X),
				text(" " + current.Op.String()),
				soft(),
				b.expr(current.Y),
			),
		}, true
	case *ast.CallExpr:
		return b.callExpr(current), true
	case *ast.IndexExpr:
		return concat(b.expr(current.X), text("["), b.expr(current.Index), text("]")), true
	default:
		return nil, false
	}
}

func (b *astBuilder) valueExpr(node ast.Expr) (Doc, bool) {
	switch current := node.(type) {
	case *ast.SliceExpr:
		return b.sliceExpr(current), true
	case *ast.CompositeLit:
		return b.compositeLit(current), true
	case *ast.KeyValueExpr:
		core := concat(b.expr(current.Key), text(": "), b.expr(current.Value))
		return b.withComments(current, core), true
	case *ast.FuncLit:
		return concat(text("func"), b.signature(current.Type), text(" "), b.block(current.Body)), true
	case *ast.TypeAssertExpr:
		if current.Type == nil {
			return concat(b.expr(current.X), text(".(type)")), true
		}
		return concat(b.expr(current.X), text(".("), b.expr(current.Type), text(")")), true
	case *ast.StarExpr:
		return concat(text("*"), b.expr(current.X)), true
	default:
		return nil, false
	}
}

func (b *astBuilder) sliceExpr(slice *ast.SliceExpr) Doc {
	parts := []Doc{b.expr(slice.X), text("["), b.expr(slice.Low), text(":")}
	if slice.High != nil {
		parts = append(parts, b.expr(slice.High))
	}
	if slice.Slice3 {
		parts = append(parts, text(":"), b.expr(slice.Max))
	}
	return concat(append(parts, text("]"))...)
}

func (b *astBuilder) typeExpr(node ast.Expr) (Doc, bool) {
	switch current := node.(type) {
	case *ast.ArrayType:
		return concat(text("["), b.expr(current.Len), text("]"), b.expr(current.Elt)), true
	case *ast.MapType:
		return concat(text("map["), b.expr(current.Key), text("]"), b.expr(current.Value)), true
	case *ast.StructType:
		return b.structType(current), true
	case *ast.InterfaceType:
		return b.interfaceType(current), true
	case *ast.FuncType:
		return concat(text("func"), b.signature(current)), true
	case *ast.ChanType:
		return concat(text(channelPrefix(current.Dir)), b.expr(current.Value)), true
	case *ast.Ellipsis:
		return concat(text("..."), b.expr(current.Elt)), true
	default:
		return nil, false
	}
}

func channelPrefix(direction ast.ChanDir) string {
	if direction == ast.RECV {
		return "<-chan "
	}
	if direction == ast.SEND {
		return "chan<- "
	}
	return "chan "
}

func (b *astBuilder) callExpr(call *ast.CallExpr) Doc {
	if len(call.Args) == 0 {
		return concat(b.expr(call.Fun), text("()"))
	}
	args := b.exprs(call.Args)
	if call.Ellipsis.IsValid() {
		args[len(args) - 1] = concat(args[len(args) - 1], text("..."))
	}
	return Group{
		Doc: concat(
			b.expr(call.Fun),
			text("("),
			Indent{Doc: concat(softBreak(), join(concat(text(","), soft()), args))},
			IfBreak{Broken: text(",")},
			softBreak(),
			text(")"),
		),
	}
}

func (b *astBuilder) compositeLit(literal *ast.CompositeLit) Doc {
	prefix := b.expr(literal.Type)
	if len(literal.Elts) == 0 {
		return concat(prefix, text("{}"))
	}
	elements := make([]Doc, 0, len(literal.Elts))
	for index, element := range literal.Elts {
		suffix := Doc(text(","))
		if index == len(literal.Elts) - 1 {
			suffix = IfBreak{Broken: text(",")}
		}
		elements = append(elements, b.compositeElement(element, suffix))
	}
	return Group{
		Doc: concat(
			prefix,
			text("{"),
			Indent{Doc: concat(softBreak(), join(soft(), elements))},
			softBreak(),
			text("}"),
		),
	}
}

func (b *astBuilder) compositeElement(element ast.Expr, suffix Doc) Doc {
	switch current := element.(type) {
	case *ast.KeyValueExpr:
		if value, ok := current.Value.(*ast.BasicLit); ok {
			core := concat(b.expr(current.Key), text(": "), text(value.Value))
			return b.withComments(current, b.withCommentsAndSuffix(value, core, suffix))
		}
		core := concat(b.expr(current.Key), text(": "), b.expr(current.Value))
		return b.withCommentsAndSuffix(current, core, suffix)
	case *ast.BasicLit:
		return b.withCommentsAndSuffix(current, text(current.Value), suffix)
	default:
		return concat(b.expr(element), suffix)
	}
}

func (b *astBuilder) structType(structure *ast.StructType) Doc {
	if structure.Fields == nil || len(structure.Fields.List) == 0 {
		return text("struct{}")
	}
	fields := make([]Doc, 0, len(structure.Fields.List))
	for _, field := range structure.Fields.List {
		fields = append(fields, b.field(field))
	}
	return concat(
		text("struct {"),
		Indent{Doc: concat(hard(), join(hard(), fields))},
		hard(),
		text("}"),
	)
}

func (b *astBuilder) interfaceType(iface *ast.InterfaceType) Doc {
	if iface.Methods == nil || len(iface.Methods.List) == 0 {
		return text("interface{}")
	}
	methods := make([]Doc, 0, len(iface.Methods.List))
	for _, method := range iface.Methods.List {
		if function, ok := method.Type.(*ast.FuncType); ok && len(method.Names) != 0 {
			methods = append(
				methods,
				b.withComments(
					method,
					Group{Doc: concat(text(method.Names[0].Name), b.signature(function))},
				),
			)
		} else {
			methods = append(methods, b.field(method))
		}
	}
	return concat(
		text("interface {"),
		Indent{Doc: concat(hard(), join(hard(), methods))},
		hard(),
		text("}"),
	)
}
