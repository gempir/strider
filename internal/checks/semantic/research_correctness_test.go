package semantic

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestResearchCorrectnessRuleMetadata(t *testing.T) {
	tests := []struct {
		rule     Rule
		code     string
		severity diagnostic.Severity
	}{
		{nilErrorReturnRule{}, "nil-error-return", diagnostic.SeverityError},
		{nilValueWithNilErrorRule{}, "nil-value-with-nil-error", diagnostic.SeverityWarning},
		{unclosedHTTPResponseBodyRule{}, "unclosed-http-response-body", diagnostic.SeverityError},
		{unclosedSQLResourceRule{}, "unclosed-sql-resource", diagnostic.SeverityError},
		{contextCancelInLoopRule{}, "context-cancel-in-loop", diagnostic.SeverityWarning},
		{copyLockValueRule{}, "copy-lock-value", diagnostic.SeverityError},
	}
	for _, test := range tests {
		meta := test.rule.Meta()
		if meta.Code != test.code || meta.DefaultSeverity != test.severity {
			t.Errorf(
				"%T metadata = (%q, %q), want (%q, %q)",
				test.rule,
				meta.Code,
				meta.DefaultSeverity,
				test.code,
				test.severity,
			)
		}
		if meta.Summary == "" || meta.Explanation == "" || meta.GoodExample == "" || meta.BadExample == "" {
			t.Errorf("%s has incomplete metadata: %#v", test.code, meta)
		}
	}
}

func TestNilErrorReturnReportsContradictoryReturns(t *testing.T) {
	reports := runResearchCorrectnessRule(
		t,
		nilErrorReturnRule{},
		`package fixture

func bad(err error) (*int, error) {
	if err != nil {
		return nil, nil
	}
	return new(int), nil
}

func badElse(err error) (*int, error) {
	if err == nil {
		return new(int), nil
	} else {
		return nil, nil
	}
}

func good(err error) (*int, error) {
	if err != nil {
		return nil, err
	}
	return new(int), nil
}

func reassigned(err error) (*int, error) {
	if err != nil {
		err = nil
		return nil, nil
	}
	return new(int), nil
}

func reassignedAfterBadReturn(err error, cond bool) (*int, error) {
	if err != nil {
		if cond {
			return nil, nil
		}
		err = nil
		return nil, nil
	}
	return new(int), nil
}
`,
	)
	assertResearchReportCount(t, reports, 3)
	assertResearchMessagesContain(t, reports, "proves an error is non-nil")
}

func TestNilValueWithNilErrorReportsOnlyPayloadFunctions(t *testing.T) {
	reports := runResearchCorrectnessRule(
		t,
		nilValueWithNilErrorRule{},
		`package fixture

func badPointer() (*int, error) { return nil, nil }
func badSlice() ([]int, error) { return nil, nil }
func errorOnly() error { return nil }
func noErrorResult() (*int, bool) { return nil, false }
func good() (*int, error) { return nil, errMissing }

var errMissing = missingError{}
type missingError struct{}
func (missingError) Error() string { return "missing" }
`,
	)
	assertResearchReportCount(t, reports, 2)
	assertResearchMessagesContain(t, reports, "nil payload")
}

func TestUnclosedHTTPResponseBodyTracksCloseAndTransfer(t *testing.T) {
	reports := runResearchCorrectnessRule(
		t,
		unclosedHTTPResponseBodyRule{},
		`package fixture

import (
	"encoding/json"
	"errors"
	"net/http"
)

func bad(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	_ = response.StatusCode
	return nil
}

func badDecode(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	var value any
	return json.NewDecoder(response.Body).Decode(&value)
}

func good(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	defer response.Body.Close()
	return nil
}

func goodBodyAlias(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	body := response.Body
	defer body.Close()
	return nil
}

func goodDeferredWrapper(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	defer func() { _ = response.Body.Close() }()
	return nil
}

func conditionalClose(url string, closeBody bool) error {
	response, err := http.Get(url)
	if err != nil { return err }
	if closeBody {
		defer response.Body.Close()
	}
	return nil
}

func conditionalDeferredWrapper(url string, closeBody bool) error {
	response, err := http.Get(url)
	if err != nil { return err }
	defer func() {
		if closeBody { _ = response.Body.Close() }
	}()
	return nil
}

func asynchronousClose(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	go func() { _ = response.Body.Close() }()
	return nil
}

func reusedError(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	err = errors.New("later failure")
	if err != nil { return err }
	return response.Body.Close()
}

func transferred(url string) (*http.Response, error) {
	response, err := http.Get(url)
	if err != nil { return nil, err }
	return response, nil
}

func sent(url string, output chan<- *http.Response) error {
	response, err := http.Get(url)
	if err != nil { return err }
	output <- response
	return nil
}
`,
	)
	assertResearchReportCount(t, reports, 6)
	assertResearchMessagesContain(t, reports, "HTTP response body")
}

func TestUnclosedHTTPResponseBodyTracksAliasGenerations(t *testing.T) {
	source := `package fixture

import "net/http"

func bodyAlias() {
	response, _ := http.Get("body-first")
	oldBody := response.Body
	response, _ = http.Get("body-second")
	defer oldBody.Close()
}

func responseAlias() {
	response, _ := http.Get("response-first")
	oldResponse := response
	response, _ = http.Get("response-second")
	defer oldResponse.Body.Close()
}

func bothClosed() {
	response, _ := http.Get("closed-first")
	oldResponse := response
	response, _ = http.Get("closed-second")
	defer oldResponse.Body.Close()
	defer response.Body.Close()
}

func pathDependent(useNew bool) {
	response, _ := http.Get("path-first")
	alias := response
	response, _ = http.Get("path-second")
	if useNew {
		alias = response
	}
	defer alias.Body.Close()
}
`
	reports := runResearchCorrectnessRule(t, unclosedHTTPResponseBodyRule{}, source)
	assertResearchReportCount(t, reports, 4)
	assertResearchReportNeedles(
		t,
		reports,
		source,
		"http.Get(\"body-second\")",
		"http.Get(\"response-second\")",
		"http.Get(\"path-first\")",
		"http.Get(\"path-second\")",
	)
}

func TestUnclosedHTTPResponseBodyTracksBodyReplacementAndAliasTransfer(t *testing.T) {
	source := `package fixture

import (
	"io"
	"net/http"
	"strings"
)

func replaced() {
	response, _ := http.Get("replaced")
	response.Body = io.NopCloser(strings.NewReader("replacement"))
	defer response.Body.Close()
}

func transferredAlias() (io.ReadCloser, error) {
	response, err := http.Get("transferred")
	if err != nil { return nil, err }
	body := response.Body
	return body, nil
}

func conditionalDeferred(skip bool) {
	response, _ := http.Get("conditional-deferred")
	defer func() {
		if skip { return }
		_ = response.Body.Close()
	}()
}
`
	reports := runResearchCorrectnessRule(t, unclosedHTTPResponseBodyRule{}, source)
	assertResearchReportCount(t, reports, 2)
	assertResearchReportNeedles(
		t,
		reports,
		source,
		"http.Get(\"replaced\")",
		"http.Get(\"conditional-deferred\")",
	)
}

func TestUnclosedHTTPResponseBodyTreatsNamedResultsAsTransfers(t *testing.T) {
	reports := runResearchCorrectnessRule(
		t,
		unclosedHTTPResponseBodyRule{},
		`package fixture

import (
	"io"
	"net/http"
)

func namedResponse() (response *http.Response, err error) {
	response, err = http.Get("named-response")
	return
}

func namedBody() (body io.ReadCloser, err error) {
	response, err := http.Get("named-body")
	if err != nil { return nil, err }
	body = response.Body
	return
}
`,
	)
	assertResearchReportCount(t, reports, 0)
}

func TestUnclosedSQLResourceTracksRowsAndStatements(t *testing.T) {
	reports := runResearchCorrectnessRule(
		t,
		unclosedSQLResourceRule{},
		`package fixture

import "database/sql"

func badRows(database *sql.DB) error {
	rows, err := database.Query("select 1")
	if err != nil { return err }
	_ = rows
	return nil
}

func goodRows(database *sql.DB) error {
	rows, err := database.Query("select 1")
	if err != nil { return err }
	defer rows.Close()
	return nil
}

func goodRowsDeferredWrapper(database *sql.DB) error {
	rows, err := database.Query("select 1")
	if err != nil { return err }
	defer func() { _ = rows.Close() }()
	return nil
}

func conditionalRowsClose(database *sql.DB, closeRows bool) error {
	rows, err := database.Query("select 1")
	if err != nil { return err }
	if closeRows {
		defer rows.Close()
	}
	return nil
}

func badStatement(database *sql.DB) error {
	statement, err := database.Prepare("select 1")
	if err != nil { return err }
	_ = statement
	return nil
}

func goodStatement(database *sql.DB) error {
	statement, err := database.Prepare("select 1")
	if err != nil { return err }
	return statement.Close()
}

func transferred(database *sql.DB) (*sql.Rows, error) {
	rows, err := database.Query("select 1")
	if err != nil { return nil, err }
	return rows, nil
}
`,
	)
	assertResearchReportCount(t, reports, 3)
	assertResearchMessagesContain(t, reports, "sql.")
}

func TestUnclosedSQLResourceTracksAliasGenerations(t *testing.T) {
	source := `package fixture

import "database/sql"

func rowsAlias(database *sql.DB) {
	rows, _ := database.Query("rows-first")
	oldRows := rows
	rows, _ = database.Query("rows-second")
	defer oldRows.Close()
}

func statementAlias(database *sql.DB) {
	statement, _ := database.Prepare("statement-first")
	oldStatement := statement
	statement, _ = database.Prepare("statement-second")
	defer oldStatement.Close()
}

func bothClosed(database *sql.DB) {
	rows, _ := database.Query("closed-first")
	oldRows := rows
	rows, _ = database.Query("closed-second")
	defer oldRows.Close()
	defer rows.Close()
}

func pathDependent(database *sql.DB, useNew bool) {
	rows, _ := database.Query("path-first")
	alias := rows
	rows, _ = database.Query("path-second")
	if useNew {
		alias = rows
	}
	defer alias.Close()
}
`
	reports := runResearchCorrectnessRule(t, unclosedSQLResourceRule{}, source)
	assertResearchReportCount(t, reports, 4)
	assertResearchReportNeedles(
		t,
		reports,
		source,
		"database.Query(\"rows-second\")",
		"database.Prepare(\"statement-second\")",
		"database.Query(\"path-first\")",
		"database.Query(\"path-second\")",
	)
}

func TestUnclosedSQLResourceTreatsNamedResultsAsTransfers(t *testing.T) {
	reports := runResearchCorrectnessRule(
		t,
		unclosedSQLResourceRule{},
		`package fixture

import "database/sql"

func namedRows(database *sql.DB) (rows *sql.Rows, err error) {
	rows, err = database.Query("rows")
	return
}

func namedStatement(database *sql.DB) (statement *sql.Stmt, err error) {
	statement, err = database.Prepare("statement")
	return
}
`,
	)
	assertResearchReportCount(t, reports, 0)
}

func TestContextCancelInLoopRequiresIterationBoundedCancel(t *testing.T) {
	reports := runResearchCorrectnessRule(
		t,
		contextCancelInLoopRule{},
		`package fixture

import (
	"context"
	"time"
)

func badDeferred(ctx context.Context, items []int) {
	for range items {
		_, cancel := context.WithCancel(ctx)
		defer cancel()
	}
}

func badOmitted(ctx context.Context) {
	for i := 0; i < 2; i++ {
		_, cancel := context.WithTimeout(ctx, time.Second)
		_ = cancel
	}
}

func badIgnored(ctx context.Context, items []int) {
	for range items {
		_, _ = context.WithDeadline(ctx, time.Now())
	}
}

func badConditional(ctx context.Context, items []int, ok bool) {
	for range items {
		_, cancel := context.WithCancel(ctx)
		if ok {
			cancel()
		}
	}
}

func badEarlyContinue(ctx context.Context, items []int, skip bool) {
	for range items {
		_, cancel := context.WithCancel(ctx)
		if skip {
			continue
		}
		cancel()
	}
}

func good(ctx context.Context, items []int) {
	for range items {
		_, cancel := context.WithCancel(ctx)
		cancel()
	}
}

func goodHelper(ctx context.Context, items []int) {
	for range items {
		func() {
			_, cancel := context.WithCancel(ctx)
			defer cancel()
		}()
	}
}
`,
	)
	assertResearchReportCount(t, reports, 5)
}

func TestCopyLockValueReportsConservativeCopySites(t *testing.T) {
	reports := runResearchCorrectnessRule(
		t,
		copyLockValueRule{},
		`package fixture

import "sync"

type State struct {
	mu sync.Mutex
	value int
}

func receive(state State) {}
func (state State) valueMethod() {}
func pointer(state *State) {}

func copies(source State) State {
	target := source
	_ = target
	receive(source)
	output := make(chan State, 1)
	output <- source
	_ = struct { State State }{State: source}
	_ = map[State]int{source: 1}
	var states []State
	states = append(states, source)
	for _, item := range []State{source} {
		pointer(&item)
	}
	return source
}

func fresh() State { return State{} }
func good(source *State) { pointer(source) }
`,
	)
	assertResearchReportCount(t, reports, 12)
	assertResearchMessagesContain(t, reports, "sync.Mutex")
}

type researchCorrectnessReport struct {
	position token.Position
	message  string
}

func runResearchCorrectnessRule(t *testing.T, rule Rule, source string) []researchCorrectnessReport {
	t.Helper()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "fixture.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	ssaPackage, typeInfo, err := ssautil.BuildPackage(
		&types.Config{Importer: importer.Default()},
		fileSet,
		types.NewPackage("example.com/researchfixture", "fixture"),
		[]*ast.File{file},
		ssa.InstantiateGenerics,
	)
	if err != nil {
		t.Fatal(err)
	}
	reports := make([]researchCorrectnessReport, 0)
	pass := &Pass{
		PackagePath: "example.com/researchfixture",
		Files:       []*ast.File{file},
		FileSet:     fileSet,
		Types:       ssaPackage.Pkg,
		TypesSizes:  types.SizesFor("gc", runtime.GOARCH),
		TypesInfo:   typeInfo,
		SSAProgram:  ssaPackage.Prog,
		SSAPackage:  ssaPackage,
		Functions:   collectPackageFunctions(ssaPackage.Prog, []*ssa.Package{ssaPackage})[ssaPackage],
	}
	pass.report = func(node ast.Node, message string) {
		reports = append(
			reports,
			researchCorrectnessReport{position: fileSet.Position(node.Pos()), message: message},
		)
	}
	rule.Run(pass)
	return reports
}

func assertResearchReportCount(t *testing.T, reports []researchCorrectnessReport, want int) {
	t.Helper()
	if len(reports) == want {
		return
	}
	t.Fatalf("got %d reports, want %d: %#v", len(reports), want, reports)
}

func assertResearchMessagesContain(
	t *testing.T,
	reports []researchCorrectnessReport,
	fragment string,
) {
	t.Helper()
	for _, report := range reports {
		if !strings.Contains(report.message, fragment) {
			t.Errorf(
				"report at %s has message %q; want fragment %q",
				report.position,
				report.message,
				fragment,
			)
		}
	}
}

func assertResearchReportNeedles(
	t *testing.T,
	reports []researchCorrectnessReport,
	source string,
	needles ...string,
) {
	t.Helper()
	want := make(map[int]bool, len(needles))
	for _, needle := range needles {
		index := strings.Index(source, needle)
		if index < 0 {
			t.Fatalf("test source does not contain %q", needle)
		}
		want[1+strings.Count(source[:index], "\n")] = true
	}
	for _, report := range reports {
		if !want[report.position.Line] {
			t.Errorf("unexpected report at %s: %s", report.position, report.message)
			continue
		}
		delete(want, report.position.Line)
	}
	for line := range want {
		t.Errorf("missing report on fixture.go:%d", line)
	}
}
