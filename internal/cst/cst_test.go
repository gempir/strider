package cst

import (
	"go/token"
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
	tree, err := Parse(
		"fixture.go",
		[]byte("package p\nfunc F(ok bool) { if ok { return } else { panic(ok) } }\n"),
	)
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
		for _, child := range Children(node) {
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
		wantTokens := referenceNodeTokens(node)
		gotTokens := NodeTokens(node)
		if !slices.Equal(gotTokens, wantTokens) {
			t.Fatalf(
				"%s tokens differ: got %d, want %d",
				Kind(node),
				len(gotTokens),
				len(wantTokens),
			)
		}
		wantStart, wantEnd := referenceRange(wantTokens)
		gotStart, gotEnd := Range(node)
		if gotStart != wantStart || gotEnd != wantEnd {
			t.Fatalf(
				"%s range = %d:%d, want %d:%d",
				Kind(node),
				gotStart,
				gotEnd,
				wantStart,
				wantEnd,
			)
		}
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
			if len(kinds) == 0 || kinds[0] != "SourceFile" || !slices.Contains(
				kinds,
				"FunctionDecl",
			) {
				t.Fatalf("unexpected ancestors: %v", kinds)
			}
			return true
		},
	)
	if !foundReturn {
		t.Fatal("return statement was not visited")
	}
}

func referenceNodeTokens(node Node) []Token {
	if current, ok := node.(Token); ok {
		if current.IsValid() {
			return[]Token{current}
		}
		return nil
	}
	result := []Token{}
	for _, child := range Children(node) {
		result = append(result, referenceNodeTokens(child)...)
	}
	sort.SliceStable(
		result,
		func(i, j int) bool {
			return result[i].Position().Offset < result[j].Position().Offset
		},
	)
	return result
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
