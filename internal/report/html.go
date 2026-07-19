package report

import (
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gempir/strider/internal/diagnostic"
)

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
:root{color-scheme:dark;
--bg:#0e120f;--panel:#121713;--panel-2:#0c100d;--code:#0b0f0c;
--text:#d9dbd2;--muted:#8a8f86;--faint:#565c55;--line:#232b24;--line-soft:#1c231d;
--accent:#3cad4c;--accent-text:#65cc73;--accent-low:#123819;
--error:#e5695e;--warning:#f0b13c;--note:#5794d8;--mark:#f0b13c}
:root[data-theme="light"]{color-scheme:light;
--bg:#faf7f1;--panel:#f4f0e7;--panel-2:#f6f2ea;--code:#eee9de;
--text:#2c322c;--muted:#5c635c;--faint:#8a908a;--line:#ddd6c8;--line-soft:#e7e1d4;
--accent:#3cad4c;--accent-text:#287d34;--accent-low:#daf2de;
--error:#b8433a;--warning:#95660a;--note:#2f6cad;--mark:#f0b13c}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--text);font:13px/1.5 ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif}
code,.mono{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:12px}
main{max-width:1600px;margin:0 auto;padding:0 20px 64px}
a{color:var(--accent-text)}

.timings{display:inline-flex;flex-wrap:wrap;gap:6px 18px;padding-right:18px;border-right:1px solid var(--line)}
.timing strong{color:var(--text)}
.timing small{color:var(--muted);font-weight:400;font-size:11px}

.tally{display:flex;flex-wrap:wrap;align-items:center;gap:6px 18px;padding:10px 0;border-bottom:1px solid var(--line)}
.tally .stat{display:inline-flex;align-items:baseline;gap:6px;white-space:nowrap}
.tally .stat strong{font-size:15px;font-variant-numeric:tabular-nums;letter-spacing:-.02em}
.tally .stat span{color:var(--muted);font-size:11px;letter-spacing:.06em;text-transform:uppercase}
.stat.error strong{color:var(--error)}.stat.warning strong{color:var(--warning)}.stat.note strong{color:var(--note)}.stat.none strong{color:var(--muted)}
.bar{flex:1 1 220px;display:flex;height:6px;min-width:160px;background:var(--line-soft);overflow:hidden}
.bar i{display:block;height:100%}
.bar .b-error{background:var(--error)}.bar .b-warning{background:var(--warning)}.bar .b-note{background:var(--note)}.bar .b-none{background:var(--faint)}
.limit-note{color:var(--muted);margin:10px 0 0}

.layout{display:grid;grid-template-columns:minmax(220px,300px) 1fr;gap:0 28px;align-items:start;margin-top:14px}
aside.rules{position:sticky;top:10px;max-height:calc(100vh - 20px);overflow:auto}
aside.rules h2{margin:0 0 6px;font-size:11px;font-weight:600;letter-spacing:.08em;text-transform:uppercase;color:var(--muted)}
.rules table{width:100%;border-collapse:collapse}
.rules td{padding:3px 0;border-bottom:1px solid var(--line-soft);vertical-align:middle}
.rules td:last-child{text-align:right;color:var(--muted);font-variant-numeric:tabular-nums;padding-left:10px;width:1%}
.rules button{all:unset;display:block;width:100%;cursor:pointer;color:var(--warning)}
.rules button[data-severity="error"]{color:var(--error)}
.rules button[data-severity="note"]{color:var(--note)}
.rules button[data-severity="none"]{color:var(--muted)}
.rules button:hover code{text-decoration:underline}
.rules i{display:block;height:2px;margin-top:2px;background:currentColor;opacity:.55}

.controls{position:sticky;top:0;z-index:5;display:flex;flex-wrap:wrap;gap:8px;padding:8px 0;background:var(--bg);border-bottom:1px solid var(--line)}
.controls input{flex:1 1 220px;min-height:30px;border:1px solid var(--line);background:var(--panel-2);color:var(--text);font:inherit;padding:4px 9px;outline:0}
.controls input:focus{border-color:var(--accent)}
.seg{display:inline-flex;border:1px solid var(--line)}
.seg button{all:unset;cursor:pointer;padding:4px 10px;color:var(--muted);border-right:1px solid var(--line);font-size:12px;font-variant-numeric:tabular-nums}
.seg button:last-child{border-right:0}
.seg button:hover{color:var(--text)}
.seg button[aria-pressed="true"]{background:var(--accent-low);color:var(--accent-text)}
.seg button b{font-weight:600}
.seg .n-error b{color:var(--error)}.seg .n-warning b{color:var(--warning)}.seg .n-note b{color:var(--note)}

.file{margin:0}
.file-head{display:flex;align-items:baseline;gap:10px;margin:0;padding:9px 0 4px;font-size:12px;font-weight:600}
.file-head code{color:var(--accent-text);font-weight:600}
.file-head span{color:var(--faint);font-weight:400;font-variant-numeric:tabular-nums}
details.diagnostic{border-top:1px solid var(--line-soft)}
details.diagnostic:last-child{border-bottom:1px solid var(--line-soft)}
summary{display:grid;grid-template-columns:3.2rem minmax(11rem,17rem) 1fr;gap:12px;align-items:baseline;cursor:pointer;padding:4px 6px 4px 0;list-style:none}
summary:hover{background:var(--panel-2)}
summary::-webkit-details-marker{display:none}
.where{color:var(--faint);text-align:right;font-variant-numeric:tabular-nums}
.rule{color:var(--warning)}
[data-severity="error"] .rule{color:var(--error)}
[data-severity="note"] .rule{color:var(--note)}
[data-severity="none"] .rule{color:var(--muted)}
details[open] summary{background:var(--panel-2)}
.message{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
details[open] .message{white-space:normal}
.details{padding:6px 0 12px 3.2rem}
.location{color:var(--muted);margin:0 0 6px}
.source{background:var(--code);border:1px solid var(--line-soft);margin:0;padding:5px 0;overflow:auto}
.source-line{display:grid;grid-template-columns:3.4rem 1fr;min-width:max-content;padding:1px 12px 1px 0}
.source-line.current{background:var(--accent-low)}
.line-number{color:var(--faint);padding-right:12px;text-align:right;user-select:none}
.source code{white-space:pre}
.source mark{background:var(--mark);color:#141810;padding:0}
.extras{margin:8px 0 0;padding-left:18px;color:var(--muted)}
.extras strong{color:var(--text);font-weight:600}
.empty{border-block:1px solid var(--line);text-align:center;padding:48px 20px;color:var(--muted)}
.empty strong{color:var(--text)}
.hidden{display:none}

html.embedded main{padding:0 14px 24px}
@media(max-width:900px){.layout{grid-template-columns:1fr}aside.rules{position:static;max-height:none}}
@media(max-width:640px){summary{grid-template-columns:3.2rem 1fr}.message{grid-column:1/-1;padding-left:0}.details{padding-left:0}}
</style>
</head>
<body>
<main>
</header>
<section class="tally" aria-label="Diagnostic summary">
{{if .Timings}}<span class="timings" aria-label="Operation timings">{{range .Timings}}<span class="stat timing"><strong>{{.DurationMS}} <small>ms</small></strong><span>{{.Name}}</span></span>{{end}}</span>{{end}}
<span class="stat"><strong>{{.Total}}</strong><span>total</span></span>
<span class="stat error"><strong>{{.Errors}}</strong><span>errors</span></span>
<span class="stat warning"><strong>{{.Warnings}}</strong><span>warnings</span></span>
<span class="stat note"><strong>{{.Notes}}</strong><span>notes</span></span>
{{if .Nones}}<span class="stat none"><strong>{{.Nones}}</strong><span>none</span></span>{{end}}
{{if .Total}}<span class="bar" role="img" aria-label="Severity distribution"><i class="b-error" style="width:{{.ErrorPct}}%"></i><i class="b-warning" style="width:{{.WarningPct}}%"></i><i class="b-note" style="width:{{.NotePct}}%"></i><i class="b-none" style="width:{{.NonePct}}%"></i></span>{{end}}
</section>
{{if .Omitted}}<p class="limit-note">Showing {{.Shown}} of {{.Total}} detailed findings. The summary includes all {{.Total}} findings.</p>{{end}}
<div class="layout">
{{if .Rules}}<aside class="rules" aria-label="Findings by rule"><h2>Findings by rule</h2><table><tbody>{{range .Rules}}<tr><td><button type="button" data-rule="{{.Code}}" data-severity="{{.Severity}}"><code>{{.Code}}</code><i style="width:{{.Percent}}%"></i></button></td><td>{{.Count}}</td></tr>{{end}}</tbody></table></aside>{{else}}<aside class="rules"></aside>{{end}}
<div class="findings">
{{if .Files}}
<section class="controls" aria-label="Report filters">
<input id="search" type="search" placeholder="Filter by rule, message, or file…" aria-label="Search diagnostics">
<span class="seg" id="severity" role="group" aria-label="Filter by severity">
<button type="button" data-severity="" aria-pressed="true">all</button>
<button type="button" class="n-error" data-severity="error" aria-pressed="false"><b>{{.Errors}}</b> error</button>
<button type="button" class="n-warning" data-severity="warning" aria-pressed="false"><b>{{.Warnings}}</b> warning</button>
<button type="button" class="n-note" data-severity="note" aria-pressed="false"><b>{{.Notes}}</b> note</button>
{{if .Nones}}<button type="button" data-severity="none" aria-pressed="false"><b>{{.Nones}}</b> none</button>{{end}}
</span>
</section>
<section id="diagnostics">
{{range .Files}}<section class="file">{{if .File}}<h2 class="file-head"><code>{{.File}}</code><span>{{len .Diagnostics}}</span></h2>{{end}}
{{range .Diagnostics}}<details class="diagnostic" data-severity="{{.Severity}}" data-search="{{.Code}} {{.Message}} {{.File}}">
<summary><code class="where">{{.Where}}</code><code class="rule">{{.Code}}</code><span class="message">{{.Message}}</span></summary>
<div class="details"><p class="location"><code>{{.Location}}</code></p>{{if .Source}}<div class="source">{{range .Source}}<div class="source-line{{if .Current}} current{{end}}"><span class="line-number">{{.Number}}</span><code>{{.Before}}{{if .Highlight}}<mark>{{.Highlight}}</mark>{{end}}{{.After}}</code></div>{{end}}</div>{{end}}{{if or .Notes .Fixes}}<ul class="extras">{{range .Notes}}<li><strong>Note:</strong> {{.Message}}</li>{{end}}{{range .Fixes}}<li><strong>Fix ({{.Safety}}):</strong> {{.Message}}</li>{{end}}</ul>{{end}}</div>
</details>{{end}}
</section>{{end}}
</section>
{{else}}<section class="empty"><strong>No diagnostics found.</strong><br><span>This run completed cleanly.</span></section>{{end}}
</div>
</div>
</main>
<script>
(()=>{const search=document.querySelector('#search');if(!search)return;
const buttons=[...document.querySelectorAll('#severity button')];
const items=[...document.querySelectorAll('.diagnostic')];
const groups=[...document.querySelectorAll('.file')];
let level='';
const filter=()=>{const query=search.value.toLocaleLowerCase();
for(const item of items)item.classList.toggle('hidden',!!((level&&item.dataset.severity!==level)||(query&&!item.dataset.search.toLocaleLowerCase().includes(query))));
for(const group of groups)group.classList.toggle('hidden',!group.querySelector('.diagnostic:not(.hidden)'))};
search.addEventListener('input',filter);
for(const button of buttons)button.addEventListener('click',()=>{level=button.dataset.severity;for(const other of buttons)other.setAttribute('aria-pressed',String(other===button));filter()});
for(const rule of document.querySelectorAll('.rules button'))rule.addEventListener('click',()=>{search.value=search.value===rule.dataset.rule?'':rule.dataset.rule;filter()});
})();
</script>
</body>
</html>
`,
	),
)

type htmlReport struct {
	Title      string
	Timings    []HTMLTiming
	Files      []htmlFileGroup
	Total      int
	Errors     int
	Warnings   int
	Notes      int
	Nones      int
	ErrorPct   int
	WarningPct int
	NotePct    int
	NonePct    int
	Shown      int
	Omitted    int
	Rules      []htmlRuleCount
}

type htmlFileGroup struct {
	File        string
	Diagnostics []htmlDiagnostic
}

type htmlRuleCount struct {
	Code     string
	Count    int
	Percent  int
	Severity diagnostic.Severity
}

type htmlDiagnostic struct {
	Code     string
	Message  string
	Severity diagnostic.Severity
	File     string
	Location string
	Where    string
	Source   []htmlSourceLine
	Notes    []diagnostic.Note
	Fixes    []diagnostic.Fix
}

type htmlSourceLine struct {
	Number    int
	Before    string
	Highlight string
	After     string
	Current   bool
}

// HTMLTiming records one operation duration in milliseconds.
type HTMLTiming struct {
	Name       string
	DurationMS int64
}

// HTMLOptions configures a self-contained diagnostic report.
type HTMLOptions struct {
	Title          string
	SourceRoot     string
	Timings        []HTMLTiming
	MaxDiagnostics int
}

// HTML writes a deterministic, self-contained diagnostic report. The report
// has no external assets and can be saved directly from stdout.
func HTML(writer io.Writer, title string, diagnostics []diagnostic.Diagnostic) error {
	return HTMLWithOptions(writer, HTMLOptions{
		Title: title,
	}, diagnostics)
}

// HTMLWithOptions writes a deterministic, self-contained diagnostic report
// and resolves relative diagnostic filenames against SourceRoot.
func HTMLWithOptions(writer io.Writer, options HTMLOptions, diagnostics []diagnostic.Diagnostic) error {
	data := htmlReport{
		Title:   options.Title,
		Timings: options.Timings,
		Total:   len(diagnostics),
	}
	counts := make(map[string]int)
	severities := make(map[string]diagnostic.Severity)
	for _, item := range diagnostics {
		counts[item.Code]++
		if !severities[item.Code].AtLeast(item.Severity) {
			severities[item.Code] = item.Severity
		}
		switch item.Severity {
		case diagnostic.SeverityError:
			data.Errors++
		case diagnostic.SeverityNote:
			data.Notes++
		case diagnostic.SeverityNone:
			data.Nones++
		default:
			data.Warnings++
		}
	}
	data.Rules = sortedRuleCounts(counts, severities)
	if data.Total > 0 {
		data.ErrorPct = data.Errors * 100 / data.Total
		data.WarningPct = data.Warnings * 100 / data.Total
		data.NotePct = data.Notes * 100 / data.Total
		data.NonePct = data.Nones * 100 / data.Total
	}
	displayed := limitedDiagnostics(diagnostics, options.MaxDiagnostics)
	data.Shown = len(displayed)
	data.Omitted = len(diagnostics) - len(displayed)
	sources := make(map[string][]string)
	missing := make(map[string]bool)
	for _, item := range displayed {
		entry := htmlDiagnostic{
			Code:     item.Code,
			Message:  item.Message,
			Severity: item.Severity,
			File:     item.File,
			Location: htmlLocation(item),
			Where:    htmlWhere(item),
			Source:   htmlSourceContext(item, options.SourceRoot, sources, missing),
			Notes:    item.Notes,
			Fixes:    item.Fixes,
		}
		if count := len(data.Files); count > 0 && data.Files[count-1].File == item.File {
			data.Files[count-1].Diagnostics = append(data.Files[count-1].Diagnostics, entry)
			continue
		}
		data.Files = append(data.Files, htmlFileGroup{
			File: item.File,
			Diagnostics: []htmlDiagnostic{
				entry,
			},
		})
	}
	return htmlTemplate.Execute(writer, data)
}

func sortedRuleCounts(counts map[string]int, severities map[string]diagnostic.Severity) []htmlRuleCount {
	result := make([]htmlRuleCount, 0, len(counts))
	for code, count := range counts {
		result = append(result, htmlRuleCount{
			Code:     code,
			Count:    count,
			Severity: severities[code],
		})
	}
	sort.Slice(
		result,
		func(left, right int) bool {
			if result[left].Count != result[right].Count {
				return result[left].Count > result[right].Count
			}
			return result[left].Code < result[right].Code
		},
	)
	if len(result) > 0 && result[0].Count > 0 {
		for index := range result {
			result[index].Percent = result[index].Count * 100 / result[0].Count
		}
	}
	return result
}

func limitedDiagnostics(items []diagnostic.Diagnostic, limit int) []diagnostic.Diagnostic {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	byCode := make(map[string][]diagnostic.Diagnostic)
	for _, item := range items {
		byCode[item.Code] = append(byCode[item.Code], item)
	}
	codes := make([]string, 0, len(byCode))
	for code := range byCode {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	result := make([]diagnostic.Diagnostic, 0, limit)
	for index := 0; len(result) < limit; index++ {
		added := false
		for _, code := range codes {
			if index >= len(byCode[code]) {
				continue
			}
			result = append(result, byCode[code][index])
			added = true
			if len(result) == limit {
				break
			}
		}
		if !added {
			break
		}
	}
	return result
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

func htmlWhere(item diagnostic.Diagnostic) string {
	if item.Start.Line <= 0 {
		return ""
	}
	where := strconv.Itoa(item.Start.Line)
	if item.Start.Column > 0 {
		where += ":" + strconv.Itoa(item.Start.Column)
	}
	return where
}

func htmlSourceContext(item diagnostic.Diagnostic, root string, cache map[string][]string, missing map[string]bool) []htmlSourceLine {
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
	first := max(1, item.Start.Line-1)
	last := min(len(lines), item.Start.Line+1)
	result := make([]htmlSourceLine, 0, last-first+1)
	for number := first; number <= last; number++ {
		line := htmlSourceLine{
			Number: number,
		}
		contents := lines[number-1]
		if number != item.Start.Line {
			line.Before = contents
			result = append(result, line)
			continue
		}
		line.Current = true
		start := min(max(item.Start.Column-1, 0), len(contents))
		end := len(contents)
		if item.End.Line == item.Start.Line && item.End.Column > item.Start.Column {
			end = min(max(item.End.Column-1, start), len(contents))
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
