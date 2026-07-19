// Package report renders shared human-readable diagnostics.
package report

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/ui"
)

// Text writes rich, source-annotated diagnostics and a severity summary.
func Text(writer io.Writer, diagnostics []diagnostic.Diagnostic, colorMode ui.ColorMode) error {
	return TextWithOptions(writer, diagnostics, colorMode, TextOptions{})
}

// TextOptions controls which parts of a text report are emitted.
type TextOptions struct {
	SummaryOnly bool
}

// TextWithOptions writes diagnostics according to options.
func TextWithOptions(writer io.Writer, diagnostics []diagnostic.Diagnostic, colorMode ui.ColorMode, options TextOptions) error {
	palette := ui.NewPalette(writer, colorMode)
	sources := make(map[string][]string)
	missing := make(map[string]bool)
	counts := make(map[diagnostic.Severity]int)
	fixCounts := make(map[fixability]int)
	for _, item := range diagnostics {
		counts[item.Severity]++
		fixCounts[diagnosticFixability(item)]++
	}
	if options.SummaryOnly {
		if len(diagnostics) != 0 {
			if err := writeCheckCounts(writer, diagnostics, palette, false); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintln(writer, summary(diagnostics, counts, fixCounts, palette))
		return err
	}

	for index, item := range diagnostics {
		if index != 0 {
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}
		if err := writeDiagnostic(writer, item, palette, sources, missing); err != nil {
			return err
		}
	}
	if len(diagnostics) == 0 {
		_, err := fmt.Fprintln(writer, summary(diagnostics, counts, fixCounts, palette))
		return err
	}
	if err := writeCheckCounts(writer, diagnostics, palette, true); err != nil {
		return err
	}
	_, err := fmt.Fprintln(writer, summary(diagnostics, counts, fixCounts, palette))
	return err
}

type checkCount struct {
	code       string
	count      int
	severity   diagnostic.Severity
	fixability fixability
}

type fixability uint8

const (
	notFixable fixability = iota
	safelyFixable
	unsafelyFixable
)

func writeCheckCounts(writer io.Writer, diagnostics []diagnostic.Diagnostic, palette ui.Palette, leadingBlank bool) error {
	byCode := make(map[string]checkCount)
	codeWidth := 0
	for _, item := range diagnostics {
		entry := byCode[item.Code]
		entry.code = item.Code
		entry.count++
		if item.Severity.AtLeast(entry.severity) || entry.severity == "" {
			entry.severity = item.Severity
		}
		entry.fixability = max(entry.fixability, diagnosticFixability(item))
		byCode[item.Code] = entry
		codeWidth = max(codeWidth, utf8.RuneCountInString(item.Code)+fixabilityWidth(entry.fixability))
	}
	entries := make([]checkCount, 0, len(byCode))
	for _, entry := range byCode {
		entries = append(entries, entry)
	}
	sort.Slice(
		entries,
		func(left, right int) bool {
			if entries[left].count != entries[right].count {
				return entries[left].count > entries[right].count
			}
			return entries[left].code < entries[right].code
		},
	)
	if leadingBlank {
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		marker := fixabilityMarker(entry.fixability, palette)
		padding := codeWidth - utf8.RuneCountInString(entry.code) - fixabilityWidth(entry.fixability)
		code := styledSeverity(entry.severity, entry.code, palette) + marker + strings.Repeat(" ", padding)
		count := styledSeverity(entry.severity, strconv.Itoa(entry.count), palette)
		if _, err := fmt.Fprintf(writer, "%s  %s\n", code, count); err != nil {
			return err
		}
	}
	return nil
}

func writeDiagnostic(writer io.Writer, item diagnostic.Diagnostic, palette ui.Palette, sources map[string][]string, missing map[string]bool) error {
	code := styledSeverity(item.Severity, item.Code, palette) + fixabilityMarker(diagnosticFixability(item), palette)
	if _, err := fmt.Fprintf(writer, "%s: %s\n", code, palette.Bold(item.Message)); err != nil {
		return err
	}
	lines := sourceLines(item.File, sources, missing)
	contextStart, contextEnd := sourceContext(item.Start.Line, len(lines))
	widthLine := max(item.Start.Line, 1)
	if contextEnd > 0 {
		widthLine = contextEnd
	}
	location := fmt.Sprintf("%s:%d:%d", item.File, item.Start.Line, item.Start.Column)
	width := len(strconv.Itoa(widthLine))
	if _, err := fmt.Fprintf(writer, "%*s %s %s\n", width, "", palette.Code("┌─"), palette.Code(location)); err != nil {
		return err
	}

	if contextStart > 0 {
		gutter := palette.Code("│")
		if _, err := fmt.Fprintf(writer, "%*s %s\n", width, "", gutter); err != nil {
			return err
		}
		for lineNumber := contextStart; lineNumber <= contextEnd; lineNumber++ {
			line := lines[lineNumber-1]
			if lineNumber == item.Start.Line {
				line = styledSourceSpan(item, line, palette)
			}
			if _, err := fmt.Fprintf(writer, "%*d %s %s\n", width, lineNumber, gutter, line); err != nil {
				return err
			}
		}
	}

	for _, note := range item.Notes {
		if _, err := fmt.Fprintf(writer, "  %s %s: %s\n", palette.Accent("="), palette.Note("note"), note.Message); err != nil {
			return err
		}
	}
	return nil
}

func diagnosticFixability(item diagnostic.Diagnostic) fixability {
	result := notFixable
	for _, fix := range item.Fixes {
		if !fix.Automatic {
			continue
		}
		if fix.Safety == diagnostic.Safe {
			return safelyFixable
		}
		result = unsafelyFixable
	}
	return result
}

func fixabilityWidth(value fixability) int {
	if value == notFixable {
		return 0
	}
	return 1
}

func fixabilityMarker(value fixability, palette ui.Palette) string {
	switch value {
	case safelyFixable:
		return palette.Success("*")
	case unsafelyFixable:
		return palette.Accent("*")
	default:
		return ""
	}
}

func sourceContext(line, lineCount int) (int, int) {
	if line <= 0 || line > lineCount {
		return 0, 0
	}
	return max(line-1, 1), min(line+1, lineCount)
}

func sourceLines(filename string, cache map[string][]string, missing map[string]bool) []string {
	if lines, ok := cache[filename]; ok {
		return lines
	}
	if missing[filename] {
		return nil
	}
	contents, err := os.ReadFile(filename)
	if err != nil {
		missing[filename] = true
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n")
	cache[filename] = lines
	return lines
}

func sourceSpan(item diagnostic.Diagnostic, line string) (int, int) {
	start := min(max(item.Start.Column-1, 0), len(line))
	if item.End.Line == item.Start.Line && item.End.Column > item.Start.Column {
		end := min(max(item.End.Column-1, start), len(line))
		return start, end
	}
	return start, len(line)
}

func styledSourceSpan(item diagnostic.Diagnostic, line string, palette ui.Palette) string {
	start, end := sourceSpan(item, line)
	if start == end {
		return line
	}
	return line[:start] + styledSeverity(item.Severity, line[start:end], palette) + line[end:]
}

func styledSeverity(severity diagnostic.Severity, text string, palette ui.Palette) string {
	switch severity {
	case diagnostic.SeverityError:
		return palette.Error(text)
	case diagnostic.SeverityNote:
		return palette.Note(text)
	case diagnostic.SeverityNone:
		return palette.Muted(text)
	default:
		return palette.Warning(text)
	}
}

func summary(diagnostics []diagnostic.Diagnostic, counts map[diagnostic.Severity]int, fixCounts map[fixability]int, palette ui.Palette) string {
	label := "issues"
	if len(diagnostics) == 1 {
		label = "issue"
	}
	prefix := fmt.Sprintf("%d %s", len(diagnostics), label)
	if len(diagnostics) == 0 {
		return palette.Success(prefix)
	}
	parts := make([]string, 0, 6)
	for _, severity := range []diagnostic.Severity{diagnostic.SeverityError, diagnostic.SeverityWarning, diagnostic.SeverityNote, diagnostic.SeverityNone} {
		if count := counts[severity]; count != 0 {
			part := fmt.Sprintf("%d %s", count, plural(string(severity), count))
			parts = append(parts, styledSeverity(severity, part, palette))
		}
	}
	if count := fixCounts[safelyFixable]; count != 0 {
		parts = append(parts, palette.Success(fmt.Sprintf("%d fixable", count)))
	}
	if count := fixCounts[unsafelyFixable]; count != 0 {
		parts = append(parts, palette.Accent(fmt.Sprintf("%d unsafe fixable", count)))
	}
	return palette.White(prefix+": ") + strings.Join(parts, palette.White(", "))
}

func plural(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}
