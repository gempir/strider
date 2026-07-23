package semantic

import (
	"go/ast"
	"go/constant"
	"strings"

	"github.com/gempir/strider/internal/diagnostic"
)

type invalidExecCommandCheck struct{}

func (invalidExecCommandCheck) Meta() Meta {
	return Meta{
		Code:            "invalid-exec-command",
		Summary:         "detect shell commands passed as exec.Command programs",
		Explanation:     "exec.Command executes a program directly. A constant first argument containing spaces but no path separators usually combines a program and its arguments as though a shell would split them.",
		GoodExample:     "exec.Command(\"ls\", \"/\", \"/tmp\")",
		BadExample:      "exec.Command(\"ls / /tmp\")",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (invalidExecCommandCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 || !isPackageFunction(pass.TypesInfo, call.Fun, "os/exec", "Command") {
				return true
			}
			value := pass.TypesInfo.Types[call.Args[0]].Value
			if value == nil || value.Kind() != constant.String {
				return true
			}
			program := constant.StringVal(value)
			if !strings.Contains(program, " ") || strings.Contains(program, `\`) || strings.Contains(program, "/") {
				return true
			}
			pass.Report(call.Args[0], "first argument to exec.Command looks like a shell command, but a program name or path are expected")
			return true
		},
	)
}
