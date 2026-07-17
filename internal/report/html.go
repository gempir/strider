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
<script>if(self!==top)document.documentElement.classList.add('embedded')</script>
<style>
:root{color-scheme:dark;--bg:#0d1117;--panel:#121820;--panel-strong:#171e27;--text:#e6edf3;--muted:#8f9baa;--line:#2a3440;--accent:#54d6b0;--error:#ff7b72;--warning:#e6b85c;--note:#79c0ff;--code:#0a0f14;--marked:#f0c75e}*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:15px/1.55 Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;font-feature-settings:"ss01","cv02","cv11"}main{width:min(1440px,calc(100% - 48px));margin:56px auto 88px}header{border-left:2px solid var(--accent);padding-left:20px}h1{margin:0;font-size:clamp(2rem,5vw,3.6rem);line-height:1.04;letter-spacing:-.045em}.lede{color:var(--muted);margin:.6rem 0 2.25rem}.summary{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));margin:0 0 24px;border-block:1px solid var(--line)}.card{min-width:0;padding:18px 20px;border-right:1px solid var(--line)}.card:last-child{border-right:0}.card strong{display:block;font-size:1.85rem;line-height:1.1;letter-spacing:-.035em}.card span{color:var(--muted);font-size:.8rem;letter-spacing:.05em;text-transform:uppercase}.card.error strong{color:var(--error)}.card.warning strong{color:var(--warning)}.card.note strong{color:var(--note)}.controls{position:sticky;top:0;z-index:5;display:grid;grid-template-columns:1fr 190px;gap:10px;margin-bottom:14px;padding:12px 0;background:color-mix(in srgb,var(--bg) 92%,transparent);backdrop-filter:blur(14px)}.controls input,.controls select{width:100%;min-height:43px;border:1px solid var(--line);border-radius:3px;outline:0;background:var(--panel);color:var(--text);font:inherit;padding:9px 12px}.controls input:focus,.controls select:focus{border-color:var(--accent);box-shadow:0 0 0 1px var(--accent)}.diagnostic{background:var(--panel);border:1px solid var(--line);border-left:2px solid var(--warning);border-radius:3px;margin:8px 0;overflow:hidden}.diagnostic:hover{border-color:color-mix(in srgb,var(--accent) 40%,var(--line));border-left-color:var(--warning)}.diagnostic[data-severity="error"]{border-left-color:var(--error)}.diagnostic[data-severity="note"]{border-left-color:var(--note)}summary{display:grid;grid-template-columns:6.2rem minmax(12rem,auto) 1fr;gap:14px;align-items:baseline;cursor:pointer;padding:15px 17px;list-style:none}summary::-webkit-details-marker{display:none}.severity{font-size:.69rem;font-weight:750;letter-spacing:.1em;text-transform:uppercase;color:var(--warning)}[data-severity="error"] .severity{color:var(--error)}[data-severity="note"] .severity{color:var(--note)}code{font:13px/1.5 ui-monospace,SFMono-Regular,Consolas,monospace}.rule{color:var(--accent)}.message{font-weight:620}.details{border-top:1px solid var(--line);padding:15px 17px 18px}.location{color:var(--muted);margin:0 0 10px}.source{background:var(--code);border:1px solid color-mix(in srgb,var(--line) 75%,transparent);border-radius:2px;margin:0;padding:10px 0;overflow:auto}.source-line{display:grid;grid-template-columns:4rem 1fr;min-width:max-content;padding:2px 14px}.source-line.current{background:color-mix(in srgb,var(--accent) 9%,transparent)}.line-number{color:var(--muted);padding-right:14px;text-align:right;user-select:none}.source code{white-space:pre}.source mark{background:color-mix(in srgb,var(--marked) 78%,transparent);color:#111820;border-radius:1px;padding:1px 0}.extras{margin:12px 0 0;padding-left:20px}.empty{border-block:1px solid var(--line);text-align:center;padding:56px 20px}.hidden{display:none}html.embedded main{width:100%;margin:0;padding:24px}html.embedded header{display:none}html.embedded .summary{margin-top:0}:root[data-theme="light"]{color-scheme:light;--bg:#f7f9f8;--panel:#fff;--panel-strong:#f0f4f2;--text:#17201d;--muted:#65736d;--line:#ced9d4;--accent:#087a5c;--error:#c93c37;--warning:#a05a00;--note:#0969a9;--code:#f0f4f2;--marked:#f5d765}@media(max-width:700px){main{width:min(100% - 28px,1440px);margin-top:28px}.summary{grid-template-columns:repeat(2,1fr)}.card:nth-child(2){border-right:0}.card:nth-child(-n+2){border-bottom:1px solid var(--line)}.controls{grid-template-columns:1fr}.controls select{min-height:43px}summary{grid-template-columns:auto 1fr;gap:8px 12px}.message{grid-column:1/-1}html.embedded main{padding:14px}}
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
