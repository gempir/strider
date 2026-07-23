package semantic

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

func TestRunContextRejectsPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RunContext(ctx, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("RunContext error = %v, want context.Canceled", err)
	}
}

func TestCheckRequirementsCoverCatalog(t *testing.T) {
	seen := make(map[string]bool, len(checkCatalog))
	codes := make([]string, 0, len(checkCatalog))
	for _, check := range checkCatalog {
		code := check.Meta().Code
		if seen[code] {
			t.Fatalf("duplicate check %q", code)
		}
		seen[code] = true
		codes = append(codes, code)
		requirements := check.requirements
		switch requirements.Stage {
		case AnalysisStageTypes:
		case AnalysisStageSSA:
		default:
			t.Fatalf("check %q has invalid stage %d", code, requirements.Stage)
		}
		if requirements.Facts.Has(FactStaticCalls) != (len(requirements.staticCallPackages) != 0) {
			t.Fatalf("check %q has inconsistent static-call requirements", code)
		}
	}
	sort.Strings(codes)
	_, testFile, _, _ := runtime.Caller(0)
	goldenPath := filepath.Join(filepath.Dir(testFile), "testdata", "check_codes.txt")
	got := strings.Join(codes, "\n") + "\n"
	if os.Getenv("STRIDER_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("check catalog differs from testdata/check_codes.txt\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestExecutionPlanSelectsNamedFacts(t *testing.T) {
	tests := []struct {
		code  string
		facts FactSet
	}{
		{
			code:  "invalid-regexp",
			facts: FactCallArguments | FactStaticCalls,
		},
		{
			code:  "unsupported-binary-write",
			facts: FactCallArguments | FactStaticCalls,
		},
		{
			code:  "identical-binary-operands",
			facts: FactParents,
		},
		{
			code:  "range-value-capture",
			facts: FactParents,
		},
		{
			code:  "deprecated-api-usage",
			facts: FactDeprecations,
		},
		{
			code:  "invalid-template",
			facts: 0,
		},
	}
	for _, test := range tests {
		t.Run(
			test.code,
			func(t *testing.T) {
				registry, err := newRegistry([]string{
					test.code,
				})
				if err != nil {
					t.Fatal(err)
				}
				if got := registry.executionPlan().requirements.Facts; got != test.facts {
					t.Fatalf("got facts %d, want %d", got, test.facts)
				}
			},
		)
	}
}

func TestExecutionPlanSelectsStaticCallPackages(t *testing.T) {
	registry, err := newRegistry([]string{
		"invalid-regexp",
		"unsupported-marshal-type",
	})
	if err != nil {
		t.Fatal(err)
	}
	packages := registry.executionPlan().staticCallPackages
	if len(packages) != 3 || !packages["regexp"] || !packages["encoding/json"] || !packages["encoding/xml"] {
		t.Fatalf("unexpected static-call package selection: %#v", packages)
	}
}

func TestStaticCallFactsInitializeOnce(t *testing.T) {
	facts := newPackageFacts(FactStaticCalls)
	var builds atomic.Int32
	facts.ssaBuilder = func(_ []*ssa.Function, required FactSet) packageSSAFactData {
		builds.Add(1)
		if !required.Has(FactStaticCalls) {
			t.Fatal("static-call capability missing from fact builder")
		}
		return packageSSAFactData{
			staticCallsByPackage: map[string][]ssa.CallInstruction{
				"strings": {
					nil,
				},
			},
		}
	}
	pass := &Pass{
		facts: facts,
	}
	var group sync.WaitGroup
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			if got := len(pass.staticCallsInPackage("strings")); got != 1 {
				t.Errorf("static-call index has %d entries, want 1", got)
			}
		}()
	}
	group.Wait()
	if got := builds.Load(); got != 1 {
		t.Fatalf("SSA facts built %d times, want 1", got)
	}
}

func TestTypedOnlyPlanDoesNotBuildSSA(t *testing.T) {
	root := analysisModule(t, `package sample

func valid() string { return "ok" }
`)
	registry, err := newRegistry([]string{
		"invalid-template",
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	_, err = run([]string{
		root,
	}, registry, func([]*packages.Package, ssa.BuilderMode) ssaBuildResult {
		calls++
		return ssaBuildResult{}
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("typed-only plan built SSA %d times", calls)
	}
}

func TestSSADebugMetadataIsCapabilityDriven(t *testing.T) {
	tests := []struct {
		code        string
		globalDebug bool
	}{
		{
			code: "invalid-regexp",
		},
		{
			code:        "overwritten-before-use",
			globalDebug: true,
		},
	}
	for _, test := range tests {
		t.Run(
			test.code,
			func(t *testing.T) {
				registry, err := newRegistry([]string{
					test.code,
				})
				if err != nil {
					t.Fatal(err)
				}
				calls := 0
				prepareSSA(
					nil,
					registry.executionPlan(),
					func(_ []*packages.Package, mode ssa.BuilderMode) ssaBuildResult {
						calls++
						if got := mode&ssa.GlobalDebug != 0; got != test.globalDebug {
							t.Fatalf("GlobalDebug = %t, want %t", got, test.globalDebug)
						}
						return ssaBuildResult{}
					},
				)
				if calls != 1 {
					t.Fatalf("SSA builder called %d times, want 1", calls)
				}
			},
		)
	}
}

func TestNamedPackageFactsInitializeOnce(t *testing.T) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "fixture.go", "package fixture\nfunc f() { g(1, 2) }\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	required := FactCallArguments | FactParents
	facts := newPackageFacts(required)
	var builds atomic.Int32
	facts.builder = func(files []*ast.File, required FactSet) packageFactData {
		builds.Add(1)
		return buildPackageFacts(files, required)
	}
	pass := &Pass{
		Files: []*ast.File{
			file,
		},
		facts: facts,
	}
	var group sync.WaitGroup
	for index := range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			switch index % 3 {
			case 0:
				_ = pass.argumentsByCallPosition()
			case 1:
				for position := range pass.argumentsByCallPosition() {
					_ = pass.firstArgumentByCallPosition(position)
					break
				}
			default:
				_ = pass.analysisParents()
			}
		}()
	}
	group.Wait()
	if got := builds.Load(); got != 1 {
		t.Fatalf("package facts built %d times, want 1", got)
	}
	if len(pass.argumentsByCallPosition()) != 2 {
		t.Fatalf("call argument index has %d entries, want 2", len(pass.argumentsByCallPosition()))
	}
	firstArguments := 0
	for position := range pass.argumentsByCallPosition() {
		if pass.firstArgumentByCallPosition(position) != nil {
			firstArguments++
		}
	}
	if firstArguments != 2 {
		t.Fatalf("first call argument index has %d entries, want 2", firstArguments)
	}
	if len(pass.analysisParents()) == 0 {
		t.Fatal("parent index is empty")
	}
}
