package semantic

import "testing"

func TestConstructorInterfaceReturnConservativeCases(t *testing.T) {
	assertCheckDiagnostics(
		t,
		"constructor-interface-return",
		`package sample

type Store interface { Get(string) string }

type memoryStore struct{}

func (*memoryStore) Get(key string) string { return key }

func NewStore() Store { return &memoryStore{} }
func BuildStore() Store { return &memoryStore{} }
func NewAny() any { return &memoryStore{} }
`,
		1,
		"concrete result",
	)
}
