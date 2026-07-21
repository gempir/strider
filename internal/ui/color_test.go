package ui

import (
	"bytes"
	"testing"
)

func TestColorModesAndEnvironmentPrecedence(t *testing.T) {
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("NO_COLOR", "")
	writer := &bytes.Buffer{}
	if NewPalette(writer, ColorAuto).enabled {
		t.Fatal("auto color should be disabled for a non-terminal writer")
	}
	if !NewPalette(writer, ColorAlways).enabled {
		t.Fatal("always color should be enabled")
	}
	if NewPalette(writer, ColorNever).enabled {
		t.Fatal("never color should be disabled")
	}

	t.Setenv("NO_COLOR", "1")
	if NewPalette(writer, ColorAlways).enabled {
		t.Fatal("NO_COLOR should disable an always palette")
	}
	t.Setenv("FORCE_COLOR", "1")
	if !NewPalette(writer, ColorNever).enabled {
		t.Fatal("FORCE_COLOR should take precedence over mode and NO_COLOR")
	}
	t.Setenv("FORCE_COLOR", "0")
	if NewPalette(writer, ColorAlways).enabled {
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
	if got := palette.Warning("warning"); got != "\x1b[1;33mwarning\x1b[0m" {
		t.Fatalf("unexpected warning style %q", got)
	}
	if got := palette.Note("note"); got != "\x1b[1;34mnote\x1b[0m" {
		t.Fatalf("unexpected note style %q", got)
	}
	if got := palette.Code("code"); got != "\x1b[38;5;2mcode\x1b[0m" {
		t.Fatalf("unexpected code style %q", got)
	}
	if got := palette.Success("success"); got != "\x1b[38;5;10msuccess\x1b[0m" {
		t.Fatalf("unexpected success style %q", got)
	}
	if got := palette.Accent("accent"); got != "\x1b[1;35maccent\x1b[0m" {
		t.Fatalf("unexpected accent style %q", got)
	}
}
