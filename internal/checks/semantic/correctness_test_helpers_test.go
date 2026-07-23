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

type checkReport struct {
	position token.Position
	message  string
}

func runCheckFixture(t *testing.T, check Check, source string) []checkReport {
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
		types.NewPackage("example.com/checkfixture", "fixture"),
		[]*ast.File{
			file,
		},
		ssa.InstantiateGenerics,
	)
	if err != nil {
		t.Fatal(err)
	}
	reports := make([]checkReport, 0)
	pass := &Pass{
		PackagePath: "example.com/checkfixture",
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
		reports = append(reports, checkReport{
			position: fileSet.Position(start),
			message:  message,
		})
	}
	check.Run(pass)
	return reports
}

func assertReportCount(t *testing.T, reports []checkReport, want int) {
	t.Helper()
	if len(reports) == want {
		return
	}
	t.Fatalf("got %d reports, want %d: %#v", len(reports), want, reports)
}

func assertMessagesContain(t *testing.T, reports []checkReport, fragment string) {
	t.Helper()
	for _, report := range reports {
		if !strings.Contains(report.message, fragment) {
			t.Errorf("report at %s has message %q; want fragment %q", report.position, report.message, fragment)
		}
	}
}
