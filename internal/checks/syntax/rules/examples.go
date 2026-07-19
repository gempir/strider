package rules

// extendedExamples is the source of truth for examples shown by both
// `strider lint --explain` and the generated lint reference pages.
var extendedExamples = map[string]ruleExample{
	"add-constant": {
		Good: "const stateReady = \"ready\"\nif state == stateReady { start() }",
		Bad:  "if state == \"ready\" { start() }\nif next == \"ready\" { queue() }\nif fallback == \"ready\" { recover() }",
	},
	"redundant-atomic-result-assignment": {
		Good: "atomic.AddInt64(&counter, 1)",
		Bad:  "counter = atomic.AddInt64(&counter, 1)",
	},
	"banned-characters": {
		Good: "var userID string",
		Bad:  "var user_id string // when underscore is configured as banned",
	},
	"bidirectional-control-character": {
		Good: "// access denied",
		Bad:  "// access \u202edenied",
	},
	"blank-imports": {
		Good: "import _ \"example.com/driver\" // register the driver",
		Bad:  "import _ \"example.com/driver\"",
	},
	"boolean-literal-comparison": {
		Good: "if ready { start() }",
		Bad:  "if ready == true { start() }",
	},
	"call-to-gc": {
		Good: "buffer = nil // let Go collect it when appropriate",
		Bad:  "runtime.GC() // in normal application code",
	},
	"cognitive-complexity": {
		Good: "if !ready { return }\nprocess()",
		Bad:  "func process(items []int) { for _, item := range items { if item > 0 { for retry := 0; retry < 3; retry++ { if ready(item) { use(item) } } } } }",
	},
	"confusing-naming": {
		Good: "type User struct { ID string; Name string }",
		Bad:  "type User struct { ID string; Id string }",
	},
	"confusing-results": {
		Good: "func bounds() (min int, max int)",
		Bad:  "func bounds() (int, int)",
	},
	"constant-logical-expr": {
		Good: "if ready && connected { start() }",
		Bad:  "if false && connected { start() }",
	},
	"context-as-argument": {
		Good: "func Load(ctx context.Context, id string) error",
		Bad:  "func Load(id string, ctx context.Context) error",
	},
	"deep-exit": {
		Good: "func run() error { return load() }",
		Bad:  "func loadConfig() { if failed() { os.Exit(1) } }",
	},
	"deferred-recover-call": {
		Good: "defer func() { _ = recover() }()",
		Bad:  "defer recover()",
	},
	"discarded-deferred-result": {
		Good: "defer func() { cleanup() }()",
		Bad:  "defer func() error { return cleanup() }()",
	},
	"dot-imports": {
		Good: "import \"fmt\"\nfmt.Println(message)",
		Bad:  "import . \"fmt\"\nPrintln(message)",
	},
	"double-negation": {
		Good: "return ready",
		Bad:  "return !!ready",
	},
	"duplicated-imports": {
		Good: "import \"strings\"",
		Bad:  "import (\n\t\"strings\"\n\ttext \"strings\"\n)",
	},
	"early-return": {
		Good: "if !ready { return }\nprocess()",
		Bad:  "if ready { process() } else { return }",
	},
	"empty-conditional-block": {
		Good: "if ready { process() }",
		Bad:  "if ready {}",
	},
	"enforce-switch-style": {
		Good: "switch value { case 1: one(); default: fallback() }",
		Bad:  "switch value { default: fallback(); case 1: one() }",
	},
	"error-naming": {
		Good: "var ErrNotFound = errors.New(\"not found\")",
		Bad:  "var NotFoundError = errors.New(\"not found\")",
	},
	"error-last-result": {
		Good: "func Load() (string, error)",
		Bad:  "func Load() (error, string)",
	},
	"error-strings": {
		Good: "errors.New(\"connection refused\")",
		Bad:  "errors.New(\"Connection refused.\")",
	},
	"prefer-fmt-errorf": {
		Good: "fmt.Errorf(\"load %s\", name)",
		Bad:  "errors.New(fmt.Sprintf(\"load %s\", name))",
	},
	"exported-declaration-comment": {
		Good: "// Client sends requests.\ntype Client struct{}",
		Bad:  "type Client struct{}",
	},
	"file-length-limit": {
		Good: "// With max = 3 lines:\npackage service\nvar ready bool",
		Bad:  "// With max = 2 lines:\npackage service\nvar ready bool",
	},
	"filename-format": {
		Good: "// user_service.go",
		Bad:  "// user.service.go",
	},
	"flag-parameter": {
		Good: "func Open(path string, mode Mode) error",
		Bad:  "func Open(path string, readOnly bool) error { if readOnly { return openReadOnly(path) }; return openReadWrite(path) }",
	},
	"function-length": {
		Good: "func run() { load(); process(); save() }",
		Bad:  "// With max-statements = 3:\nfunc run() { load(); process(); save(); notify() }",
	},
	"function-result-limit": {
		Good: "func Parse() (Value, error)",
		Bad:  "func Parse() (Value, Metadata, Warnings, error)",
	},
	"get-function-return-value": {
		Good: "func GetClient() *Client { return client }",
		Bad:  "func GetClient() { initializeClient() }",
	},
	"identical-branches": {
		Good: "if ready { start() } else { wait() }",
		Bad:  "if ready { start() } else { start() }",
	},
	"identical-if-chain-branches": {
		Good: "if first { one() } else if second { two() }",
		Bad:  "if first { run() } else if second { run() }",
	},
	"identical-if-chain-conditions": {
		Good: "if first { one() } else if second { two() }",
		Bad:  "if ready { one() } else if ready { two() }",
	},
	"identical-switch-branches": {
		Good: "switch value { case 1: one(); case 2: two() }",
		Bad:  "switch value { case 1: run(); case 2: run() }",
	},
	"identical-switch-conditions": {
		Good: "switch { case first: one(); case second: two() }",
		Bad:  "switch { case ready: one(); case ready: two() }",
	},
	"redundant-error-return-check": {
		Good: "err := save()\nreturn err",
		Bad:  "if err := save(); err != nil { return err }\nreturn nil",
	},
	"ineffective-pointer-copy": {
		Good: "copy := *pointer",
		Bad:  "copy := &*pointer",
	},
	"import-alias-naming": {
		Good: "import jsonapi \"example.com/json-api\"",
		Bad:  "import JSON_API \"example.com/json-api\"",
	},
	"import-shadowing": {
		Good: "encoded := json.Marshal(value)",
		Bad:  "json := loadConfig() // shadows the json import",
	},
	"imports-blocklist": {
		Good: "import \"log/slog\"",
		Bad:  "import \"log\" // when log is configured as blocked",
	},
	"increment-decrement": {
		Good: "count++",
		Bad:  "count += 1",
	},
	"inefficient-map-lookup": {
		Good: "if value, ok := values[key]; ok { use(value) }",
		Bad:  "if _, ok := values[key]; ok { use(values[key]) }",
	},
	"marshal-receiver": {
		Good: "func (value *Value) MarshalJSON() ([]byte, error)\nfunc (value *Value) UnmarshalJSON([]byte) error",
		Bad:  "func (value Value) MarshalJSON() ([]byte, error)\nfunc (value *Value) UnmarshalJSON([]byte) error",
	},
	"max-control-nesting": {
		Good: "if !ready { return }\nfor _, item := range items { process(item) }",
		Bad:  "if ready { for { switch value { case 1: if valid { for retry() { if connected { process() } } } } } }",
	},
	"max-public-structs": {
		Good: "type Request struct{}\ntype Response struct{}",
		Bad:  "type One struct{}\ntype Two struct{}\ntype Three struct{}\ntype Four struct{}\ntype Five struct{}\ntype Six struct{}",
	},
	"modifies-parameter": {
		Good: "func normalize(value string) string { normalized := strings.TrimSpace(value); return normalized }",
		Bad:  "func normalize(value string) string { value = strings.TrimSpace(value); return value }",
	},
	"modifies-value-receiver": {
		Good: "func (item *Item) Rename(name string) { item.Name = name }",
		Bad:  "func (item Item) Rename(name string) { item.Name = name }",
	},
	"modulo-one": {
		Good: "remainder := value % divisor",
		Bad:  "remainder := value % 1",
	},
	"nested-structs": {
		Good: "type Address struct { City string }\ntype User struct { Address Address }",
		Bad:  "type User struct { Address struct { City string } }",
	},
	"optimize-operands-order": {
		Good: "if ready && expensiveCheck() { start() }",
		Bad:  "if expensiveCheck() && ready { start() }",
	},
	"package-comments": {
		Good: "// Package store persists application data.\npackage store",
		Bad:  "package store",
	},
	"package-directory-mismatch": {
		Good: "// store/client.go\npackage store",
		Bad:  "// storage/client.go\npackage store",
	},
	"package-naming": {
		Good: "package httputil",
		Bad:  "package http_util",
	},
	"simplify-range": {
		Good: "for _, character := range text { use(character) }",
		Bad:  "for _, character := range []rune(text) { use(character) }",
	},
	"range-value-address": {
		Good: "for index := range values { pointers = append(pointers, &values[index]) }",
		Bad:  "for _, value := range values { pointers = append(pointers, &value) }",
	},
	"receiver-naming": {
		Good: "func (c *Client) Send() error",
		Bad:  "func (self *Client) Send() error",
	},
	"redefines-builtin-id": {
		Good: "func parse(input string) error",
		Bad:  "func parse(error string) error",
	},
	"redundant-build-tag": {
		Good: "//go:build linux || darwin",
		Bad:  "//go:build linux || darwin\n// +build linux darwin",
	},
	"redundant-import-alias": {
		Good: "import \"strings\"",
		Bad:  "import strings \"strings\"",
	},
	"redundant-final-return": {
		Good: "func run() { work() }",
		Bad:  "func run() { work(); return }",
	},
	"redundant-switch-break": {
		Good: "switch value { case 1: work() }",
		Bad:  "switch value { case 1: work(); break }",
	},
	"single-case-switch": {
		Good: "if value == 1 { work() }",
		Bad:  "switch value { case 1: work() }",
	},
	"spaced-compiler-directive": {
		Good: "//go:noinline\nfunc call() {}",
		Bad:  "// go:noinline\nfunc call() {}",
	},
	"spinning-select-default": {
		Good: "for { select { case value := <-values: use(value) } }",
		Bad:  "for { select { case value := <-values: use(value); default: } }",
	},
	"string-of-int": {
		Good: "text := strconv.Itoa(value)",
		Bad:  "text := string(value)",
	},
	"invalid-struct-tag": {
		Good: "type User struct { Name string `json:\"name\"` }",
		Bad:  "type User struct { Name string `json:name` }",
	},
	"time-date": {
		Good: "time.Date(2026, time.July, 16, 12, 0, 0, 0, time.UTC)",
		Bad:  "time.Date(2026, 13, 16, 12, 0, 0, 0, time.UTC)",
	},
	"time-naming": {
		Good: "var timeout time.Duration",
		Bad:  "var timeoutSeconds time.Duration",
	},
	"unchecked-type-assertion": {
		Good: "value, ok := input.(string)",
		Bad:  "value := input.(string)",
	},
	"unexported-naming": {
		Good: "var clientID string",
		Bad:  "var _clientID string",
	},
	"unexported-return": {
		Good: "func NewClient() *Client",
		Bad:  "func NewClient() *client",
	},
	"unnecessary-format": {
		Good: "fmt.Sprint(value)",
		Bad:  "fmt.Sprintf(\"static message\")",
	},
	"unnecessary-if": {
		Good: "return ready",
		Bad:  "if ready { return true } else { return false }",
	},
	"unreachable-code": {
		Good: "save()\nreturn nil",
		Bad:  "return nil\nsave()",
	},
	"insecure-url-scheme": {
		Good: "endpoint := \"https://example.com\"",
		Bad:  "endpoint := \"http://example.com\"",
	},
	"unused-parameter": {
		Good: "func greet(name string) { fmt.Println(name) }",
		Bad:  "func greet(name string) { fmt.Println(\"hello\") }",
	},
	"unused-receiver": {
		Good: "func (client *Client) Send() { client.flush() }",
		Bad:  "func (client *Client) Version() string { return version }",
	},
	"use-any": {
		Good: "var value any",
		Bad:  "var value interface{}",
	},
	"use-errors-new": {
		Good: "errors.New(\"not found\")",
		Bad:  "fmt.Errorf(\"not found\")",
	},
	"use-fmt-print": {
		Good: "fmt.Print(message)",
		Bad:  "print(message)",
	},
	"use-slices-sort": {
		Good: "slices.Sort(values)",
		Bad:  "sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })",
	},
	"var-declaration": {
		Good: "var count int",
		Bad:  "count := 0",
	},
	"var-naming": {
		Good: "var userID string",
		Bad:  "var userId string",
	},
	"waitgroup-by-value": {
		Good: "func wait(group *sync.WaitGroup)",
		Bad:  "func wait(group sync.WaitGroup)",
	},
	"zero-integer-division": {
		Good: "ratio := 2.0 / 3.0",
		Bad:  "ratio := 2 / 3",
	},
}

type ruleExample struct {
	Good string
	Bad  string
}
