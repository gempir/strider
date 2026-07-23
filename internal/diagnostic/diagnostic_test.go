package diagnostic

import (
	"go/token"
	"testing"
)

func TestSeverityOrdering(t *testing.T) {
	severities := []Severity{
		SeverityNone,
		SeverityNote,
		SeverityWarning,
		SeverityError,
	}
	for index, severity := range severities {
		if !ValidSeverity(severity) {
			t.Fatalf("known severity %q is invalid", severity)
		}
		for minimumIndex, minimum := range severities {
			if got, want := severity.AtLeast(minimum), index >= minimumIndex; got != want {
				t.Errorf("%s.AtLeast(%s) = %t, want %t", severity, minimum, got, want)
			}
		}
	}
	if ValidSeverity("") || ValidSeverity("fatal") {
		t.Fatal("unknown severity reported as valid")
	}
	if SeverityError.AtLeast("") || Severity("fatal").AtLeast(SeverityNote) {
		t.Fatal("unknown severity participated in ordering")
	}
}

func TestSortUsesEveryPresentationKey(t *testing.T) {
	base := Diagnostic{
		File:     "b.go",
		Code:     "b",
		Message:  "b",
		Severity: SeverityWarning,
		Start: token.Position{
			Offset: 20,
		},
		End: token.Position{
			Offset: 40,
		},
	}
	tests := []struct {
		name   string
		change func(*Diagnostic)
	}{
		{
			name: "file",
			change: func(item *Diagnostic) {
				item.File = "a.go"
			},
		},
		{
			name: "start offset",
			change: func(item *Diagnostic) {
				item.Start.Offset = 10
			},
		},
		{
			name: "code",
			change: func(item *Diagnostic) {
				item.Code = "a"
			},
		},
		{
			name: "message",
			change: func(item *Diagnostic) {
				item.Message = "a"
			},
		},
		{
			name: "end offset",
			change: func(item *Diagnostic) {
				item.End.Offset = 30
			},
		},
		{
			name: "severity",
			change: func(item *Diagnostic) {
				item.Severity = SeverityNote
			},
		},
	}
	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				earlier := base
				test.change(&earlier)
				diagnostics := []Diagnostic{
					base,
					earlier,
				}
				Sort(diagnostics)
				if Compare(diagnostics[0], earlier) != 0 || Compare(earlier, base) >= 0 {
					t.Fatalf("got %#v, want changed diagnostic first", diagnostics)
				}
			},
		)
	}
}
