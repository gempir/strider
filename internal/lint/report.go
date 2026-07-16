package lint

import (
	"encoding/json"
	"io"

	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/report"
	"github.com/gempir/strider/internal/ui"
)

func ReportText(writer io.Writer, diagnostics []diagnostic.Diagnostic, colorMode ui.ColorMode) error {
	return report.Text(writer, diagnostics, colorMode)
}

func ReportJSON(writer io.Writer, diagnostics []diagnostic.Diagnostic) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(diagnostics)
}
