package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gempir/strider/internal/diagnostic"
)

type spec struct {
	Code     string
	Summary  string
	Defaults string
}

var defaultCatalog = []definition{
	{
		meta: Meta{
			Code:            "cyclomatic-complexity",
			Summary:         "limit branching complexity",
			DefaultSeverity: diagnostic.SeverityWarning,
			Explanation:     "Functions with too many independent control-flow paths are difficult to understand and test. The spike limit is 10.",
			GoodExample:     "func sign(n int) int { if n < 0 { return -1 }; return 1 }",
			BadExample:      "func tangled() { /* more than ten branches */ }",
		},
		defaultRule: true,
	},
	{
		meta: Meta{
			Code:            "max-parameters",
			Summary:         "limit function parameter count",
			DefaultSeverity: diagnostic.SeverityWarning,
			Explanation:     "Functions with more than five parameters tend to hide missing domain objects and are difficult to call correctly.",
			GoodExample:     "func Open(path string, flags Flags) error",
			BadExample:      "func Open(path string, read, write, create, truncate, appendMode bool) error",
		},
		defaultRule: true,
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
		defaultRule: true,
	},
	{
		meta: Meta{
			Code:            "no-init",
			Summary:         "avoid implicit package initialization",
			DefaultSeverity: diagnostic.SeverityWarning,
			Explanation:     "init functions hide ordering and side effects. Prefer explicit construction called from main or tests.",
			GoodExample:     "func Configure() error { return register() }",
			BadExample:      "func init() { register() }",
		},
		defaultRule: true,
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
		defaultRule: true,
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
		defaultRule: true,
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
		defaultRule: true,
	},
}

// The non-default rules share the same native analysis pass as the default
// profile. This compact list supplies metadata that does not need custom
// examples.
var extendedCatalog = []spec{
	{"add-constant", "suggest named constants for repeated literals", "strings after 2 repetitions"},
	{"argument-limit", "limit function parameter count", "maximum 8"},
	{"atomic", "detect non-atomic operations on atomic values", "standard sync/atomic patterns"},
	{"banned-characters", "reject configured characters in identifiers", "no banned characters"},
	{"bare-return", "warn about bare returns with named results", "enabled"},
	{"blank-imports", "require blank imports to be justified", "main and test packages exempt"},
	{"bool-literal-in-expr", "remove boolean literals from logical comparisons", "enabled"},
	{"call-to-gc", "discourage explicit garbage collection", "runtime.GC"},
	{"cognitive-complexity", "limit nested control-flow complexity", "maximum 7"},
	{"comment-spacings", "require a space after line-comment markers", "directives exempt"},
	{"comments-density", "require a minimum comment density", "minimum 0 percent"},
	{"confusing-naming", "detect names differing only by capitalization", "methods and fields"},
	{"confusing-results", "name adjacent results of the same type", "enabled"},
	{"constant-logical-expr", "detect constant logical expressions", "enabled"},
	{"context-as-argument", "place context.Context first in parameter lists", "enabled"},
	{"context-keys-type", "avoid basic types as context keys", "enabled"},
	{"cyclomatic", "limit independent control-flow paths", "maximum 10"},
	{"datarace", "detect goroutines capturing changing variables", "enabled"},
	{"deep-exit", "keep process exits in main or init", "enabled"},
	{"defer", "detect common defer mistakes", "all checks enabled"},
	{"dot-imports", "discourage dot imports", "no allowed packages"},
	{"duplicated-imports", "detect packages imported more than once", "enabled"},
	{"early-return", "prefer early returns that reduce nesting", "enabled"},
	{"empty-block", "detect empty statement blocks", "enabled"},
	{"empty-lines", "detect edge blank lines in blocks", "enabled"},
	{"enforce-map-style", "enforce consistent empty-map construction", "any style"},
	{"enforce-repeated-arg-type-style", "enforce repeated argument type style", "any style"},
	{"enforce-slice-style", "enforce consistent empty-slice construction", "any style"},
	{"enforce-switch-style", "require default clauses to be last", "default optional"},
	{"epoch-naming", "include epoch units in variable names", "enabled"},
	{"error-naming", "name package errors with an Err prefix", "enabled"},
	{"error-return", "place error last in result lists", "enabled"},
	{"error-strings", "use lower-case unpunctuated error messages", "enabled"},
	{"errorf", "replace errors.New around fmt.Sprintf", "enabled"},
	{"exported", "document exported declarations", "enabled"},
	{"file-header", "require a configured source header", "no required header"},
	{"file-length-limit", "limit source-file length", "disabled at 0 lines"},
	{"filename-format", "enforce Go source filename format", "conventional characters"},
	{"flag-parameter", "detect boolean control parameters", "enabled"},
	{"forbidden-call-in-wg-go", "reject panic and Done inside WaitGroup.Go", "enabled"},
	{"function-length", "limit function statements and lines", "50 statements; 75 lines"},
	{"function-result-limit", "limit function result count", "maximum 3"},
	{"get-return", "require Get-prefixed functions to return values", "enabled"},
	{"identical-branches", "detect identical if branches", "enabled"},
	{"identical-ifelseif-branches", "detect repeated if-chain branches", "enabled"},
	{"identical-ifelseif-conditions", "detect repeated if-chain conditions", "enabled"},
	{"identical-switch-branches", "detect repeated switch branches", "enabled"},
	{"identical-switch-conditions", "detect repeated switch conditions", "enabled"},
	{"if-return", "simplify redundant error checks before returning", "enabled"},
	{"import-alias-naming", "enforce conventional import aliases", "lower-case letters and digits"},
	{"import-shadowing", "detect declarations shadowing imports", "enabled"},
	{"imports-blocklist", "reject configured imports", "empty blocklist"},
	{"increment-decrement", "prefer increment and decrement statements", "enabled"},
	{"indent-error-flow", "remove else after returning if branches", "enabled"},
	{"inefficient-map-lookup", "avoid repeated map lookups", "enabled"},
	{"line-length-limit", "limit source line length", "maximum 80 runes"},
	{"marshal-receiver", "keep marshal receiver types consistent", "standard method families"},
	{"max-control-nesting", "limit nested control structures", "maximum depth 5"},
	{"max-public-structs", "limit exported structs per file", "maximum 5"},
	{"modifies-parameter", "detect parameter mutation", "enabled"},
	{"modifies-value-receiver", "detect value receiver mutation", "enabled"},
	{"multiline-if-init", "move multiline if initializers above conditions", "enabled"},
	{"nested-structs", "discourage anonymous nested struct types", "enabled"},
	{"optimize-operands-order", "put cheap logical operands first", "enabled"},
	{"package-comments", "require package documentation", "enabled"},
	{"package-directory-mismatch", "match package and directory names", "testdata ignored"},
	{"package-naming", "enforce conventional package names", "lower-case letters and digits"},
	{"range-val-address", "avoid taking addresses of range values", "enabled"},
	{"range-val-in-closure", "avoid capturing range values in closures", "enabled"},
	{"range", "simplify range statements", "enabled"},
	{"receiver-naming", "enforce consistent receiver names", "no maximum length"},
	{"redefines-builtin-id", "avoid redefining predeclared identifiers", "enabled"},
	{"redundant-build-tag", "remove redundant legacy build tags", "enabled"},
	{"redundant-import-alias", "remove aliases equal to package names", "enabled"},
	{"redundant-test-main-exit", "remove redundant os.Exit in TestMain", "enabled"},
	{"string-format", "enforce configured string constraints", "no constraints"},
	{"string-of-int", "make integer-to-string intent explicit", "enabled"},
	{"struct-tag", "validate struct tag syntax and options", "standard tags"},
	{"superfluous-else", "remove else after terminating branches", "enabled"},
	{"time-date", "detect suspicious time.Date arguments", "enabled"},
	{"time-equal", "prefer time.Time.Equal over equality", "enabled"},
	{"time-naming", "avoid unit suffixes on time.Duration values", "enabled"},
	{"unchecked-type-assertion", "require checked type assertions", "enabled"},
	{"unconditional-recursion", "detect recursion on every path", "enabled"},
	{"unexported-naming", "avoid leading underscores in private names", "enabled"},
	{"unexported-return", "avoid exported APIs returning private types", "enabled"},
	{"unhandled-error", "detect ignored error-returning calls", "common functions"},
	{"unnecessary-if", "replace boolean-returning if chains", "enabled"},
	{"unnecessary-format", "avoid formatting calls without directives", "enabled"},
	{"unnecessary-stmt", "detect redundant control flow", "enabled"},
	{"unreachable-code", "detect statements after unconditional flow", "enabled"},
	{"unsecure-url-scheme", "detect insecure URL schemes", "HTTP, WS, and FTP"},
	{"unused-parameter", "detect unused function parameters", "enabled"},
	{"unused-receiver", "detect unused method receivers", "enabled"},
	{"use-any", "prefer any to interface{}", "enabled"},
	{"use-errors-new", "prefer errors.New for static errors", "enabled"},
	{"use-fmt-print", "prefer fmt.Print over builtin print", "enabled"},
	{"use-slices-sort", "prefer slices.Sort over sort.Slice", "enabled"},
	{"use-waitgroup-go", "prefer WaitGroup.Go", "Go 1.25 and newer"},
	{"useless-break", "remove redundant break statements", "enabled"},
	{"useless-fallthrough", "remove final-case fallthrough", "enabled"},
	{"var-declaration", "simplify zero-value declarations", "enabled"},
	{"var-naming", "enforce idiomatic identifier naming", "common initialisms"},
	{"waitgroup-by-value", "pass sync.WaitGroup by pointer", "enabled"},
}

// Select returns the rules requested by the CLI. With no explicit selection it
// returns the default profile; enableAll selects the complete catalog.
func Select(only []string, enableAll bool) ([]Rule, error) {
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
		if len(only) == 0 && !enableAll && !rule.defaultEnabled() {
			continue
		}
		selected = append(selected, rule)
	}
	return selected, nil
}

func allRules() []Rule {
	rules := make([]Rule, 0, len(defaultCatalog)+len(extendedCatalog))
	for _, rule := range defaultCatalog {
		rules = append(rules, rule)
	}
	for _, item := range extendedCatalog {
		explanation := item.Summary + "."
		if item.Defaults != "" {
			explanation += " Strider default: " + item.Defaults + "."
		}
		rules = append(
			rules,
			definition{
				meta: Meta{
					Code:            item.Code,
					Summary:         item.Summary,
					Explanation:     explanation,
					GoodExample:     "See the rule reference for accepted forms.",
					BadExample:      "See the rule reference for reported forms.",
					DefaultSeverity: diagnostic.SeverityWarning,
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
