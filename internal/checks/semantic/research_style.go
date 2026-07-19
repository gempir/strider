package semantic

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"
	"unicode"

	"github.com/gempir/strider/internal/diagnostic"
)

var standardHTTPMethods = map[string]string{
	"CONNECT": "MethodConnect",
	"DELETE":  "MethodDelete",
	"GET":     "MethodGet",
	"HEAD":    "MethodHead",
	"OPTIONS": "MethodOptions",
	"PATCH":   "MethodPatch",
	"POST":    "MethodPost",
	"PUT":     "MethodPut",
	"TRACE":   "MethodTrace",
}

type excessiveBlankIdentifiersRule struct{}

type taskCommentRule struct{}

type docCommentPeriodRule struct{}

type errorTypeNamingRule struct{}

type standardHTTPMethodConstantRule struct{}

type weakCryptographyRule struct{}

func (excessiveBlankIdentifiersRule) Meta() Meta {
	return Meta{
		Code:            "excessive-blank-identifiers",
		Summary:         "detect assignments that discard too many results",
		Explanation:     "Discarding several adjacent results hides the contract of the called function and makes it easy to overlook an important value. Name the results that matter or return a cohesive result type.",
		GoodExample:     "value, metadata, err := load(); _ = metadata",
		BadExample:      "value, _, _, _, err := load()",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (excessiveBlankIdentifiersRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				assignment,
					ok := node.(*ast.AssignStmt)
				if !ok {
					return true
				}
				blanks := 0
				for _, expression := range assignment.Lhs {
					identifier,
						_ := expression.(*ast.Ident)
					if identifier != nil && identifier.Name == "_" {
						blanks++
					}
				}
				if blanks >= 3 {
					pass.Report(assignment, "assignment discards three or more results; name meaningful results or simplify the return contract")
				}
				return true
			},
		)
	}
}

func (taskCommentRule) Meta() Meta {
	return Meta{
		Code:            "task-comment",
		Summary:         "surface TODO, FIXME, and BUG comments",
		Explanation:     "Task markers in source are easy to forget and invisible to normal issue tracking. Resolve the task or link it to an owned work item before enforcing this advisory check.",
		GoodExample:     "// Retry only errors classified as transient.",
		BadExample:      "// TODO: decide which errors should be retried.",
		DefaultSeverity: diagnostic.SeverityNote,
	}
}

func (taskCommentRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if marker := taskMarker(comment.Text); marker != "" {
					pass.Report(comment, marker+" comment should be resolved or linked to an owned work item")
				}
			}
		}
	}
}

func taskMarker(text string) string {
	fields := strings.FieldsFunc(text, func(character rune) bool {
		return !unicode.IsLetter(character)
	})
	for _, field := range fields {
		switch field {
		case "TODO", "FIXME", "BUG":
			return field
		}
	}
	return ""
}

func (docCommentPeriodRule) Meta() Meta {
	return Meta{
		Code:            "doc-comment-period",
		Summary:         "require declaration documentation to end with punctuation",
		Explanation:     "Complete documentation sentences are easier to read in generated API references. Documentation attached to packages, exported declarations, and exported specs should end with terminal punctuation.",
		GoodExample:     "// Client sends requests.",
		BadExample:      "// Client sends requests",
		DefaultSeverity: diagnostic.SeverityNote,
	}
}

func (docCommentPeriodRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		reported := make(map[*ast.CommentGroup]bool)
		reportDocCommentPeriod(pass, file.Doc, reported)
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Name.IsExported() {
					reportDocCommentPeriod(pass, declaration.Doc, reported)
				}
			case *ast.GenDecl:
				for _, raw := range declaration.Specs {
					switch spec := raw.(type) {
					case *ast.TypeSpec:
						if spec.Name.IsExported() {
							doc := spec.Doc
							if doc == nil {
								doc = declaration.Doc
							}
							reportDocCommentPeriod(pass, doc, reported)
						}
					case *ast.ValueSpec:
						exported := false
						for _, name := range spec.Names {
							exported = exported || name.IsExported()
						}
						if exported {
							doc := spec.Doc
							if doc == nil {
								doc = declaration.Doc
							}
							reportDocCommentPeriod(pass, doc, reported)
						}
					}
				}
			}
		}
	}
}

func reportDocCommentPeriod(pass *Pass, group *ast.CommentGroup, reported map[*ast.CommentGroup]bool) {
	if group == nil || reported[group] {
		return
	}
	reported[group] = true
	text := strings.TrimSpace(group.Text())
	if text == "" {
		return
	}
	last := text[len(text)-1]
	if last == '.' || last == '!' || last == '?' || last == ':' {
		return
	}
	pass.Report(group, "documentation comment should end with punctuation")
}

func (errorTypeNamingRule) Meta() Meta {
	return Meta{
		Code:            "error-type-naming",
		Summary:         "name error implementations with an Error suffix",
		Explanation:     "A named type whose value or pointer method set implements error should use an Error suffix so its role is recognizable at API boundaries and in type assertions.",
		GoodExample:     "type ParseError struct { Offset int }",
		BadExample:      "type ParseFailure struct { Offset int } // implements Error() string",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (errorTypeNamingRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				spec,
					ok := node.(*ast.TypeSpec)
				if !ok || spec.Assign.IsValid() || strings.HasSuffix(spec.Name.Name, "Error") {
					return true
				}
				object,
					_ := pass.TypesInfo.Defs[spec.Name].(*types.TypeName)
				if object == nil {
					return true
				}
				valueType := object.Type()
				errorInterface,
					_ := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
				if types.Implements(valueType, errorInterface) || types.Implements(types.NewPointer(valueType), errorInterface) {
					pass.Report(spec.Name, "error implementation type should have an Error suffix")
				}
				return true
			},
		)
	}
}

func (standardHTTPMethodConstantRule) Meta() Meta {
	return Meta{
		Code:            "standard-http-method-constant",
		Summary:         "prefer net/http method constants",
		Explanation:     "Using net/http method constants avoids spelling drift and makes the protocol role of an argument explicit. This check is limited to method arguments of net/http request constructors.",
		GoodExample:     "http.NewRequest(http.MethodGet, endpoint, nil)",
		BadExample:      "http.NewRequest(\"GET\", endpoint, nil)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (standardHTTPMethodConstantRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				call,
					ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				function := calledFunction(pass.TypesInfo, call.Fun)
				if function == nil || function.Pkg() == nil || function.Pkg().Path() != "net/http" {
					return true
				}
				argument := -1
				switch function.Name() {
				case "NewRequest":
					argument = 0
				case "NewRequestWithContext":
					argument = 1
				}
				if argument < 0 || argument >= len(call.Args) {
					return true
				}
				if standardHTTPMethodObject(pass, call.Args[argument]) {
					return true
				}
				value := pass.TypesInfo.Types[call.Args[argument]].Value
				if value == nil || value.Kind() != constant.String {
					return true
				}
				method := constant.StringVal(value)
				name := standardHTTPMethods[method]
				if name != "" {
					pass.Report(call.Args[argument], "replace the HTTP method literal with http."+name)
				}
				return true
			},
		)
	}
}

func standardHTTPMethodObject(pass *Pass, expression ast.Expr) bool {
	var object types.Object
	switch expression := ast.Unparen(expression).(type) {
	case *ast.Ident:
		object = pass.TypesInfo.ObjectOf(expression)
	case *ast.SelectorExpr:
		object = pass.TypesInfo.ObjectOf(expression.Sel)
	}
	constantObject, _ := object.(*types.Const)
	return constantObject != nil && constantObject.Pkg() != nil && constantObject.Pkg().Path() == "net/http" && strings.HasPrefix(constantObject.Name(), "Method")
}

func (weakCryptographyRule) Meta() Meta {
	return Meta{
		Code:            "weak-cryptography",
		Summary:         "detect deprecated cryptographic primitives",
		Explanation:     "MD5, SHA-1, DES, 3DES, and RC4 are unsuitable for new security-sensitive designs. Use a modern authenticated primitive or a collision-resistant hash; explicitly exclude checksum-only legacy code when necessary.",
		GoodExample:     "sum := sha256.Sum256(data)",
		BadExample:      "sum := md5.Sum(data)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (weakCryptographyRule) Run(pass *Pass) {
	for _, file := range pass.Files {
		ast.Inspect(
			file,
			func(node ast.Node) bool {
				call,
					ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				function := calledFunction(pass.TypesInfo, call.Fun)
				if function == nil || function.Pkg() == nil {
					return true
				}
				switch function.Pkg().Path() {
				case "crypto/md5",
					"crypto/sha1",
					"crypto/des",
					"crypto/rc4":
					pass.Report(call, "deprecated cryptographic primitive "+function.Pkg().Path()+"."+function.Name()+" should not protect new data")
				}
				return true
			},
		)
	}
}
