// Package syntax implements Strider's fast, syntax-only check engine.
//
//strider:ignore-file cognitive-complexity,confusing-naming,use-slices-sort
package syntax

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/gempir/strider/internal/checks/catalog"
	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/pathfilter"
	"github.com/gempir/strider/internal/source"
)

type Plan struct {
	checks          []Check
	settings        map[string]configuredCheck
	root            string
	preparedByFile  sync.Map
	preparedByGroup sync.Map
}

type configuredCheck struct {
	severity diagnostic.Severity
	excludes []string
	options  catalog.ResolvedOptions
}

type preparedPlan struct {
	checks   []Check
	dispatch map[NodeKind][]Check
	options  map[string]catalog.ResolvedOptions
}

// SelectedCheck is a fully bound syntax check produced by the unified
// selection boundary.
type SelectedCheck struct {
	Check    Check
	Severity diagnostic.Severity
	Excludes []string
	Options  catalog.ResolvedOptions
}

type Context struct {
	filename        string
	displayFilename string
	diagnostics     []diagnostic.Diagnostic
	concreteIgnores map[string]bool
	concreteNodes   []concreteSuppression
}

type concreteSuppression struct {
	start int
	end   int
	codes map[string]bool
}

// NewPlan prepares syntax execution from already-selected, schema-bound
// checks. It deliberately has no selection or configuration policy.
func NewPlan(selected []SelectedCheck, root string) *Plan {
	registry := &Plan{
		settings: make(map[string]configuredCheck, len(selected)),
		root:     source.ResolveRoot(root),
	}
	for _, item := range selected {
		meta := item.Check.Meta()
		registry.checks = append(registry.checks, item.Check)
		registry.settings[meta.Code] = configuredCheck{
			severity: item.Severity,
			excludes: append([]string(nil), item.Excludes...),
			options:  item.Options,
		}
	}
	return registry
}

func (r *Plan) Severity(code string) diagnostic.Severity {
	return r.settings[code].severity
}

// CacheIdentity returns the effective syntax configuration for filename,
// including selected checks, severities, exclusions, and resolved options.
func (r *Plan) CacheIdentity(filename string) string {
	if r == nil {
		return "syntax=disabled"
	}
	prepared := r.prepare(filename)
	var identity strings.Builder
	for _, check := range prepared.checks {
		meta := check.Meta()
		setting := r.settings[meta.Code]
		fmt.Fprintf(&identity, "check=%s\nseverity=%s\n", meta.Code, setting.severity)
		excludes := append([]string(nil), setting.excludes...)
		sort.Strings(excludes)
		for _, exclude := range excludes {
			fmt.Fprintf(&identity, "exclude=%s\n", exclude)
		}
		options := setting.options
		for _, spec := range meta.Options {
			if spec.Kind == catalog.OptionInt {
				value, _ := options.Int(spec.Name)
				fmt.Fprintf(&identity, "option=%s:int:%d\n", spec.Name, value)
			}
			if spec.Kind == catalog.OptionStrings {
				values, _ := options.Strings(spec.Name)
				fmt.Fprintf(&identity, "option=%s:strings:%q\n", spec.Name, values)
			}
		}
	}
	return identity.String()
}

func (r *Plan) prepare(filename string) *preparedPlan {
	if cached, ok := r.preparedByFile.Load(filename); ok {
		if prepared, valid := cached.(*preparedPlan); valid {
			return prepared
		}
	}
	active := make([]Check, 0, len(r.checks))
	codes := make([]string, 0, len(r.checks))
	for _, check := range r.checks {
		if pathfilter.Excluded(r.root, filename, r.settings[check.Meta().Code].excludes) {
			continue
		}
		active = append(active, check)
		codes = append(codes, check.Meta().Code)
	}
	groupKey := strings.Join(codes, "\x00")
	cached, ok := r.preparedByGroup.Load(groupKey)
	if !ok {
		prepared := &preparedPlan{
			checks:   active,
			dispatch: buildDispatch(active),
			options:  r.boundOptions(active),
		}
		cached, _ = r.preparedByGroup.LoadOrStore(groupKey, prepared)
	}
	prepared, valid := cached.(*preparedPlan)
	if !valid {
		prepared = &preparedPlan{}
	}
	cached, _ = r.preparedByFile.LoadOrStore(filename, prepared)
	if stored, valid := cached.(*preparedPlan); valid {
		return stored
	}
	return prepared
}

// Applies reports whether at least one selected concrete-syntax check applies
// to filename.
func (r *Plan) Applies(filename string) bool {
	if r == nil {
		return false
	}
	return len(r.prepare(filename).checks) != 0
}

// AnalyzeTree runs the selected concrete-syntax checks against a shared tree.
func AnalyzeTree(filename string, concreteTree *cst.Tree, registry *Plan) []diagnostic.Diagnostic {
	if concreteTree == nil || registry == nil {
		return nil
	}
	return AnalyzeTreeWithDisplay(filename, source.DiagnosticPath(registry.root, filename), concreteTree, registry)
}

// AnalyzeTreeWithDisplay runs the selected syntax checks with a display path
// already materialized by the caller's per-run path cache.
func AnalyzeTreeWithDisplay(filename, displayFilename string, concreteTree *cst.Tree, registry *Plan) []diagnostic.Diagnostic {
	if concreteTree == nil || registry == nil {
		return nil
	}
	prepared := registry.prepare(filename)
	if len(prepared.checks) == 0 {
		return nil
	}
	concreteIgnores, concreteNodes := concreteSuppressions(concreteTree)
	context := &Context{
		filename:        filename,
		displayFilename: displayFilename,
		concreteIgnores: concreteIgnores,
		concreteNodes:   concreteNodes,
	}
	AnalyzeCST(
		CSTInput{
			Filename: filename,
			Tree:     concreteTree,
			Checks:   prepared.checks,
			Dispatch: prepared.dispatch,
			Options:  prepared.options,
			Report: func(finding Finding) {
				context.reportConcrete(concreteTree, finding, registry.Severity(finding.Code))
			},
		},
	)
	return context.diagnostics
}

func (r *Plan) boundOptions(checks []Check) map[string]catalog.ResolvedOptions {
	options := make(map[string]catalog.ResolvedOptions, len(checks))
	for _, check := range checks {
		options[check.Meta().Code] = r.settings[check.Meta().Code].options
	}
	return options
}

func (c *Context) reportConcrete(tree *cst.Tree, finding Finding, severity diagnostic.Severity) {
	startOffset, endOffset := cst.Range(finding.Node)
	if finding.HasRange {
		startOffset, endOffset = finding.Start, finding.End
	}
	if c.suppressedRange(finding.Code, startOffset, endOffset) {
		return
	}
	start := tree.Position(startOffset)
	end := tree.Position(endOffset)
	display := c.displayFilename
	start.Filename = display
	end.Filename = display
	c.diagnostics = append(
		c.diagnostics,
		diagnostic.Diagnostic{
			Code:     finding.Code,
			Message:  finding.Message,
			Severity: severity,
			File:     display,
			Start:    start,
			End:      end,
			Fixes:    finding.Fixes,
		},
	)
}

func (c *Context) suppressedRange(code string, start, end int) bool {
	if c.concreteIgnores["all"] || c.concreteIgnores[code] {
		return true
	}
	for _, ignored := range c.concreteNodes {
		if ignored.start <= start && ignored.end >= end && (ignored.codes["all"] || ignored.codes[code]) {
			return true
		}
	}
	return false
}

func concreteSuppressions(tree *cst.Tree) (map[string]bool, []concreteSuppression) {
	if !bytes.Contains(tree.Bytes(), []byte("strider:ignore")) {
		return nil, nil
	}
	fileIgnores := make(map[string]bool)
	candidates := concreteSuppressionCandidates(tree)
	packageStart, _ := cst.Range(tree.Root())
	comments := tree.Comments()
	result := make([]concreteSuppression, 0, len(comments))
	for _, comment := range comments {
		literal := comment.Text
		end := comment.End
		if codes, ok := directiveCodes(literal, "strider:ignore-file"); ok && end < packageStart {
			for _, code := range codes {
				fileIgnores[code] = true
			}
		}
		codes, ok := directiveCodes(literal, "strider:ignore")
		if !ok {
			continue
		}
		index := sort.Search(len(candidates), func(index int) bool {
			return candidates[index].start > end
		})
		if index == len(candidates) {
			continue
		}
		ignored := concreteSuppression{
			start: candidates[index].start,
			end:   candidates[index].end,
			codes: make(map[string]bool, len(codes)),
		}
		for _, code := range codes {
			ignored.codes[code] = true
		}
		result = append(result, ignored)
	}
	return fileIgnores, result
}

func concreteSuppressionCandidates(tree *cst.Tree) []concreteSuppression {
	result := []concreteSuppression{}
	cst.Walk(
		tree.Root(),
		func(node cst.Node) bool {
			kind := cst.Kind(node)
			if !strings.HasSuffix(kind, "Decl") && !strings.HasSuffix(kind, "Stmt") {
				return true
			}
			start, end := cst.Range(node)
			if end > start {
				result = append(result, concreteSuppression{
					start: start,
					end:   end,
				})
			}
			return true
		},
	)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].start != result[j].start {
			return result[i].start < result[j].start
		}
		return result[i].end > result[j].end
	})
	return result
}

func directiveCodes(comment, directive string) ([]string, bool) {
	index := strings.Index(comment, directive)
	if index < 0 {
		return nil, false
	}
	remainder := comment[index+len(directive):]
	if remainder != "" && remainder[0] != ' ' && remainder[0] != '\t' && remainder[0] != '*' && remainder[0] != '/' {
		return nil, false
	}
	remainder = strings.Trim(remainder, " \t*/")
	if remainder == "" {
		return nil, false
	}
	parts := strings.Split(remainder, ",")
	codes := make([]string, 0, len(parts))
	for _, part := range parts {
		if code := strings.TrimSpace(part); code != "" {
			codes = append(codes, code)
		}
	}
	return codes, len(codes) != 0
}
