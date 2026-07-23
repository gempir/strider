// Command catalogreview generates the evidence used to review Strider's
// built-in checks as a product surface. The report deliberately combines
// stable catalog facts with the latest pinned-corpus measurement.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gempir/strider/internal/checks"
	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
)

type options struct {
	root         string
	baselinePath string
	corpusPath   string
	configPath   string
	outputPath   string
}

type report struct {
	Version       int                  `json:"version"`
	Summary       reportSummary        `json:"summary"`
	Corpus        corpusSummary        `json:"corpus"`
	Checks        []checkReview        `json:"checks"`
	RemovedChecks []removedCheckReview `json:"removed_checks"`
}

type reportSummary struct {
	Retained          int            `json:"retained"`
	Removed           int            `json:"removed"`
	ByEngine          map[string]int `json:"by_engine"`
	ByDefaultSeverity map[string]int `json:"by_default_severity"`
	ProjectOverrides  int            `json:"project_overrides"`
}

type corpusSummary struct {
	Projects       int   `json:"projects"`
	CheckRuntimeMS int64 `json:"check_runtime_ms"`
	Findings       int   `json:"findings"`
}

type checkReview struct {
	Code                     string `json:"code"`
	Engine                   string `json:"engine"`
	Stage                    string `json:"stage"`
	ImplementationLOC        int    `json:"implementation_loc"`
	ImplementationUnit       string `json:"implementation_unit"`
	CorpusFindings           int    `json:"corpus_findings"`
	DefaultSeverity          string `json:"default_severity"`
	ProjectEffectiveSeverity string `json:"project_effective_severity"`
	ProjectOverride          bool   `json:"project_override"`
	OverrideRationale        string `json:"override_rationale,omitempty"`
	FixAvailability          string `json:"fix_availability"`
	AnalyzerOverlap          string `json:"analyzer_overlap"`
	Decision                 string `json:"decision"`
	SignalRationale          string `json:"signal_rationale"`
	DecisionRationale        string `json:"decision_rationale"`
	FalsePositiveBoundary    string `json:"false_positive_boundary"`
	ExampleEvidence          string `json:"example_evidence"`
}

type removedCheckReview struct {
	Code                     string `json:"code"`
	PreviousStage            string `json:"previous_stage"`
	RemovedImplementationLOC int    `json:"removed_implementation_loc"`
	CorpusFindings           int    `json:"corpus_findings"`
	PreviousSeverity         string `json:"previous_severity"`
	AnalyzerOverlap          string `json:"analyzer_overlap"`
	Decision                 string `json:"decision"`
	DecisionRationale        string `json:"decision_rationale"`
	FalsePositiveBoundary    string `json:"false_positive_boundary"`
}

type decision struct {
	action    string
	rationale string
	boundary  string
	overlap   string
}

var reviewedDecisions = map[string]decision{
	"append-to-sized-slice": {
		action:    "keep",
		rationale: "Retained as a correctness check: appending to a positive-length make commonly leaves unintended zero-value elements. The pinned corpus has a concrete occurrence and focused tests cover reslicing, aliases, and non-local values.",
		boundary:  "Only local slices created with a provably positive length and traced to append are reported; unknown lengths, cycles, and explicit reslicing to zero are left alone.",
		overlap:   "none identified in go vet or Staticcheck",
	},
	"context-cancel-in-loop": {
		action:    "keep",
		rationale: "Retained because cancellation deferred to the surrounding function retains timers and parent references across iterations. Corpus occurrences and path-adversarial tests justify warning severity.",
		boundary:  "Only directly acquired context cancellation functions inside a loop are followed; cancellation proven on every local control-flow path is accepted.",
		overlap:   "go vet lostcancel checks discarded cancellation functions but does not diagnose cancellation retained across loop iterations",
	},
	"slice-preallocation": {
		action:    "demote",
		rationale: "Retained as an advisory optimization, not a correctness policy. Corpus signal is substantial, but allocation benefit depends on workload and therefore cannot justify a default warning.",
		boundary:  "Only a local empty slice appended exactly once per iteration over a source with a useful length is reported; mutation, conditional append, aliasing, and self-ranging cases are excluded.",
		overlap:   "overlaps the established prealloc analyzer",
	},
	"unclosed-http-response-body": {
		action:    "keep",
		rationale: "Retained because an unclosed response body prevents connection reuse and can leak resources. The corpus has repeated actionable findings and focused ownership-boundary tests.",
		boundary:  "Only directly owned local responses are followed. Returned, aliased, conditionally managed, or interprocedurally closed bodies are intentionally not guessed about.",
		overlap:   "go vet and Staticcheck do not provide the same local response-body ownership check",
	},
	"unclosed-sql-resource": {
		action:    "keep",
		rationale: "Retained because sql.Rows and sql.Stmt have explicit Close ownership. It shares the conservative resource engine and adversarial tests with the corpus-proven HTTP check, so its incremental complexity is small despite zero current corpus findings.",
		boundary:  "Only directly owned local rows and statements are followed. Returned, aliased, conditionally managed, or interprocedurally closed resources are left unreported.",
		overlap:   "none identified in go vet or Staticcheck",
	},
}

var removedDecisions = []removedCheckReview{
	{
		Code:                     "failed-assertion-shadow-read",
		PreviousStage:            "types",
		RemovedImplementationLOC: 238,
		PreviousSeverity:         "warning",
		AnalyzerOverlap:          "Staticcheck SA9008",
		Decision:                 "delete",
		DecisionRationale:        "The behavior is already covered by Staticcheck SA9008, had no pinned-corpus findings, and required a bespoke shadow-use traversal.",
		FalsePositiveBoundary:    "Staticcheck owns the type-assertion else-branch pattern; Strider no longer maintains a second approximation.",
	},
	{
		Code:                     "invalid-printf-call",
		PreviousStage:            "types",
		RemovedImplementationLOC: 388,
		PreviousSeverity:         "error",
		AnalyzerOverlap:          "go vet printf analyzer",
		Decision:                 "delete",
		DecisionRationale:        "The standard go vet printf analyzer is authoritative, extensible through printf wrappers, and already maintained with the language. The local parser had no pinned-corpus findings.",
		FalsePositiveBoundary:    "Printf format validation is delegated to go vet instead of maintaining a divergent format-language implementation.",
	},
	{
		Code:                     "possible-nil-dereference",
		PreviousStage:            "ssa",
		RemovedImplementationLOC: 151,
		CorpusFindings:           31,
		PreviousSeverity:         "error",
		AnalyzerOverlap:          "Staticcheck SA5011",
		Decision:                 "delete",
		DecisionRationale:        "Staticcheck SA5011 covers the same nil-check/dereference relationship with a mature analyzer. Corpus volume did not justify retaining a duplicate SSA approximation.",
		FalsePositiveBoundary:    "Nil-path reasoning is delegated to Staticcheck; Strider no longer reports from evidence of a nil comparison alone.",
	},
}

var analyzerOverlap = map[string]string{
	"copy-lock-value":    "go vet copylocks analyzer",
	"invalid-struct-tag": "go vet structtag analyzer",
	"lost-cancel":        "go vet lostcancel analyzer",
	"unreachable-code":   "overlaps Staticcheck unreachable-code analyzers",
	"unused-result":      "go vet unusedresult analyzer",
}

var safeFixes = map[string]bool{
	"double-negation":        true,
	"format":                 true,
	"redundant-break":        true,
	"single-argument-append": true,
}

func main() {
	var options options
	flag.StringVar(&options.root, "root", ".", "repository root")
	flag.StringVar(&options.baselinePath, "baseline", "benchmarks/baseline.json", "reviewed corpus baseline")
	flag.StringVar(&options.corpusPath, "corpus-report", "target/corpus/report.json", "latest measured corpus report")
	flag.StringVar(&options.configPath, "config", "strider.toml", "project configuration")
	flag.StringVar(&options.outputPath, "output", "benchmarks/catalog-review.json", "generated review report")
	flag.Parse()
	if err := run(options); err != nil {
		fmt.Fprintln(os.Stderr, "catalogreview:", err)
		os.Exit(1)
	}
}

func run(options options) error {
	root, err := filepath.Abs(options.root)
	if err != nil {
		return err
	}
	configuration, err := config.Load(root, options.configPath, false)
	if err != nil {
		return err
	}
	registry, err := checks.NewRegistry(
		checks.RegistryOptions{
			Settings:        configuration.Checks.Settings,
			MinimumSeverity: diagnostic.SeverityNone,
			Root:            configuration.Root,
			Directory:       root,
		},
	)
	if err != nil {
		return err
	}
	findings, err := corpusFindings(filepath.Join(root, options.baselinePath))
	if err != nil {
		return err
	}
	corpus, err := measuredCorpus(filepath.Join(root, options.corpusPath))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		corpus, err = previousCorpusSummary(filepath.Join(root, options.outputPath))
		if err != nil {
			return fmt.Errorf("read measured or previous corpus evidence: %w", err)
		}
	}
	sources, err := implementationSources(root)
	if err != nil {
		return err
	}
	stages, err := semanticStages(root)
	if err != nil {
		return err
	}

	result := report{
		Version:       1,
		Corpus:        corpus,
		RemovedChecks: append([]removedCheckReview(nil), removedDecisions...),
		Summary: reportSummary{
			ByEngine:          make(map[string]int),
			ByDefaultSeverity: make(map[string]int),
		},
	}
	for _, descriptor := range registry.Checks() {
		meta := descriptor.Meta()
		code := meta.Code
		source := sources[code]
		stage := string(descriptor.Engine())
		if descriptor.Engine() == checks.EngineSyntax {
			stage = "cst"
		} else if descriptor.Engine() == checks.EngineSemantic {
			stage = stages[code]
		}
		effective := registry.Severity(code)
		review := decisionFor(meta)
		entry := checkReview{
			Code:                     code,
			Engine:                   string(descriptor.Engine()),
			Stage:                    stage,
			ImplementationLOC:        source.loc,
			ImplementationUnit:       source.unit,
			CorpusFindings:           findings[code],
			DefaultSeverity:          string(meta.DefaultSeverity),
			ProjectEffectiveSeverity: string(effective),
			ProjectOverride:          effective != meta.DefaultSeverity,
			FixAvailability:          fixAvailability(code),
			AnalyzerOverlap:          review.overlap,
			Decision:                 review.action,
			SignalRationale:          meta.Explanation,
			DecisionRationale:        review.rationale,
			FalsePositiveBoundary:    review.boundary,
			ExampleEvidence:          exampleEvidence(root, descriptor),
		}
		if entry.ProjectOverride {
			entry.OverrideRationale = "The repository predates this policy and keeps existing violations advisory; the grouped policy comments in strider.toml make the exception explicit."
			result.Summary.ProjectOverrides++
		}
		result.Checks = append(result.Checks, entry)
		result.Summary.ByEngine[entry.Engine]++
		result.Summary.ByDefaultSeverity[entry.DefaultSeverity]++
	}
	result.Summary.Retained = len(result.Checks)
	result.Summary.Removed = len(result.RemovedChecks)
	sort.Slice(result.Checks, func(i, j int) bool {
		return result.Checks[i].Code < result.Checks[j].Code
	})
	sort.Slice(result.RemovedChecks, func(i, j int) bool {
		return result.RemovedChecks[i].Code < result.RemovedChecks[j].Code
	})
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	output := filepath.Join(root, options.outputPath)
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	return os.WriteFile(output, encoded, 0o644)
}

func decisionFor(meta checks.Meta) decision {
	if reviewed, ok := reviewedDecisions[meta.Code]; ok {
		return reviewed
	}
	overlap := analyzerOverlap[meta.Code]
	if overlap == "" {
		overlap = "none identified in go vet or Staticcheck"
	}
	action := "keep"
	rationale := "Retained at its existing default because its documented pattern has a direct remediation and focused catalog tests protect the implementation contract."
	if meta.DefaultSeverity == diagnostic.SeverityNote {
		rationale = "Retained as an opt-in advisory policy; projects can enable it without making subjective style or optimization guidance fail default checks."
	}
	return decision{
		action:    action,
		rationale: rationale,
		boundary:  "Reports only the documented syntactic, type, or data-flow pattern; the focused good example and negative tests define intentional exclusions.",
		overlap:   overlap,
	}
}

func fixAvailability(code string) string {
	if safeFixes[code] {
		return "safe automatic fix"
	}
	return "none"
}

type implementationSource struct {
	unit string
	loc  int
}

func implementationSources(root string) (map[string]implementationSource, error) {
	result := map[string]implementationSource{
		"format": {
			unit: "internal/checks/run.go",
			loc:  32,
		},
	}
	for _, engine := range []string{
		"syntax",
		"semantic",
	} {
		directory := filepath.Join(root, "internal", "checks", engine)
		entries, err := os.ReadDir(directory)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") || entry.Name() == "catalog.go" || entry.Name() == "registry.go" {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			contents, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			loc := logicalLOC(contents)
			for _, code := range quotedCheckCodes(contents) {
				result[code] = implementationSource{
					unit: filepath.ToSlash(filepath.Join("internal", "checks", engine, entry.Name())),
					loc:  loc,
				}
			}
		}
	}
	syntax, err := syntaxImplementationSources(root)
	if err != nil {
		return nil, err
	}
	for code, source := range syntax {
		if _, found := result[code]; !found {
			result[code] = source
		}
	}
	return result, nil
}

func quotedCheckCodes(contents []byte) []string {
	codes := make([]string, 0, 2)
	scanner := bufio.NewScanner(strings.NewReader(string(contents)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "Code:") {
			continue
		}
		start := strings.IndexByte(line, '"')
		end := strings.LastIndexByte(line, '"')
		if start >= 0 && end > start {
			codes = append(codes, line[start+1:end])
		}
	}
	return codes
}

func logicalLOC(contents []byte) int {
	count := 0
	scanner := bufio.NewScanner(strings.NewReader(string(contents)))
	inBlockComment := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if inBlockComment {
			if end := strings.Index(line, "*/"); end >= 0 {
				line = strings.TrimSpace(line[end+2:])
				inBlockComment = false
			} else {
				continue
			}
		}
		if strings.HasPrefix(line, "/*") {
			if end := strings.Index(line[2:], "*/"); end >= 0 {
				line = strings.TrimSpace(line[end+4:])
			} else {
				inBlockComment = true
				continue
			}
		}
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		count++
	}
	return count
}

func syntaxImplementationSources(root string) (map[string]implementationSource, error) {
	// Syntax checks share a single dispatch catalog. Their implementation LOC
	// is the primary behavior declaration referenced by each catalog entry.
	path := filepath.Join(root, "internal", "checks", "syntax", "catalog.go")
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return nil, err
	}
	declarations, err := syntaxDeclarations(root, fileSet)
	if err != nil {
		return nil, err
	}
	result := make(map[string]implementationSource)
	ast.Inspect(
		file,
		func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			code := ""
			var behavior ast.Expr
			for _, element := range literal.Elts {
				field, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				name, ok := field.Key.(*ast.Ident)
				if !ok {
					continue
				}
				switch name.Name {
				case "behavior":
					behavior = field.Value
				case "meta":
					ast.Inspect(
						field.Value,
						func(node ast.Node) bool {
							metaField, ok := node.(*ast.KeyValueExpr)
							if !ok {
								return true
							}
							metaName, nameOK := metaField.Key.(*ast.Ident)
							value, valueOK := metaField.Value.(*ast.BasicLit)
							if nameOK && valueOK && metaName.Name == "Code" {
								decoded, unquoteErr := strconv.Unquote(value.Value)
								if unquoteErr == nil {
									code = decoded
								}
								return false
							}
							return true
						},
					)
				}
			}
			if code == "" || behavior == nil {
				return true
			}
			sources := make([]implementationSource, 0)
			seen := make(map[string]bool)
			for _, name := range primaryBehaviorNames(behavior) {
				source, found := declarations[name]
				if !found || seen[source.unit] {
					continue
				}
				seen[source.unit] = true
				sources = append(sources, source)
			}
			if len(sources) == 0 {
				start := fileSet.Position(behavior.Pos()).Line
				end := fileSet.Position(behavior.End()).Line
				sources = append(sources, implementationSource{
					unit: "internal/checks/syntax/catalog.go:" + strconv.Itoa(start),
					loc:  end - start + 1,
				})
			}
			sort.Slice(sources, func(i, j int) bool {
				return sources[i].unit < sources[j].unit
			})
			units := make([]string, len(sources))
			loc := 0
			for index, source := range sources {
				units[index] = source.unit
				loc += source.loc
			}
			result[code] = implementationSource{
				unit: strings.Join(units, ", "),
				loc:  loc,
			}
			return true
		},
	)
	return result, nil
}

func syntaxDeclarations(root string, fileSet *token.FileSet) (map[string]implementationSource, error) {
	result := make(map[string]implementationSource)
	directory := filepath.Join(root, "internal", "checks", "syntax")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), "_test.go") || !strings.HasSuffix(entry.Name(), ".go") || entry.Name() == "catalog.go" {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			return nil, parseErr
		}
		add := func(name string, node ast.Node) {
			start := fileSet.Position(node.Pos()).Line
			end := fileSet.Position(node.End()).Line
			result[name] = implementationSource{
				unit: filepath.ToSlash(filepath.Join("internal", "checks", "syntax", entry.Name())) + ":" + strconv.Itoa(start),
				loc:  end - start + 1,
			}
		}
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				add(declaration.Name.Name, declaration)
			case *ast.GenDecl:
				if declaration.Tok != token.VAR {
					continue
				}
				for _, specification := range declaration.Specs {
					value, ok := specification.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, name := range value.Names {
						add(name.Name, value)
					}
				}
			}
		}
	}
	return result, nil
}

func primaryBehaviorNames(expression ast.Expr) []string {
	names := make(map[string]bool)
	ast.Inspect(
		expression,
		func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.SelectorExpr:
				if strings.HasPrefix(node.Sel.Name, "check") || strings.HasPrefix(node.Sel.Name, "inspect") || strings.HasPrefix(node.Sel.Name, "observe") {
					names[node.Sel.Name] = true
				}
			case *ast.Ident:
				if strings.HasSuffix(node.Name, "Behavior") || strings.HasPrefix(node.Name, "inspect") {
					names[node.Name] = true
				}
			}
			return true
		},
	)
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func semanticStages(root string) (map[string]string, error) {
	path := filepath.Join(root, "internal", "checks", "semantic", "registry.go")
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	ast.Inspect(
		file,
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			constructor, ok := call.Fun.(*ast.Ident)
			if !ok || constructor.Name != "typeCheck" && constructor.Name != "ssaCheck" && constructor.Name != "semanticCheck" {
				return true
			}
			checkType := firstCompositeType(call.Args[0])
			if checkType == "" {
				return true
			}
			stage := "types"
			if constructor.Name == "ssaCheck" || constructor.Name == "semanticCheck" && expressionContains(call, "AnalysisStageSSA") {
				stage = "ssa"
			}
			result[checkType] = stage
			return true
		},
	)
	// Join receiver type to code through the Meta methods in semantic sources.
	byCode := make(map[string]string)
	directory := filepath.Join(root, "internal", "checks", "semantic")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), "_test.go") || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		sourcePath := filepath.Join(directory, entry.Name())
		parsed, parseErr := parser.ParseFile(fileSet, sourcePath, nil, 0)
		if parseErr != nil {
			return nil, parseErr
		}
		codes := quotedCheckCodesFromAST(parsed)
		for receiver, code := range codes {
			if stage := result[receiver]; stage != "" {
				byCode[code] = stage
			}
		}
	}
	return byCode, nil
}

func firstCompositeType(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.CompositeLit:
		if name, ok := expression.Type.(*ast.Ident); ok {
			return name.Name
		}
	case *ast.CallExpr:
		if len(expression.Args) != 0 {
			return firstCompositeType(expression.Args[0])
		}
	}
	return ""
}

func expressionContains(node ast.Node, name string) bool {
	found := false
	ast.Inspect(node, func(current ast.Node) bool {
		if identifier, ok := current.(*ast.Ident); ok && identifier.Name == name {
			found = true
			return false
		}
		return !found
	})
	return found
}

func quotedCheckCodesFromAST(file *ast.File) map[string]string {
	result := make(map[string]string)
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "Meta" || function.Recv == nil || len(function.Recv.List) != 1 {
			continue
		}
		receiver, ok := function.Recv.List[0].Type.(*ast.Ident)
		if !ok {
			continue
		}
		code := ""
		ast.Inspect(
			function.Body,
			func(node ast.Node) bool {
				field, ok := node.(*ast.KeyValueExpr)
				if !ok {
					return true
				}
				name, ok := field.Key.(*ast.Ident)
				value, literal := field.Value.(*ast.BasicLit)
				if ok && literal && name.Name == "Code" {
					decoded, unquoteErr := strconv.Unquote(value.Value)
					if unquoteErr == nil {
						code = decoded
					}
					return false
				}
				return true
			},
		)
		if code != "" {
			result[receiver.Name] = code
		}
	}
	return result
}

func corpusFindings(path string) (map[string]int, error) {
	var baseline struct {
		Projects []struct {
			Operations map[string]struct {
				ByCode map[string]int `json:"by_code"`
			} `json:"operations"`
		} `json:"projects"`
	}
	if err := decodeJSON(path, &baseline); err != nil {
		return nil, err
	}
	result := make(map[string]int)
	for _, project := range baseline.Projects {
		for code, count := range project.Operations["check"].ByCode {
			result[code] += count
		}
	}
	return result, nil
}

func measuredCorpus(path string) (corpusSummary, error) {
	var measured struct {
		Projects []struct {
			Operations []struct {
				Name       string `json:"name"`
				DurationMS int64  `json:"duration_ms"`
				Findings   int    `json:"findings"`
			} `json:"operations"`
		} `json:"projects"`
	}
	if err := decodeJSON(path, &measured); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return corpusSummary{}, fmt.Errorf("run make corpus-check before catalog review: %w", err)
		}
		return corpusSummary{}, err
	}
	result := corpusSummary{
		Projects: len(measured.Projects),
	}
	for _, project := range measured.Projects {
		for _, operation := range project.Operations {
			if operation.Name == "check" {
				result.CheckRuntimeMS += operation.DurationMS
				result.Findings += operation.Findings
			}
		}
	}
	return result, nil
}

func previousCorpusSummary(path string) (corpusSummary, error) {
	var previous struct {
		Corpus corpusSummary `json:"corpus"`
	}
	if err := decodeJSON(path, &previous); err != nil {
		return corpusSummary{}, err
	}
	if previous.Corpus.Projects == 0 {
		return corpusSummary{}, fmt.Errorf("previous catalog review has no corpus evidence")
	}
	return previous.Corpus, nil
}

func decodeJSON(path string, destination any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(destination)
}

func exampleEvidence(root string, descriptor checks.Descriptor) string {
	code := strings.ReplaceAll(descriptor.Meta().Code, "-", "_")
	directory := filepath.Join(root, "internal", "checks", string(descriptor.Engine()))
	matches, globErr := filepath.Glob(filepath.Join(directory, "*"+code+"*_test.go"))
	if globErr == nil && len(matches) != 0 {
		return "focused positive and adversarial test"
	}
	return "documented good/bad fragment validated by the catalog contract"
}
