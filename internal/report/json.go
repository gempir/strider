package report

import (
	"encoding/json"
	"io"

	"github.com/gempir/strider/internal/diagnostic"
)

// JSON writes diagnostics as indented JSON without HTML escaping.
func JSON(writer io.Writer, diagnostics []diagnostic.Diagnostic) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(diagnostics)
}
