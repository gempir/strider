package semantic

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"github.com/gempir/strider/internal/diagnostic"
)

var knownOperatingSystems = map[string]bool{
	"aix":       true,
	"android":   true,
	"darwin":    true,
	"dragonfly": true,
	"freebsd":   true,
	"illumos":   true,
	"ios":       true,
	"js":        true,
	"linux":     true,
	"netbsd":    true,
	"openbsd":   true,
	"plan9":     true,
	"solaris":   true,
	"wasip1":    true,
	"windows":   true,
}

var unixOperatingSystems = map[string]bool{
	"aix":       true,
	"android":   true,
	"darwin":    true,
	"dragonfly": true,
	"freebsd":   true,
	"illumos":   true,
	"ios":       true,
	"linux":     true,
	"netbsd":    true,
	"openbsd":   true,
	"solaris":   true,
}

var knownArchitectures = map[string]bool{
	"386":      true,
	"amd64":    true,
	"arm":      true,
	"arm64":    true,
	"loong64":  true,
	"mips":     true,
	"mipsle":   true,
	"mips64":   true,
	"mips64le": true,
	"ppc64":    true,
	"ppc64le":  true,
	"riscv64":  true,
	"s390x":    true,
	"wasm":     true,
}

type impossiblePlatformComparisonRule struct{}

func (impossiblePlatformComparisonRule) Meta() Meta {
	return Meta{
		Code:            "impossible-platform-comparison",
		Summary:         "detect GOOS and GOARCH comparisons excluded by build constraints",
		Explanation:     "A file's build constraints limit the operating systems and architectures on which its code can run. Comparing runtime.GOOS or runtime.GOARCH with an excluded target has a fixed result.",
		GoodExample:     `//go:build linux\nif runtime.GOOS == "linux" { use() }`,
		BadExample:      `//go:build linux\nif runtime.GOOS == "windows" { unreachable() }`,
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (impossiblePlatformComparisonRule) Run(pass *Pass) {
	constraints := make(map[*ast.File]constraint.Expr)
	for _, file := range pass.Files {
		buildConstraint, ok := parsedFileConstraint(file)
		if ok {
			constraints[file] = buildConstraint
		}
	}
	pass.Inspect(
		[]ast.Node{
			(*ast.BinaryExpr)(nil),
		},
		func(node ast.Node) bool {
			buildConstraint := constraints[pass.File(node.Pos())]
			if buildConstraint == nil {
				return true
			}
			binary,
				ok := node.(*ast.BinaryExpr)
			if !ok || binary.Op != token.EQL && binary.Op != token.NEQ {
				return true
			}
			kind,
				target,
				ok := platformComparison(pass, binary.X, binary.Y)
			if !ok {
				kind,
					target,
					ok = platformComparison(pass, binary.Y, binary.X)
			}
			if !ok || !knownPlatformTarget(kind, target) {
				return true
			}
			possible,
				checked := platformConstraintPossible(buildConstraint, kind, target)
			if checked && !possible {
				pass.Report(binary, fmt.Sprintf("runtime.%s can never equal %q under this file's build constraints", kind, target))
			}
			return true
		},
	)
}

func parsedFileConstraint(file *ast.File) (constraint.Expr, bool) {
	legacy := make([]constraint.Expr, 0)
	for _, group := range file.Comments {
		if group.Pos() > file.Package {
			break
		}
		for _, comment := range group.List {
			text := strings.TrimSpace(comment.Text)
			if strings.HasPrefix(text, "//go:build ") {
				expression, err := constraint.Parse(text)
				return expression, err == nil
			}
			if strings.HasPrefix(text, "// +build ") {
				expression, err := constraint.Parse(text)
				if err == nil {
					legacy = append(legacy, expression)
				}
			}
		}
	}
	if len(legacy) == 0 {
		return nil, false
	}
	expression := legacy[0]
	for _, next := range legacy[1:] {
		expression = &constraint.AndExpr{
			X: expression,
			Y: next,
		}
	}
	return expression, true
}

func platformComparison(pass *Pass, platform, literal ast.Expr) (string, string, bool) {
	selector, ok := ast.Unparen(platform).(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	object, ok := pass.TypesInfo.ObjectOf(selector.Sel).(*types.Const)
	if !ok || object.Pkg() == nil || object.Pkg().Path() != "runtime" || object.Name() != "GOOS" && object.Name() != "GOARCH" {
		return "", "", false
	}
	value := pass.TypesInfo.Types[literal].Value
	if value == nil || value.Kind() != constant.String {
		return "", "", false
	}
	return object.Name(), constant.StringVal(value), true
}

func platformConstraintPossible(expression constraint.Expr, kind, target string) (bool, bool) {
	unknown := make(map[string]int)
	evaluateSpecial := func(tag string) (bool, bool) {
		if kind == "GOOS" {
			return matchOperatingSystemTag(tag, target)
		}
		return matchArchitectureTag(tag, target)
	}
	possible := expression.Eval(
		func(tag string) bool {
			matched,
				special := evaluateSpecial(tag)
			if !special {
				if _,
					exists := unknown[tag]; !exists {
					unknown[tag] = len(unknown)
				}
			}
			return matched
		},
	)
	if possible || len(unknown) == 0 {
		return possible, true
	}
	if len(unknown) > 10 {
		return false, false
	}
	for bits := 0; bits < 1<<len(unknown); bits++ {
		if expression.Eval(func(tag string) bool {
			matched, special := evaluateSpecial(tag)
			if special {
				return matched
			}
			return bits&(1<<unknown[tag]) != 0
		}) {
			return true, true
		}
	}
	return false, true
}

func matchOperatingSystemTag(tag, target string) (bool, bool) {
	switch tag {
	case "aix", "android", "dragonfly", "freebsd", "illumos", "ios", "js", "netbsd", "openbsd", "plan9", "wasip1", "windows":
		return target == tag, true
	case "darwin":
		return target == "darwin" || target == "ios", true
	case "linux":
		return target == "linux" || target == "android", true
	case "solaris":
		return target == "solaris" || target == "illumos", true
	case "unix":
		return unixOperatingSystems[target], true
	default:
		return false, false
	}
}

func matchArchitectureTag(tag, target string) (bool, bool) {
	if knownArchitectures[tag] {
		return target == tag, true
	}
	return false, false
}

func knownPlatformTarget(kind, target string) bool {
	if kind == "GOOS" {
		return knownOperatingSystems[target]
	}
	return knownArchitectures[target]
}

func (impossiblePlatformComparisonRule) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
