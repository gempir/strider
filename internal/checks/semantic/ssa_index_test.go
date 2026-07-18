package semantic

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

var benchmarkStaticCallCount int

func TestStaticCallIndexMatchesInstructionScan(t *testing.T) {
	functions := staticCallFixture(t)
	indexed := buildPackageSSAFacts(functions, FactStaticCalls).staticCallsByPackage
	for _, packagePath := range []string{
		"bytes",
		"context",
		"regexp",
		"strconv",
		"strings",
		"time",
		"missing/package",
	} {
		want := scanStaticCallsInPackage(functions, packagePath)
		got := indexed[packagePath]
		if len(got) != len(want) {
			t.Fatalf(
				"%s index has %d calls, instruction scan has %d",
				packagePath,
				len(got),
				len(want),
			)
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("%s call %d differs from instruction order", packagePath, index)
			}
		}
	}
}

func TestStaticCallIndexFiltersUnselectedPackages(t *testing.T) {
	functions := staticCallFixture(t)
	indexed := buildPackageSSAFacts(functions, FactStaticCalls, map[string]bool{"regexp": true}).staticCallsByPackage
	if len(indexed["regexp"]) == 0 {
		t.Fatal("selected regexp calls were not indexed")
	}
	if len(indexed) != 1 {
		t.Fatalf("indexed unselected packages: %#v", indexed)
	}
}

func BenchmarkStaticCallCohort(benchmark *testing.B) {
	functions := staticCallFixture(benchmark)
	packagePaths := []string{"regexp", "time", "strings", "bytes", "strconv", "context"}
	const consumers = 23
	benchmark.Run(
		"repeated-instruction-scans",
		func(benchmark *testing.B) {
			benchmark.ReportAllocs()
			for range benchmark.N {
				total := 0
				for consumer := range consumers {
					total += countStaticCallsInPackage(
						functions,
						packagePaths[consumer%len(packagePaths)],
					)
				}
				benchmarkStaticCallCount = total
			}
		},
	)
	benchmark.Run(
		"shared-package-index",
		func(benchmark *testing.B) {
			benchmark.ReportAllocs()
			for range benchmark.N {
				indexed := buildPackageSSAFacts(functions, FactStaticCalls).staticCallsByPackage
				total := 0
				for consumer := range consumers {
					total += len(indexed[packagePaths[consumer%len(packagePaths)]])
				}
				benchmarkStaticCallCount = total
			}
		},
	)
}

func countStaticCallsInPackage(functions []*ssa.Function, packagePath string) int {
	count := 0
	for _, function := range functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}
				callee := call.Common().StaticCallee()
				if callee != nil && callee.Object() != nil && callee.Object().Pkg() != nil && callee.Object().Pkg().Path() == packagePath {
					count++
				}
			}
		}
	}
	return count
}

func scanStaticCallsInPackage(functions []*ssa.Function, packagePath string) []ssa.CallInstruction {
	result := make([]ssa.CallInstruction, 0)
	for _, function := range functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}
				callee := call.Common().StaticCallee()
				if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil || callee.Object().Pkg().Path() != packagePath {
					continue
				}
				result = append(result, call)
			}
		}
	}
	return result
}

func staticCallFixture(testingObject testing.TB) []*ssa.Function {
	testingObject.Helper()
	var source strings.Builder
	source.WriteString(
		`package fixture

import (
	"bytes"
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"
)

`,
	)
	for index := range 128 {
		fmt.Fprintf(
			&source,
			`func check%d(input string, data []byte) string {
	_, _ = regexp.MatchString("[a-z]+", input)
	_, _ = time.Parse("2006-01-02", input)
	input = strings.Trim(input, " ")
	input += strconv.Itoa(%d)
	_ = bytes.Equal(data, data)
	_ = context.WithValue(context.Background(), "key", input)
	return input
}

`,
			index,
			index,
		)
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "fixture.go", source.String(), 0)
	if err != nil {
		testingObject.Fatal(err)
	}
	ssaPackage, _, err := ssautil.BuildPackage(
		&types.Config{Importer: importer.Default()},
		fileSet,
		types.NewPackage("example.com/fixture", "fixture"),
		[]*ast.File{file},
		ssa.InstantiateGenerics,
	)
	if err != nil {
		testingObject.Fatal(err)
	}
	return collectPackageFunctions(ssaPackage.Prog, []*ssa.Package{ssaPackage})[ssaPackage]
}
