// Package formatter implements Strider's strict, width-aware Go formatter.
package formatter

import (
	"bytes"
	"fmt"
	goformat "go/format"
	"go/scanner"
	"go/token"
	"strings"

	"github.com/gempir/strider/internal/cst"
)

const PrintWidth = 180

type Options struct {
	PrintWidth int
}

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

func DefaultOptions() Options {
	return Options{
		PrintWidth: PrintWidth,
	}
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
		return Result{
			Source:  copyOfSource,
			Ignored: true,
		}, nil
	}
	originalTree, err := cst.Parse(filename, source)
	if err != nil {
		return Result{}, err
	}
	return s.FormatTree(filename, originalTree, options)
}

// IsIgnored reports whether a header comment before the package clause opts
// out of canonical formatting.
func IsIgnored(source []byte) bool {
	file := token.NewFileSet().AddFile("", -1, len(source))
	var lexer scanner.Scanner
	lexer.Init(file, source, nil, scanner.ScanComments)
	for {
		_, kind, literal := lexer.Scan()
		if kind == token.EOF || kind != token.COMMENT {
			return false
		}
		for _, line := range strings.Split(literal, "\n") {
			line = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "//"), "/*"))
			line = strings.TrimSpace(strings.TrimSuffix(line, "*/"))
			if line == "strider:format-ignore" {
				return true
			}
		}
	}
}

// FormatTree formats a previously parsed source tree. The tree and its source
// remain immutable and may be shared with other checks.
func (s *Session) FormatTree(filename string, originalTree *cst.Tree, options Options) (Result, error) {
	options = normalizeOptions(options)
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
	second, err := renderCandidate(filename, formattedTree, options, module)
	if err != nil {
		return Result{}, fmt.Errorf("formatter idempotence check: %w", err)
	}
	if !bytes.Equal(preview.Source, second) {
		return Result{}, fmt.Errorf("formatter idempotence check failed for %s", filename)
	}
	return preview, nil
}

// FormatTreeUnverified renders a read-only formatting candidate without reparsing it.
// It is intended for drift checks; callers that may expose or write the
// candidate must use FormatTree and its equivalence/idempotence checks.
func (s *Session) FormatTreeUnverified(filename string, originalTree *cst.Tree, options Options) (Result, error) {
	result, _, err := s.previewTree(filename, originalTree, options)
	return result, err
}

func (s *Session) previewTree(filename string, originalTree *cst.Tree, options Options) (Result, string, error) {
	if originalTree == nil {
		return Result{}, "", fmt.Errorf("format %s: nil concrete syntax tree", filename)
	}
	options = normalizeOptions(options)
	if options.PrintWidth < 40 || options.PrintWidth > 500 {
		return Result{}, "", fmt.Errorf("format %s: print width must be between 40 and 500", filename)
	}
	source := originalTree.Bytes()
	if IsIgnored(source) {
		copyOfSource := append([]byte(nil), source...)
		return Result{
			Source:  copyOfSource,
			Ignored: true,
		}, "", nil
	}
	hasImports := treeHasImports(originalTree)
	module := ""
	if hasImports {
		module = s.modules.find(filename)
	}
	formatted, err := renderCandidate(filename, originalTree, options, module)
	if err != nil {
		return Result{}, "", err
	}
	for range 100 {
		formattedTree, parseErr := cst.Parse(filename, formatted)
		if parseErr != nil {
			return Result{}, "", fmt.Errorf("formatter convergence check: formatted output does not parse: %w", parseErr)
		}
		next, formatErr := renderCandidate(filename, formattedTree, options, module)
		if formatErr != nil {
			return Result{}, "", formatErr
		}
		if bytes.Equal(formatted, next) {
			return Result{
				Source:  formatted,
				Changed: !bytes.Equal(source, formatted),
			}, module, nil
		}
		formatted = next
	}
	return Result{}, "", fmt.Errorf("formatter did not converge for %s", filename)
}

func renderCandidate(filename string, tree *cst.Tree, options Options, module string) ([]byte, error) {
	formatted, err := goformat.Source([]byte(renderConcreteWithModule(tree, options, module)))
	if err != nil {
		return nil, fmt.Errorf("formatter gofmt compatibility: %w", err)
	}
	ordered, err := orderTopLevelDeclarations(filename, formatted)
	if err != nil {
		return nil, fmt.Errorf("formatter declaration ordering: %w", err)
	}
	return ordered, nil
}

func normalizeOptions(options Options) Options {
	defaults := DefaultOptions()
	if options.PrintWidth == 0 {
		options.PrintWidth = defaults.PrintWidth
	}
	return options
}

func treeHasImports(tree *cst.Tree) bool {
	hasImports := false
	for _, current := range tree.Tokens() {
		if current.Ch() == token.IMPORT {
			hasImports = true
		}
	}
	return hasImports
}
