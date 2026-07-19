package cst

import (
	"go/token"
	"os/exec"
	"slices"
	"testing"
)

func TestGeneratedAccessorsAreCurrent(t *testing.T) {
	command := exec.Command("go", "run", "./cmd/gencst", "-check", "-output", "zz_nodes_generated.go")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated accessors are stale: %v\n%s", err, output)
	}
}

func TestGeneratedAccessorsPreserveNilAndFallbackBehavior(t *testing.T) {
	var typedNil Node = (*Block)(nil)
	if Kind(typedNil) != "" {
		t.Fatalf("typed nil kind is %q", Kind(typedNil))
	}
	if children := Children(typedNil); children != nil {
		t.Fatalf("typed nil children = %#v", children)
	}
	visited := false
	Walk(typedNil, func(Node) bool {
		visited = true
		return true
	})
	if visited {
		t.Fatal("walk visited a typed nil node")
	}
	if start, end := Range(typedNil); start != 0 || end != 0 {
		t.Fatalf("typed nil range = %d:%d", start, end)
	}

	tree, err := Parse("fixture.go", []byte("package p\nvar value = 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	wrapper := &fallbackNode{
		Child: tree.Root(),
	}
	if got, want := Children(wrapper), []Node{
		tree.Root(),
	}; !slices.Equal(got, want) {
		t.Fatalf("fallback children = %#v, want %#v", got, want)
	}
	if got, want := NodeTokens(wrapper), NodeTokens(tree.Root()); !slices.Equal(got, want) {
		t.Fatalf("fallback tokens = %d, want %d", len(got), len(want))
	}
	if gotStart, gotEnd := Range(wrapper); gotStart != 0 || gotEnd != len("package p\nvar value = 1") {
		t.Fatalf("fallback range = %d:%d", gotStart, gotEnd)
	}
}

func TestProductionWalkSkipsTokenValuesAndPointers(t *testing.T) {
	tree, err := Parse("fixture.go", []byte("package p\n"))
	if err != nil {
		t.Fatal(err)
	}
	valid := tree.Tokens()[0]
	var zero Token
	var nilPointer *Token
	tests := []struct {
		name string
		node Node
	}{
		{
			name: "valid-value",
			node: valid,
		},
		{
			name: "zero-value",
			node: zero,
		},
		{
			name: "valid-pointer",
			node: &valid,
		},
		{
			name: "nil-pointer",
			node: nilPointer,
		},
	}
	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				visited := false
				WalkProductionsWithAncestors(test.node, func(Node, []Node) bool {
					visited = true
					return true
				})
				if visited {
					t.Fatal("production walk visited a token")
				}
			},
		)
	}

	wrapper := &fallbackNode{
		Child: &valid,
	}
	visits := []Node{}
	WalkProductionsWithAncestors(wrapper, func(node Node, _ []Node) bool {
		visits = append(visits, node)
		return true
	})
	if !slices.Equal(visits, []Node{
		wrapper,
	}) {
		t.Fatalf("fallback production visits = %#v", visits)
	}
}

type fallbackNode struct {
	Child Node
}

func (n *fallbackNode) Position() token.Position {
	if n == nil || n.Child == nil {
		return token.Position{}
	}
	return n.Child.Position()
}

func (n *fallbackNode) Source(full bool) string {
	if n == nil || n.Child == nil {
		return ""
	}
	return n.Child.Source(full)
}
