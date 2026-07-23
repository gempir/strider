package syntax

import (
	"fmt"

	"github.com/gempir/strider/internal/checkconfig"
	"github.com/gempir/strider/internal/checks/catalog"
	"github.com/gempir/strider/internal/diagnostic"
)

// catalog is the single declaration point for syntax-check metadata. Behavior,
// interests, and options are attached to these same definitions as the
// execution migration proceeds; examples and severities are intentionally not
// mirrored in auxiliary maps.
var definitions = []definition{
	{
		behavior: functionCheck((*Pass).checkCyclomaticComplexity),
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
		behavior: functionCheck((*Pass).checkMaxParameters),
		meta: Meta{
			Code:            "max-parameters",
			Summary:         "limit function parameter count",
			DefaultSeverity: diagnostic.SeverityWarning,
			Options: []catalog.Option{
				catalog.NonNegativeIntOption("max-parameters", 8, "Maximum number of parameters allowed on a function or method."),
			},
			Explanation: "Functions with more than eight parameters tend to hide missing domain objects and are difficult to call correctly.",
			GoodExample: "func Open(path string, flags Flags) error",
			BadExample:  "func Open(path string, read, write, create, truncate, appendMode, sync, exclusive, temporary bool) error",
		},
	},
	{
		behavior: nakedReturnBehavior,
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
		behavior: noInitBehavior,
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
		behavior: packageVarBehavior,
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
		behavior: deferCheck((*Pass).checkDeferInLoop),
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
		behavior: noElseAfterReturnBehavior,
		meta: Meta{
			Code:            "no-else-after-return",
			Summary:         "remove else after terminal return",
			DefaultSeverity: diagnostic.SeverityWarning,
			Explanation:     "When the if branch returns, the else branch can be unindented. This reduces nesting without changing control flow.",
			GoodExample:     "if err != nil { return err }\nuse(value)",
			BadExample:      "if err != nil { return err } else { use(value) }",
		},
	},
	{
		behavior: repeatedLiteralBehavior,
		meta: Meta{
			Code:            "add-constant",
			Summary:         "suggest named constants for repeated literals",
			Explanation:     "suggest named constants for repeated literals. Default: strings after 2 repetitions.",
			GoodExample:     "const stateReady = \"ready\"\nif state == stateReady { start() }",
			BadExample:      "if state == \"ready\" { start() }\nif next == \"ready\" { queue() }\nif fallback == \"ready\" { recover() }",
			DefaultSeverity: diagnostic.SeverityNote,
		},
	},
	{
		behavior: assignmentBehavior,
		meta: Meta{
			Code:            "redundant-atomic-result-assignment",
			Summary:         "avoid assigning an atomic result back to its operand",
			Explanation:     "avoid assigning an atomic result back to its operand. Default: standard sync/atomic patterns.",
			GoodExample:     "atomic.AddInt64(&counter, 1)",
			BadExample:      "counter = atomic.AddInt64(&counter, 1)",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: identifierCheck((*Pass).checkBannedCharacters),
		meta: Meta{
			Code:            "banned-characters",
			Summary:         "reject configured characters in identifiers",
			Explanation:     "reject configured characters in identifiers. Default: ᐸ and ᐳ.",
			GoodExample:     "var userID string",
			BadExample:      "var user_id string // when underscore is configured as banned",
			DefaultSeverity: diagnostic.SeverityError,
			Options: []catalog.Option{
				func() catalog.Option {
					option := catalog.StringListOption("characters", []string{
						"ᐸ",
						"ᐳ",
					}, "Unicode characters forbidden in identifiers.")
					option.Validate = validateSingleCharacters
					return option
				}(),
			},
		},
	},
	{
		behavior: startBehavior((*Pass).checkBidirectionalControlCharacters),
		meta: Meta{
			Code:            "bidirectional-control-character",
			Summary:         "reject invisible bidirectional source controls",
			Explanation:     "reject invisible bidirectional source controls. Default: enabled.",
			GoodExample:     "// access denied",
			BadExample:      "// access \u202edenied",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: importCheck((*Pass).checkBlankImports),
		meta: Meta{
			Code:            "blank-imports",
			Summary:         "require explanatory comments on blank imports outside main and test files",
			Explanation:     "require explanatory comments on blank imports outside main and test files. Default: main and test packages exempt.",
			GoodExample:     "import _ \"example.com/driver\" // register the driver",
			BadExample:      "import _ \"example.com/driver\"",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: documentationPeriodBehavior,
		meta: Meta{
			Code:            "doc-comment-period",
			Summary:         "require declaration documentation to end with punctuation",
			Explanation:     "Complete documentation sentences are easier to read in generated API references. Default: package and exported-declaration documentation must end in ., !, ?, or :.",
			GoodExample:     "// Client sends requests.",
			BadExample:      "// Client sends requests",
			DefaultSeverity: diagnostic.SeverityNote,
		},
	},
	{
		behavior: excessiveBlankIdentifiersBehavior,
		meta: Meta{
			Code:            "excessive-blank-identifiers",
			Summary:         "detect assignments that discard too many results",
			Explanation:     "Discarding several adjacent results hides the contract of the called function and makes it easy to overlook an important value. Default: report assignments with at least three blank identifiers.",
			GoodExample:     "value, metadata, err := load(); _ = metadata",
			BadExample:      "value, _, _, _, err := load()",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: startBehavior((*Pass).checkTaskComments),
		meta: Meta{
			Code:            "task-comment",
			Summary:         "surface TODO, FIXME, and BUG comments",
			Explanation:     "Task markers in source are easy to forget and invisible to normal issue tracking. Default: report TODO, FIXME, and BUG markers.",
			GoodExample:     "// Retry only errors classified as transient.",
			BadExample:      "// TODO: decide which errors should be retried.",
			DefaultSeverity: diagnostic.SeverityNote,
		},
	},
	{
		behavior: topLevelDeclarationOrderBehavior,
		meta: Meta{
			Code:            "top-level-declaration-order",
			Summary:         "keep top-level declarations in const, var, type, and func order",
			Explanation:     "A consistent top-level declaration order makes files easier to scan. Default: constants, variables, types, then functions; imports are ignored.",
			GoodExample:     "const timeout = 1; var defaultClient Client; type Client struct{}; func New() Client { return Client{} }",
			BadExample:      "var defaultClient Client; type Client struct{}",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: binaryCheck((*Pass).checkBooleanLiteralComparison),
		meta: Meta{
			Code:            "boolean-literal-comparison",
			Summary:         "simplify comparisons between booleans and literals",
			Explanation:     "simplify comparisons between booleans and literals. Default: enabled.",
			GoodExample:     "if ready { start() }",
			BadExample:      "if ready == true { start() }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: callCheck((*Pass).checkCallToGC),
		meta: Meta{
			Code:            "call-to-gc",
			Summary:         "discourage explicit garbage collection",
			Explanation:     "discourage explicit garbage collection. Default: runtime.GC.",
			GoodExample:     "buffer = nil // let Go collect it when appropriate",
			BadExample:      "runtime.GC() // in normal application code",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: functionCheck((*Pass).checkCognitiveComplexity),
		meta: Meta{
			Code:            "cognitive-complexity",
			Summary:         "limit nested control-flow complexity",
			Explanation:     "limit nested control-flow complexity. Default: maximum 7.",
			GoodExample:     "if !ready { return }\nprocess()",
			BadExample:      "func process(items []int) { for _, item := range items { if item > 0 { for retry := 0; retry < 3; retry++ { if ready(item) { use(item) } } } } }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: behavior([]NodeKind{
			nodeFunctionDecl,
			nodeMethodDecl,
			nodeFieldDecl,
		}, inspectConfusingNamingCheck),
		meta: Meta{
			Code:            "confusing-naming",
			Summary:         "reject declarations whose names differ only by capitalization",
			Explanation:     "reject declarations whose names differ only by capitalization. Default: methods and fields.",
			GoodExample:     "type User struct { ID string; Name string }",
			BadExample:      "type User struct { ID string; Id string }",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: functionCheck((*Pass).checkConfusingResults),
		meta: Meta{
			Code:            "confusing-results",
			Summary:         "name consecutive unnamed results that have the same type",
			Explanation:     "name consecutive unnamed results that have the same type. Default: enabled.",
			GoodExample:     "func bounds() (min int, max int)",
			BadExample:      "func bounds() (int, int)",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: binaryCheck((*Pass).checkConstantLogicalExpression),
		meta: Meta{
			Code:            "constant-logical-expr",
			Summary:         "detect constant logical expressions",
			Explanation:     "detect constant logical expressions. Default: enabled.",
			GoodExample:     "if ready && connected { start() }",
			BadExample:      "if false && connected { start() }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: functionCheck((*Pass).checkContextAsArgument),
		meta: Meta{
			Code:            "context-as-argument",
			Summary:         "require context.Context to be the first parameter",
			Explanation:     "require context.Context to be the first parameter. Default: enabled.",
			GoodExample:     "func Load(ctx context.Context, id string) error",
			BadExample:      "func Load(id string, ctx context.Context) error",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: callCheck((*Pass).checkDeepExit),
		meta: Meta{
			Code:            "deep-exit",
			Summary:         "keep process-terminating calls in main or init",
			Explanation:     "keep process-terminating calls in main or init. Default: enabled.",
			GoodExample:     "func run() error { return load() }",
			BadExample:      "func loadConfig() { if failed() { os.Exit(1) } }",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: deferCheck((*Pass).checkDeferredRecoverCall),
		meta: Meta{
			Code:            "deferred-recover-call",
			Summary:         "defer recover through a function closure",
			Explanation:     "defer recover through a function closure. Default: enabled.",
			GoodExample:     "defer func() { _ = recover() }()",
			BadExample:      "defer recover()",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: deferCheck((*Pass).checkDiscardedDeferredResult),
		meta: Meta{
			Code:            "discarded-deferred-result",
			Summary:         "avoid return values discarded by deferred calls",
			Explanation:     "avoid return values discarded by deferred calls. Default: enabled.",
			GoodExample:     "defer func() { cleanup() }()",
			BadExample:      "defer func() error { return cleanup() }()",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: importCheck((*Pass).checkDotImports),
		meta: Meta{
			Code:            "dot-imports",
			Summary:         "discourage dot imports",
			Explanation:     "discourage dot imports. Default: no allowed packages.",
			GoodExample:     "import \"fmt\"\nfmt.Println(message)",
			BadExample:      "import . \"fmt\"\nPrintln(message)",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: unaryCheck((*Pass).checkDoubleNegation),
		meta: Meta{
			Code:            "double-negation",
			Summary:         "remove redundant double boolean negation",
			Explanation:     "remove redundant double boolean negation. Default: enabled.",
			GoodExample:     "return ready",
			BadExample:      "return !!ready",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: importCheck((*Pass).checkDuplicatedImports),
		meta: Meta{
			Code:            "duplicated-imports",
			Summary:         "reject importing the same package more than once",
			Explanation:     "reject importing the same package more than once. Default: enabled.",
			GoodExample:     "import \"strings\"",
			BadExample:      "import (\n\t\"strings\"\n\ttext \"strings\"\n)",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: conditionalCheck((*Pass).checkEarlyReturn),
		meta: Meta{
			Code:            "early-return",
			Summary:         "prefer early returns that reduce nesting",
			Explanation:     "prefer early returns that reduce nesting. Default: enabled.",
			GoodExample:     "if !ready { return }\nprocess()",
			BadExample:      "if ready { process() } else { return }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: blockCheck((*Pass).checkEmptyConditionalBlock),
		meta: Meta{
			Code:            "empty-conditional-block",
			Summary:         "detect empty conditional blocks",
			Explanation:     "detect empty conditional blocks. Default: enabled.",
			GoodExample:     "if ready { process() }",
			BadExample:      "if ready {}",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: switchCheck((*Pass).checkSwitchDefaultLast),
		meta: Meta{
			Code:            "enforce-switch-style",
			Summary:         "require default clauses to be last",
			Explanation:     "require default clauses to be last. Default: default optional.",
			GoodExample:     "switch value { case 1: one(); default: fallback() }",
			BadExample:      "switch value { default: fallback(); case 1: one() }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: varSpecCheck((*Pass).checkErrorNaming),
		meta: Meta{
			Code:            "error-naming",
			Summary:         "name package errors with an Err prefix",
			Explanation:     "name package errors with an Err prefix. Default: enabled.",
			GoodExample:     "var ErrNotFound = errors.New(\"not found\")",
			BadExample:      "var NotFoundError = errors.New(\"not found\")",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: functionCheck((*Pass).checkErrorLastResult),
		meta: Meta{
			Code:            "error-last-result",
			Summary:         "place error last in result lists",
			Explanation:     "place error last in result lists. Default: enabled.",
			GoodExample:     "func Load() (string, error)",
			BadExample:      "func Load() (error, string)",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: callCheck((*Pass).checkErrorStringCall),
		meta: Meta{
			Code:            "error-strings",
			Summary:         "use lower-case unpunctuated error messages",
			Explanation:     "use lower-case unpunctuated error messages. Default: enabled.",
			GoodExample:     "errors.New(\"connection refused\")",
			BadExample:      "errors.New(\"Connection refused.\")",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: callCheck((*Pass).checkPreferFmtErrorf),
		meta: Meta{
			Code:            "prefer-fmt-errorf",
			Summary:         "replace errors.New around fmt.Sprintf",
			Explanation:     "replace errors.New around fmt.Sprintf. Default: enabled.",
			GoodExample:     "fmt.Errorf(\"load %s\", name)",
			BadExample:      "errors.New(fmt.Sprintf(\"load %s\", name))",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: exportedDeclarationBehavior,
		meta: Meta{
			Code:            "exported-declaration-comment",
			Summary:         "document exported declarations",
			Explanation:     "document exported declarations. Default: enabled.",
			GoodExample:     "// Client sends requests.\ntype Client struct{}",
			BadExample:      "type Client struct{}",
			DefaultSeverity: diagnostic.SeverityNote,
		},
	},
	{
		behavior: startBehavior((*Pass).checkFileLength),
		meta: Meta{
			Code:            "file-length-limit",
			Summary:         "limit source-file length",
			Explanation:     "limit source-file length. Default: maximum 500 lines; set 0 to disable.",
			GoodExample:     "// With max = 3 lines:\npackage service\nvar ready bool",
			BadExample:      "// With max = 2 lines:\npackage service\nvar ready bool",
			DefaultSeverity: diagnostic.SeverityNote,
			Options: []catalog.Option{
				catalog.NonNegativeIntOption("max-lines", 500, "Maximum number of lines allowed in a source file; zero disables the limit."),
			},
		},
	},
	{
		behavior: startBehavior((*Pass).checkFilenameFormat),
		meta: Meta{
			Code:            "filename-format",
			Summary:         "enforce Go source filename format",
			Explanation:     "enforce Go source filename format. Default: conventional characters.",
			GoodExample:     "// user_service.go",
			BadExample:      "// user.service.go",
			DefaultSeverity: diagnostic.SeverityNote,
		},
	},
	{
		behavior: functionCheck((*Pass).checkFlagParameter),
		meta: Meta{
			Code:            "flag-parameter",
			Summary:         "detect boolean control parameters",
			Explanation:     "detect boolean control parameters. Default: enabled.",
			GoodExample:     "func Open(path string, mode Mode) error",
			BadExample:      "func Open(path string, readOnly bool) error { if readOnly { return openReadOnly(path) }; return openReadWrite(path) }",
			DefaultSeverity: diagnostic.SeverityNote,
		},
	},
	{
		behavior: functionCheck((*Pass).checkFunctionLength),
		meta: Meta{
			Code:            "function-length",
			Summary:         "limit function statements and lines",
			Explanation:     "limit function statements and lines. Default: 50 statements; 75 lines.",
			GoodExample:     "func run() { load(); process(); save() }",
			BadExample:      "// With max-statements = 3:\nfunc run() { load(); process(); save(); notify() }",
			DefaultSeverity: diagnostic.SeverityWarning,
			Options: []catalog.Option{
				catalog.NonNegativeIntOption("max-lines", 75, "Maximum number of lines allowed in a function."),
				catalog.NonNegativeIntOption("max-statements", 50, "Maximum number of statements allowed in a function."),
			},
		},
	},
	{
		behavior: functionCheck((*Pass).checkFunctionResultLimit),
		meta: Meta{
			Code:            "function-result-limit",
			Summary:         "limit function result count",
			Explanation:     "limit function result count. Default: maximum 3.",
			GoodExample:     "func Parse() (Value, error)",
			BadExample:      "func Parse() (Value, Metadata, Warnings, error)",
			DefaultSeverity: diagnostic.SeverityWarning,
			Options: []catalog.Option{
				catalog.NonNegativeIntOption("max-results", 3, "Maximum number of result values allowed on a function."),
			},
		},
	},
	{
		behavior: functionCheck((*Pass).checkGetFunctionReturnValue),
		meta: Meta{
			Code:            "get-function-return-value",
			Summary:         "require Get-prefixed functions to return values",
			Explanation:     "require Get-prefixed functions to return values. Default: enabled.",
			GoodExample:     "func GetClient() *Client { return client }",
			BadExample:      "func GetClient() { initializeClient() }",
			DefaultSeverity: diagnostic.SeverityNote,
		},
	},
	{
		behavior: conditionalCheck((*Pass).checkIdenticalBranches),
		meta: Meta{
			Code:            "identical-branches",
			Summary:         "detect identical if branches",
			Explanation:     "detect identical if branches. Default: enabled.",
			GoodExample:     "if ready { start() } else { wait() }",
			BadExample:      "if ready { start() } else { start() }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: conditionalCheck((*Pass).checkIdenticalIfChainBranches),
		meta: Meta{
			Code:            "identical-if-chain-branches",
			Summary:         "detect repeated if-chain branches",
			Explanation:     "detect repeated if-chain branches. Default: enabled.",
			GoodExample:     "if first { one() } else if second { two() }",
			BadExample:      "if first { run() } else if second { run() }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: conditionalCheck((*Pass).checkIdenticalIfChainConditions),
		meta: Meta{
			Code:            "identical-if-chain-conditions",
			Summary:         "detect repeated if-chain conditions",
			Explanation:     "detect repeated if-chain conditions. Default: enabled.",
			GoodExample:     "if first { one() } else if second { two() }",
			BadExample:      "if ready { one() } else if ready { two() }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: switchCheck((*Pass).checkIdenticalSwitchBranches),
		meta: Meta{
			Code:            "identical-switch-branches",
			Summary:         "detect repeated switch branches",
			Explanation:     "detect repeated switch branches. Default: enabled.",
			GoodExample:     "switch value { case 1: one(); case 2: two() }",
			BadExample:      "switch value { case 1: run(); case 2: run() }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: switchCheck((*Pass).checkIdenticalSwitchConditions),
		meta: Meta{
			Code:            "identical-switch-conditions",
			Summary:         "detect repeated switch conditions",
			Explanation:     "detect repeated switch conditions. Default: enabled.",
			GoodExample:     "switch { case first: one(); case second: two() }",
			BadExample:      "switch { case ready: one(); case ready: two() }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: blockCheck((*Pass).checkRedundantErrorReturn),
		meta: Meta{
			Code:            "redundant-error-return-check",
			Summary:         "simplify redundant error checks before returning",
			Explanation:     "simplify redundant error checks before returning. Default: enabled.",
			GoodExample:     "err := save()\nreturn err",
			BadExample:      "if err := save(); err != nil { return err }\nreturn nil",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: unaryCheck((*Pass).checkIneffectivePointerCopy),
		meta: Meta{
			Code:            "ineffective-pointer-copy",
			Summary:         "detect pointer round trips that do not copy values",
			Explanation:     "detect pointer round trips that do not copy values. Default: enabled.",
			GoodExample:     "copy := *pointer",
			BadExample:      "copy := &*pointer",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: importCheck((*Pass).checkImportAliasNaming),
		meta: Meta{
			Code:            "import-alias-naming",
			Summary:         "enforce conventional import aliases",
			Explanation:     "enforce conventional import aliases. Default: lower-case letters and digits.",
			GoodExample:     "import jsonapi \"example.com/json-api\"",
			BadExample:      "import JSON_API \"example.com/json-api\"",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: importShadowingBehavior,
		meta: Meta{
			Code:            "import-shadowing",
			Summary:         "detect declarations shadowing imports",
			Explanation:     "detect declarations shadowing imports. Default: enabled.",
			GoodExample:     "encoded := json.Marshal(value)",
			BadExample:      "json := loadConfig() // shadows the json import",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: importCheck((*Pass).checkImportsBlocklist),
		meta: Meta{
			Code:            "imports-blocklist",
			Summary:         "reject configured imports",
			Explanation:     "reject configured imports. Default: empty blocklist.",
			GoodExample:     "import \"log/slog\"",
			BadExample:      "import \"log\" // when log is configured as blocked",
			DefaultSeverity: diagnostic.SeverityWarning,
			Options: []catalog.Option{
				catalog.StringListOption("blocked-imports", nil, "Import paths that this check rejects."),
			},
		},
	},
	{
		behavior: incrementBehavior,
		meta: Meta{
			Code:            "increment-decrement",
			Summary:         "prefer increment and decrement statements",
			Explanation:     "prefer increment and decrement statements. Default: enabled.",
			GoodExample:     "count++",
			BadExample:      "count += 1",
			DefaultSeverity: diagnostic.SeverityNote,
		},
	},
	{
		behavior: conditionalCheck((*Pass).checkInefficientMapLookup),
		meta: Meta{
			Code:            "inefficient-map-lookup",
			Summary:         "avoid repeated map lookups",
			Explanation:     "avoid repeated map lookups. Default: enabled.",
			GoodExample:     "if value, ok := values[key]; ok { use(value) }",
			BadExample:      "if _, ok := values[key]; ok { use(values[key]) }",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: functionCheck((*Pass).checkMarshalReceiver),
		meta: Meta{
			Code:            "marshal-receiver",
			Summary:         "keep marshal receiver types consistent",
			Explanation:     "keep marshal receiver types consistent. Default: standard method families.",
			GoodExample:     "func (value *Value) MarshalJSON() ([]byte, error)\nfunc (value *Value) UnmarshalJSON([]byte) error",
			BadExample:      "func (value Value) MarshalJSON() ([]byte, error)\nfunc (value *Value) UnmarshalJSON([]byte) error",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: controlNestingBehavior,
		meta: Meta{
			Code:            "max-control-nesting",
			Summary:         "limit nested control structures",
			Explanation:     "limit nested control structures. Default: maximum depth 5.",
			GoodExample:     "if !ready { return }\nfor _, item := range items { process(item) }",
			BadExample:      "if ready { for { switch value { case 1: if valid { for retry() { if connected { process() } } } } } }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: typeDefinitionBehavior,
		meta: Meta{
			Code:            "max-public-structs",
			Summary:         "limit exported structs per file",
			Explanation:     "limit exported structs per file. Default: maximum 5.",
			GoodExample:     "type Request struct{}\ntype Response struct{}",
			BadExample:      "type One struct{}\ntype Two struct{}\ntype Three struct{}\ntype Four struct{}\ntype Five struct{}\ntype Six struct{}",
			DefaultSeverity: diagnostic.SeverityNote,
			Options: []catalog.Option{
				catalog.NonNegativeIntOption("max-public-structs", 5, "Maximum number of exported struct declarations allowed per file."),
			},
		},
	},
	{
		behavior: functionCheck((*Pass).checkModifiesParameter),
		meta: Meta{
			Code:            "modifies-parameter",
			Summary:         "detect parameter mutation",
			Explanation:     "detect parameter mutation. Default: enabled.",
			GoodExample:     "func normalize(value string) string { normalized := strings.TrimSpace(value); return normalized }",
			BadExample:      "func normalize(value string) string { value = strings.TrimSpace(value); return value }",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: functionCheck((*Pass).checkModifiesValueReceiver),
		meta: Meta{
			Code:            "modifies-value-receiver",
			Summary:         "detect value receiver mutation",
			Explanation:     "detect value receiver mutation. Default: enabled.",
			GoodExample:     "func (item *Item) Rename(name string) { item.Name = name }",
			BadExample:      "func (item Item) Rename(name string) { item.Name = name }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: binaryCheck((*Pass).checkModuloOne),
		meta: Meta{
			Code:            "modulo-one",
			Summary:         "detect remainder operations that are always zero",
			Explanation:     "detect remainder operations that are always zero. Default: enabled.",
			GoodExample:     "remainder := value % divisor",
			BadExample:      "remainder := value % 1",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: structBehavior,
		meta: Meta{
			Code:            "nested-structs",
			Summary:         "discourage anonymous nested struct types",
			Explanation:     "discourage anonymous nested struct types. Default: enabled.",
			GoodExample:     "type Address struct { City string }\ntype User struct { Address Address }",
			BadExample:      "type User struct { Address struct { City string } }",
			DefaultSeverity: diagnostic.SeverityNote,
		},
	},
	{
		behavior: startBehavior((*Pass).checkPackageComment),
		meta: Meta{
			Code:            "package-comments",
			Summary:         "require package documentation",
			Explanation:     "require package documentation. Default: enabled.",
			GoodExample:     "// Package store persists application data.\npackage store",
			BadExample:      "package store",
			DefaultSeverity: diagnostic.SeverityNote,
		},
	},
	{
		behavior: startBehavior((*Pass).checkPackageDirectoryMismatch),
		meta: Meta{
			Code:            "package-directory-mismatch",
			Summary:         "match package and directory names",
			Explanation:     "match package and directory names. Default: testdata ignored.",
			GoodExample:     "// store/client.go\npackage store",
			BadExample:      "// storage/client.go\npackage store",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: startBehavior((*Pass).checkPackageNaming),
		meta: Meta{
			Code:            "package-naming",
			Summary:         "enforce conventional package names",
			Explanation:     "enforce conventional package names. Default: lower-case letters and digits.",
			GoodExample:     "package httputil",
			BadExample:      "package http_util",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: forCheck((*Pass).checkRangeValueAddress),
		meta: Meta{
			Code:            "range-value-address",
			Summary:         "avoid taking addresses of range values",
			Explanation:     "avoid taking addresses of range values. Default: enabled.",
			GoodExample:     "for index := range values { pointers = append(pointers, &values[index]) }",
			BadExample:      "for _, value := range values { pointers = append(pointers, &value) }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: forCheck((*Pass).checkSimplifyRange),
		meta: Meta{
			Code:            "simplify-range",
			Summary:         "simplify range statements",
			Explanation:     "simplify range statements. Default: enabled.",
			GoodExample:     "for _, character := range text { use(character) }",
			BadExample:      "for _, character := range []rune(text) { use(character) }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: functionCheck((*Pass).checkReceiverNaming),
		meta: Meta{
			Code:            "receiver-naming",
			Summary:         "enforce consistent receiver names",
			Explanation:     "enforce consistent receiver names. Default: no maximum length.",
			GoodExample:     "func (c *Client) Send() error",
			BadExample:      "func (self *Client) Send() error",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: identifierCheck((*Pass).checkRedefinesBuiltin),
		meta: Meta{
			Code:            "redefines-builtin-id",
			Summary:         "avoid redefining predeclared identifiers",
			Explanation:     "avoid redefining predeclared identifiers. Default: enabled.",
			GoodExample:     "func parse(input string) error",
			BadExample:      "func parse(error string) error",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: startBehavior((*Pass).checkBuildTags),
		meta: Meta{
			Code:            "redundant-build-tag",
			Summary:         "remove redundant legacy build tags",
			Explanation:     "remove redundant legacy build tags. Default: enabled.",
			GoodExample:     "//go:build linux || darwin",
			BadExample:      "//go:build linux || darwin\n// +build linux darwin",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: importCheck((*Pass).checkRedundantImportAlias),
		meta: Meta{
			Code:            "redundant-import-alias",
			Summary:         "remove aliases equal to package names",
			Explanation:     "remove aliases equal to package names. Default: enabled.",
			GoodExample:     "import \"strings\"",
			BadExample:      "import strings \"strings\"",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: functionCheck((*Pass).checkRedundantFinalReturn),
		meta: Meta{
			Code:            "redundant-final-return",
			Summary:         "remove final returns from resultless functions",
			Explanation:     "remove final returns from resultless functions. Default: enabled.",
			GoodExample:     "func run() { work() }",
			BadExample:      "func run() { work(); return }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: breakBehavior,
		meta: Meta{
			Code:            "redundant-switch-break",
			Summary:         "remove redundant breaks from switch cases",
			Explanation:     "remove redundant breaks from switch cases. Default: enabled.",
			GoodExample:     "switch value { case 1: work() }",
			BadExample:      "switch value { case 1: work(); break }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: switchCheck((*Pass).checkSingleCaseSwitch),
		meta: Meta{
			Code:            "single-case-switch",
			Summary:         "replace single-case switches with if statements",
			Explanation:     "replace single-case switches with if statements. Default: enabled.",
			GoodExample:     "if value == 1 { work() }",
			BadExample:      "switch value { case 1: work() }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: callCheck((*Pass).checkStringOfInt),
		meta: Meta{
			Code:            "string-of-int",
			Summary:         "make integer-to-string intent explicit",
			Explanation:     "make integer-to-string intent explicit. Default: enabled.",
			GoodExample:     "text := strconv.Itoa(value)",
			BadExample:      "text := string(value)",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: startBehavior((*Pass).checkCompilerDirectiveSpacing),
		meta: Meta{
			Code:            "spaced-compiler-directive",
			Summary:         "detect compiler directives disabled by leading whitespace",
			Explanation:     "detect compiler directives disabled by leading whitespace. Default: enabled.",
			GoodExample:     "//go:noinline\nfunc call() {}",
			BadExample:      "// go:noinline\nfunc call() {}",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: forCheck((*Pass).checkSpinningSelectDefault),
		meta: Meta{
			Code:            "spinning-select-default",
			Summary:         "detect select loops that spin on an empty default",
			Explanation:     "detect select loops that spin on an empty default. Default: enabled.",
			GoodExample:     "for { select { case value := <-values: use(value) } }",
			BadExample:      "for { select { case value := <-values: use(value); default: } }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: invalidStructTagBehavior,
		meta: Meta{
			Code:            "invalid-struct-tag",
			Summary:         "validate struct tag syntax and options",
			Explanation:     "validate struct tag syntax and options. Default: standard tags.",
			GoodExample:     "type User struct { Name string `json:\"name\"` }",
			BadExample:      "type User struct { Name string `json:name` }",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: callCheck((*Pass).checkTimeDateCall),
		meta: Meta{
			Code:            "time-date",
			Summary:         "detect suspicious time.Date arguments",
			Explanation:     "detect suspicious time.Date arguments. Default: enabled.",
			GoodExample:     "time.Date(2026, time.July, 16, 12, 0, 0, 0, time.UTC)",
			BadExample:      "time.Date(2026, 13, 16, 12, 0, 0, 0, time.UTC)",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: timeNamingBehavior,
		meta: Meta{
			Code:            "time-naming",
			Summary:         "avoid unit suffixes on time.Duration values",
			Explanation:     "avoid unit suffixes on time.Duration values. Default: enabled.",
			GoodExample:     "var timeout time.Duration",
			BadExample:      "var timeoutSeconds time.Duration",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: typeAssertionBehavior,
		meta: Meta{
			Code:            "unchecked-type-assertion",
			Summary:         "require checked type assertions",
			Explanation:     "require checked type assertions. Default: enabled.",
			GoodExample:     "value, ok := input.(string)",
			BadExample:      "value := input.(string)",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: identifierCheck((*Pass).checkUnexportedNaming),
		meta: Meta{
			Code:            "unexported-naming",
			Summary:         "avoid leading underscores in private names",
			Explanation:     "avoid leading underscores in private names. Default: enabled.",
			GoodExample:     "var clientID string",
			BadExample:      "var _clientID string",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: functionCheck((*Pass).checkUnexportedReturn),
		meta: Meta{
			Code:            "unexported-return",
			Summary:         "avoid exported APIs returning private types",
			Explanation:     "avoid exported APIs returning private types. Default: enabled.",
			GoodExample:     "func NewClient() *Client",
			BadExample:      "func NewClient() *client",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: conditionalCheck((*Pass).checkUnnecessaryIf),
		meta: Meta{
			Code:            "unnecessary-if",
			Summary:         "replace boolean-returning if chains",
			Explanation:     "replace boolean-returning if chains. Default: enabled.",
			GoodExample:     "return ready",
			BadExample:      "if ready { return true } else { return false }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: callCheck((*Pass).checkUnnecessaryFormat),
		meta: Meta{
			Code:            "unnecessary-format",
			Summary:         "avoid formatting calls without directives",
			Explanation:     "avoid formatting calls without directives. Default: enabled.",
			GoodExample:     "fmt.Sprint(value)",
			BadExample:      "fmt.Sprintf(\"static message\")",
			DefaultSeverity: diagnostic.SeverityNote,
		},
	},
	{
		behavior: blockCheck((*Pass).checkUnreachableCode),
		meta: Meta{
			Code:            "unreachable-code",
			Summary:         "detect statements after unconditional flow",
			Explanation:     "detect statements after unconditional flow. Default: enabled.",
			GoodExample:     "save()\nreturn nil",
			BadExample:      "return nil\nsave()",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: stringLiteralBehavior,
		meta: Meta{
			Code:            "insecure-url-scheme",
			Summary:         "detect insecure URL schemes",
			Explanation:     "detect insecure URL schemes. Default: HTTP, WS, and FTP.",
			GoodExample:     "endpoint := \"https://example.com\"",
			BadExample:      "endpoint := \"http://example.com\"",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: functionCheck((*Pass).checkUnusedParameter),
		meta: Meta{
			Code:            "unused-parameter",
			Summary:         "detect unused function parameters",
			Explanation:     "detect unused function parameters. Default: enabled.",
			GoodExample:     "func greet(name string) { fmt.Println(name) }",
			BadExample:      "func greet(name string) { fmt.Println(\"hello\") }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: functionCheck((*Pass).checkUnusedReceiver),
		meta: Meta{
			Code:            "unused-receiver",
			Summary:         "detect unused method receivers",
			Explanation:     "detect unused method receivers. Default: enabled.",
			GoodExample:     "func (client *Client) Send() { client.flush() }",
			BadExample:      "func (client *Client) Version() string { return version }",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: interfaceBehavior,
		meta: Meta{
			Code:            "use-any",
			Summary:         "prefer any to interface{}",
			Explanation:     "prefer any to interface{}. Default: enabled.",
			GoodExample:     "var value any",
			BadExample:      "var value interface{}",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: callCheck((*Pass).checkUseErrorsNew),
		meta: Meta{
			Code:            "use-errors-new",
			Summary:         "prefer errors.New for static errors",
			Explanation:     "prefer errors.New for static errors. Default: enabled.",
			GoodExample:     "errors.New(\"not found\")",
			BadExample:      "fmt.Errorf(\"not found\")",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: callCheck((*Pass).checkUseFmtPrint),
		meta: Meta{
			Code:            "use-fmt-print",
			Summary:         "prefer fmt.Print over builtin print",
			Explanation:     "prefer fmt.Print over builtin print. Default: enabled.",
			GoodExample:     "fmt.Print(message)",
			BadExample:      "print(message)",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: callCheck((*Pass).checkUseSlicesSort),
		meta: Meta{
			Code:            "use-slices-sort",
			Summary:         "prefer slices.Sort over sort.Slice",
			Explanation:     "prefer slices.Sort over sort.Slice. Default: enabled.",
			GoodExample:     "slices.Sort(values)",
			BadExample:      "sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: varSpecCheck((*Pass).checkVarDeclaration),
		meta: Meta{
			Code:            "var-declaration",
			Summary:         "simplify zero-value declarations",
			Explanation:     "simplify zero-value declarations. Default: enabled.",
			GoodExample:     "var count int",
			BadExample:      "count := 0",
			DefaultSeverity: diagnostic.SeverityNote,
		},
	},
	{
		behavior: identifierCheck((*Pass).checkVarNaming),
		meta: Meta{
			Code:            "var-naming",
			Summary:         "enforce idiomatic identifier naming",
			Explanation:     "enforce idiomatic identifier naming. Default: common initialisms.",
			GoodExample:     "var userID string",
			BadExample:      "var userId string",
			DefaultSeverity: diagnostic.SeverityWarning,
		},
	},
	{
		behavior: functionCheck((*Pass).checkWaitgroupByValue),
		meta: Meta{
			Code:            "waitgroup-by-value",
			Summary:         "pass sync.WaitGroup by pointer",
			Explanation:     "pass sync.WaitGroup by pointer. Default: enabled.",
			GoodExample:     "func wait(group *sync.WaitGroup)",
			BadExample:      "func wait(group sync.WaitGroup)",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
	{
		behavior: binaryCheck((*Pass).checkZeroIntegerDivision),
		meta: Meta{
			Code:            "zero-integer-division",
			Summary:         "detect literal integer division that truncates to zero",
			Explanation:     "detect literal integer division that truncates to zero. Default: enabled.",
			GoodExample:     "ratio := 2.0 / 3.0",
			BadExample:      "ratio := 2 / 3",
			DefaultSeverity: diagnostic.SeverityError,
		},
	},
}

// Catalog returns the immutable syntax descriptor catalog.
func Catalog() []Check {
	checks := make([]Check, len(definitions))
	for index := range definitions {
		checks[index] = definitions[index]
	}
	return checks
}

func validateSingleCharacters(value checkconfig.Value) error {
	characters, _ := value.Strings()
	for _, character := range characters {
		if len([]rune(character)) != 1 {
			return fmt.Errorf("must contain exactly one Unicode character per entry")
		}
	}
	return nil
}
