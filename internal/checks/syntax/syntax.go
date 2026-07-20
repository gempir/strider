// Package syntax implements Strider's fast, syntax-only check engine.
package syntax

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/gempir/strider/internal/checks/core"
	builtinchecks "github.com/gempir/strider/internal/checks/syntax/rules"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/cst"
	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/pathfilter"
	"github.com/gempir/strider/internal/source"
)

type Registry struct {
	checks     []builtinchecks.Check
	settings   map[string]configuredCheck
	knownCodes map[string]bool
	root       string
}

type configuredCheck struct {
	severity   diagnostic.Severity
	excludes   []string
	characters []rune
	config     config.CheckConfig
}

// RegistryOptions selects and configures concrete-syntax checks.
type RegistryOptions struct {
	Only            []string
	Settings        map[string]config.CheckConfig
	Root            string
	MinimumSeverity diagnostic.Severity
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

type fileResult struct {
	filename    string
	diagnostics []diagnostic.Diagnostic
	err         error
}

func NewRegistry(only []string) (*Registry, error) {
	return NewRegistryConfigured(only, nil, "")
}

// NewRegistryConfigured applies project check settings.
func NewRegistryConfigured(only []string, settings map[string]config.CheckConfig, root string) (*Registry, error) {
	return NewRegistryWithOptions(RegistryOptions{
		Only:            only,
		Settings:        settings,
		Root:            root,
		MinimumSeverity: diagnostic.SeverityNote,
	})
}

// NewRegistryWithOptions applies project settings and a minimum effective
// severity. Explicit selection never bypasses the severity threshold.
func NewRegistryWithOptions(options RegistryOptions) (*Registry, error) {
	all, err := builtinchecks.Select(nil)
	if err != nil {
		return nil, err
	}
	selection, err := core.Select(
		core.SelectionOptions[builtinchecks.Check]{
			Checks:          all,
			Only:            options.Only,
			Settings:        options.Settings,
			MinimumSeverity: options.MinimumSeverity,
			Validate:        validateConfiguredCheck,
		},
	)
	if err != nil {
		return nil, err
	}
	registry := &Registry{
		settings:   make(map[string]configuredCheck, len(all)),
		knownCodes: selection.KnownCodes,
		root:       source.ResolveRoot(options.Root),
	}
	for _, check := range selection.Checks {
		meta := check.Meta()
		ruleConfig := selection.Settings[strings.ToLower(meta.Code)]
		severity := selection.Severities[meta.Code]
		registry.checks = append(registry.checks, check)
		configured := configuredCheck{
			severity: severity,
			excludes: ruleConfig.Excludes,
			config:   ruleConfig,
		}
		if meta.Code == "banned-characters" {
			characters, _ := core.StringsOption(meta, ruleConfig, "characters")
			configured.characters = make([]rune, 0, len(characters))
			for _, character := range characters {
				configured.characters = append(configured.characters, []rune(character)[0])
			}
		}
		registry.settings[meta.Code] = configured
	}
	return registry, nil
}

func validateConfiguredCheck(code string, setting config.CheckConfig) error {
	if setting.Characters != nil && code != "banned-characters" {
		return fmt.Errorf("check %q does not support characters", code)
	}
	for _, character := range setting.Characters {
		if len([]rune(character)) != 1 {
			return fmt.Errorf("check %q characters must contain exactly one Unicode character each", code)
		}
	}
	return nil
}

func (r *Registry) Checks() []builtinchecks.Check {
	return append([]builtinchecks.Check(nil), r.checks...)
}

// KnownCodes returns every concrete-syntax check code, including checks that
// are disabled or below the current severity threshold.
func (r *Registry) KnownCodes() map[string]bool {
	if r == nil {
		return nil
	}
	result := make(map[string]bool, len(r.knownCodes))
	for code := range r.knownCodes {
		result[code] = true
	}
	return result
}

func (r *Registry) Severity(code string) diagnostic.Severity {
	return r.settings[code].severity
}

func (r *Registry) bannedCharacters() []rune {
	return append([]rune(nil), r.settings["banned-characters"].characters...)
}

func (r *Registry) limits() map[string]int {
	limits := make(map[string]int)
	for _, check := range r.checks {
		code := check.Meta().Code
		setting := r.settings[code]
		switch code {
		case "file-length-limit":
			limits[code], _ = core.IntOption(check.Meta(), setting.config, "max-lines")
		case "function-length":
			limits[code+"-lines"], _ = core.IntOption(check.Meta(), setting.config, "max-lines")
			limits[code+"-statements"], _ = core.IntOption(check.Meta(), setting.config, "max-statements")
		case "function-result-limit":
			limits[code], _ = core.IntOption(check.Meta(), setting.config, "max-results")
		case "max-parameters":
			limits[code], _ = core.IntOption(check.Meta(), setting.config, "max-parameters")
		case "max-public-structs":
			limits[code], _ = core.IntOption(check.Meta(), setting.config, "max-public-structs")
		}
	}
	return limits
}

func (r *Registry) blockedImports() []string {
	for _, check := range r.checks {
		if check.Meta().Code == "imports-blocklist" {
			paths, _ := core.StringsOption(check.Meta(), r.settings[check.Meta().Code].config, "blocked-imports")
			return paths
		}
	}
	return nil
}

func (r *Registry) activeChecks(filename string) []builtinchecks.Check {
	active := make([]builtinchecks.Check, 0, len(r.checks))
	for _, check := range r.checks {
		if pathfilter.Excluded(r.root, filename, r.settings[check.Meta().Code].excludes) {
			continue
		}
		active = append(active, check)
	}
	return active
}

// Applies reports whether at least one selected concrete-syntax check applies
// to filename.
func (r *Registry) Applies(filename string) bool {
	if r == nil {
		return false
	}
	for _, check := range r.checks {
		if !pathfilter.Excluded(r.root, filename, r.settings[check.Meta().Code].excludes) {
			return true
		}
	}
	return false
}

func Run(files []string, registry *Registry) ([]diagnostic.Diagnostic, error) {
	if len(files) == 0 {
		return []diagnostic.Diagnostic{}, nil
	}
	if len(files) == 1 {
		diagnostics, err := checkFile(files[0], registry)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", source.DisplayPath(files[0]), err)
		}
		if diagnostics == nil {
			diagnostics = []diagnostic.Diagnostic{}
		}
		sortDiagnostics(diagnostics)
		return diagnostics, nil
	}
	workers := min(runtime.GOMAXPROCS(0), max(1, len(files)))
	jobs := make(chan string)
	results := make(chan fileResult, len(files))
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for filename := range jobs {
				diagnostics, err := checkFile(filename, registry)
				results <- fileResult{
					filename:    filename,
					diagnostics: diagnostics,
					err:         err,
				}
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
		sort.Slice(errorsByFile, func(i, j int) bool {
			return errorsByFile[i].filename < errorsByFile[j].filename
		})
		return nil, fmt.Errorf("%s: %w", source.DisplayPath(errorsByFile[0].filename), errorsByFile[0].err)
	}
	sortDiagnostics(allDiagnostics)
	return allDiagnostics, nil
}

func sortDiagnostics(diagnostics []diagnostic.Diagnostic) {
	sort.SliceStable(
		diagnostics,
		func(i, j int) bool {
			left, right := diagnostics[i], diagnostics[j]
			if left.File != right.File {
				return left.File < right.File
			}
			if left.Start.Offset != right.Start.Offset {
				return left.Start.Offset < right.Start.Offset
			}
			if left.Code != right.Code {
				return left.Code < right.Code
			}
			if left.Message != right.Message {
				return left.Message < right.Message
			}
			if left.End.Offset != right.End.Offset {
				return left.End.Offset < right.End.Offset
			}
			return left.Severity < right.Severity
		},
	)
}

func checkFile(filename string, registry *Registry) ([]diagnostic.Diagnostic, error) {
	activeChecks := registry.activeChecks(filename)
	if len(activeChecks) == 0 {
		return nil, nil
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	concreteTree, err := cst.Parse(filename, content)
	if err != nil {
		return nil, err
	}
	return analyzeTree(filename, concreteTree, activeChecks, registry), nil
}

// AnalyzeTree runs the selected concrete-syntax checks against a shared tree.
func AnalyzeTree(filename string, concreteTree *cst.Tree, registry *Registry) []diagnostic.Diagnostic {
	if concreteTree == nil || registry == nil {
		return nil
	}
	activeChecks := registry.activeChecks(filename)
	return analyzeTree(filename, concreteTree, activeChecks, registry)
}

func analyzeTree(filename string, concreteTree *cst.Tree, activeChecks []builtinchecks.Check, registry *Registry) []diagnostic.Diagnostic {
	if len(activeChecks) == 0 {
		return nil
	}
	concreteIgnores, concreteNodes := concreteSuppressions(concreteTree)
	context := &Context{
		filename:        filename,
		displayFilename: source.DiagnosticPath(registry.root, filename),
		concreteIgnores: concreteIgnores,
		concreteNodes:   concreteNodes,
	}
	builtinchecks.AnalyzeCST(
		builtinchecks.CSTInput{
			Filename:         filename,
			Tree:             concreteTree,
			Checks:           activeChecks,
			BannedCharacters: registry.bannedCharacters(),
			Limits:           registry.limits(),
			BlockedImports:   registry.blockedImports(),
			Report: func(finding builtinchecks.Finding) {
				context.reportConcrete(concreteTree, finding, registry.Severity(finding.Code))
			},
		},
	)
	return context.diagnostics
}

func (c *Context) reportConcrete(tree *cst.Tree, finding builtinchecks.Finding, severity diagnostic.Severity) {
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
