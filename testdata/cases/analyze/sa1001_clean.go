package analyze_cases

import texttemplate "text/template"

func parseValidAndConfiguredTemplates() {
	texttemplate.New("").Parse("{{.Name}}")
	template := texttemplate.New("")
	template.Parse(invalidTemplate)
	texttemplate.New("").Delims("[[", "]]").Parse("{{broken-}}")
}
