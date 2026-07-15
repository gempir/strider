package analyze

import (
	"go/ast"
	"go/constant"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"

	"github.com/gempir/strider/internal/diagnostic"
)

type invalidTemplateRule struct{}

func (invalidTemplateRule) Meta() Meta {
	return Meta{
		Code:            "SA1001",
		Summary:         "detect invalid templates",
		Explanation:     "Constant text/template and html/template strings parsed directly from template.New must use valid template syntax.",
		GoodExample:     "template.New(\"greeting\").Parse(`Hello, {{.Name}}`)",
		BadExample:      "template.New(\"greeting\").Parse(`Hello, {{.Name}`)",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (invalidTemplateRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				kind := templateParseKind(pass, selector)
				if kind == "" || !isDirectTemplateNew(pass, selector.X, kind) {
					return true
				}
				value := pass.TypesInfo.Types[call.Args[0]].Value
				if value == nil || value.Kind() != constant.String {
					return true
				}
				message := parseTemplate(kind, constant.StringVal(value))
				if strings.Contains(message, "unexpected") ||
					strings.Contains(message, "bad character") {
					pass.Report(call.Args[0], message)
				}
				return true
			},
		)
	}
}

func templateParseKind(pass *Pass, selector *ast.SelectorExpr) string {
	function := calledFunction(pass.TypesInfo, selector)
	if function == nil || function.Pkg() == nil || function.Name() != "Parse" {
		return ""
	}
	switch function.Pkg().Path() {
	case "text/template":
		return "text/template"
	case "html/template":
		return "html/template"
	default:
		return ""
	}
}

func isDirectTemplateNew(pass *Pass, receiver ast.Expr, kind string) bool {
	call, ok := receiver.(*ast.CallExpr)
	return ok && isPackageFunction(pass.TypesInfo, call.Fun, kind, "New")
}

func parseTemplate(kind, source string) string {
	var err error
	if kind == "text/template" {
		_, err = texttemplate.New("").Parse(source)
	} else {
		_, err = htmltemplate.New("").Parse(source)
	}
	if err == nil {
		return ""
	}
	return err.Error()
}
