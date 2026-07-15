package rules

import (
	"fmt"
	"go/ast"
	"strings"
)

func (a *analyzer) checkMarshalReceivers() {
	types := map[string]string{}
	receiverNames := map[string]string{}
	for _, decl := range a.file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		base := strings.TrimPrefix(exprText(fn.Recv.List[0].Type), "*")
		if len(fn.Recv.List[0].Names) > 0 {
			name := fn.Recv.List[0].Names[0]
			if first, ok := receiverNames[base]; ok && first != name.Name {
				a.report(
					"receiver-naming",
					name,
					fmt.Sprintf("receiver name %s is inconsistent with %s", name.Name, first),
				)
			} else {
				receiverNames[base] = name.Name
			}
		}
		if !marshalMethod(fn.Name.Name) {
			continue
		}
		kind := "value"
		if strings.HasPrefix(exprText(fn.Recv.List[0].Type), "*") {
			kind = "pointer"
		}
		if first, ok := types[base]; ok && first != kind {
			a.report(
				"marshal-receiver",
				fn.Recv.List[0],
				"marshal and unmarshal methods should use a consistent receiver type",
			)
		} else {
			types[base] = kind
		}
	}
}

func marshalMethod(name string) bool {
	return strings.HasPrefix(name, "Marshal") || strings.HasPrefix(name, "Unmarshal") ||
		strings.HasPrefix(name, "Encode") ||
		strings.HasPrefix(name, "Decode")
}
