// Package formatter implements Strider's strict, width-aware Go formatter.
//
//strider:ignore-file cognitive-complexity,confusing-naming,cyclomatic-complexity,modifies-parameter
package formatter

import (
	"bytes"
	"fmt"
	goformat "go/format"
	"go/scanner"
	"go/token"
	"strings"

	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/telemetry"
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

// Formatter reuses formatter metadata across a batch of files. A Formatter is safe
// for concurrent use by independent formatting calls.
type Formatter struct {
	modules modulePathCache
}

func DefaultOptions() Options {
	return Options{
		PrintWidth: PrintWidth,
	}
}

func NewFormatter() *Formatter {
	return &Formatter{}
}

// CacheIdentity returns every formatter input not already represented by the
// source content and logical path cache-key components.
func (s *Formatter) CacheIdentity(filename string, options Options) string {
	options = normalizeOptions(options)
	module := s.modules.findInfo(filename)
	return fmt.Sprintf("print-width=%d\nmodule=%s\ngo-mod=%s", options.PrintWidth, module.path, module.identity)
}

func Format(filename string, source []byte) (Result, error) {
	return FormatWithOptions(filename, source, DefaultOptions())
}

func FormatWithOptions(filename string, source []byte, options Options) (Result, error) {
	return NewFormatter().FormatWithOptions(filename, source, options)
}

func (s *Formatter) FormatWithOptions(filename string, source []byte, options Options) (Result, error) {
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
	return s.formatTree(filename, originalTree, normalizeOptions(options), false)
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
func (s *Formatter) FormatTree(filename string, originalTree *cst.Tree, options Options) (Result, error) {
	return s.formatTree(filename, originalTree, normalizeOptions(options), true)
}

// FormatTreeKnownActive formats a tree after the caller has made the sole
// ignore decision for the path and established that formatting is active.
func (s *Formatter) FormatTreeKnownActive(filename string, originalTree *cst.Tree, options Options) (Result, error) {
	return s.formatTree(filename, originalTree, normalizeOptions(options), false)
}

// WouldChangeTree reports formatting drift without constructing a candidate
// that escapes the call. It guarantees the same Changed and Ignored values as
// FormatTree whenever the full formatter succeeds. Convergence-error parity is
// intentionally not part of this status-only contract.
func (s *Formatter) WouldChangeTree(filename string, originalTree *cst.Tree, options Options) (changed bool, ignored bool, err error) {
	return s.wouldChangeTree(filename, originalTree, normalizeOptions(options), true)
}

// WouldChangeTreeKnownActive is the status-only counterpart to
// FormatTreeKnownActive.
func (s *Formatter) WouldChangeTreeKnownActive(filename string, originalTree *cst.Tree, options Options) (changed bool, err error) {
	changed, _, err = s.wouldChangeTree(filename, originalTree, normalizeOptions(options), false)
	return changed, err
}

func (s *Formatter) wouldChangeTree(filename string, originalTree *cst.Tree, options Options, checkIgnored bool) (changed bool, ignored bool, err error) {
	if err := validateTreeInput(filename, originalTree, options); err != nil {
		return false, false, err
	}
	source := originalTree.Bytes()
	if checkIgnored && IsIgnored(source) {
		return false, true, nil
	}
	module := ""
	if treeHasImports(originalTree) {
		module = s.modules.find(filename)
	}
	formatted, err := renderCandidate(originalTree, options, module)
	if err != nil {
		return false, false, err
	}
	return !bytes.Equal(source, formatted), false, nil
}

func (s *Formatter) formatTree(filename string, originalTree *cst.Tree, options Options, checkIgnored bool) (Result, error) {
	finish := telemetry.Start("formatter.verified")
	defer finish()
	preview, module, err := s.previewTree(filename, originalTree, options, checkIgnored)
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
	second, err := renderCandidate(formattedTree, options, module)
	if err != nil {
		return Result{}, fmt.Errorf("formatter idempotence check: %w", err)
	}
	if !bytes.Equal(preview.Source, second) {
		return Result{}, fmt.Errorf("formatter idempotence check failed for %s", filename)
	}
	return preview, nil
}

func (s *Formatter) previewTree(filename string, originalTree *cst.Tree, options Options, checkIgnored bool) (Result, string, error) {
	finish := telemetry.Start("formatter.preview")
	defer finish()
	if err := validateTreeInput(filename, originalTree, options); err != nil {
		return Result{}, "", err
	}
	source := originalTree.Bytes()
	if checkIgnored && IsIgnored(source) {
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
	formatted, err := renderCandidate(originalTree, options, module)
	if err != nil {
		return Result{}, "", err
	}
	if bytes.Equal(source, formatted) {
		return Result{
			Source: formatted,
		}, module, nil
	}
	// Rendering uses CST trivia to preserve deliberate vertical space. Because a
	// render can move that trivia to its canonical token boundary, render the new
	// tree until those boundaries stabilize. The bound is defensive; the separate
	// idempotence check above still verifies the stable candidate once more.
	for range 100 {
		formattedTree, parseErr := cst.Parse(filename, formatted)
		if parseErr != nil {
			return Result{}, "", fmt.Errorf("formatter convergence check: formatted output does not parse: %w", parseErr)
		}
		next, formatErr := renderCandidate(formattedTree, options, module)
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

func validateTreeInput(filename string, originalTree *cst.Tree, options Options) error {
	if originalTree == nil {
		return fmt.Errorf("format %s: nil concrete syntax tree", filename)
	}
	if options.PrintWidth < 40 || options.PrintWidth > 500 {
		return fmt.Errorf("format %s: print width must be between 40 and 500", filename)
	}
	return nil
}

func renderCandidate(tree *cst.Tree, options Options, module string) ([]byte, error) {
	renderFinish := telemetry.Start("formatter.render")
	rendered := []byte(renderWithModule(tree, options, module))
	renderFinish()
	goFormatFinish := telemetry.Start("formatter.go-format")
	formatted, err := goformat.Source(rendered)
	goFormatFinish()
	if err != nil {
		return nil, fmt.Errorf("formatter gofmt compatibility: %w", err)
	}
	return formatted, nil
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
