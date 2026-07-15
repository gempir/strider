// Package formatter implements Strider's strict, width-aware Go formatter.
package formatter

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	goformat "go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"reflect"
	"sort"
	"strings"
)

const PrintWidth = 100

type unsupportedErrorValue string

func (value unsupportedErrorValue) Error() string { return string(value) }

const ErrUnsupported unsupportedErrorValue = "unsupported Go syntax"

type UnsupportedError struct {
	Filename string
	Line     int
	Column   int
	Feature  string
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("%s:%d:%d: %v: %s", e.Filename, e.Line, e.Column, ErrUnsupported, e.Feature)
}

func (e *UnsupportedError) Unwrap() error { return ErrUnsupported }

type Result struct {
	Source  []byte
	Changed bool
	Ignored bool
}

func Format(filename string, source []byte) (Result, error) {
	if bytes.Contains(source, []byte("//strider:format-ignore")) {
		copyOfSource := append([]byte(nil), source...)
		return Result{Source: copyOfSource, Ignored: true}, nil
	}

	formatted, err := formatInternal(filename, source)
	if err != nil {
		return Result{}, err
	}
	if err := equivalent(filename, source, formatted); err != nil {
		return Result{}, fmt.Errorf("formatter safety check: %w", err)
	}
	second, err := formatInternal(filename, formatted)
	if err != nil {
		return Result{}, fmt.Errorf("formatter idempotence check: %w", err)
	}
	if !bytes.Equal(formatted, second) {
		return Result{}, fmt.Errorf("formatter idempotence check failed for %s", filename)
	}
	return Result{Source: formatted, Changed: !bytes.Equal(source, formatted)}, nil
}

func formatInternal(filename string, source []byte) ([]byte, error) {
	fset, file, err := parse(filename, source)
	if err != nil {
		return nil, err
	}
	if err := validateSupported(filename, fset, file); err != nil {
		return nil, err
	}
	builder, err := newASTBuilder(filename, fset, file)
	if err != nil {
		return nil, err
	}
	output := strings.TrimRight(Render(builder.file(file), PrintWidth), " \t\r\n") + "\n"
	return []byte(output), nil
}

func parse(filename string, source []byte) (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, parser.ParseComments|parser.AllErrors|parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}

	var scan scanner.Scanner
	tokenFile := fset.AddFile(filename+"#scan", -1, len(source))
	scan.Init(tokenFile, source, nil, scanner.ScanComments)
	for {
		_, tok, _ := scan.Scan()
		if tok == token.EOF {
			break
		}
	}
	if scan.ErrorCount != 0 {
		return nil, nil, fmt.Errorf("%s: scanner found %d error(s)", filename, scan.ErrorCount)
	}
	return fset, file, nil
}

func validateSupported(filename string, fset *token.FileSet, file *ast.File) error {
	var unsupported ast.Node
	feature := ""
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil || unsupported != nil {
			return unsupported == nil
		}
		feature = unsupportedFeature(node)
		if feature != "" {
			unsupported = node
		}
		return unsupported == nil
	})
	if unsupported == nil {
		return nil
	}
	position := fset.Position(unsupported.Pos())
	return &UnsupportedError{Filename: filename, Line: position.Line, Column: position.Column, Feature: feature}
}

func unsupportedFeature(node ast.Node) string {
	if feature := unsupportedNodeKind(node); feature != "" {
		return feature
	}
	return unsupportedTypeParameters(node)
}

func unsupportedNodeKind(node ast.Node) string {
	switch current := node.(type) {
	case *ast.BadDecl, *ast.BadExpr, *ast.BadStmt:
		return "invalid AST node"
	case *ast.IndexListExpr:
		return "generic instantiations"
	case *ast.BranchStmt:
		if current.Tok == token.GOTO || current.Tok == token.FALLTHROUGH {
			return strings.ToLower(current.Tok.String()) + " statements"
		}
	}
	return ""
}

func unsupportedTypeParameters(node ast.Node) string {
	switch current := node.(type) {
	case *ast.FuncType:
		if current.TypeParams != nil && len(current.TypeParams.List) != 0 {
			return "type parameters"
		}
	case *ast.TypeSpec:
		if current.TypeParams != nil && len(current.TypeParams.List) != 0 {
			return "type parameters"
		}
	}
	return ""
}

func equivalent(filename string, original, formatted []byte) error {
	originalSet, originalFile, err := parse(filename, original)
	if err != nil {
		return err
	}
	formattedSet, formattedFile, err := parse(filename, formatted)
	if err != nil {
		return fmt.Errorf("formatted output does not parse: %w", err)
	}
	if structuralFingerprint(originalFile) != structuralFingerprint(formattedFile) {
		return errors.New("formatted output changed the syntax tree")
	}
	if commentFingerprint(originalFile) != commentFingerprint(formattedFile) {
		return errors.New("formatted output changed comment contents or ordering")
	}

	// Exercise the standard formatter as an additional AST validity oracle.
	var before, after bytes.Buffer
	if err := goformat.Node(&before, originalSet, originalFile); err != nil {
		return err
	}
	if err := goformat.Node(&after, formattedSet, formattedFile); err != nil {
		return err
	}
	return nil
}

func structuralFingerprint(file *ast.File) string {
	var output strings.Builder
	writeImportFingerprint(&output, file)
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			output.WriteString(")")
			return true
		}
		if decl, ok := node.(*ast.GenDecl); ok && decl.Tok == token.IMPORT {
			return false
		}
		fmt.Fprintf(&output, "(%s", reflect.TypeOf(node).String())
		writeNodeFingerprint(&output, node)
		return true
	})
	return output.String()
}

func writeImportFingerprint(output *strings.Builder, file *ast.File) {
	imports := make([]string, 0, len(file.Imports))

	for _, spec := range file.Imports {
		name := ""
		if spec.Name != nil {
			name = spec.Name.Name
		}

		imports = append(imports, name+"\x00"+spec.Path.Value)
	}

	sort.Strings(imports)
	output.WriteString("imports:")
	output.WriteString(strings.Join(imports, "\x01"))
	output.WriteByte('\n')
}

func writeNodeFingerprint(output *strings.Builder, node ast.Node) {
	if writeNamedNodeFingerprint(output, node) {
		return
	}
	writeOperatorNodeFingerprint(output, node)
}

func writeNamedNodeFingerprint(output *strings.Builder, node ast.Node) bool {
	switch current := node.(type) {
	case *ast.Ident:
		fmt.Fprintf(output, ":%q", current.Name)
	case *ast.BasicLit:
		fmt.Fprintf(output, ":%s:%q", current.Kind, current.Value)
	case *ast.CallExpr:
		fmt.Fprintf(output, ":ellipsis=%t", current.Ellipsis.IsValid())
	case *ast.ChanType:
		fmt.Fprintf(output, ":%d", current.Dir)
	case *ast.TypeSpec:
		fmt.Fprintf(output, ":alias=%t", current.Assign.IsValid())
	default:
		return false
	}
	return true
}

func writeOperatorNodeFingerprint(output *strings.Builder, node ast.Node) {
	switch current := node.(type) {
	case *ast.GenDecl:
		fmt.Fprintf(output, ":%s", current.Tok)
	case *ast.AssignStmt:
		fmt.Fprintf(output, ":%s", current.Tok)
	case *ast.IncDecStmt:
		fmt.Fprintf(output, ":%s", current.Tok)
	case *ast.BranchStmt:
		fmt.Fprintf(output, ":%s", current.Tok)
	case *ast.UnaryExpr:
		fmt.Fprintf(output, ":%s", current.Op)
	case *ast.BinaryExpr:
		fmt.Fprintf(output, ":%s", current.Op)
	case *ast.RangeStmt:
		fmt.Fprintf(output, ":%s", current.Tok)
	}
}

func commentFingerprint(file *ast.File) string {
	comments := make([]string, 0, len(file.Comments))
	for _, group := range file.Comments {
		for _, comment := range group.List {
			comments = append(comments, comment.Text)
		}
	}
	return strings.Join(comments, "\x00")
}
