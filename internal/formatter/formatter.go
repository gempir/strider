// Package formatter implements Strider's strict, width-aware Go formatter.
package formatter

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

const PrintWidth = 100

type Options struct {
	PrintWidth  int
	IndentWidth int
	EndOfLine   string
}

func DefaultOptions() Options {
	return Options{PrintWidth: PrintWidth, IndentWidth: 4, EndOfLine: "lf"}
}

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
	return FormatWithOptions(filename, source, DefaultOptions())
}

func FormatWithOptions(filename string, source []byte, options Options) (Result, error) {
	if bytes.Contains(source, []byte("//strider:format-ignore")) {
		copyOfSource := append([]byte(nil), source...)
		return Result{Source: copyOfSource, Ignored: true}, nil
	}

	formatted, err := formatInternal(filename, source, options)
	if err != nil {
		return Result{}, err
	}
	if err := equivalent(filename, source, formatted); err != nil {
		return Result{}, fmt.Errorf("formatter safety check: %w", err)
	}
	second, err := formatInternal(filename, formatted, options)
	if err != nil {
		return Result{}, fmt.Errorf("formatter idempotence check: %w", err)
	}
	if !bytes.Equal(formatted, second) {
		return Result{}, fmt.Errorf("formatter idempotence check failed for %s", filename)
	}
	return Result{Source: formatted, Changed: !bytes.Equal(source, formatted)}, nil
}

func formatInternal(filename string, source []byte, options Options) ([]byte, error) {
	concreteTree, err := cst.Parse(filename, source)
	if err != nil {
		return nil, err
	}
	if err := validateConcreteSyntax(filename, concreteTree); err != nil {
		return nil, err
	}
	fset, file, err := parse(filename, source)
	if err != nil {
		return nil, err
	}
	builder, err := newASTBuilder(filename, fset, file)
	if err != nil {
		return nil, err
	}
	output := strings.TrimRight(
		RenderWithIndentWidth(builder.file(file), options.PrintWidth, options.IndentWidth),
		" \t\r\n",
	) + "\n"
	if options.EndOfLine == "crlf" {
		output = strings.ReplaceAll(output, "\n", "\r\n")
	}
	return []byte(output), nil
}

func validateConcreteSyntax(filename string, tree *cst.Tree) error {
	var unsupported cst.Node
	feature := ""
	cst.Walk(tree.Root(), func(node cst.Node) bool {
		if unsupported != nil {
			return false
		}
		switch current := node.(type) {
		case cst.Token:
			switch current.Ch() {
			case token.GOTO, token.FALLTHROUGH:
				unsupported = node
				feature = strings.ToLower(current.Ch().String()) + " statements"
			}
		case *cst.TypeParameters:
			unsupported = node
			feature = "type parameters"
		case *cst.TypeArgs:
			unsupported = node
			feature = "generic instantiations"
		}
		return unsupported == nil
	})
	if unsupported == nil {
		return nil
	}
	start, _ := cst.Range(unsupported)
	position := tree.Position(start)
	return &UnsupportedError{
		Filename: filename,
		Line:     position.Line,
		Column:   position.Column,
		Feature:  feature,
	}
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
