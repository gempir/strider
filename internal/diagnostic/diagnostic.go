// Package diagnostic defines the shared issue and edit model used by Strider's
// source-code engines.
package diagnostic

import "go/token"

type Severity string

const (
	SeverityNote    Severity = "note"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// ValidSeverity reports whether severity is one of Strider's supported
// diagnostic levels.
func ValidSeverity(severity Severity) bool {
	switch severity {
	case SeverityNote, SeverityWarning, SeverityError:
		return true
	default:
		return false
	}
}

// AtLeast reports whether severity meets minimum. Diagnostic severity is
// ordered note < warning < error.
func (severity Severity) AtLeast(minimum Severity) bool {
	if !ValidSeverity(severity) || !ValidSeverity(minimum) {
		return false
	}
	return severityRank(severity) >= severityRank(minimum)
}

func severityRank(severity Severity) uint8 {
	switch severity {
	case SeverityNote:
		return 1
	case SeverityWarning:
		return 2
	case SeverityError:
		return 3
	default:
		return 0
	}
}

type Safety string

const (
	Safe              Safety = "safe"
	PotentiallyUnsafe Safety = "potentially-unsafe"
	Unsafe            Safety = "unsafe"
)

type Note struct {
	Message string         `json:"message"`
	Start   token.Position `json:"start"`
	End     token.Position `json:"end"`
}

type TextEdit struct {
	Start   int    `json:"start"`
	End     int    `json:"end"`
	NewText string `json:"new_text"`
}

type Fix struct {
	Message string     `json:"message"`
	Safety  Safety     `json:"safety"`
	Edits   []TextEdit `json:"edits,omitempty"`
}

type Diagnostic struct {
	Code     string         `json:"code"`
	Message  string         `json:"message"`
	Severity Severity       `json:"severity"`
	File     string         `json:"file"`
	Start    token.Position `json:"start"`
	End      token.Position `json:"end"`
	Notes    []Note         `json:"notes,omitempty"`
	Fixes    []Fix          `json:"fixes,omitempty"`
}
