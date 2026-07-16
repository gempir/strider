package ui

import (
	"bytes"
	"testing"
)

func TestColorModesAndEnvironmentPrecedence(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	writer := &bytes.Buffer{}
	if NewPalette(writer, ColorAuto).Enabled() {
		t.Fatal("auto color should be disabled for a non-terminal writer")
	}
	if !NewPalette(writer, ColorAlways).Enabled() {
		t.Fatal("always color should be enabled")
	}
	if NewPalette(writer, ColorNever).Enabled() {
		t.Fatal("never color should be disabled")
	}

	t.Setenv("NO_COLOR", "1")
	if NewPalette(writer, ColorAlways).Enabled() {
		t.Fatal("NO_COLOR should disable an always palette")
	}
	t.Setenv("FORCE_COLOR", "1")
	if !NewPalette(writer, ColorNever).Enabled() {
		t.Fatal("FORCE_COLOR should take precedence over mode and NO_COLOR")
	}
	t.Setenv("FORCE_COLOR", "0")
	if NewPalette(writer, ColorAlways).Enabled() {
		t.Fatal("FORCE_COLOR=0 should disable color")
	}
}

func TestPalettePaintsSemanticStyles(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	palette := NewPalette(&bytes.Buffer{}, ColorAlways)
	if got := palette.Error("error"); got != "\x1b[1;31merror\x1b[0m" {
		t.Fatalf("unexpected error style %q", got)
	}
}
