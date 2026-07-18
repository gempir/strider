// Package formatter implements Strider's strict, width-aware Go formatter.
package formatter

import (
	"bytes"
	"fmt"
	"go/token"

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
	if IsIgnored(source) {
		copyOfSource := append([]byte(nil), source...)
		return Result{Source: copyOfSource, Ignored: true}, nil
	}
	originalTree, err := cst.Parse(filename, source)
	if err != nil {
		return Result{}, err
	}
	return s.FormatTree(filename, originalTree, options)
}

// IsIgnored reports whether source opts out of canonical formatting.
func IsIgnored(source []byte) bool {
	return bytes.Contains(source, []byte("//strider:format-ignore"))
}

// FormatTree formats a previously parsed source tree. The tree and its source
// remain immutable and may be shared with other checks.
func (s *Session) FormatTree(filename string, originalTree *cst.Tree, options Options) (Result, error) {
	preview, module, err := s.previewTree(filename, originalTree, options)
	if err != nil || preview.Ignored {
		return preview, err
	}
	formattedTree, err := cst.Parse(filename, preview.Source)
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
	if !bytes.Equal(preview.Source, second) {
		return Result{}, fmt.Errorf("formatter idempotence check failed for %s", filename)
	}
	return preview, nil
}

// PreviewTree renders a read-only formatting candidate without reparsing it.
// It is intended for drift checks; callers that may expose or write the
// candidate must use FormatTree and its equivalence/idempotence checks.
func (s *Session) PreviewTree(
	filename string,
	originalTree *cst.Tree,
	options Options,
) (Result, error) {
	result, _, err := s.previewTree(filename, originalTree, options)
	return result, err
}

func (s *Session) previewTree(
	filename string,
	originalTree *cst.Tree,
	options Options,
) (Result, string, error) {
	if originalTree == nil {
		return Result{}, "", fmt.Errorf("format %s: nil concrete syntax tree", filename)
	}
	source := originalTree.Bytes()
	if IsIgnored(source) {
		copyOfSource := append([]byte(nil), source...)
		return Result{Source: copyOfSource, Ignored: true}, "", nil
	}
	hasImports, err := validateConcreteSyntax(filename, originalTree)
	if err != nil {
		return Result{}, "", err
	}
	module := ""
	if hasImports {
		module = s.modules.find(filename)
	}
	formatted := []byte(renderConcreteWithModule(originalTree, options, module))
	return Result{Source: formatted, Changed: !bytes.Equal(source, formatted)}, module, nil
}

func validateConcreteSyntax(_ string, tree *cst.Tree) (bool, error) {
	hasImports := false
	for _, current := range tree.Tokens() {
		if current.Ch() == token.IMPORT {
			hasImports = true
		}
	}
	return hasImports, nil
}
