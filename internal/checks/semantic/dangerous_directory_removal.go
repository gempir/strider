package semantic

import (
	"fmt"

	"golang.org/x/tools/go/ssa"

	"github.com/gempir/strider/internal/diagnostic"
)

type dangerousDirectoryRemovalRule struct{}

func (dangerousDirectoryRemovalRule) Meta() Meta {
	return Meta{
		Code:            "dangerous-directory-removal",
		Summary:         "detect removal of whole system or user directories",
		Explanation:     "Passing the direct result of os.TempDir or a user directory helper to os.RemoveAll deletes the entire shared directory rather than an application-owned child. This is commonly caused by confusing TempDir with a directory-creation helper or forgetting to append a suffix.",
		GoodExample:     "directory, err := os.MkdirTemp(os.TempDir(), `app-*`); defer os.RemoveAll(directory)",
		BadExample:      "directory := os.TempDir(); defer os.RemoveAll(directory)",
		DefaultSeverity: diagnostic.SeverityError,
	}
}

func (dangerousDirectoryRemovalRule) Run(pass *Pass) {
	for _, call := range pass.staticCallsInPackage("os") {
		if !isStaticFunction(call, "os", "RemoveAll") || len(call.Common().Args) == 0 {
			continue
		}
		helper, kind := dangerousDirectorySource(call.Common().Args[0])
		if helper == "" {
			continue
		}
		pass.Report(
			positionNode{position: call.Pos()},
			fmt.Sprintf(
				"os.RemoveAll receives the entire %s directory from os.%s; remove only an application-owned subdirectory",
				kind,
				helper,
			),
		)
	}
}

func dangerousDirectorySource(value ssa.Value) (string, string) {
	value = flattenSSAValue(unwrapSSAValue(value))
	if extract, ok := value.(*ssa.Extract); ok {
		if extract.Index != 0 {
			return "", ""
		}
		value = flattenSSAValue(extract.Tuple)
	}
	call, ok := value.(*ssa.Call)
	if !ok {
		return "", ""
	}
	callee := call.Common().StaticCallee()
	if callee == nil || callee.Object() == nil || callee.Object().Pkg() == nil || callee.Object().Pkg().Path() != "os" {
		return "", ""
	}
	switch callee.Object().Name() {
	case "TempDir":
		return "TempDir", "temporary"
	case "UserCacheDir":
		return "UserCacheDir", "user cache"
	case "UserConfigDir":
		return "UserConfigDir", "user configuration"
	case "UserHomeDir":
		return "UserHomeDir", "user home"
	default:
		return "", ""
	}
}
