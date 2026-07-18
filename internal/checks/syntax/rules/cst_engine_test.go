package rules

import (
	"fmt"
	"go/token"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/cst"
)

func benchmarkCSTTree(tb testing.TB) *cst.Tree {
	tb.Helper()
	function := `
func process(value int, flag bool, unused string, first, second, third int) (int, error) {
	message := "repeated message"
	_ = "repeated message"
	_ = "repeated message"
	defer func() int { return value }()
	if flag && value > 0 {
		for index := 0; index < value; index++ {
			if index%2 == 0 { value += index }
		}
	} else if value < 0 {
		value = -value
	}
	switch value {
	case 0: return 0, nil
	case 1: return 1, nil
	default: _ = message
	}
	return value, nil
}
`
	source := "package fixture\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n" + strings.Repeat(function, 20)
	tree, err := cst.Parse("fixture.go", []byte(source))
	if err != nil {
		tb.Fatal(err)
	}
	return tree
}

func TestCSTFunctionFactsMatchReferenceWalks(t *testing.T) {
	tree, err := cst.Parse(
		"facts.go",
		[]byte(`package facts

func complex(flag bool, value any, channel chan int) {
	if flag && (flag || false) {
		for index := 0; index < 3; index++ {
			if index == 1 { continue }
		}
	} else if !flag {
		goto done
	}
	switch current := value.(type) {
	case int:
		_ = current
	case string:
		break
	default:
	}
	select {
	case <-channel:
		break
	default:
	}
	_ = func() bool {
		if flag { return true }
		return false
	}()
done:
	return
}

type item struct{}
func (item) method() {}
`),
	)
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &cstAnalyzer{
		plan: cstExecutionPlan{functions: true, functionTraversal: true, functionComplexity: true, functionCognitive: true, functionStatements: true, functionFinal: true},
	}
	cst.WalkProductionsWithAncestors(tree.Root(), func(node cst.Node, ancestors []cst.Node) bool {
		analyzer.observe(node, ancestors)
		return true
	})
	if len(analyzer.functions) != 2 {
		t.Fatalf("collected %d functions, want 2", len(analyzer.functions))
	}
	for _, facts := range analyzer.functions {
		name := facts.name.Src()
		if got, want := facts.complexity, referenceCyclomaticComplexity(facts.body); got != want {
			t.Errorf("%s cyclomatic complexity = %d, want %d", name, got, want)
		}
		if got, want := facts.cognitiveComplexity, referenceCognitiveComplexity(facts.body); got != want {
			t.Errorf("%s cognitive complexity = %d, want %d", name, got, want)
		}
		if got, want := facts.statements, referenceStatementCount(facts.body); got != want {
			t.Errorf("%s statement count = %d, want %d", name, got, want)
		}
		if got, want := facts.finalStatement, referenceFinalStatement(facts.body); got != want {
			t.Errorf("%s final statement = %T, want %T", name, got, want)
		}
	}
}

func TestAnalyzeCSTSingleRuleParity(t *testing.T) {
	tree, err := cst.Parse(
		"bad_file.go",
		[]byte(`//bad comment
package bad_pkg

import (
	fmtAlias "fmt"
	"fmt"
	bad_Alias "strings"
	_ "embed"
	. "math"
)

var packageValue = 1
type Exported struct {
	bad_name string ` + "`json:\"name,,unknown\"`" + `
	fmt int
}

func init() {}
func GetThing(first, second, third, fourth, fifth, sixth int, flag bool, unused string) (value int) {
	message := "repeated message"
	_ = "repeated message"
	_ = "repeated message"
	defer fmtAlias.Println(message)
	if flag && value > 0 {
		for index := 0; index < value; index++ {
			if index%2 == 0 { value += index }
		}
	} else if value < 0 {
		value = -value
	}
	switch value {
	case 0: value = 1
	case 1: value = 2; break
	default: value = 3
	}
	return
}
`),
	)
	if err != nil {
		t.Fatal(err)
	}
	all, err := Select(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	allFindings := analyzeCSTFindings("bad_file.go", tree, all)
	byCode := make(map[string][]string)
	for _, finding := range allFindings {
		byCode[finding.Code] = append(byCode[finding.Code], findingKey(finding))
	}
	if len(byCode) < 12 {
		t.Fatalf("fixture exercised only %d CST rules: %v", len(byCode), sortedFindingCodes(byCode))
	}
	for code, want := range byCode {
		code, want := code, want
		t.Run(
			code,
			func(t *testing.T) {
				rules,
				selectErr := Select([]string{code}, false)
				if selectErr != nil {
					t.Fatal(selectErr)
				}
				gotFindings := analyzeCSTFindings("bad_file.go", tree, rules)
				got := make([]string, 0, len(gotFindings))
				for _,
				finding := range gotFindings {
					got = append(got, findingKey(finding))
				}
				sort.Strings(got)
				sort.Strings(want)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("single-rule findings differ\ngot:  %v\nwant: %v", got, want)
				}
			},
		)
	}
}

func TestAnalyzeCSTReceiverRulesAreIndependent(t *testing.T) {
	tree, err := cst.Parse(
		"receiver.go",
		[]byte(`package receiver

type item struct{}

func (first item) Alpha() {}
func (second item) Beta() {}
func (value item) MarshalJSON() ([]byte, error) { return nil, nil }
func (value *item) UnmarshalJSON([]byte) error { return nil }
`),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range[]string{"receiver-naming", "marshal-receiver"} {
		code := code
		t.Run(
			code,
			func(t *testing.T) {
				rules,
				selectErr := Select([]string{code}, false)
				if selectErr != nil {
					t.Fatal(selectErr)
				}
				findings := analyzeCSTFindings("receiver.go", tree, rules)
				if len(findings) == 0 {
					t.Fatalf("%s produced no findings", code)
				}
				for _,
				finding := range findings {
					if finding.Code != code {
						t.Fatalf("%s run reported %s", code, finding.Code)
					}
				}
			},
		)
	}
}

func analyzeCSTFindings(filename string, tree *cst.Tree, rules []Rule) []Finding {
	result := []Finding{}
	AnalyzeCST(CSTInput{Filename: filename, Tree: tree, Rules: rules, Report: func(finding Finding) {
		result = append(result, finding)
	}})
	return result
}

func findingKey(finding Finding) string {
	start, end := finding.ConcreteStart, finding.ConcreteEnd
	if !finding.HasConcreteRange {
		start, end = cst.Range(finding.ConcreteNode)
	}
	return fmt.Sprintf("%s:%d:%d:%s", finding.Code, start, end, finding.Message)
}

func sortedFindingCodes(findings map[string][]string) []string {
	result := make([]string, 0, len(findings))
	for code := range findings {
		result = append(result, code)
	}
	sort.Strings(result)
	return result
}

func referenceCyclomaticComplexity(body cst.Node) int {
	if body == nil {
		return 0
	}
	complexity := 1
	cst.Walk(
		body,
		func(node cst.Node) bool {
			switch current := node.(type) {
			case cst.Token:
				switch current.Ch() {
				case token.IF,
					token.FOR,
					token.CASE,
					token.LAND,
					token.LOR:
					complexity++
				}
			case *cst.TypeSwitchStmt:
				complexity++
			}
			return true
		},
	)
	return complexity
}

func referenceCognitiveComplexity(body cst.Node) int {
	if body == nil {
		return 0
	}
	total := 0
	var visit func(cst.Node, int)
	visit = func(node cst.Node, nesting int) {
		next := nesting
		switch cst.Kind(node) {
		case "IfStmt", "IfElseStmt", "ForStmt", "ExprSwitchStmt", "TypeSwitchStmt", "SelectStmt":
			total += 1 + nesting
			next++
		case "BreakStmt", "ContinueStmt", "GotoStmt", "FallthroughStmt":
			total++
		}
		for _, child := range cst.Children(node) {
			visit(child, next)
		}
	}
	visit(body, 0)
	return total
}

func referenceStatementCount(body cst.Node) int {
	count := 0
	cst.Walk(body, func(node cst.Node) bool {
		if list, ok := node.(*cst.StatementList); ok && list.Statement != nil {
			count++
		}
		return true
	})
	return count
}

func referenceFinalStatement(body cst.Node) cst.Node {
	var block *cst.Block
	cst.Walk(body, func(node cst.Node) bool {
		if current, ok := node.(*cst.Block); ok && block == nil {
			block = current
			return false
		}
		return block == nil
	})
	if block == nil {
		return nil
	}
	var final cst.Node
	for list := block.StatementList; list != nil; list = list.List {
		if list.Statement != nil {
			final = list.Statement
		}
	}
	return final
}

func benchmarkAnalyzeCST(b *testing.B, all bool) {
	tree := benchmarkCSTTree(b)
	rules, err := Select(nil, all)
	if err != nil {
		b.Fatal(err)
	}
	reports := 0
	input := CSTInput{Filename: "fixture.go", Tree: tree, Rules: rules, Report: func(Finding) {
		reports++
	}}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		AnalyzeCST(input)
	}
	if reports == 0 {
		b.Fatal("benchmark produced no findings")
	}
}

func BenchmarkAnalyzeCSTDefaults(b *testing.B) {
	benchmarkAnalyzeCST(b, false)
}

func BenchmarkAnalyzeCSTAll(b *testing.B) {
	benchmarkAnalyzeCST(b, true)
}
