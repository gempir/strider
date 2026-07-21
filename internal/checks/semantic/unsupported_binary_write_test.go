package semantic

import (
	"strings"
	"testing"
)

func TestUnsupportedBinaryWriteReportsUnsupportedBinaryWriteValues(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"encoding/binary"
	"io"
)

type valid struct { A int32 }
type invalid struct { A int }

func check() {
	var architectureSized int
	var validInteger int32
	var invalidSlice []int
	var validSlice []int32
	var invalidMap map[string]int32
	var invalidChannel chan int32
	var invalidStruct invalid
	var validStruct valid
	binary.Write(io.Discard, binary.LittleEndian, architectureSized)
	binary.Write(io.Discard, binary.LittleEndian, validInteger)
	binary.Write(io.Discard, binary.LittleEndian, invalidSlice)
	binary.Write(io.Discard, binary.LittleEndian, validSlice)
	binary.Write(io.Discard, binary.LittleEndian, invalidMap)
	binary.Write(io.Discard, binary.LittleEndian, invalidChannel)
	binary.Write(io.Discard, binary.LittleEndian, invalidStruct)
	binary.Write(io.Discard, binary.LittleEndian, &validStruct)
}
`,
	)
	registry, err := newRegistry([]string{
		"unsupported-binary-write",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnosticGolden(t, diagnostics)
	for _, item := range diagnostics {
		if item.Code != "unsupported-binary-write" || !strings.Contains(item.Message, "binary.Write") {
			t.Fatalf("unexpected diagnostic: %#v", item)
		}
	}
}
