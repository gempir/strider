package cst

import (
	"os/exec"
	"testing"
)

func TestGeneratedAccessorsAreCurrent(t *testing.T) {
	command := exec.Command("go", "run", "./cmd/gencst", "-check", "-output", "zz_nodes_generated.go")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated accessors are stale: %v\n%s", err, output)
	}
}

func TestGeneratedAccessorsPreserveTypedNilBehavior(t *testing.T) {
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
}
