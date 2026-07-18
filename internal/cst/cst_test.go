package cst

import (
	"fmt"
	"go/token"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

func TestParseIsLossless(t *testing.T) {
	source := []byte("// Package p documents p.\npackage p\n\nfunc F[T any](value T) T { // keep\n\treturn value\n}\n")
	tree, err := Parse("fixture.go", source)
	if err != nil {
		t.Fatal(err)
	}
	var rebuilt strings.Builder
	for _, current := range tree.Tokens() {
		rebuilt.WriteString(current.Sep())
		rebuilt.WriteString(current.Src())
	}
	if rebuilt.String() != string(source) {
		t.Fatalf("rebuilt source:\n%q\nwant:\n%q", rebuilt.String(), source)
	}
	if string(tree.Source()) != string(source) {
		t.Fatal("Tree.Source did not preserve the input")
	}
}

func TestWalkIncludesProductionsAndTokens(t *testing.T) {
	tree, err := Parse("fixture.go", []byte("package p\nfunc F(ok bool) { if ok { return } else { panic(ok) } }\n"))
	if err != nil {
		t.Fatal(err)
	}
	kinds := []string{}
	Walk(tree.Root(), func(node Node) bool {
		kinds = append(kinds, Kind(node))
		return true
	})
	for _, wanted := range[]string{"SourceFile", "FunctionDecl", "IfElseStmt", "func", "IDENT"} {
		if !slices.Contains(kinds, wanted) {
			t.Errorf("walk did not include %q in %v", wanted, kinds)
		}
	}
}

func TestRangeExcludesLeadingTrivia(t *testing.T) {
	source := []byte("// header\npackage p\n")
	tree, err := Parse("fixture.go", source)
	if err != nil {
		t.Fatal(err)
	}
	start, end := Range(tree.Root())
	if got := string(source[start:end]); got != "package p" {
		t.Fatalf("range selected %q", got)
	}
	position := tree.Position(start)
	if position.Line != 2 || position.Column != 1 || position.Offset != len("// header\n") {
		t.Fatalf("unexpected position: %#v", position)
	}
}

func TestTokensIncludeImplicitSemicolonAndEOF(t *testing.T) {
	tree, err := Parse("fixture.go", []byte("package p\n"))
	if err != nil {
		t.Fatal(err)
	}
	tokens := tree.Tokens()
	if len(tokens) < 4 || tokens[len(tokens) - 1].Ch() != token.EOF {
		t.Fatalf("unexpected tokens: %#v", tokens)
	}
}

func TestCommentsRetainSpellingAndRanges(t *testing.T) {
	source := []byte("// first\npackage p // second\n")
	tree, err := Parse("fixture.go", source)
	if err != nil {
		t.Fatal(err)
	}
	comments := tree.Comments()
	if len(comments) != 2 {
		t.Fatalf("got %d comments", len(comments))
	}
	for _, comment := range comments {
		if string(source[comment.Start:comment.End]) != comment.Text {
			t.Errorf("comment range does not select %q", comment.Text)
		}
	}
	if comments[1].Line != 2 || comments[1].Column != 11 {
		t.Fatalf("unexpected second comment position: %#v", comments[1])
	}
}

func TestLinearTraversalAndTokenRangesMatchConcreteChildren(t *testing.T) {
	source := []byte(`// Package p documents p.
package p

import (
	"fmt"
	alias "example.com/value"
)

type Pair[T any] struct {
	Left, Right T
}

func F[T ~int](values []T) (total T) {
	for index, value := range values {
		if index%2 == 0 {
			total += value
		} else {
			fmt.Println(alias.Value, value)
		}
	}
	return total
}
`)
	tree, err := Parse("fixture.go", source)
	if err != nil {
		t.Fatal(err)
	}

	wantWalk := []Node{}
	var collectWalk func(Node)
	collectWalk = func(node Node) {
		wantWalk = append(wantWalk, node)
		for _, child := range referenceChildren(node) {
			collectWalk(child)
		}
	}
	collectWalk(tree.Root())
	gotWalk := []Node{}
	Walk(tree.Root(), func(node Node) bool {
		gotWalk = append(gotWalk, node)
		return true
	})
	if !slices.Equal(gotWalk, wantWalk) {
		t.Fatal("linear walk order differs from concrete child traversal")
	}

	for _, node := range wantWalk {
		wantChildren := referenceChildren(node)
		gotChildren := Children(node)
		if !slices.Equal(gotChildren, wantChildren) {
			t.Fatalf("%s children differ: got %d, want %d", Kind(node), len(gotChildren), len(wantChildren))
		}
		wantTokens := referenceNodeTokens(node)
		gotTokens := NodeTokens(node)
		if !slices.Equal(gotTokens, wantTokens) {
			t.Fatalf("%s tokens differ: got %d, want %d", Kind(node), len(gotTokens), len(wantTokens))
		}
		wantStart, wantEnd := referenceRange(wantTokens)
		gotStart, gotEnd := Range(node)
		if gotStart != wantStart || gotEnd != wantEnd {
			t.Fatalf("%s range = %d:%d, want %d:%d", Kind(node), gotStart, gotEnd, wantStart, wantEnd)
		}
	}

	wantSkipped := []Node{}
	var collectSkipped func(Node)
	collectSkipped = func(node Node) {
		wantSkipped = append(wantSkipped, node)
		if Kind(node) == "IfElseStmt" {
			return
		}
		for _, child := range referenceChildren(node) {
			collectSkipped(child)
		}
	}
	collectSkipped(tree.Root())
	gotSkipped := []Node{}
	Walk(tree.Root(), func(node Node) bool {
		gotSkipped = append(gotSkipped, node)
		return Kind(node) != "IfElseStmt"
	})
	if !slices.Equal(gotSkipped, wantSkipped) {
		t.Fatal("walk descendant skipping differs from reflection traversal")
	}
}

func TestGeneratedAccessorsMatchReflectionAcrossSyntax(t *testing.T) {
	sources := []string{
		`package p
func communicate(send chan<- int, receive <-chan int) {
	select {
	case send <- <-receive:
	case value := <-receive:
		_ = value
	default:
	}
}
`,
		`package p
func switches(input any) {
	switch value := input.(type) {
	case int, string:
		_ = value
	default:
	}
	switch value := 1; value {
	case 1, 2:
		fallthrough
	default:
		break
	}
}
`,
		"package p\n" + "type Box[T any] struct { Value T `json:\"value\"` }\n" + "func (box *Box[T]) Method(values ...T) (result []T) {\n" + "defer func() { recover() }()\n" + "go func(T) {}(box.Value)\n" + "result = append(result, values[1:]...)\n" + "_ = map[string]T{\"value\": box.Value}\n" + "return\n}\n",
		`package p
type Number interface { ~int | ~int64 }
type Alias = map[string]int
const (
	first = iota
	second
)
var (
	left, right int = 1, 2
)
`,
	}
	for index, source := range sources {
		t.Run(
			fmt.Sprintf("fixture-%d", index),
			func(t *testing.T) {
				tree,
				err := Parse("fixture.go", []byte(source))
				if err != nil {
					t.Fatal(err)
				}
				wantWalk := referenceWalk(tree.Root())
				gotWalk := []Node{}
				Walk(tree.Root(), func(node Node) bool {
					gotWalk = append(gotWalk, node)
					return true
				})
				if !slices.Equal(gotWalk, wantWalk) {
					t.Fatal("generated walk differs from reflection")
				}
				for _,
				node := range wantWalk {
					if got,
					want := Children(node),
					referenceChildren(node); !slices.Equal(got, want) {
						t.Fatalf("%s children differ", Kind(node))
					}
					if got,
					want := NodeTokens(node),
					referenceNodeTokens(node); !slices.Equal(got, want) {
						t.Fatalf("%s tokens differ", Kind(node))
					}
					gotStart,
					gotEnd := Range(node)
					wantStart,
					wantEnd := referenceRange(referenceNodeTokens(node))
					if gotStart != wantStart || gotEnd != wantEnd {
						t.Fatalf("%s range = %d:%d, want %d:%d", Kind(node), gotStart, gotEnd, wantStart, wantEnd)
					}
				}
			},
		)
	}
}

func TestWalkWithAncestors(t *testing.T) {
	tree, err := Parse("fixture.go", []byte("package p\nfunc F() { return }\n"))
	if err != nil {
		t.Fatal(err)
	}
	foundReturn := false
	WalkWithAncestors(
		tree.Root(),
		func(node Node, ancestors []Node) bool {
			if Kind(node) != "ReturnStmt" {
				return true
			}
			foundReturn = true
			kinds := make([]string, len(ancestors))
			for index,
			ancestor := range ancestors {
				kinds[index] = Kind(ancestor)
			}
			if len(kinds) == 0 || kinds[0] != "SourceFile" || !slices.Contains(kinds, "FunctionDecl") {
				t.Fatalf("unexpected ancestors: %v", kinds)
			}
			return true
		},
	)
	if !foundReturn {
		t.Fatal("return statement was not visited")
	}

	type visitRecord struct {
		node Node
		ancestors []Node
	}
	want := []visitRecord{}
	var collect func(Node, []Node)
	collect = func(node Node, ancestors []Node) {
		want = append(want, visitRecord{node: node, ancestors: append([]Node(nil), ancestors...)})
		ancestors = append(ancestors, node)
		for _, child := range referenceChildren(node) {
			collect(child, ancestors)
		}
	}
	collect(tree.Root(), nil)
	got := []visitRecord{}
	WalkWithAncestors(tree.Root(), func(node Node, ancestors []Node) bool {
		got = append(got, visitRecord{node: node, ancestors: append([]Node(nil), ancestors...)})
		return true
	})
	if len(got) != len(want) {
		t.Fatalf("ancestor walk visits = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index].node != want[index].node || !slices.Equal(got[index].ancestors, want[index].ancestors) {
			t.Fatalf("ancestor walk differs at visit %d (%s)", index, Kind(want[index].node))
		}
	}
	wantProductions := []visitRecord{}
	for _, current := range want {
		if !referenceTokenNode(current.node) {
			wantProductions = append(wantProductions, current)
		}
	}
	gotProductions := []visitRecord{}
	WalkProductionsWithAncestors(
		tree.Root(),
		func(node Node, ancestors []Node) bool {
			if referenceTokenNode(node) {
				t.Fatalf("production walk visited token %s", Kind(node))
			}
			gotProductions = append(gotProductions, visitRecord{node: node, ancestors: append([]Node(nil), ancestors...)})
			return true
		},
	)
	if len(gotProductions) != len(wantProductions) {
		t.Fatalf("production ancestor walk visits = %d, want %d", len(gotProductions), len(wantProductions))
	}
	for index := range wantProductions {
		if gotProductions[index].node != wantProductions[index].node || !slices.Equal(gotProductions[index].ancestors, wantProductions[index].ancestors) {
			t.Fatalf("production ancestor walk differs at visit %d (%s)", index, Kind(wantProductions[index].node))
		}
	}
}

func BenchmarkGeneratedWalk(b *testing.B) {
	tree := benchmarkTree(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		Walk(tree.Root(), func(Node) bool {
			return true
		})
	}
}

func BenchmarkGeneratedProductionAncestorWalk(b *testing.B) {
	tree := benchmarkTree(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		WalkProductionsWithAncestors(tree.Root(), func(Node, []Node) bool {
			return true
		})
	}
}

func BenchmarkReflectionWalk(b *testing.B) {
	tree := benchmarkTree(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		stack := []Node{tree.Root()}
		for len(stack) != 0 {
			last := len(stack) - 1
			current := stack[last]
			stack = stack[:last]
			stack = appendReflectionChildrenReverse(stack, current)
		}
	}
}

func BenchmarkGeneratedRange(b *testing.B) {
	tree := benchmarkTree(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		Range(tree.Root())
	}
}

func BenchmarkReflectionRange(b *testing.B) {
	tree := benchmarkTree(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result := bounds{}
		collectTokenBounds(reflect.ValueOf(tree.Root()), true, &result)
	}
}

func appendReflectionChildrenReverse(stack []Node, node Node) []Node {
	if _, ok := node.(Token); ok {
		return stack
	}
	value := referenceIndirect(reflect.ValueOf(node))
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return stack
	}
	valueType := value.Type()
	for fieldIndex := value.NumField() - 1; fieldIndex >= 0; fieldIndex-- {
		if !valueType.Field(fieldIndex).IsExported() {
			continue
		}
		field := value.Field(fieldIndex)
		if child, ok := referenceNodeValue(field); ok {
			stack = append(stack, child)
			continue
		}
		if field.Kind() != reflect.Slice {
			continue
		}
		for item := field.Len() - 1; item >= 0; item-- {
			if child, ok := referenceNodeValue(field.Index(item)); ok {
				stack = append(stack, child)
			}
		}
	}
	return stack
}

func benchmarkTree(tb testing.TB) *Tree {
	tb.Helper()
	tree, err := Parse(
		"fixture.go",
		[]byte(`package p

import "fmt"

type Pair[T any] struct { Left, Right T }

func Sum[T ~int](values []T) (total T) {
	for index, value := range values {
		if index%2 == 0 { total += value } else { fmt.Println(value) }
	}
	return total
}
`),
	)
	if err != nil {
		tb.Fatal(err)
	}
	return tree
}

func referenceNodeTokens(node Node) []Token {
	result := []Token{}
	referenceCollectTokens(reflect.ValueOf(node), &result)
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Position().Offset < result[j].Position().Offset
	})
	return result
}

func referenceWalk(root Node) []Node {
	result := []Node{}
	var collect func(Node)
	collect = func(node Node) {
		result = append(result, node)
		for _, child := range referenceChildren(node) {
			collect(child)
		}
	}
	collect(root)
	return result
}

func referenceChildren(node Node) []Node {
	if referenceNilNode(node) {
		return nil
	}
	if _, ok := node.(Token); ok {
		return nil
	}
	value := referenceIndirect(reflect.ValueOf(node))
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return[]Node{}
	}
	result := []Node{}
	valueType := value.Type()
	for index := 0; index < value.NumField(); index++ {
		if !valueType.Field(index).IsExported() {
			continue
		}
		field := value.Field(index)
		if child, ok := referenceNodeValue(field); ok {
			result = append(result, child)
			continue
		}
		if field.Kind() != reflect.Slice {
			continue
		}
		for item := 0; item < field.Len(); item++ {
			if child, ok := referenceNodeValue(field.Index(item)); ok {
				result = append(result, child)
			}
		}
	}
	return result
}

func referenceCollectTokens(value reflect.Value, result *[]Token) {
	if !value.IsValid() {
		return
	}
	if value.CanInterface() {
		if current, ok := value.Interface().(Token); ok {
			if current.IsValid() {
				*result = append(*result, current)
			}
			return
		}
	}
	value = referenceIndirect(value)
	if !value.IsValid() {
		return
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		referenceCollectTokens(value.Elem(), result)
	case reflect.Struct:
		valueType := value.Type()
		for index := 0; index < value.NumField(); index++ {
			if valueType.Field(index).IsExported() {
				referenceCollectTokens(value.Field(index), result)
			}
		}
	case reflect.Slice:
		for index := 0; index < value.Len(); index++ {
			referenceCollectTokens(value.Index(index), result)
		}
	}
}

func referenceNodeValue(value reflect.Value) (Node, bool) {
	if !value.IsValid() || (referenceNilable(value.Kind()) && value.IsNil()) || !value.CanInterface() {
		return nil, false
	}
	if current, ok := value.Interface().(Token); ok && !current.IsValid() {
		return nil, false
	}
	node, ok := value.Interface().(Node)
	return node, ok && !referenceNilNode(node)
}

func referenceIndirect(value reflect.Value) reflect.Value {
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

func referenceNilNode(node Node) bool {
	if node == nil {
		return true
	}
	value := reflect.ValueOf(node)
	return referenceNilable(value.Kind()) && value.IsNil()
}

func referenceNilable(kind reflect.Kind) bool {
	return kind == reflect.Chan || kind == reflect.Func || kind == reflect.Interface || kind == reflect.Map || kind == reflect.Pointer || kind == reflect.Slice
}

func referenceTokenNode(node Node) bool {
	switch node.(type) {
	case Token, *Token:
		return true
	default:
		return false
	}
}

func referenceRange(tokens []Token) (start, end int) {
	for _, current := range tokens {
		if concreteToken(current) {
			start = current.Position().Offset
			break
		}
	}
	for index := len(tokens) - 1; index >= 0; index-- {
		current := tokens[index]
		if concreteToken(current) {
			end = current.Position().Offset + len(current.Src())
			break
		}
	}
	return start, end
}
