// Package ui provides the terminal styling shared by Strider's commands.
package ui

import (
	"io"
	"os"
	"strings"
)

const (
	ColorAuto   ColorMode = "auto"
	ColorAlways ColorMode = "always"
	ColorNever  ColorMode = "never"
)

// ColorMode controls when ANSI styling is emitted.
type ColorMode string

// Palette applies semantic terminal styles to strings.
type Palette struct {
	enabled bool
}

// ValidColorMode reports whether value is an accepted color mode.
func ValidColorMode(value string) bool {
	return value == string(ColorAuto) || value == string(ColorAlways) || value == string(ColorNever)
}

// NewPalette resolves mode for writer. FORCE_COLOR and NO_COLOR follow the
// conventions used by other developer tools, including Mago.
func NewPalette(writer io.Writer, mode ColorMode) Palette {
	return Palette{
		enabled: colorsEnabled(writer, mode),
	}
}

// Enabled reports whether this palette emits ANSI escape sequences.
func (palette Palette) Enabled() bool {
	return palette.enabled
}

func colorsEnabled(writer io.Writer, mode ColorMode) bool {
	if value, ok := os.LookupEnv("FORCE_COLOR"); ok && value != "" {
		return value != "0"
	}
	if value, ok := os.LookupEnv("NO_COLOR"); ok && value != "" {
		return false
	}
	switch mode {
	case ColorAlways:
		return true
	case ColorNever:
		return false
	default:
		file, ok := writer.(*os.File)
		if !ok || strings.EqualFold(os.Getenv("TERM"), "dumb") {
			return false
		}
		info, err := file.Stat()
		return err == nil && info.Mode()&os.ModeCharDevice != 0
	}
}

func (palette Palette) paint(code, text string) string {
	if !palette.enabled || text == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (palette Palette) Bold(text string) string {
	return palette.paint("1", text)
}

func (palette Palette) White(text string) string {
	return palette.paint("1;37", text)
}

func (palette Palette) Muted(text string) string {
	return palette.paint("2", text)
}

func (palette Palette) Error(text string) string {
	return palette.paint("1;31", text)
}

func (palette Palette) Warning(text string) string {
	return palette.paint("1;33", text)
}

func (palette Palette) Note(text string) string {
	return palette.paint("1;34", text)
}

func (palette Palette) Success(text string) string {
	return palette.paint("38;5;10", text)
}

func (palette Palette) Path(text string) string {
	return palette.paint("1;36", text)
}

func (palette Palette) Code(text string) string {
	return palette.paint("38;5;2", text)
}

func (palette Palette) Accent(text string) string {
	return palette.paint("1;35", text)
}

func (palette Palette) Added(text string) string {
	return palette.paint("38;5;10", text)
}

func (palette Palette) Removed(text string) string {
	return palette.paint("31", text)
}

func (palette Palette) Hunk(text string) string {
	return palette.paint("36", text)
}
