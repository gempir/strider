// Package diagnostic defines the shared issue and edit model used by Strider's
// source-code engines.
package diagnostic

import "go/token"

type Severity string

const (
	SeverityNote Severity = "note"
	SeverityWarning Severity = "warning"
	SeverityError Severity = "error"
)

type Safety string

const (
	Safe Safety = "safe"
	PotentiallyUnsafe Safety = "potentially-unsafe"
	Unsafe Safety = "unsafe"
)

type Note struct {
	Message string `json:"message"`
	Start token.Position `json:"start"`
	End token.Position `json:"end"`
}

type TextEdit struct {
	Start int `json:"start"`
	End int `json:"end"`
	NewText string `json:"new_text"`
}

type Fix struct {
	Message string `json:"message"`
	Safety Safety `json:"safety"`
	Edits []TextEdit `json:"edits"`
}

type Diagnostic struct {
	Code string `json:"code"`
	Message string `json:"message"`
	Severity Severity `json:"severity"`
	File string `json:"file"`
	Start token.Position `json:"start"`
	End token.Position `json:"end"`
	Notes []Note `json:"notes,omitempty"`
	Fixes []Fix `json:"fixes,omitempty"`
}
