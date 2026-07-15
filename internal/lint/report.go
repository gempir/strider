package lint

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/gempir/strider/internal/diagnostic"
)

func ReportText(writer io.Writer, diagnostics []diagnostic.Diagnostic) error {
	for _, item := range diagnostics {
		if _, err := fmt.Fprintf(
			writer,
			"%s:%d:%d: %s[%s]: %s\n",
			item.File,
			item.Start.Line,
			item.Start.Column,
			item.Severity,
			item.Code,
			item.Message,
		); err != nil {
			return err
		}
	}
	return nil
}

func ReportJSON(writer io.Writer, diagnostics []diagnostic.Diagnostic) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(diagnostics)
}
