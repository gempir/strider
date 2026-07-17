package report

import (
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gempir/strider/internal/diagnostic"
)

type htmlReport struct {
	Title string
	Diagnostics []htmlDiagnostic
	Total int
	Errors int
	Warnings int
	Notes int
}

type htmlDiagnostic struct {
	Code string
	Message string
	Severity diagnostic.Severity
	File string
	Location string
	Source []htmlSourceLine
	Notes []diagnostic.Note
	Fixes []diagnostic.Fix
}

type htmlSourceLine struct {
	Number int
	Before string
	Highlight string
	After string
	Current bool
}

// HTMLOptions configures a self-contained diagnostic report.
type HTMLOptions struct {
	Title string
	SourceRoot string
}

// HTML writes a deterministic, self-contained diagnostic report. The report
// has no external assets and can be saved directly from stdout.
func HTML(writer io.Writer, title string, diagnostics []diagnostic.Diagnostic) error {
	return HTMLWithOptions(writer, HTMLOptions{Title: title}, diagnostics)
}

// HTMLWithOptions writes a deterministic, self-contained diagnostic report
// and resolves relative diagnostic filenames against SourceRoot.
func HTMLWithOptions(writer io.Writer, options HTMLOptions, diagnostics []diagnostic.Diagnostic) error {
	data := htmlReport{
		Title: options.Title,
		Diagnostics: make([]htmlDiagnostic, 0, len(diagnostics)),
		Total: len(diagnostics),
	}
	sources := make(map[string][]string)
	missing := make(map[string]bool)
	for _, item := range diagnostics {
		switch item.Severity {
		case diagnostic.SeverityError:
			data.Errors++
		case diagnostic.SeverityNote:
			data.Notes++
		default:
			data.Warnings++
		}
		data.Diagnostics = append(
			data.Diagnostics,
			htmlDiagnostic{
				Code: item.Code,
				Message: item.Message,
				Severity: item.Severity,
				File: item.File,
				Location: htmlLocation(item),
				Source: htmlSourceContext(item, options.SourceRoot, sources, missing),
				Notes: item.Notes,
				Fixes: item.Fixes,
			},
		)
	}
	return htmlTemplate.Execute(writer, data)
}

func htmlLocation(item diagnostic.Diagnostic) string {
	location := item.File
	if item.Start.Line > 0 {
		location += ":" + strconv.Itoa(item.Start.Line)
		if item.Start.Column > 0 {
			location += ":" + strconv.Itoa(item.Start.Column)
		}
	}
	return location
}

func htmlSourceContext(
	item diagnostic.Diagnostic,
	root string,
	cache map[string][]string,
	missing map[string]bool,
) []htmlSourceLine {
	if item.Start.Line <= 0 {
		return nil
	}
	filename := item.File
	if root != "" && !filepath.IsAbs(filename) {
		filename = filepath.Join(root, filepath.FromSlash(filename))
	}
	lines, ok := cache[filename]
	if !ok && !missing[filename] {
		contents, err := os.ReadFile(filename)
		if err != nil {
			missing[filename] = true
		} else {
			lines = strings.Split(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n")
			cache[filename] = lines
		}
	}
	if item.Start.Line > len(lines) {
		return nil
	}
	first := max(1, item.Start.Line - 1)
	last := min(len(lines), item.Start.Line + 1)
	result := make([]htmlSourceLine, 0, last - first + 1)
	for number := first; number <= last; number++ {
		line := htmlSourceLine{Number: number}
		contents := lines[number - 1]
		if number != item.Start.Line {
			line.Before = contents
			result = append(result, line)
			continue
		}
		line.Current = true
		start := min(max(item.Start.Column - 1, 0), len(contents))
		end := len(contents)
		if item.End.Line == item.Start.Line && item.End.Column > item.Start.Column {
			end = min(max(item.End.Column - 1, start), len(contents))
		}
		if end == start && start < len(contents) {
			_, width := utf8.DecodeRuneInString(contents[start:])
			end += width
		}
		line.Before = contents[:start]
		line.Highlight = contents[start:end]
		line.After = contents[end:]
		result = append(result, line)
	}
	return result
}

var htmlTemplate = template.Must(
	template.New("report").Parse(
		`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
:root{color-scheme:light dark;--bg:#f6f7fb;--panel:#fff;--text:#172033;--muted:#64748b;--line:#dbe1ea;--accent:#4f46e5;--error:#dc2626;--warning:#d97706;--note:#0284c7;--code:#f1f5f9;--marked:#fef08a}*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:15px/1.5 system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}main{width:min(1120px,calc(100% - 32px));margin:48px auto 80px}h1{margin:0;font-size:clamp(2rem,5vw,3.25rem);letter-spacing:-.04em}.lede{color:var(--muted);margin:.35rem 0 2rem}.summary{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin-bottom:24px}.card{background:var(--panel);border:1px solid var(--line);border-radius:12px;padding:16px}.card strong{display:block;font-size:1.8rem;line-height:1.1}.card span{color:var(--muted)}.card.error strong{color:var(--error)}.card.warning strong{color:var(--warning)}.card.note strong{color:var(--note)}.controls{display:grid;grid-template-columns:1fr 180px;gap:12px;margin-bottom:16px}.controls input,.controls select{width:100%;border:1px solid var(--line);border-radius:9px;background:var(--panel);color:var(--text);font:inherit;padding:10px 12px}.diagnostic{background:var(--panel);border:1px solid var(--line);border-left:4px solid var(--warning);border-radius:10px;margin:10px 0;overflow:hidden}.diagnostic[data-severity="error"]{border-left-color:var(--error)}.diagnostic[data-severity="note"]{border-left-color:var(--note)}summary{display:grid;grid-template-columns:auto auto 1fr;gap:10px;align-items:baseline;cursor:pointer;padding:15px 16px;list-style:none}summary::-webkit-details-marker{display:none}.severity{font-size:.72rem;font-weight:800;letter-spacing:.08em;text-transform:uppercase;color:var(--warning)}[data-severity="error"] .severity{color:var(--error)}[data-severity="note"] .severity{color:var(--note)}code{font:13px/1.5 ui-monospace,SFMono-Regular,Consolas,monospace}.rule{background:var(--code);border-radius:5px;padding:2px 6px}.message{font-weight:650}.details{border-top:1px solid var(--line);padding:14px 16px 18px}.location{color:var(--muted);margin:0 0 10px}.source{background:var(--code);border-radius:7px;margin:0;padding:9px 0;overflow:auto}.source-line{display:grid;grid-template-columns:4rem 1fr;min-width:max-content;padding:2px 13px}.source-line.current{background:color-mix(in srgb,var(--accent) 10%,transparent)}.line-number{color:var(--muted);padding-right:14px;text-align:right;user-select:none}.source code{white-space:pre}.source mark{background:var(--marked);color:#172033;border-radius:2px;padding:1px 0;box-shadow:0 0 0 1px color-mix(in srgb,var(--warning) 45%,transparent)}.extras{margin:12px 0 0;padding-left:20px}.empty{background:var(--panel);border:1px solid var(--line);border-radius:12px;text-align:center;padding:48px 20px}.hidden{display:none}@media(max-width:700px){main{margin-top:28px}.summary{grid-template-columns:repeat(2,1fr)}.controls{grid-template-columns:1fr}summary{grid-template-columns:auto 1fr}.message{grid-column:1/-1}}@media(prefers-color-scheme:dark){:root{--bg:#0d1117;--panel:#161b22;--text:#e6edf3;--muted:#8b949e;--line:#30363d;--code:#21262d;--marked:#facc15}}
</style>
</head>
<body>
<main>
<header><h1>{{.Title}}</h1><p class="lede">Deterministic diagnostics generated by Strider.</p></header>
<section class="summary" aria-label="Diagnostic summary">
<div class="card"><strong>{{.Total}}</strong><span>Total</span></div>
<div class="card error"><strong>{{.Errors}}</strong><span>Errors</span></div>
<div class="card warning"><strong>{{.Warnings}}</strong><span>Warnings</span></div>
<div class="card note"><strong>{{.Notes}}</strong><span>Notes</span></div>
</section>
{{if .Diagnostics}}
<section class="controls" aria-label="Report filters"><input id="search" type="search" placeholder="Search code, message, or file…" aria-label="Search diagnostics"><select id="severity" aria-label="Filter by severity"><option value="">All severities</option><option value="error">Errors</option><option value="warning">Warnings</option><option value="note">Notes</option></select></section>
<section id="diagnostics">
{{range .Diagnostics}}<details class="diagnostic" data-severity="{{.Severity}}" data-search="{{.Code}} {{.Message}} {{.File}}">
<summary><span class="severity">{{.Severity}}</span><code class="rule">{{.Code}}</code><span class="message">{{.Message}}</span></summary>
<div class="details"><p class="location"><code>{{.Location}}</code></p>{{if .Source}}<div class="source">{{range .Source}}<div class="source-line{{if .Current}} current{{end}}"><span class="line-number">{{.Number}}</span><code>{{.Before}}{{if .Highlight}}<mark>{{.Highlight}}</mark>{{end}}{{.After}}</code></div>{{end}}</div>{{end}}{{if or .Notes .Fixes}}<ul class="extras">{{range .Notes}}<li><strong>Note:</strong> {{.Message}}</li>{{end}}{{range .Fixes}}<li><strong>Fix ({{.Safety}}):</strong> {{.Message}}</li>{{end}}</ul>{{end}}</div>
</details>{{end}}
</section>
{{else}}<section class="empty"><strong>No diagnostics found.</strong><br><span class="lede">This run completed cleanly.</span></section>{{end}}
</main>
<script>
(()=>{const search=document.querySelector('#search'),severity=document.querySelector('#severity'),items=[...document.querySelectorAll('.diagnostic')];if(!search)return;const filter=()=>{const query=search.value.toLocaleLowerCase(),level=severity.value;for(const item of items)item.classList.toggle('hidden',!!((level&&item.dataset.severity!==level)||(query&&!item.dataset.search.toLocaleLowerCase().includes(query))))};search.addEventListener('input',filter);severity.addEventListener('change',filter)})();
</script>
</body>
</html>
`,
	),
)
