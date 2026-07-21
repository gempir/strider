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

type researchCorrectnessReport struct {
	position token.Position
	message  string
}

func runResearchCorrectnessCheck(t *testing.T, check Check, source string) []researchCorrectnessReport {
	t.Helper()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "fixture.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	ssaPackage, typeInfo, err := ssautil.BuildPackage(
		&types.Config{
			Importer: importer.Default(),
		},
		fileSet,
		types.NewPackage("example.com/researchfixture", "fixture"),
		[]*ast.File{
			file,
		},
		ssa.InstantiateGenerics,
	)
	if err != nil {
		t.Fatal(err)
	}
	reports := make([]researchCorrectnessReport, 0)
	pass := &Pass{
		PackagePath: "example.com/researchfixture",
		Files: []*ast.File{
			file,
		},
		FileSet:    fileSet,
		Types:      ssaPackage.Pkg,
		TypesSizes: types.SizesFor("gc", runtime.GOARCH),
		TypesInfo:  typeInfo,
		SSAProgram: ssaPackage.Prog,
		SSAPackage: ssaPackage,
		Functions: collectPackageFunctions(ssaPackage.Prog, []*ssa.Package{
			ssaPackage,
		})[ssaPackage],
	}
	pass.report = func(start, _ token.Pos, message string, _ []diagnostic.Fix) {
		reports = append(reports, researchCorrectnessReport{
			position: fileSet.Position(start),
			message:  message,
		})
	}
	check.Run(pass)
	return reports
}

func assertResearchReportCount(t *testing.T, reports []researchCorrectnessReport, want int) {
	t.Helper()
	if len(reports) == want {
		return
	}
	t.Fatalf("got %d reports, want %d: %#v", len(reports), want, reports)
}

func assertResearchMessagesContain(t *testing.T, reports []researchCorrectnessReport, fragment string) {
	t.Helper()
	for _, report := range reports {
		if !strings.Contains(report.message, fragment) {
			t.Errorf("report at %s has message %q; want fragment %q", report.position, report.message, fragment)
		}
	}
}

func assertResearchReportNeedles(t *testing.T, reports []researchCorrectnessReport, source string, needles ...string) {
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
