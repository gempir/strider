package analyze_cases

import (
	htmltemplate "html/template"
	texttemplate "text/template"
)

const invalidTemplate = "{{.Name}} {{.LastName}"

func parseInvalidTemplates() {
	texttemplate.New("").Parse(invalidTemplate)
	htmltemplate.New("").Parse(invalidTemplate)
}
