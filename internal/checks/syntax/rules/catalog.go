package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gempir/strider/internal/diagnostic"
)

var coreCatalog = []definition{
	{
		meta: Meta{
			Code:            "cyclomatic-complexity",
			Summary:         "limit branching complexity",
			DefaultSeverity: diagnostic.SeverityWarning,
			Explanation:     "Functions with too many independent control-flow paths are difficult to understand and test. The maximum complexity is 10.",
			GoodExample:     "func sign(n int) int { if n < 0 { return -1 }; return 1 }",
			BadExample:      "func tangled(n int) { if n > 0 {}; if n > 1 {}; if n > 2 {}; if n > 3 {}; if n > 4 {}; if n > 5 {}; if n > 6 {}; if n > 7 {}; if n > 8 {}; if n > 9 {}; if n > 10 {} }",
		},
	},
	{
		meta: Meta{
			Code:            "max-parameters",
			Summary:         "limit function parameter count",
			DefaultSeverity: diagnostic.SeverityWarning,
			Explanation:     "Functions with more than eight parameters tend to hide missing domain objects and are difficult to call correctly.",
			GoodExample:     "func Open(path string, flags Flags) error",
			BadExample:      "func Open(path string, read, write, create, truncate, appendMode, sync, exclusive, temporary bool) error",
		},
	},
	{
		meta: Meta{
			Code:            "no-naked-return",
			Summary:         "require explicit return values",
			DefaultSeverity: diagnostic.SeverityWarning,
			Explanation:     "A bare return in a function with named results makes data flow implicit, especially in longer functions.",
			GoodExample:     "func value() (n int) { n = 1; return n }",
			BadExample:      "func value() (n int) { n = 1; return }",
		},
	},
	{
		meta: Meta{
			Code:            "no-init",
			Summary:         "avoid implicit package initialization",
			DefaultSeverity: diagnostic.SeverityNote,
			Explanation:     "init functions hide ordering and side effects. Prefer explicit construction called from main or tests.",
			GoodExample:     "func Configure() error { return register() }",
			BadExample:      "func init() { register() }",
		},
	},
	{
		meta: Meta{
			Code:            "no-package-var",
			Summary:         "avoid mutable package state",
			DefaultSeverity: diagnostic.SeverityWarning,
			Explanation:     "Package variables create shared mutable state and make dependencies, tests, and concurrency harder to reason about. Blank-identifier compile-time assertions are exempt.",
			GoodExample:     "const defaultLimit = 10",
			BadExample:      "var defaultClient = newClient()",
		},
	},
	{
		meta: Meta{
			Code:            "no-defer-in-loop",
			Summary:         "avoid accumulating defers in loops",
			DefaultSeverity: diagnostic.SeverityWarning,
			Explanation:     "A defer runs when the surrounding function returns, not when an iteration ends, so resources can accumulate unexpectedly.",
			GoodExample:     "for rows.Next() { if err := handleRow(rows); err != nil { return err } }",
			BadExample:      "for rows.Next() { defer rows.Close() }",
		},
	},
	{
		meta: Meta{
			Code:            "no-else-after-return",
			Summary:         "remove else after terminal return",
			DefaultSeverity: diagnostic.SeverityWarning,
			Explanation:     "When the if branch returns, the else branch can be unindented. This reduces nesting without changing control flow.",
			GoodExample:     "if err != nil { return err }\nuse(value)",
			BadExample:      "if err != nil { return err } else { use(value) }",
		},
	},
}

// The extended rules share the same native analysis pass as the core rules.
// Examples live in examples.go because they are also used to generate the lint
// reference pages.
var extendedCatalog = []spec{
	{
		"add-constant",
		"suggest named constants for repeated literals",
		"strings after 2 repetitions",
	},
	{
		"redundant-atomic-result-assignment",
		"avoid assigning an atomic result back to its operand",
		"standard sync/atomic patterns",
	},
	{
		"banned-characters",
		"reject configured characters in identifiers",
		"ᐸ and ᐳ",
	},
	{
		"bidirectional-control-character",
		"reject invisible bidirectional source controls",
		"enabled",
	},
	{
		"blank-imports",
		"require explanatory comments on blank imports outside main and test files",
		"main and test packages exempt",
	},
	{
		"boolean-literal-comparison",
		"simplify comparisons between booleans and literals",
		"enabled",
	},
	{
		"call-to-gc",
		"discourage explicit garbage collection",
		"runtime.GC",
	},
	{
		"cognitive-complexity",
		"limit nested control-flow complexity",
		"maximum 7",
	},
	{
		"confusing-naming",
		"reject declarations whose names differ only by capitalization",
		"methods and fields",
	},
	{
		"confusing-results",
		"name consecutive unnamed results that have the same type",
		"enabled",
	},
	{
		"constant-logical-expr",
		"detect constant logical expressions",
		"enabled",
	},
	{
		"context-as-argument",
		"require context.Context to be the first parameter",
		"enabled",
	},
	{
		"deep-exit",
		"keep process-terminating calls in main or init",
		"enabled",
	},
	{
		"deferred-recover-call",
		"defer recover through a function closure",
		"enabled",
	},
	{
		"discarded-deferred-result",
		"avoid return values discarded by deferred calls",
		"enabled",
	},
	{
		"dot-imports",
		"discourage dot imports",
		"no allowed packages",
	},
	{
		"double-negation",
		"remove redundant double boolean negation",
		"enabled",
	},
	{
		"duplicated-imports",
		"reject importing the same package more than once",
		"enabled",
	},
	{
		"early-return",
		"prefer early returns that reduce nesting",
		"enabled",
	},
	{
		"empty-conditional-block",
		"detect empty conditional blocks",
		"enabled",
	},
	{
		"enforce-switch-style",
		"require default clauses to be last",
		"default optional",
	},
	{
		"error-naming",
		"name package errors with an Err prefix",
		"enabled",
	},
	{
		"error-last-result",
		"place error last in result lists",
		"enabled",
	},
	{
		"error-strings",
		"use lower-case unpunctuated error messages",
		"enabled",
	},
	{
		"prefer-fmt-errorf",
		"replace errors.New around fmt.Sprintf",
		"enabled",
	},
	{
		"exported-declaration-comment",
		"document exported declarations",
		"enabled",
	},
	{
		"file-length-limit",
		"limit source-file length",
		"maximum 500 lines; set 0 to disable",
	},
	{
		"filename-format",
		"enforce Go source filename format",
		"conventional characters",
	},
	{
		"flag-parameter",
		"detect boolean control parameters",
		"enabled",
	},
	{
		"function-length",
		"limit function statements and lines",
		"50 statements; 75 lines",
	},
	{
		"function-result-limit",
		"limit function result count",
		"maximum 3",
	},
	{
		"get-function-return-value",
		"require Get-prefixed functions to return values",
		"enabled",
	},
	{
		"identical-branches",
		"detect identical if branches",
		"enabled",
	},
	{
		"identical-if-chain-branches",
		"detect repeated if-chain branches",
		"enabled",
	},
	{
		"identical-if-chain-conditions",
		"detect repeated if-chain conditions",
		"enabled",
	},
	{
		"identical-switch-branches",
		"detect repeated switch branches",
		"enabled",
	},
	{
		"identical-switch-conditions",
		"detect repeated switch conditions",
		"enabled",
	},
	{
		"redundant-error-return-check",
		"simplify redundant error checks before returning",
		"enabled",
	},
	{
		"ineffective-pointer-copy",
		"detect pointer round trips that do not copy values",
		"enabled",
	},
	{
		"import-alias-naming",
		"enforce conventional import aliases",
		"lower-case letters and digits",
	},
	{
		"import-shadowing",
		"detect declarations shadowing imports",
		"enabled",
	},
	{
		"imports-blocklist",
		"reject configured imports",
		"empty blocklist",
	},
	{
		"increment-decrement",
		"prefer increment and decrement statements",
		"enabled",
	},
	{
		"inefficient-map-lookup",
		"avoid repeated map lookups",
		"enabled",
	},
	{
		"marshal-receiver",
		"keep marshal receiver types consistent",
		"standard method families",
	},
	{
		"max-control-nesting",
		"limit nested control structures",
		"maximum depth 5",
	},
	{
		"max-public-structs",
		"limit exported structs per file",
		"maximum 5",
	},
	{
		"modifies-parameter",
		"detect parameter mutation",
		"enabled",
	},
	{
		"modifies-value-receiver",
		"detect value receiver mutation",
		"enabled",
	},
	{
		"modulo-one",
		"detect remainder operations that are always zero",
		"enabled",
	},
	{
		"nested-structs",
		"discourage anonymous nested struct types",
		"enabled",
	},
	{
		"optimize-operands-order",
		"put cheap logical operands first",
		"enabled",
	},
	{
		"package-comments",
		"require package documentation",
		"enabled",
	},
	{
		"package-directory-mismatch",
		"match package and directory names",
		"testdata ignored",
	},
	{
		"package-naming",
		"enforce conventional package names",
		"lower-case letters and digits",
	},
	{
		"range-value-address",
		"avoid taking addresses of range values",
		"enabled",
	},
	{
		"simplify-range",
		"simplify range statements",
		"enabled",
	},
	{
		"receiver-naming",
		"enforce consistent receiver names",
		"no maximum length",
	},
	{
		"redefines-builtin-id",
		"avoid redefining predeclared identifiers",
		"enabled",
	},
	{
		"redundant-build-tag",
		"remove redundant legacy build tags",
		"enabled",
	},
	{
		"redundant-import-alias",
		"remove aliases equal to package names",
		"enabled",
	},
	{
		"redundant-final-return",
		"remove final returns from resultless functions",
		"enabled",
	},
	{
		"redundant-switch-break",
		"remove redundant breaks from switch cases",
		"enabled",
	},
	{
		"single-case-switch",
		"replace single-case switches with if statements",
		"enabled",
	},
	{
		"string-of-int",
		"make integer-to-string intent explicit",
		"enabled",
	},
	{
		"spaced-compiler-directive",
		"detect compiler directives disabled by leading whitespace",
		"enabled",
	},
	{
		"spinning-select-default",
		"detect select loops that spin on an empty default",
		"enabled",
	},
	{
		"invalid-struct-tag",
		"validate struct tag syntax and options",
		"standard tags",
	},
	{
		"time-date",
		"detect suspicious time.Date arguments",
		"enabled",
	},
	{
		"time-naming",
		"avoid unit suffixes on time.Duration values",
		"enabled",
	},
	{
		"unchecked-type-assertion",
		"require checked type assertions",
		"enabled",
	},
	{
		"unexported-naming",
		"avoid leading underscores in private names",
		"enabled",
	},
	{
		"unexported-return",
		"avoid exported APIs returning private types",
		"enabled",
	},
	{
		"unnecessary-if",
		"replace boolean-returning if chains",
		"enabled",
	},
	{
		"unnecessary-format",
		"avoid formatting calls without directives",
		"enabled",
	},
	{
		"unreachable-code",
		"detect statements after unconditional flow",
		"enabled",
	},
	{
		"insecure-url-scheme",
		"detect insecure URL schemes",
		"HTTP, WS, and FTP",
	},
	{
		"unused-parameter",
		"detect unused function parameters",
		"enabled",
	},
	{
		"unused-receiver",
		"detect unused method receivers",
		"enabled",
	},
	{
		"use-any",
		"prefer any to interface{}",
		"enabled",
	},
	{
		"use-errors-new",
		"prefer errors.New for static errors",
		"enabled",
	},
	{
		"use-fmt-print",
		"prefer fmt.Print over builtin print",
		"enabled",
	},
	{
		"use-slices-sort",
		"prefer slices.Sort over sort.Slice",
		"enabled",
	},
	{
		"var-declaration",
		"simplify zero-value declarations",
		"enabled",
	},
	{
		"var-naming",
		"enforce idiomatic identifier naming",
		"common initialisms",
	},
	{
		"waitgroup-by-value",
		"pass sync.WaitGroup by pointer",
		"enabled",
	},
	{
		"zero-integer-division",
		"detect literal integer division that truncates to zero",
		"enabled",
	},
}

// Most source-level policy checks are advisory. Keep the smaller sets whose
// findings indicate a likely defect, security problem, or material runtime
// cost explicit so every catalog entry receives a deliberate default.
var extendedWarningSeverities = map[string]bool{
	"blank-imports":                 true,
	"boolean-literal-comparison":    true,
	"call-to-gc":                    true,
	"cognitive-complexity":          true,
	"confusing-results":             true,
	"constant-logical-expr":         true,
	"context-as-argument":           true,
	"dot-imports":                   true,
	"early-return":                  true,
	"empty-conditional-block":       true,
	"enforce-switch-style":          true,
	"error-last-result":             true,
	"error-naming":                  true,
	"error-strings":                 true,
	"function-length":               true,
	"function-result-limit":         true,
	"identical-branches":            true,
	"identical-if-chain-branches":   true,
	"identical-if-chain-conditions": true,
	"identical-switch-branches":     true,
	"identical-switch-conditions":   true,
	"ineffective-pointer-copy":      true,
	"import-alias-naming":           true,
	"import-shadowing":              true,
	"imports-blocklist":             true,
	"inefficient-map-lookup":        true,
	"marshal-receiver":              true,
	"max-control-nesting":           true,
	"max-parameters":                true,
	"modifies-value-receiver":       true,
	"no-else-after-return":          true,
	"no-naked-return":               true,
	"no-package-var":                true,
	"optimize-operands-order":       true,
	"package-naming":                true,
	"prefer-fmt-errorf":             true,
	"range-value-address":           true,
	"receiver-naming":               true,
	"redefines-builtin-id":          true,
	"redundant-build-tag":           true,
	"redundant-error-return-check":  true,
	"redundant-final-return":        true,
	"redundant-import-alias":        true,
	"redundant-switch-break":        true,
	"simplify-range":                true,
	"single-case-switch":            true,
	"spaced-compiler-directive":     true,
	"spinning-select-default":       true,
	"time-date":                     true,
	"time-naming":                   true,
	"unexported-naming":             true,
	"unexported-return":             true,
	"unnecessary-if":                true,
	"unreachable-code":              true,
	"unused-parameter":              true,
	"unused-receiver":               true,
	"use-any":                       true,
	"use-errors-new":                true,
	"use-fmt-print":                 true,
	"use-slices-sort":               true,
	"var-naming":                    true,
	"insecure-url-scheme":           true,
}

var extendedErrorSeverities = map[string]bool{
	"banned-characters":                  true,
	"bidirectional-control-character":    true,
	"confusing-naming":                   true,
	"deep-exit":                          true,
	"deferred-recover-call":              true,
	"discarded-deferred-result":          true,
	"double-negation":                    true,
	"duplicated-imports":                 true,
	"empty-conditional-block":            true,
	"ineffective-pointer-copy":           true,
	"inefficient-map-lookup":             true,
	"invalid-struct-tag":                 true,
	"modifies-parameter":                 true,
	"modulo-one":                         true,
	"package-directory-mismatch":         true,
	"redundant-atomic-result-assignment": true,
	"string-of-int":                      true,
	"unchecked-type-assertion":           true,
	"waitgroup-by-value":                 true,
	"zero-integer-division":              true,
}

type spec struct {
	Code     string
	Summary  string
	Defaults string
}

func extendedDefaultSeverity(code string) diagnostic.Severity {
	if extendedErrorSeverities[code] {
		return diagnostic.SeverityError
	}
	if extendedWarningSeverities[code] {
		return diagnostic.SeverityWarning
	}
	return diagnostic.SeverityNote
}

// Select returns every rule, or only the explicitly requested rules.
func Select(only []string) ([]Rule, error) {
	all := allRules()
	wanted := make(map[string]bool, len(only))
	for _, code := range only {
		wanted[code] = true
	}
	for _, rule := range all {
		delete(wanted, rule.Meta().Code)
	}
	if len(wanted) != 0 {
		unknown := make([]string, 0, len(wanted))
		for code := range wanted {
			unknown = append(unknown, code)
		}
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown lint rule(s): %s", strings.Join(unknown, ", "))
	}

	selected := make([]Rule, 0, len(all))
	for _, rule := range all {
		meta := rule.Meta()
		if len(only) != 0 && !contains(only, meta.Code) {
			continue
		}
		selected = append(selected, rule)
	}
	return selected, nil
}

func allRules() []Rule {
	rules := make([]Rule, 0, len(coreCatalog)+len(extendedCatalog))
	for _, rule := range coreCatalog {
		rules = append(rules, rule)
	}
	for _, item := range extendedCatalog {
		example, ok := extendedExamples[item.Code]
		if !ok {
			panic("lint rule has no examples: " + item.Code)
		}
		explanation := fmt.Sprintf("%s. Default: %s.", item.Summary, item.Defaults)
		rules = append(
			rules,
			definition{
				meta: Meta{
					Code:            item.Code,
					Summary:         item.Summary,
					Explanation:     explanation,
					GoodExample:     example.Good,
					BadExample:      example.Bad,
					DefaultSeverity: extendedDefaultSeverity(item.Code),
				},
			},
		)
	}
	return rules
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
