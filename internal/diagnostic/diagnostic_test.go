package diagnostic

import "testing"

func TestSeverityOrdering(t *testing.T) {
	severities := []Severity{SeverityNote, SeverityWarning, SeverityError}
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
