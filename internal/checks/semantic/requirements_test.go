package semantic

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sync"
	"sync/atomic"
	"testing"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

func TestRuleRequirementsCoverCatalog(t *testing.T) {
	seen := make(map[string]bool, len(ruleCatalog))
	typed := 0
	ssaRules := 0
	for _, definition := range ruleCatalog {
		code := definition.rule.Meta().Code
		if seen[code] {
			t.Fatalf("duplicate rule %q", code)
		}
		seen[code] = true
		requirements, ok := RequirementsFor(code)
		if !ok {
			t.Fatalf("rule %q has no requirements", code)
		}
		switch requirements.Stage {
		case AnalysisStageTypes:
			typed++
		case AnalysisStageSSA:
			ssaRules++
		default:
			t.Fatalf("rule %q has invalid stage %d", code, requirements.Stage)
		}
		if UsesSSA(code) != (requirements.Stage == AnalysisStageSSA) {
			t.Fatalf("UsesSSA(%q) disagrees with its requirements", code)
		}
	}
	if typed != 66 || ssaRules != 44 {
		t.Fatalf("got %d typed and %d SSA rules, want 66 and 44", typed, ssaRules)
	}
}

func TestExecutionPlanSelectsNamedFacts(t *testing.T) {
	tests := []struct {
		code string
		facts FactSet
	}{
		{code: "invalid-regexp", facts: FactFirstCallArgument | FactStaticCalls},
		{code: "unsupported-binary-write", facts: FactCallArguments | FactStaticCalls},
		{code: "identical-binary-operands", facts: FactParents},
		{code: "deprecated-api-usage", facts: FactDeprecations},
		{code: "invalid-template", facts: 0},
	}
	for _, test := range tests {
		t.Run(
			test.code,
			func(t *testing.T) {
				registry,
				err := NewRegistry([]string{test.code})
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
	registry, err := NewRegistry([]string{"invalid-regexp", "unsupported-marshal-type"})
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
		return packageSSAFactData{staticCallsByPackage: map[string][]ssa.CallInstruction{"strings": {nil}}}
	}
	pass := &Pass{facts: facts}
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
	registry, err := NewRegistry([]string{"invalid-template"})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	_, err = run([]string{root}, registry, func([]*packages.Package, ssa.BuilderMode) ssaBuildResult {
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
		code string
		globalDebug bool
	}{{code: "invalid-regexp"}, {code: "overwritten-before-use", globalDebug: true}}
	for _, test := range tests {
		t.Run(
			test.code,
			func(t *testing.T) {
				registry,
				err := NewRegistry([]string{test.code})
				if err != nil {
					t.Fatal(err)
				}
				calls := 0
				prepareSSA(
					nil,
					registry.executionPlan(),
					func(_ []*packages.Package, mode ssa.BuilderMode) ssaBuildResult {
						calls++
						if got := mode & ssa.GlobalDebug != 0; got != test.globalDebug {
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
	required := FactCallArguments | FactFirstCallArgument | FactParents
	facts := newPackageFacts(required)
	var builds atomic.Int32
	facts.builder = func(files []*ast.File, required FactSet) packageFactData {
		builds.Add(1)
		return buildPackageFacts(files, required)
	}
	pass := &Pass{Files: []*ast.File{file}, facts: facts}
	var group sync.WaitGroup
	for index := range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			switch index % 3 {
			case 0:
				_ = pass.argumentsByCallPosition()
			case 1:
				_ = pass.firstArgumentsByCallPosition()
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
	if len(pass.firstArgumentsByCallPosition()) != 2 {
		t.Fatalf("first call argument index has %d entries, want 2", len(pass.firstArgumentsByCallPosition()))
	}
	if len(pass.analysisParents()) == 0 {
		t.Fatal("parent index is empty")
	}
}
