// Package lint implements Strider's fast, syntax-only lint engine.
package lint

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
	builtinrules "github.com/gempir/strider/internal/lint/rules"
	"github.com/gempir/strider/internal/pathfilter"
	"github.com/gempir/strider/internal/source"
)

type Registry struct {
	rules    []builtinrules.Rule
	settings map[string]configuredRule
	root     string
}

type configuredRule struct {
	severity diagnostic.Severity
	excludes []string
}

func NewRegistry(only []string) (*Registry, error) {
	return newRegistry(only, false)
}

func NewRegistryAll() (*Registry, error) {
	return newRegistry(nil, true)
}

func newRegistry(only []string, enableAll bool) (*Registry, error) {
	return NewRegistryConfigured(only, enableAll, nil, "")
}

// NewRegistryConfigured applies project rule settings. Explicit CLI selection
// takes precedence over configured enabled states.
func NewRegistryConfigured(
	only []string,
	enableAll bool,
	settings map[string]config.RuleConfig,
	root string,
) (*Registry, error) {
	all, err := builtinrules.Select(nil, true)
	if err != nil {
		return nil, err
	}
	byCode := make(map[string]builtinrules.Rule, len(all))
	for _, rule := range all {
		byCode[rule.Meta().Code] = rule
	}
	if err := validateConfiguredRules("lint", settings, byCode); err != nil {
		return nil, err
	}
	if len(only) != 0 {
		if _, err := builtinrules.Select(only, false); err != nil {
			return nil, err
		}
	}
	defaults, _ := builtinrules.Select(nil, false)
	enabledByDefault := make(map[string]bool, len(defaults))
	for _, rule := range defaults {
		enabledByDefault[rule.Meta().Code] = true
	}
	wanted := make(map[string]bool, len(only))
	for _, code := range only {
		wanted[code] = true
	}
	registry := &Registry{
		settings: make(map[string]configuredRule, len(all)),
		root:     root,
	}
	for _, rule := range all {
		meta := rule.Meta()
		ruleConfig := settings[meta.Code]
		enabled := enabledByDefault[meta.Code]
		switch {
		case len(only) != 0:
			enabled = wanted[meta.Code]
		case enableAll:
			enabled = true
		case ruleConfig.Enabled != nil:
			enabled = *ruleConfig.Enabled
		}
		if !enabled {
			continue
		}
		severity := meta.DefaultSeverity
		if ruleConfig.Severity != "" {
			severity = diagnostic.Severity(ruleConfig.Severity)
		}
		registry.rules = append(registry.rules, rule)
		registry.settings[meta.Code] = configuredRule{severity: severity, excludes: ruleConfig.Excludes}
	}
	return registry, nil
}

func validateConfiguredRules(
	tool string,
	settings map[string]config.RuleConfig,
	available map[string]builtinrules.Rule,
) error {
	unknown := make([]string, 0)
	for code := range settings {
		if available[code] == nil {
			unknown = append(unknown, code)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("unknown %s rule(s) in configuration: %s", tool, strings.Join(unknown, ", "))
}

func (r *Registry) Rules() []builtinrules.Rule {
	return append([]builtinrules.Rule(nil), r.rules...)
}

func (r *Registry) Severity(code string) diagnostic.Severity {
	return r.settings[code].severity
}

func (r *Registry) activeRules(filename string) []builtinrules.Rule {
	active := make([]builtinrules.Rule, 0, len(r.rules))
	for _, rule := range r.rules {
		if pathfilter.Matches(r.root, filename, r.settings[rule.Meta().Code].excludes) {
			continue
		}
		active = append(active, rule)
	}
	return active
}

type Context struct {
	filename        string
	fset            *token.FileSet
	diagnostics     []diagnostic.Diagnostic
	ancestors       []ast.Node
	fileIgnores     map[string]bool
	nodeIgnores     map[ast.Node]map[string]bool
	concreteIgnores map[string]bool
	concreteNodes   []concreteSuppression
	current         ast.Node
}

type concreteSuppression struct {
	start int
	end   int
	codes map[string]bool
}

func (c *Context) report(node ast.Node, code, message string, severity diagnostic.Severity) {
	if c.suppressed(code) {
		return
	}
	start := c.fset.Position(node.Pos())
	end := c.fset.Position(node.End())
	display := source.DisplayPath(c.filename)
	start.Filename = display
	end.Filename = display
	c.diagnostics = append(
		c.diagnostics,
		diagnostic.Diagnostic{
			Code:     code,
			Message:  message,
			Severity: severity,
			File:     display,
			Start:    start,
			End:      end,
		},
	)
}

func (c *Context) suppressed(code string) bool {
	if c.fileIgnores["all"] || c.fileIgnores[code] {
		return true
	}
	nodes := append(append([]ast.Node(nil), c.ancestors...), c.current)
	for _, node := range nodes {
		ignored := c.nodeIgnores[node]
		if ignored["all"] || ignored[code] {
			return true
		}
	}
	return false
}

type fileResult struct {
	filename    string
	diagnostics []diagnostic.Diagnostic
	err         error
}

type suppressionSet struct {
	file        *ast.File
	candidates  []ast.Node
	fileIgnores map[string]bool
	nodeIgnores map[ast.Node]map[string]bool
}

func Run(files []string, registry *Registry) ([]diagnostic.Diagnostic, error) {
	workers := min(runtime.GOMAXPROCS(0), max(1, len(files)))
	jobs := make(chan string)
	results := make(chan fileResult, len(files))
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for filename := range jobs {
				diagnostics, err := lintFile(filename, registry)
				results <- fileResult{filename: filename, diagnostics: diagnostics, err: err}
			}
		}()
	}
	go func() {
		for _, filename := range files {
			jobs <- filename
		}
		close(jobs)
		group.Wait()
		close(results)
	}()
	allDiagnostics := []diagnostic.Diagnostic{}
	errorsByFile := []fileResult{}
	for result := range results {
		if result.err != nil {
			errorsByFile = append(errorsByFile, result)
			continue
		}
		allDiagnostics = append(allDiagnostics, result.diagnostics...)
	}
	if len(errorsByFile) != 0 {
		sort.Slice(
			errorsByFile,
			func(i, j int) bool {
				return errorsByFile[i].filename < errorsByFile[j].filename
			},
		)
		return nil, fmt.Errorf(
			"%s: %w",
			source.DisplayPath(errorsByFile[0].filename),
			errorsByFile[0].err,
		)
	}
	sort.Slice(
		allDiagnostics,
		func(i, j int) bool {
			left, right := allDiagnostics[i], allDiagnostics[j]
			if left.File != right.File {
				return left.File < right.File
			}
			if left.Start.Offset != right.Start.Offset {
				return left.Start.Offset < right.Start.Offset
			}
			if left.Code != right.Code {
				return left.Code < right.Code
			}
			return left.Message < right.Message
		},
	)
	return allDiagnostics, nil
}

func lintFile(filename string, registry *Registry) ([]diagnostic.Diagnostic, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	concreteTree, err := cst.Parse(filename, content)
	if err != nil {
		return nil, err
	}
	concreteIgnores, concreteNodes := concreteSuppressions(filename, concreteTree)
	context := &Context{
		filename:        filename,
		concreteIgnores: concreteIgnores,
		concreteNodes:   concreteNodes,
	}
	activeRules := registry.activeRules(filename)
	builtinrules.AnalyzeCST(
		builtinrules.CSTInput{
			Filename: filename,
			Tree:     concreteTree,
			Rules:    activeRules,
			Report: func(finding builtinrules.Finding) {
				context.reportConcrete(concreteTree, finding, registry.Severity(finding.Code))
			},
		},
	)
	legacyRules := make([]builtinrules.Rule, 0, len(activeRules))
	for _, rule := range activeRules {
		if !builtinrules.UsesCST(rule.Meta().Code) {
			legacyRules = append(legacyRules, rule)
		}
	}
	if len(legacyRules) == 0 {
		return context.diagnostics, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(
		fset,
		filename,
		content,
		parser.ParseComments|parser.AllErrors|parser.SkipObjectResolution,
	)
	if err != nil {
		return nil, err
	}
	context.fset = fset
	context.fileIgnores, context.nodeIgnores = suppressions(file)
	builtinrules.Analyze(
		builtinrules.Input{
			Filename: filename,
			FileSet:  fset,
			File:     file,
			Content:  content,
			Rules:    legacyRules,
			Report: func(finding builtinrules.Finding) {
				context.current = finding.Scope
				if context.current == nil {
					context.current = finding.Node
				}
				context.ancestors = finding.Ancestors
				context.report(
					finding.Node,
					finding.Code,
					finding.Message,
					registry.Severity(finding.Code),
				)
			},
		},
	)
	return context.diagnostics, nil
}

func (c *Context) reportConcrete(
	tree *cst.Tree,
	finding builtinrules.Finding,
	severity diagnostic.Severity,
) {
	startOffset, endOffset := cst.Range(finding.ConcreteNode)
	if c.suppressedRange(finding.Code, startOffset, endOffset) {
		return
	}
	start := tree.Position(startOffset)
	end := tree.Position(endOffset)
	display := source.DisplayPath(c.filename)
	start.Filename = display
	end.Filename = display
	c.diagnostics = append(c.diagnostics, diagnostic.Diagnostic{
		Code:     finding.Code,
		Message:  finding.Message,
		Severity: severity,
		File:     display,
		Start:    start,
		End:      end,
	})
}

func (c *Context) suppressedRange(code string, start, end int) bool {
	if c.concreteIgnores["all"] || c.concreteIgnores[code] {
		return true
	}
	for _, ignored := range c.concreteNodes {
		if ignored.start <= start && ignored.end >= end &&
			(ignored.codes["all"] || ignored.codes[code]) {
			return true
		}
	}
	return false
}

func concreteSuppressions(filename string, tree *cst.Tree) (map[string]bool, []concreteSuppression) {
	fileIgnores := make(map[string]bool)
	candidates := concreteSuppressionCandidates(tree)
	packageStart, _ := cst.Range(tree.Root())
	sourceBytes := tree.Source()
	fset := token.NewFileSet()
	tokenFile := fset.AddFile(filename, -1, len(sourceBytes))
	var lexer scanner.Scanner
	lexer.Init(tokenFile, sourceBytes, nil, scanner.ScanComments)
	result := []concreteSuppression{}
	for {
		position, kind, literal := lexer.Scan()
		if kind == token.EOF {
			break
		}
		if kind != token.COMMENT {
			continue
		}
		start := tokenFile.Offset(position)
		end := start + len(literal)
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
	cst.Walk(tree.Root(), func(node cst.Node) bool {
		kind := cst.Kind(node)
		if !strings.HasSuffix(kind, "Decl") && !strings.HasSuffix(kind, "Stmt") {
			return true
		}
		start, end := cst.Range(node)
		if end > start {
			result = append(result, concreteSuppression{start: start, end: end})
		}
		return true
	})
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].start != result[j].start {
			return result[i].start < result[j].start
		}
		return result[i].end > result[j].end
	})
	return result
}

func suppressions(file *ast.File) (map[string]bool, map[ast.Node]map[string]bool) {
	set := suppressionSet{
		file:        file,
		candidates:  suppressionCandidates(file),
		fileIgnores: make(map[string]bool),
		nodeIgnores: make(map[ast.Node]map[string]bool),
	}
	for _, group := range file.Comments {
		for _, comment := range group.List {
			set.apply(group, comment.Text)
		}
	}
	return set.fileIgnores, set.nodeIgnores
}

func suppressionCandidates(file *ast.File) []ast.Node {
	candidates := []ast.Node{}
	ast.Inspect(
		file,
		func(node ast.Node) bool {
			switch node.(type) {
			case ast.Decl, ast.Stmt:
				candidates = append(candidates, node)
			}
			return true
		},
	)
	sort.SliceStable(
		candidates,
		func(i, j int) bool {
			if candidates[i].Pos() != candidates[j].Pos() {
				return candidates[i].Pos() < candidates[j].Pos()
			}
			return candidates[i].End() > candidates[j].End()
		},
	)
	return candidates
}

func (set *suppressionSet) apply(group *ast.CommentGroup, comment string) {
	if codes, ok := directiveCodes(comment, "strider:ignore-file"); ok &&
		group.End() < set.file.Package {
		for _, code := range codes {
			set.fileIgnores[code] = true
		}
	}
	codes, ok := directiveCodes(comment, "strider:ignore")
	if !ok {
		return
	}
	index := sort.Search(
		len(set.candidates),
		func(index int) bool {
			return set.candidates[index].Pos() > group.End()
		},
	)
	if index == len(set.candidates) {
		return
	}
	target := set.candidates[index]
	if set.nodeIgnores[target] == nil {
		set.nodeIgnores[target] = make(map[string]bool)
	}
	for _, code := range codes {
		set.nodeIgnores[target][code] = true
	}
}

func directiveCodes(comment, directive string) ([]string, bool) {
	index := strings.Index(comment, directive)
	if index < 0 {
		return nil, false
	}
	remainder := comment[index+len(directive):]
	if remainder != "" && remainder[0] != ' ' && remainder[0] != '\t' && remainder[0] != '*' &&
		remainder[0] != '/' {
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
