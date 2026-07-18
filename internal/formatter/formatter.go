// Package formatter implements Strider's strict, width-aware Go formatter.
package formatter

import (
	"bytes"
	"fmt"
	"go/token"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

const PrintWidth = 100

type Options struct {
	PrintWidth    int
	IndentWidth   int
	MaxEmptyLines int
	EndOfLine     string
}

func DefaultOptions() Options {
	return Options{PrintWidth: PrintWidth, IndentWidth: 4, MaxEmptyLines: 1, EndOfLine: "lf"}
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

// Session reuses formatter metadata across a batch of files. A Session is safe
// for concurrent use by independent formatting calls.
type Session struct {
	modules modulePathCache
}

func NewSession() *Session {
	return &Session{}
}

func Format(filename string, source []byte) (Result, error) {
	return FormatWithOptions(filename, source, DefaultOptions())
}

func FormatWithOptions(filename string, source []byte, options Options) (Result, error) {
	return NewSession().FormatWithOptions(filename, source, options)
}

func (s *Session) FormatWithOptions(filename string, source []byte, options Options) (Result, error) {
	if bytes.Contains(source, []byte("//strider:format-ignore")) {
		copyOfSource := append([]byte(nil), source...)
		return Result{Source: copyOfSource, Ignored: true}, nil
	}

	originalTree, hasImports, err := parseConcrete(filename, source)
	if err != nil {
		return Result{}, err
	}
	module := ""
	if hasImports {
		module = s.modules.find(filename)
	}
	formatted := []byte(renderConcreteWithModule(originalTree, options, module))
	formattedTree, err := cst.Parse(filename, formatted)
	if err != nil {
		return Result{}, fmt.Errorf("formatter safety check: formatted output does not parse: %w", err)
	}
	if err := equivalentTrees(originalTree, formattedTree); err != nil {
		return Result{}, fmt.Errorf("formatter safety check: %w", err)
	}
	if _, err := validateConcreteSyntax(filename, formattedTree); err != nil {
		return Result{}, fmt.Errorf("formatter idempotence check: %w", err)
	}
	second := []byte(renderConcreteWithModule(formattedTree, options, module))
	if !bytes.Equal(formatted, second) {
		return Result{}, fmt.Errorf("formatter idempotence check failed for %s", filename)
	}
	return Result{Source: formatted, Changed: !bytes.Equal(source, formatted)}, nil
}

func parseConcrete(filename string, source []byte) (*cst.Tree, bool, error) {
	concreteTree, err := cst.Parse(filename, source)
	if err != nil {
		return nil, false, err
	}
	hasImports, err := validateConcreteSyntax(filename, concreteTree)
	if err != nil {
		return nil, false, err
	}
	return concreteTree, hasImports, nil
}

func validateConcreteSyntax(filename string, tree *cst.Tree) (bool, error) {
	hasImports := false
	for _, current := range tree.Tokens() {
		switch current.Ch() {
		case token.IMPORT:
			hasImports = true
		case token.GOTO, token.FALLTHROUGH:
			position := current.Position()
			return false, &UnsupportedError{
				Filename: filename,
				Line:     position.Line,
				Column:   position.Column,
				Feature:  strings.ToLower(current.Ch().String()) + " statements",
			}
		}
	}
	return hasImports, nil
}
