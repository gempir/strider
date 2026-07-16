package cst

import (
	"go/token"
	"slices"
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
	for _, wanted := range []string{"SourceFile", "FunctionDecl", "IfElseStmt", "func", "IDENT"} {
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
	if len(tokens) < 4 || tokens[len(tokens)-1].Ch() != token.EOF {
		t.Fatalf("unexpected tokens: %#v", tokens)
	}
}
