package semantic

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/config"
	"github.com/gempir/strider/internal/diagnostic"
)

func restoreSemanticWorkingDirectory(testingObject testing.TB, directory string) {
	testingObject.Helper()
	if err := os.Chdir(directory); err != nil {
		testingObject.Errorf("restore working directory: %v", err)
	}
}

func TestInvalidRegexpReportsConstantInvalidRegexps(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import rx "regexp"

const invalid = "["

func check(pattern string) {
	rx.MustCompile(invalid)
	local := "("
	rx.Compile(local)
	rx.MatchString("[a-", "value")
	compile := rx.Compile
	compile("[")
	rx.Compile(pattern)
}
`,
	)
	registry, err := NewRegistry([]string{"invalid-regexp"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 4 {
		t.Fatalf("got %d diagnostics, want 4: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != "invalid-regexp" || !strings.Contains(item.Message, "error parsing regexp") {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
		if item.Start.Filename != "main.go" && !(runtime.GOOS == "windows" && filepath.Base(filepath.FromSlash(item.Start.Filename)) == "main.go") {
			t.Fatalf("unexpected display path: %q", item.Start.Filename)
		}
	}
}

func TestInvalidRegexpAcceptsValidAndDynamicRegexps(t *testing.T) {
	root := analysisModule(t, `package sample

import "regexp"

func check(pattern string) {
	regexp.MustCompile("[a-z]+")
	regexp.Compile(pattern)
}
`)
	registry, err := NewRegistry([]string{"invalid-regexp"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
}

func TestInvalidTemplateReportsInvalidDirectTemplates(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	htmltemplate "html/template"
	texttemplate "text/template"
)

const invalid = "{{.Name}} {{.LastName}"

func check() {
	texttemplate.New("").Parse(invalid)
	htmltemplate.New("").Parse(invalid)
	texttemplate.New("").Parse("{{missingFunction}}")
	template := texttemplate.New("")
	template.Parse(invalid)
	texttemplate.New("").Delims("[[", "]]").Parse("{{broken-}}")
}
`,
	)
	registry, err := NewRegistry([]string{"invalid-template"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		validMessage := strings.Contains(item.Message, "unexpected") || strings.Contains(item.Message, "bad character")
		if item.Code != "invalid-template" || !validMessage {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}

func TestInvalidTimeLayoutReportsInvalidConstantTimeLayouts(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "time"

const invalidLayout = "12345"

func check(value string) {
	time.Parse(invalidLayout, value)
	local := "12345"
	time.Parse(local, value)
	time.Parse("2006-01-02", value)
	time.Parse(time.RFC3339Nano, value)
}
`,
	)
	registry, err := NewRegistry([]string{"invalid-time-layout"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != "invalid-time-layout" || !strings.Contains(item.Message, "parsing time") {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}

func TestUnsupportedBinaryWriteReportsUnsupportedBinaryWriteValues(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"encoding/binary"
	"io"
)

type valid struct { A int32 }
type invalid struct { A int }

func check() {
	var architectureSized int
	var validInteger int32
	var invalidSlice []int
	var validSlice []int32
	var invalidMap map[string]int32
	var invalidChannel chan int32
	var invalidStruct invalid
	var validStruct valid
	binary.Write(io.Discard, binary.LittleEndian, architectureSized)
	binary.Write(io.Discard, binary.LittleEndian, validInteger)
	binary.Write(io.Discard, binary.LittleEndian, invalidSlice)
	binary.Write(io.Discard, binary.LittleEndian, validSlice)
	binary.Write(io.Discard, binary.LittleEndian, invalidMap)
	binary.Write(io.Discard, binary.LittleEndian, invalidChannel)
	binary.Write(io.Discard, binary.LittleEndian, invalidStruct)
	binary.Write(io.Discard, binary.LittleEndian, &validStruct)
}
`,
	)
	registry, err := NewRegistry([]string{"unsupported-binary-write"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 5 {
		t.Fatalf("got %d diagnostics, want 5: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != "unsupported-binary-write" || !strings.Contains(item.Message, "binary.Write") {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}

func TestSuspiciousSleepReportsSmallBareSleepLiterals(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "time"

const one = 1

func check() {
	time.Sleep(1)
	time.Sleep(42)
	time.Sleep(0)
	time.Sleep(121)
	time.Sleep(one)
	time.Sleep(2 * time.Nanosecond)
}
`,
	)
	registry, err := NewRegistry([]string{"suspicious-sleep"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != "suspicious-sleep" || !strings.Contains(item.Message, "nanoseconds") {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}

func TestInvalidExecCommandReportsShellCommandAsProgram(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "os/exec"

func check(dynamic string) {
	exec.Command("ls -la")
	exec.Command("ls", "-la")
	exec.Command("/Applications/My Program/tool")
	exec.Command(`+"`C:\\Program Files\\tool.exe`"+`)
	exec.Command(dynamic)
}
`,
	)
	registry, err := NewRegistry([]string{"invalid-exec-command"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "invalid-exec-command" {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
}

func TestDynamicPrintfReportsLoneDynamicPrintfFormats(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"fmt"
	"os"
)

func check(message string) {
	fmt.Printf(message)
	fmt.Fprintf(os.Stdout, message)
	fmt.Printf("%s", message)
	fmt.Fprintf(os.Stdout, "%s", message)
}
`,
	)
	registry, err := NewRegistry([]string{"dynamic-printf"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestInvalidURLReportsInvalidConstantURLs(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "net/url"

func check(dynamic string) {
	url.Parse(":")
	invalid := "%gh&%"
	url.Parse(invalid)
	url.Parse("https://golang.org")
	url.Parse(dynamic)
}
`,
	)
	registry, err := NewRegistry([]string{"invalid-url"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
	for _, item := range diagnostics {
		if item.Code != "invalid-url" || !strings.Contains(item.Message, "not a valid URL") {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}

func TestNonCanonicalHeaderReportsNonCanonicalHeaderReads(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "net/http"

func check() {
	const key = "x-request-id"
	var request http.Request
	header := http.Header{}
	_ = header["content-type"]
	_ = header[key]
	_ = request.Header["etag"]
	_ = header["Content-Type"]
	header["content-type"] = nil
	request.Header["etag"] = nil
	header["Content-Type"] = request.Header["etag"]
	plain := map[string][]string{}
	_ = plain["content-type"]
}
`,
	)
	registry, err := NewRegistry([]string{"non-canonical-header"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 4 {
		t.Fatalf("got %d diagnostics, want 4: %#v", len(diagnostics), diagnostics)
	}
}

func TestRegexpFindAllZeroReportsRegexpFindAllWithZeroLimit(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "regexp"

func check(expression *regexp.Regexp, input string, dynamic int) {
	expression.FindAllString(input, 0)
	zero := 0
	expression.FindAllStringIndex(input, zero)
	expression.FindAllString(input, -1)
	expression.FindAllString(input, dynamic)
}
`,
	)
	registry, err := NewRegistry([]string{"regexp-find-all-zero"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestInvalidUTF8ReportsInvalidUTF8StringArguments(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "strings"

func check(dynamic string) {
	strings.Trim("value", "\xff")
	invalid := "\x80"
	strings.ContainsAny("value", invalid)
	strings.Trim("value", "é")
	strings.Trim("value", dynamic)
}
`,
	)
	registry, err := NewRegistry([]string{"invalid-utf8"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestNilContextReportsNilFirstContextArgument(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "context"

func first(ctx context.Context) {}
func second(value string, ctx context.Context) {}
func generic[T any](ctx context.Context, value T) {}

func check() {
	first(nil)
	generic(nil, 1)
	first(context.TODO())
	second("value", nil)
	_ = (func(context.Context))(nil)
}
`,
	)
	registry, err := NewRegistry([]string{"nil-context"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestSwappedSeekArgumentsReportsSwappedSeekArguments(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"io"
	"os"
)

type wrongSignature struct{}
func (wrongSignature) Seek(whence int, offset int64) (int64, error) { return 0, nil }

func check(seeker io.Seeker, file *os.File, custom wrongSignature) {
	seeker.Seek(io.SeekStart, 0)
	file.Seek(io.SeekEnd, 0)
	seeker.Seek(0, io.SeekStart)
	custom.Seek(io.SeekStart, 0)
}
`,
	)
	registry, err := NewRegistry([]string{"swapped-seek-arguments"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestNonPointerUnmarshalReportsNonPointerDestinations(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"encoding/json"
	"encoding/xml"
)

func check(dynamic any) {
	var value map[string]any
	var boxed any = value
	json.Unmarshal(nil, value)
	json.Unmarshal(nil, boxed)
	json.Unmarshal(nil, dynamic)
	json.Unmarshal(nil, &value)
	json.NewDecoder(nil).Decode(value)
	xml.Unmarshal(nil, value)
}
`,
	)
	registry, err := NewRegistry([]string{"non-pointer-unmarshal"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 4 {
		t.Fatalf("got %d diagnostics, want 4: %#v", len(diagnostics), diagnostics)
	}
}

func TestLeakyTimeTickReportsReturningFunctionsOnOlderGoVersions(t *testing.T) {
	root := analysisModuleVersion(
		t,
		"1.22",
		`package sample

import "time"

func returning() <-chan time.Time {
	return time.Tick(time.Second)
}

func endless() {
	for range time.Tick(time.Second) {}
}
`,
	)
	registry, err := NewRegistry([]string{"leaky-time-tick"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestLeakyTimeTickAllowsModernGoVersions(t *testing.T) {
	root := analysisModule(t, `package sample

import "time"

func returning() <-chan time.Time {
	return time.Tick(time.Second)
}
`)
	registry, err := NewRegistry([]string{"leaky-time-tick"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("got %d diagnostics, want none: %#v", len(diagnostics), diagnostics)
	}
}

func TestUntrappableSignalReportsKernelHandledSignals(t *testing.T) {
	stopArgument := ", syscall.SIGSTOP"
	want := 4
	if runtime.GOOS == "windows" {
		stopArgument = ""
		want = 3
	}
	root := analysisModule(
		t,
		fmt.Sprintf(
			`package sample

import (
	"os"
	"os/signal"
	"syscall"
)

func configure(ch chan<- os.Signal) {
	signal.Notify(ch, os.Kill, syscall.SIGKILL%s)
	signal.Ignore(os.Signal(syscall.SIGKILL))
	signal.Reset(syscall.SIGTERM)
}
`,
			stopArgument,
		),
	)
	registry, err := NewRegistry([]string{"untrappable-signal"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != want {
		t.Fatalf("got %d diagnostics, want %d: %#v", len(diagnostics), want, diagnostics)
	}
}

func TestUnbufferedSignalChannelReportsDirectUnbufferedChannels(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"os"
	"os/signal"
)

func configure() {
	unbuffered := make(chan os.Signal)
	buffered := make(chan os.Signal, 1)
	signal.Notify(unbuffered, os.Interrupt)
	signal.Notify(buffered, os.Interrupt)
}
`,
	)
	registry, err := NewRegistry([]string{"unbuffered-signal-channel"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestZeroReplacementLimitReportsZeroConstants(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"bytes"
	"strings"
)

const none = 0

func replace(value string, raw []byte) {
	strings.Replace(value, "a", "b", none)
	bytes.Replace(raw, nil, nil, 0)
	strings.Replace(value, "a", "b", -1)
}
`,
	)
	registry, err := NewRegistry([]string{"zero-replacement-limit"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestDeprecatedAPIUsageReportsDependencyMarkers(t *testing.T) {
	root := analysisModule(t, `package sample

import "example.com/analysis/legacy"

func use() int {
	legacy.Old()
	return legacy.Value{OldField: 1}.OldField
}
`)
	legacy := filepath.Join(root, "legacy")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(legacy, "legacy.go"),
		[]byte(`// Package legacy contains compatibility APIs.
//
// Deprecated: use the modern package instead.
package legacy

// Old should not be used.
//
// Deprecated: use New instead.
func Old() {}

type Value struct {
	// Deprecated: use NewField instead.
	OldField int
	NewField int
}
`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry([]string{"deprecated-api-usage"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 4 {
		t.Fatalf("got %d diagnostics, want 4: %#v", len(diagnostics), diagnostics)
	}
}

func TestDeprecatedAPIUsageReportsImportedGenericAndInterfaceMethods(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "example.com/analysis/legacy"

func use(generic legacy.Generic[int], contract legacy.Contract) {
	generic.OldMethod()
	contract.OldInterfaceMethod()
}
`,
	)
	legacy := filepath.Join(root, "legacy")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(legacy, "legacy.go"),
		[]byte(`package legacy

type Generic[T any] struct{}

// Deprecated: use NewMethod instead.
func (Generic[T]) OldMethod() {}

type Contract interface {
	// Deprecated: use NewInterfaceMethod instead.
	OldInterfaceMethod()
}
`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry([]string{"deprecated-api-usage"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestDeprecatedAPIUsageReportsImportedGenericFields(t *testing.T) {
	root := analysisModule(t, `package sample

import "example.com/analysis/legacy"

func use(value legacy.Generic[int]) int {
	return value.OldField
}
`)
	legacy := filepath.Join(root, "legacy")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(legacy, "legacy.go"),
		[]byte(`package legacy

type Generic[T any] struct {
	// Deprecated: use NewField instead.
	OldField T
}
`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry([]string{"deprecated-api-usage"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestDeprecatedAPIUsageFollowsPhysicalFilesForLineDirectives(t *testing.T) {
	root := analysisModule(t, `package sample

import "example.com/analysis/legacy"

func use() {
	legacy.Old()
}
`)
	legacy := filepath.Join(root, "legacy")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "legacy.go"), []byte(`package legacy

// Deprecated: use New instead.
//line legacy.schema:400
func Old() {}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry([]string{"deprecated-api-usage"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestDeprecatedAPIUsageReadsStandardLibraryMarkers(t *testing.T) {
	root := analysisModule(t, `package sample

import "io/ioutil"

func read() {
	_, _ = ioutil.ReadAll(nil)
}
`)
	registry, err := NewRegistry([]string{"deprecated-api-usage"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestInvalidListenAddressReportsInvalidConstants(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "net/http"

func serve(handler http.Handler, dynamic string) {
	http.ListenAndServe("localhost", handler)
	http.ListenAndServe(":70000", handler)
	http.ListenAndServe(":8080", handler)
	http.ListenAndServe(dynamic, handler)
}
`,
	)
	registry, err := NewRegistry([]string{"invalid-listen-address"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestValidListenPortAcceptsHyphenatedServiceNames(t *testing.T) {
	for port, want := range map[string]bool{"http-alt": true, "x11-1": true, "-http": false, "http-": false, "http--alt": false, "123-456": false} {
		if got := validListenPort(port); got != want {
			t.Errorf("validListenPort(%q) = %t, want %t", port, got, want)
		}
	}
}

func TestIPByteComparisonReportsTwoIPValues(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"bytes"
	"net"
)

func equal(left, right net.IP, raw []byte) bool {
	bad := bytes.Equal(left, right)
	_ = bytes.Equal(left, raw)
	good := left.Equal(right)
	return bad || good
}
`,
	)
	registry, err := NewRegistry([]string{"ip-byte-comparison"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestWriterBufferMutationReportsStoreAndAppend(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type writer struct{}

func (*writer) Write(buffer []byte) (int, error) {
	buffer[0] = 0
	buffer = append(buffer, 1)
	return len(buffer), nil
}

func (*writer) Other(buffer []byte) {
	buffer[0] = 0
}
`,
	)
	registry, err := NewRegistry([]string{"writer-buffer-mutation"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestDuplicateTrimCutsetReportsRepeatedRunes(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "strings"

const prefix = "letter"

func trim(value, dynamic string) {
	strings.TrimLeft(value, prefix)
	strings.Trim(value, "abc")
	strings.TrimRight(value, dynamic)
}
`,
	)
	registry, err := NewRegistry([]string{"duplicate-trim-cutset"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestTimerResetDrainRaceReportsConditionalDrain(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "time"

func reset(timer *time.Timer, delay time.Duration) {
	if !timer.Reset(delay) {
		<-timer.C
	}
	timer.Reset(delay)
}
`,
	)
	registry, err := NewRegistry([]string{"timer-reset-drain-race"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestUnsupportedMarshalTypeReportsNestedUnsupportedValues(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"encoding/json"
	"encoding/xml"
)

type payload struct {
	Callback func()
	Ignored chan int `+"`json:\"-\" xml:\"-\"`"+`
}

type custom chan int

func (custom) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

func encode(value payload, allowed custom) {
	json.Marshal(value)
	xml.Marshal(value)
	json.Marshal(allowed)
}
`,
	)
	registry, err := NewRegistry([]string{"unsupported-marshal-type"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestMisalignedAtomic64ReportsOn32BitTargets(t *testing.T) {
	t.Setenv("GOOS", "linux")
	t.Setenv("GOARCH", "386")
	root := analysisModule(
		t,
		`package sample

import "sync/atomic"

type counters struct {
	ready uint32
	total uint64
}

func add(value *counters) {
	atomic.AddUint64(&value.total, 1)
}
`,
	)
	registry, err := NewRegistry([]string{"misaligned-atomic-64"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestSortNonSliceReportsConcreteNonSliceValues(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "sort"

func order(array [3]int, slice []int, dynamic any) {
	less := func(left, right int) bool { return left < right }
	sort.Slice(array, less)
	sort.Slice(nil, less)
	sort.Slice(slice, less)
	sort.Slice(dynamic, less)
}
`,
	)
	registry, err := NewRegistry([]string{"sort-non-slice"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestContextKeyTypeReportsUnsafeKeys(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "context"

type key struct{}
type badKey []byte

func values(ctx context.Context) {
	context.WithValue(ctx, "name", 1)
	context.WithValue(ctx, struct{}{}, 1)
	context.WithValue(ctx, badKey(nil), 1)
	context.WithValue(ctx, key{}, 1)
}
`,
	)
	registry, err := NewRegistry([]string{"context-key-type"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 3 {
		t.Fatalf("got %d diagnostics, want 3: %#v", len(diagnostics), diagnostics)
	}
}

func TestInvalidStrconvArgumentReportsInvalidConstants(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "strconv"

func parse(value string, dynamic int) {
	strconv.ParseInt(value, 1, 128)
	strconv.ParseFloat(value, 16)
	strconv.FormatFloat(1, 'q', -1, 64)
	strconv.ParseInt(value, 10, 64)
	strconv.ParseInt(value, dynamic, 64)
}
`,
	)
	registry, err := NewRegistry([]string{"invalid-strconv-argument"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 4 {
		t.Fatalf("got %d diagnostics, want 4: %#v", len(diagnostics), diagnostics)
	}
}

func TestOverlappingEncodeBuffersReportsSharedStarts(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"encoding/base64"
	"encoding/hex"
)

func encode(buffer, other []byte) {
	hex.Encode(buffer, buffer)
	base64.StdEncoding.Encode(buffer[:], buffer[:])
	hex.Encode(buffer, other)
	hex.Encode(buffer[1:], buffer[:1])
}
`,
	)
	registry, err := NewRegistry([]string{"overlapping-encode-buffers"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestSwappedErrorsIsArgumentsReportsExternalSentinelFirst(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"errors"
	"io"
)

func match(err error) bool {
	bad := errors.Is(io.EOF, err)
	good := errors.Is(err, io.EOF)
	bothSentinels := errors.Is(io.EOF, errors.ErrUnsupported)
	return bad || good || bothSentinels
}
`,
	)
	registry, err := NewRegistry([]string{"swapped-errors-is-arguments"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestWaitGroupAddInsideGoroutineReportsRacyAdd(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "sync"

func start(group *sync.WaitGroup) {
	go func() {
		group.Add(1)
		defer group.Done()
	}()
	group.Add(1)
	go func() { defer group.Done() }()
}
`,
	)
	registry, err := NewRegistry([]string{"waitgroup-add-inside-goroutine"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestEmptyCriticalSectionReportsAdjacentUnlock(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "sync"

func lock(mutex *sync.RWMutex) {
	mutex.Lock()
	mutex.Unlock()
	mutex.RLock()
	use()
	mutex.RUnlock()
}

func use() {}
`,
	)
	registry, err := NewRegistry([]string{"empty-critical-section"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestTestingFatalInGoroutineReportsChildTermination(t *testing.T) {
	root := analysisModule(t, `package sample

import "testing"

func TestWork(t *testing.T) {
	go func() { t.Fatal("failed") }()
	t.Log("running")
}
`)
	registry, err := NewRegistry([]string{"testing-fatal-in-goroutine"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestDeferredLockAfterLockReportsRepeatedLock(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "sync"

func lock(mutex *sync.Mutex, rw *sync.RWMutex) {
	mutex.Lock()
	defer mutex.Lock()
	rw.RLock()
	defer rw.RUnlock()
}
`,
	)
	registry, err := NewRegistry([]string{"deferred-lock-after-lock"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestTestMainMissingExitReportsLegacyModules(t *testing.T) {
	root := analysisModuleVersion(t, "1.14", `package sample

import "testing"

func TestMain(m *testing.M) {
	m.Run()
}
`)
	registry, err := NewRegistry([]string{"test-main-missing-exit"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestBenchmarkIterationMutationReportsAssignment(t *testing.T) {
	root := analysisModule(t, `package sample

import "testing"

func BenchmarkWork(b *testing.B) {
	b.N = 1000
}
`)
	registry, err := NewRegistry([]string{"benchmark-iteration-mutation"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestIdenticalBinaryOperandsReportsSuspiciousSelfComparison(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func compare(value int, floating float64) bool {
	bad := value == value
	allowedFloat := floating != floating
	return bad || allowedFloat
}
`,
	)
	registry, err := NewRegistry([]string{"identical-binary-operands"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestCommandModuleEndingDotTestIsAnalyzed(t *testing.T) {
	root := analysisModule(t, `package main

func compare(value int) bool { return value == value }
`)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/analysis.test\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry([]string{"identical-binary-operands"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestImpossibleIntegerComparisonReportsTypeRangeFacts(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func compare(value uint8) bool {
	belowZero := value < 0
	atLeastZero := value >= 0
	aboveMaximum := value > 255
	return belowZero || atLeastZero || aboveMaximum
}
`,
	)
	registry, err := NewRegistry([]string{"impossible-integer-comparison"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 3 {
		t.Fatalf("got %d diagnostics, want 3: %#v", len(diagnostics), diagnostics)
	}
}

func TestSingleIterationLoopReportsUnconditionalFirstExit(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func first(values []int) int {
	for _, value := range values {
		if value < 0 {
			use(value)
		}
		return value
	}
	return 0
}

func use(int) {}
`,
	)
	registry, err := NewRegistry([]string{"single-iteration-loop"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestIneffectiveValueReceiverAssignmentReportsUnreadFieldWrite(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type item struct {
	name string
}

func (value item) rename(name string) {
	value.name = name
}

func (value item) renamed(name string) string {
	value.name = name
	return value.name
}

func (value *item) renameInPlace(name string) {
	value.name = name
}
`,
	)
	registry, err := NewRegistry([]string{"ineffective-value-receiver-assignment"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestOverwrittenBeforeUseReportsDeadAssignedValue(t *testing.T) {
	root := analysisModule(t, `package sample

func calculate() int { return 1 }

func result() int {
	value := calculate()
	value = calculate()
	return value
}
`)
	registry, err := NewRegistry([]string{"overwritten-before-use"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestUnchangedLoopConditionReportsWrongCounter(t *testing.T) {
	root := analysisModule(t, `package sample

func loop(limit int) {
	other := 0
	for index := 0; index < limit; other++ {
		if other > limit {
			return
		}
	}
}
`)
	registry, err := NewRegistry([]string{"unchanged-loop-condition"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestArgumentOverwrittenBeforeUseReportsUnusedIncomingValue(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func replaced(value string) string {
	value = "replacement"
	return value
}

func observed(value string) string {
	use(value)
	value = "replacement"
	return value
}

func use(string) {}
`,
	)
	registry, err := NewRegistry([]string{"argument-overwritten-before-use"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestUnusedAppendResultReportsUnobservedLocalSlice(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func appendAndForget() {
	values := make([]int, 0, 1)
	values = append(values, 1)
}

func appendAndReturn() []int {
	values := make([]int, 0, 1)
	values = append(values, 1)
	return values
}
`,
	)
	registry, err := NewRegistry([]string{"unused-append-result"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestNaNComparisonReportsDirectComparison(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "math"

func isMissing(value float64) bool {
	return value == math.NaN()
}

func isMissingCorrectly(value float64) bool {
	return math.IsNaN(value)
}
`,
	)
	registry, err := NewRegistry([]string{"nan-comparison"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestPointlessIntegerMathReportsConvertedInteger(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "math"

func rounded(count int) float64 {
	return math.Ceil(float64(count))
}

func roundedMeasurement(value float64) float64 {
	return math.Ceil(value)
}
`,
	)
	registry, err := NewRegistry([]string{"pointless-integer-math"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestIneffectiveBitwiseZeroReportsFixedResults(t *testing.T) {
	root := analysisModule(t, `package sample

const missingFlag = iota

func bits(value uint) uint {
	left := value & 0
	right := value ^ missingFlag
	return left | right
}
`)
	registry, err := NewRegistry([]string{"ineffective-bitwise-zero"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestDiscardedPureResultReportsIgnoredCall(t *testing.T) {
	root := analysisModule(t, `package sample

func square(value int) int { return value * value }

func ignored() {
	square(2)
}

func observed() int {
	return square(2)
}
`)
	registry, err := NewRegistry([]string{"discarded-pure-result"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestSelfAssignmentReportsSideEffectFreeIdentity(t *testing.T) {
	root := analysisModule(t, `package sample

func assign(value int, values []int, next func() int) {
	value = value
	values[next()] = values[next()]
}
`)
	registry, err := NewRegistry([]string{"self-assignment"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestUnreachableTypeSwitchCaseReportsSubsumedInterface(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type reader interface { Read([]byte) (int, error) }
type readCloser interface {
	reader
	Close() error
}

func classify(value any) {
	switch value.(type) {
	case reader:
	case readCloser:
	}
}
`,
	)
	registry, err := NewRegistry([]string{"unreachable-type-switch-case"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestSingleArgumentAppendReportsBuiltinCall(t *testing.T) {
	root := analysisModule(t, `package sample

func unchanged(source []int) []int {
	return append(source)
}
`)
	registry, err := NewRegistry([]string{"single-argument-append"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
	if len(diagnostics[0].Fixes) != 1 || !diagnostics[0].Fixes[0].Automatic || diagnostics[0].Fixes[0].Safety != diagnostic.Safe || len(diagnostics[0].Fixes[0].Edits) != 1 {
		t.Fatalf("automatic fix = %#v", diagnostics[0].Fixes)
	}
	contents, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	edit := diagnostics[0].Fixes[0].Edits[0]
	if got := string(contents[edit.Start:edit.End]); got != "append" {
		t.Fatalf("fix deletes %q, want append", got)
	}
}

func TestSingleArgumentAppendDoesNotFixCommentedCall(t *testing.T) {
	root := analysisModule(t, `package sample

func unchanged(source []int) []int {
	return append /* keep */ (source)
}
`)
	registry, err := NewRegistry([]string{"single-argument-append"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || len(diagnostics[0].Fixes) != 0 {
		t.Fatalf("commented diagnostics = %#v", diagnostics)
	}
}

func TestAddressNilComparisonReportsFixedResult(t *testing.T) {
	root := analysisModule(t, `package sample

func impossible(value int, pointer *int) bool {
	bad := &value == nil
	allowed := &*pointer == nil
	return bad || allowed
}
`)
	registry, err := NewRegistry([]string{"address-nil-comparison"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestImpossibleInterfaceNilComparisonReportsTypedNil(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type problem struct{}
func (*problem) Error() string { return "problem" }

func typedNil() error {
	var value *problem
	return value
}

func impossible() bool {
	return typedNil() == nil
}
`,
	)
	registry, err := NewRegistry([]string{"impossible-interface-nil-comparison"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestNegativeLengthCapacityComparisonReportsImpossibleCheck(t *testing.T) {
	root := analysisModule(t, `package sample

func impossible(values []int) bool {
	return len(values) < 0 || 0 > cap(values)
}
`)
	registry, err := NewRegistry([]string{"negative-length-capacity-comparison"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestConstantNegativeZeroReportsNormalizedLiterals(t *testing.T) {
	root := analysisModule(t, `package sample

var direct = -0.0
var converted = -float64(0)
var convertedAfter = float32(-0)
`)
	registry, err := NewRegistry([]string{"constant-negative-zero"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 3 {
		t.Fatalf("got %d diagnostics, want 3: %#v", len(diagnostics), diagnostics)
	}
}

func TestURLQueryCopyMutationReportsTemporaryMapChange(t *testing.T) {
	root := analysisModule(t, `package sample

import "net/url"

func update(address *url.URL) {
	address.Query().Set("mode", "fast")
}
`)
	registry, err := NewRegistry([]string{"url-query-copy-mutation"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestSortConversionWithoutSortReportsIneffectiveAssignment(t *testing.T) {
	root := analysisModule(t, `package sample

import "sort"

func order(values []int) {
	values = sort.IntSlice(values)
}
`)
	registry, err := NewRegistry([]string{"sort-conversion-without-sort"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestRandomBoundOneReportsConstantZeroRange(t *testing.T) {
	root := analysisModule(t, `package sample

import "math/rand"

func choice() int {
	return rand.Intn(1)
}
`)
	registry, err := NewRegistry([]string{"random-bound-one"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestNeverNilComparisonReportsMadeSliceCheck(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func impossible() bool {
	values := make([]int, 0)
	if values == nil {
		return true
	}
	return false
}

func possible() bool {
	var values []int
	return values == nil
}
`,
	)
	registry, err := NewRegistry([]string{"never-nil-comparison"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestImpossiblePlatformComparisonReportsExcludedTarget(t *testing.T) {
	excluded := "windows"
	if runtime.GOOS == excluded {
		excluded = "linux"
	}
	root := analysisModule(t, fmt.Sprintf(`//go:build %s

package sample

import "runtime"

func impossible() bool {
	return runtime.GOOS == %q
}
`, runtime.GOOS, excluded))
	registry, err := NewRegistry([]string{"impossible-platform-comparison"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestNilMapAssignmentReportsPanickingWrite(t *testing.T) {
	root := analysisModule(t, `package sample

func write() {
	var values map[string]int
	values["answer"] = 42
}
`)
	registry, err := NewRegistry([]string{"nil-map-assignment"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestDeferCloseBeforeErrorCheckReportsEarlyDefer(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type resource struct{}
func (*resource) Close() error { return nil }
func open() (*resource, error) { return nil, nil }

func use() error {
	resource, err := open()
	defer resource.Close()
	if err != nil {
		return err
	}
	return nil
}
`,
	)
	registry, err := NewRegistry([]string{"defer-close-before-error-check"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestSpinningEmptyLoopReportsUnsafeWait(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func spin(ready *bool) {
	for !*ready {}
}

func dynamic(ready func() bool) {
	for !ready() {}
}

func disabled() {
	for false {}
}
`,
	)
	registry, err := NewRegistry([]string{"spinning-empty-loop"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestFinalizerCapturesObjectReportsRetainedObject(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "runtime"

type resource struct{}

func leaking() {
	object := &resource{}
	runtime.SetFinalizer(object, func(*resource) {
		_ = object
	})
}

func clean() {
	object := &resource{}
	runtime.SetFinalizer(object, func(object *resource) {
		_ = object
	})
}
`,
	)
	registry, err := NewRegistry([]string{"finalizer-captures-object"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestInfiniteRecursionReportsCallWithoutExitPath(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func infinite() {
	infinite()
}

func conditional(done bool) {
	if done {
		return
	}
	conditional(done)
}

func spawned() {
	go spawned()
}
`,
	)
	registry, err := NewRegistry([]string{"infinite-recursion"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestInvalidPrintfCallReportsFormatAndArgumentMismatches(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "fmt"

func invalid() {
	fmt.Printf("%d", "wrong")
	fmt.Printf("%d")
	fmt.Printf("%*s", "wide", "value")
	fmt.Printf("plain", 1)
	fmt.Printf("%z", 1)
}

func clean() {
	fmt.Printf("%d %s", 1, "value")
	fmt.Printf("%[2]d", "unused", 2)
	fmt.Printf("%*s", 4, "value")
}
`,
	)
	registry, err := NewRegistry([]string{"invalid-printf-call"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 5 {
		t.Fatalf("got %d diagnostics, want 5: %#v", len(diagnostics), diagnostics)
	}
}

func TestContradictoryInterfaceAssertionReportsConflictingMethod(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type source interface { Read() int }
type impossible interface { Read() string }
type compatible interface { Write() string }

func inspect(value source) {
	_, _ = value.(impossible)
	_, _ = value.(compatible)
}

`,
	)
	registry, err := NewRegistry([]string{"contradictory-interface-assertion"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestPossibleNilDereferenceReportsUnprotectedPath(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func unsafe(value *int, report func()) int {
	if value == nil {
		report()
	}
	return *value
}

func guarded(value *int) int {
	if value != nil {
		return *value
	}
	return 0
}

func terminating(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
`,
	)
	registry, err := NewRegistry([]string{"possible-nil-dereference"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestOddPairedArgumentsReportsKnownOddLengths(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "strings"

func pairs(values ...string) {
	if len(values)%2 != 0 {
		panic("pairs required")
	}
}

func calls() {
	strings.NewReplacer("a", "b", "orphan")
	pairs("a", "b", "orphan")
	strings.NewReplacer("a", "b")
	pairs("a", "b")
}
`,
	)
	registry, err := NewRegistry([]string{"odd-paired-arguments"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestRegexpMatchInLoopReportsRepeatedCompilation(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "regexp"

func repeated(values []string, dynamic string) {
	for _, value := range values {
		regexp.MatchString("^[a-z]+$", value)
		regexp.MatchString(dynamic, value)
	}
	regexp.MatchString("^[a-z]+$", "outside")
}
`,
	)
	registry, err := NewRegistry([]string{"regexp-match-in-loop"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestSeparateByteStringMapKeyReportsLookupOnlyTemporary(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func allocated(items map[string]int, bytes []byte) int {
	key := string(bytes)
	return items[key] + items[key]
}

func direct(items map[string]int, bytes []byte) int {
	return items[string(bytes)]
}

func escaped(items map[string]int, bytes []byte) string {
	key := string(bytes)
	_ = items[key]
	return key
}
`,
	)
	registry, err := NewRegistry([]string{"separate-byte-string-map-key"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestNonPointerSyncPoolValueReportsBoxedValues(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "sync"

func store(pool *sync.Pool) {
	bytes := []byte("value")
	pool.Put(bytes)
	pool.Put(42)
	pool.Put(&bytes)
	pool.Put(map[string]int{})
}
`,
	)
	registry, err := NewRegistry([]string{"non-pointer-sync-pool-value"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestCaseInsensitiveStringComparisonReportsAllocatingComparison(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "strings"

func equal(left, right string) bool {
	return strings.ToLower(left) == strings.ToLower(right)
}

func differentConversions(left, right string) bool {
	return strings.ToLower(left) == strings.ToUpper(right)
}

func efficient(left, right string) bool {
	return strings.EqualFold(left, right)
}
`,
	)
	registry, err := NewRegistry([]string{"case-insensitive-string-comparison"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestByteStringWriteReportsAllocatingConversion(t *testing.T) {
	root := analysisModule(t, `package sample

import (
	"io"
	"os"
)

func write(bytes []byte) {
	io.WriteString(os.Stdout, string(bytes))
	os.Stdout.Write(bytes)
}
`)
	registry, err := NewRegistry([]string{"byte-string-write"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestDecimalFileModeReportsMissingOctalPrefix(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "os"

func write(path string, data []byte) {
	os.WriteFile(path, data, 644)
	os.WriteFile(path, data, 0o644)
	println(644)
}
`,
	)
	registry, err := NewRegistry([]string{"decimal-file-mode"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestPartiallyTypedConstantGroupReportsDefaultedTypes(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type kind int

const (
	first kind = 1
	second = 2
	third = 3
)

const (
	one kind = 1
	two kind = 2
)

const (
	initial kind = iota
	next
)
`,
	)
	registry, err := NewRegistry([]string{"partially-typed-constant-group"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestUnexportedSerializationFieldsReportsInvisibleData(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "encoding/json"

type private struct { value string }
type public struct { Value string }
type empty struct{}
type custom struct { value string }
func (custom) MarshalJSON() ([]byte, error) { return nil, nil }

func encode() {
	json.Marshal(private{})
	json.Marshal(public{})
	json.Marshal(empty{})
	json.Marshal(custom{})
	var destination private
	json.Unmarshal(nil, &destination)
}
`,
	)
	registry, err := NewRegistry([]string{"unexported-serialization-fields"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestOversizedFixedWidthShiftReportsClearedValue(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func shifts(value uint8, machine uint) uint8 {
	cleared := value << 8
	value >>= 9
	_ = machine << 64
	_ = value << 7
	return cleared
}
`,
	)
	registry, err := NewRegistry([]string{"oversized-fixed-width-shift"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestDangerousDirectoryRemovalReportsWholeDirectory(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"os"
	"path/filepath"
)

func remove() {
	temporary := os.TempDir()
	os.RemoveAll(temporary)
	home, _ := os.UserHomeDir()
	os.RemoveAll(home)
	os.RemoveAll(filepath.Join(temporary, "application"))
}
`,
	)
	registry, err := NewRegistry([]string{"dangerous-directory-removal"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestFailedAssertionShadowReadReportsZeroValueRead(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "fmt"

func inspect(value any) {
	if value, ok := value.(int); ok {
		fmt.Println(value)
	} else {
		fmt.Printf("unexpected type %T", value)
	}
	if value, ok := value.(int); ok {
		fmt.Println(value)
	} else {
		value = 0
		fmt.Println(value)
	}
}
`,
	)
	registry, err := NewRegistry([]string{"failed-assertion-shadow-read"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestDeferredReturnFunctionNotCalledReportsMissingInvocation(t *testing.T) {
	root := analysisModule(t, `package sample

func setup() func() { return func() {} }

func run() {
	defer setup()
	defer setup()()
}
`)
	registry, err := NewRegistry([]string{"deferred-return-function-not-called"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestDurationMultipliedByDurationReportsSquaredUnits(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "time"

func scale(duration time.Duration, count int) {
	_ = duration * time.Second
	_ = time.Duration(count) * time.Second
	_ = duration * 2
	_ = (1 + time.Duration(count)) * time.Millisecond
	_ = (duration / time.Second) * time.Second
	_ = (duration + time.Second) * time.Second
}
`,
	)
	registry, err := NewRegistry([]string{"duration-multiplied-by-duration"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestContextStoredInStructReportsContextField(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "context"

type stored struct {
	ctx context.Context
}

type explicit struct {
	name string
}

func (explicit) Run(ctx context.Context) { _ = ctx }
`,
	)
	registry, err := NewRegistry([]string{"context-stored-in-struct"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestUnsafeFormattedURLHostPortReportsIPv6UnsafeFormat(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"fmt"
	"net"
	"strconv"
)

func urls(host string, port int) {
	_ = fmt.Sprintf("http://%s:%d/path", host, port)
	_, _ = net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	_ = fmt.Sprintf("%s:%d", host, port)
	_ = fmt.Sprintf("%s:%d", "file.go", 42)
	_ = "http://" + net.JoinHostPort(host, strconv.Itoa(port))
}
`,
	)
	registry, err := NewRegistry([]string{"unsafe-formatted-url-host-port"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %#v", len(diagnostics), diagnostics)
	}
}

func TestUncheckedRowsErrorReportsMissingIterationCheck(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "database/sql"

func bad(rows *sql.Rows) {
	for rows.Next() {}
}

func good(rows *sql.Rows) error {
	for rows.Next() {}
	return rows.Err()
}
`,
	)
	registry, err := NewRegistry([]string{"unchecked-rows-error"})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %#v", len(diagnostics), diagnostics)
	}
}

func TestRegistryRejectsUnknownRule(t *testing.T) {
	if _, err := NewRegistry([]string{"missing-analyzer"}); err == nil || !strings.Contains(err.Error(), "missing-analyzer") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEveryAnalyzerAcceptsCommonConfiguration(t *testing.T) {
	settings := make(map[string]config.RuleConfig, len(allRules()))
	for _, rule := range allRules() {
		settings[rule.Meta().Code] = config.RuleConfig{Severity: "note"}
	}
	registry, err := NewRegistryConfigured(nil, settings, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(registry.Rules()), len(allRules()); got != want {
		t.Fatalf("configured %d analyzers; want %d", got, want)
	}
	for _, rule := range registry.Rules() {
		if severity := registry.Severity(rule.Meta().Code); severity != diagnostic.SeverityNote {
			t.Errorf("%s severity = %s", rule.Meta().Code, severity)
		}
	}
}

func TestAnalyzerRegistryFiltersByEffectiveSeverityBeforePlanning(t *testing.T) {
	registry, err := NewRegistryWithOptions(
		RegistryOptions{
			Only:            []string{"regexp-match-in-loop", "invalid-template"},
			Settings:        map[string]config.RuleConfig{"regexp-match-in-loop": {Severity: "warning"}, "invalid-template": {Severity: "error"}},
			MinimumSeverity: diagnostic.SeverityError,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(registry.Rules()); got != 1 || registry.Rules()[0].Meta().Code != "invalid-template" {
		t.Fatalf("filtered analyzers = %#v, want invalid-template", registry.Rules())
	}
	if registry.executionPlan().needsSSA() {
		t.Fatal("filtered SSA analyzer still affected the execution plan")
	}

	registry, err = NewRegistryWithOptions(
		RegistryOptions{
			Only:            []string{"regexp-match-in-loop", "invalid-template"},
			Settings:        map[string]config.RuleConfig{"regexp-match-in-loop": {Severity: "error"}, "invalid-template": {Severity: "warning"}},
			MinimumSeverity: diagnostic.SeverityError,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(registry.Rules()); got != 1 || registry.Rules()[0].Meta().Code != "regexp-match-in-loop" {
		t.Fatalf("overridden analyzers = %#v, want regexp-match-in-loop", registry.Rules())
	}
	if !registry.executionPlan().needsSSA() {
		t.Fatal("included SSA analyzer was omitted from the execution plan")
	}
}

func TestAnalyzerRegistryRejectsInvalidMinimumSeverity(t *testing.T) {
	_, err := NewRegistryWithOptions(RegistryOptions{MinimumSeverity: "fatal"})
	if err == nil || !strings.Contains(err.Error(), "minimum severity") {
		t.Fatalf("got %v, want minimum severity error", err)
	}
	_, err = NewRegistryWithOptions(RegistryOptions{Settings: map[string]config.RuleConfig{"invalid-template": {Severity: "fatal"}}})
	if err == nil || !strings.Contains(err.Error(), "severity must be") {
		t.Fatalf("got %v, want rule severity error", err)
	}
}

func TestAnalyzerRegistrySkipsLoadingWhenSeverityFilterIsEmpty(t *testing.T) {
	registry, err := NewRegistryWithOptions(
		RegistryOptions{
			Only:            []string{"suspicious-sleep"},
			Settings:        map[string]config.RuleConfig{"suspicious-sleep": {Severity: "warning"}},
			MinimumSeverity: diagnostic.SeverityError,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("not Go source"), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{root}, registry)
	if err != nil {
		t.Fatalf("empty registry attempted package loading: %v", err)
	}
	if diagnostics == nil || len(diagnostics) != 0 {
		t.Fatalf("empty registry diagnostics = %#v, want non-nil empty slice", diagnostics)
	}
}

func analysisModule(t *testing.T, source string) string {
	return analysisModuleVersion(t, "1.26", source)
}

func analysisModuleVersion(t *testing.T, goVersion, source string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/analysis\n\ngo "+goVersion+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		restoreSemanticWorkingDirectory(t, previous)
	})
	return root
}
