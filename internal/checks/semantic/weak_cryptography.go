package semantic

import (
	"go/ast"

	"github.com/gempir/strider/internal/diagnostic"
)

type weakCryptographyCheck struct{}

func (weakCryptographyCheck) Meta() Meta {
	return Meta{
		Code:            "weak-cryptography",
		Summary:         "detect deprecated cryptographic primitives",
		Explanation:     "MD5, SHA-1, DES, 3DES, and RC4 are unsuitable for new security-sensitive designs. Use a modern authenticated primitive or a collision-resistant hash; explicitly exclude checksum-only legacy code when necessary.",
		GoodExample:     "sum := sha256.Sum256(data)",
		BadExample:      "sum := md5.Sum(data)",
		DefaultSeverity: diagnostic.SeverityWarning,
	}
}

func (weakCryptographyCheck) Run(pass *Pass) {
	pass.Inspect(
		[]ast.Node{
			(*ast.CallExpr)(nil),
		},
		func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			function := calledFunction(pass.TypesInfo, call.Fun)
			if function == nil || function.Pkg() == nil {
				return true
			}
			switch function.Pkg().Path() {
			case "crypto/md5", "crypto/sha1", "crypto/des", "crypto/rc4":
				pass.Report(call, "deprecated cryptographic primitive "+function.Pkg().Path()+"."+function.Name()+" should not protect new data")
			}
			return true
		},
	)
}

func (weakCryptographyCheck) Requirements() Requirements {
	return Requirements{
		Stage: AnalysisStageTypes,
	}
}
