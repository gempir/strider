// Command corpus runs Strider against the pinned open-source benchmark corpus.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	diagnosticmodel "github.com/gempir/strider/internal/diagnostic"
	reporter "github.com/gempir/strider/internal/report"
)

const schemaVersion = 1

const projectReportDiagnosticLimit = 1000

var reportTemplate = template.Must(
	template.New("corpus").Funcs(
		template.FuncMap{
			"seconds": func(milliseconds int64) string {
				return fmt.Sprintf("%.2fs", float64(milliseconds)/1000)
			},
			"budget": func(milliseconds int) string {
				return fmt.Sprintf("%.2fs", float64(milliseconds)/1000)
			},
			"status": statusClass,
			"codes":  sortedCodes,
		},
	).Parse(
		`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Strider open-source corpus</title><script>if(self!==top)document.documentElement.classList.add('embedded')</script><style>
:root{color-scheme:dark;--bg:#0d1117;--panel:#121820;--text:#e6edf3;--muted:#8f9baa;--line:#2a3440;--accent:#54d6b0;--pass:#54d6b0;--fail:#ff7b72;--code:#0a0f14}*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:15px/1.55 Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;font-feature-settings:"ss01","cv02","cv11"}main{width:min(1440px,calc(100% - 48px));margin:56px auto 88px}header{border-left:2px solid var(--accent);padding-left:20px}h1{font-size:clamp(2rem,5vw,3.6rem);line-height:1.04;letter-spacing:-.045em;margin:0}.lede{color:var(--muted);margin:.6rem 0 2.25rem}.project{border-block-start:1px solid var(--line);margin:0}.project:last-child{border-block-end:1px solid var(--line)}.project h2{display:flex;align-items:baseline;gap:12px;margin:0;padding:18px 16px 14px;font-size:1.12rem;letter-spacing:-.02em}.project h2>a:first-child{color:var(--text);font-size:1.25rem;text-decoration:none}.project h2>a:first-child:hover{color:var(--accent)}.project h2>a:last-child{margin-left:auto;color:var(--muted);font-size:.78rem;font-weight:500;text-underline-offset:3px}.revision{max-width:24ch;overflow:hidden;color:var(--muted);font:11px ui-monospace,SFMono-Regular,Consolas,monospace;text-overflow:ellipsis;white-space:nowrap}table{border-collapse:collapse;width:100%;background:var(--panel)}th,td{text-align:left;padding:11px 16px;border-top:1px solid var(--line)}th{color:var(--muted);font-size:.69rem;font-weight:700;text-transform:uppercase;letter-spacing:.09em}tbody tr:hover{background:color-mix(in srgb,var(--accent) 5%,transparent)}.pass{color:var(--pass);font-weight:650}.fail{color:var(--fail);font-weight:650}details{padding:0 16px;border-top:1px solid var(--line)}summary{cursor:pointer;color:var(--muted);padding:12px 0;font-size:.82rem;text-transform:capitalize}details p{margin:0 0 14px;line-height:1.9}code{border-left:1px solid var(--line);color:var(--muted);font:11px ui-monospace,SFMono-Regular,Consolas,monospace;padding:1px 7px}html.embedded main{width:100%;margin:0;padding:24px}html.embedded header{display:none}:root[data-theme="light"]{color-scheme:light;--bg:#f7f9f8;--panel:#fff;--text:#17201d;--muted:#65736d;--line:#ced9d4;--accent:#087a5c;--pass:#087a5c;--fail:#c93c37;--code:#f0f4f2}@media(max-width:700px){main{width:min(100% - 28px,1440px);margin-top:28px}.project h2{align-items:flex-start;flex-wrap:wrap}.project h2>a:last-child{margin-left:0}.revision{order:3;flex-basis:100%;max-width:100%}.scroll{overflow-x:auto}table{min-width:620px}th,td{padding:10px 12px}html.embedded main{padding:14px}}
</style></head><body><main><header><h1>Strider open-source corpus</h1><p class="lede">Pinned, repeatable format and unified check results across {{len .Projects}} Go projects. Total wall time: {{seconds .TotalMS}}.</p></header>
{{range .Projects}}<section class="project"><h2><a href="projects/{{.Name}}/index.html">{{.Name}}</a><span class="revision">{{.Revision}}</span> <a href="{{.Repository}}"><small>repository</small></a></h2><div class="scroll"><table><thead><tr><th>Operation</th><th>Findings</th><th>Time</th><th>Budget</th><th>Baseline</th><th>Performance</th></tr></thead><tbody>{{range .Operations}}<tr><td>{{.Name}}</td><td>{{.Findings}}</td><td>{{seconds .DurationMS}}</td><td>{{budget .BudgetMS}}</td><td class="{{status .}}">{{if .BaselineMatch}}match{{else}}changed{{end}}</td><td class="{{status .}}">{{if .Error}}error{{else if .WithinBudget}}within budget{{else}}slower{{end}}</td></tr>{{end}}</tbody></table></div>{{range .Operations}}{{if .ByCode}}{{$operation := .}}<details><summary>{{.Name}} findings by rule</summary><p>{{range $code := codes .ByCode}}<code>{{$code}}={{index $operation.ByCode $code}}</code> {{end}}</p></details>{{end}}{{end}}</section>{{end}}
</main></body></html>`,
	),
)

type manifest struct {
	Version  int       `json:"version"`
	Projects []project `json:"projects"`
}

type project struct {
	Name           string         `json:"name"`
	Repository     string         `json:"repository"`
	Revision       string         `json:"revision"`
	BudgetsMS      map[string]int `json:"budgets_ms"`
	Paths          []string       `json:"paths,omitempty"`
	FormatExcludes []string       `json:"format_excludes,omitempty"`
}

type baseline struct {
	Version  int               `json:"version"`
	Projects []baselineProject `json:"projects"`
}

type baselineProject struct {
	Name       string               `json:"name"`
	Revision   string               `json:"revision"`
	Operations map[string]signature `json:"operations"`
}

type signature struct {
	ExitCode int            `json:"exit_code"`
	Digest   string         `json:"digest"`
	Findings int            `json:"findings"`
	ByCode   map[string]int `json:"by_code,omitempty"`
}

type report struct {
	Projects []projectResult `json:"projects"`
	Passed   bool            `json:"passed"`
	TotalMS  int64           `json:"total_ms"`
}

type projectResult struct {
	Name       string            `json:"name"`
	Repository string            `json:"repository"`
	Revision   string            `json:"revision"`
	Operations []operationResult `json:"operations"`
}

type operationResult struct {
	Name          string                       `json:"name"`
	ExitCode      int                          `json:"exit_code"`
	Digest        string                       `json:"digest"`
	Findings      int                          `json:"findings"`
	ByCode        map[string]int               `json:"by_code,omitempty"`
	DurationMS    int64                        `json:"duration_ms"`
	BudgetMS      int                          `json:"budget_ms"`
	BaselineMatch bool                         `json:"baseline_match"`
	WithinBudget  bool                         `json:"within_budget"`
	Error         string                       `json:"error,omitempty"`
	Diagnostics   []diagnosticmodel.Diagnostic `json:"-"`
}

type options struct {
	mode              string
	strider           string
	manifestPath      string
	baselinePath      string
	cachePath         string
	jsonPath          string
	htmlPath          string
	projectHTMLPath   string
	homepageStatsPath string
}

type homepageStats struct {
	Project  string `json:"project"`
	Revision string `json:"revision"`
	FormatMS int64  `json:"format_ms"`
	CheckMS  int64  `json:"check_ms"`
}

func main() {
	options := parseFlags()
	if err := run(options); err != nil {
		fmt.Fprintln(os.Stderr, "corpus:", err)
		os.Exit(1)
	}
}

func parseFlags() options {
	var result options
	flag.StringVar(&result.mode, "mode", "check", "check or update the reviewed baseline")
	flag.StringVar(&result.strider, "strider", "./strider", "path to the Strider binary")
	flag.StringVar(&result.manifestPath, "manifest", "benchmarks/projects.json", "corpus manifest")
	flag.StringVar(&result.baselinePath, "baseline", "benchmarks/baseline.json", "reviewed baseline")
	flag.StringVar(&result.cachePath, "cache", ".benchmark-cache", "checkout cache")
	flag.StringVar(&result.jsonPath, "json", "target/corpus/report.json", "JSON report output")
	flag.StringVar(&result.htmlPath, "html", "target/corpus/index.html", "HTML report output")
	flag.StringVar(&result.projectHTMLPath, "project-html", "", "project HTML report directory (defaults beside --html)")
	flag.StringVar(&result.homepageStatsPath, "homepage-stats", "", "homepage benchmark stats JSON output")
	flag.Parse()
	return result
}

func run(options options) error {
	if options.mode != "check" && options.mode != "update" {
		return fmt.Errorf("unsupported mode %q", options.mode)
	}
	strider, err := filepath.Abs(options.strider)
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(strider); statErr != nil || info.IsDir() {
		return fmt.Errorf("Strider binary is not executable at %s", strider)
	}
	configuration, err := readManifest(options.manifestPath)
	if err != nil {
		return err
	}
	expected := baseline{
		Version: schemaVersion,
	}
	if options.mode == "check" {
		expected, err = readBaseline(options.baselinePath)
		if err != nil {
			return err
		}
	}

	started := time.Now()
	results := report{
		Passed: true,
	}
	actual := baseline{
		Version: schemaVersion,
	}
	projectHTMLPath := options.projectHTMLPath
	if projectHTMLPath == "" {
		projectHTMLPath = filepath.Join(filepath.Dir(options.htmlPath), "projects")
	}
	for _, item := range configuration.Projects {
		checkout, checkoutErr := prepareProject(options.cachePath, item)
		projectReport := projectResult{
			Name:       item.Name,
			Repository: item.Repository,
			Revision:   item.Revision,
		}
		projectBaseline := baselineProject{
			Name:       item.Name,
			Revision:   item.Revision,
			Operations: map[string]signature{},
		}
		if checkoutErr != nil {
			results.Passed = false
			projectReport.Operations = append(projectReport.Operations, operationResult{
				Name:  "prepare",
				Error: checkoutErr.Error(),
			})
		} else {
			for _, operation := range []string{
				"format",
				"check",
			} {
				observed := runOperation(strider, checkout, operation, item)
				expectedSignature, found := findExpected(expected, item.Name, item.Revision, operation)
				observed.BaselineMatch = options.mode == "update" || (found && reflect.DeepEqual(expectedSignature, observed.signature()))
				if observed.Error != "" || !observed.WithinBudget || !observed.BaselineMatch {
					results.Passed = false
				}
				projectReport.Operations = append(projectReport.Operations, observed)
				projectBaseline.Operations[operation] = observed.signature()
			}
			if err := writeProjectReport(projectHTMLPath, projectReport, checkout); err != nil {
				return err
			}
		}
		results.Projects = append(results.Projects, projectReport)
		actual.Projects = append(actual.Projects, projectBaseline)
	}
	results.TotalMS = time.Since(started).Milliseconds()

	if options.mode == "update" {
		if hasProcessingErrors(results) {
			return errors.New("refusing to update a baseline containing processing errors")
		}
		if err := writeJSON(options.baselinePath, actual); err != nil {
			return err
		}
	}
	if err := writeJSON(options.jsonPath, results); err != nil {
		return err
	}
	if err := writeHTML(options.htmlPath, results); err != nil {
		return err
	}
	writeConsole(os.Stdout, results)
	if summary := os.Getenv("GITHUB_STEP_SUMMARY"); summary != "" {
		if err := writeGitHubSummary(summary, results); err != nil {
			return err
		}
	}
	if !results.Passed {
		return errors.New("behavior or performance regression detected; inspect the report above")
	}
	if options.homepageStatsPath != "" {
		if err := writeHomepageStats(options.homepageStatsPath, results, "sftpgo"); err != nil {
			return err
		}
	}
	return nil
}

func writeHomepageStats(path string, results report, projectName string) error {
	for _, project := range results.Projects {
		if project.Name != projectName {
			continue
		}
		stats := homepageStats{
			Project:  project.Name,
			Revision: project.Revision,
		}
		for _, operation := range project.Operations {
			switch operation.Name {
			case "format":
				stats.FormatMS = operation.DurationMS
			case "check":
				stats.CheckMS = operation.DurationMS
			}
		}
		if stats.FormatMS <= 0 || stats.CheckMS <= 0 {
			return fmt.Errorf("%s is missing positive format or check timings", projectName)
		}
		return writeJSON(path, stats)
	}
	return fmt.Errorf("project %s is missing from corpus results", projectName)
}

func readManifest(path string) (manifest, error) {
	var result manifest
	if err := readJSON(path, &result); err != nil {
		return result, err
	}
	if result.Version != schemaVersion {
		return result, fmt.Errorf("manifest version %d is unsupported", result.Version)
	}
	if len(result.Projects) != 11 {
		return result, fmt.Errorf("manifest must contain exactly 11 projects, got %d", len(result.Projects))
	}
	seen := map[string]bool{}
	for _, item := range result.Projects {
		if item.Name == "" || item.Repository == "" || len(item.Revision) != 40 || seen[item.Name] {
			return result, fmt.Errorf("invalid project entry %q", item.Name)
		}
		seen[item.Name] = true
		for _, operation := range []string{
			"format",
			"check",
		} {
			if item.BudgetsMS[operation] <= 0 {
				return result, fmt.Errorf("%s has no positive %s budget", item.Name, operation)
			}
		}
	}
	return result, nil
}

func readBaseline(path string) (baseline, error) {
	var result baseline
	if err := readJSON(path, &result); err != nil {
		return result, err
	}
	if result.Version != schemaVersion {
		return result, fmt.Errorf("baseline version %d is unsupported", result.Version)
	}
	return result, nil
}

func readJSON(path string, target any) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func prepareProject(cacheRoot string, item project) (string, error) {
	checkout, err := filepath.Abs(filepath.Join(cacheRoot, item.Name))
	if err != nil {
		return "", err
	}
	if _, statErr := os.Stat(filepath.Join(checkout, ".git")); os.IsNotExist(statErr) {
		if err := command("", "git", "clone", "--quiet", "--filter=blob:none", "--no-checkout", item.Repository, checkout); err != nil {
			return "", err
		}
	}
	if err := command(checkout, "git", "cat-file", "-e", item.Revision+"^{commit}"); err != nil {
		if err := command(checkout, "git", "fetch", "--quiet", "--depth", "1", "origin", item.Revision); err != nil {
			return "", err
		}
	}
	if err := command(checkout, "git", "checkout", "--quiet", "--detach", item.Revision); err != nil {
		return "", err
	}
	if err := os.Remove(filepath.Join(checkout, ".strider-corpus.toml")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if dirty, err := commandOutput(checkout, "git", "status", "--porcelain"); err != nil {
		return "", err
	} else if len(bytes.TrimSpace(dirty)) != 0 {
		return "", fmt.Errorf("%s checkout is dirty", item.Name)
	}
	if _, err := os.Stat(filepath.Join(checkout, "go.mod")); err == nil {
		if err := command(checkout, "go", "mod", "download"); err != nil {
			return "", err
		}
	}
	return checkout, nil
}

func command(directory, name string, arguments ...string) error {
	output, err := commandOutput(directory, name, arguments...)
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(arguments, " "), err, bytes.TrimSpace(output))
	}
	return nil
}

func commandOutput(directory, name string, arguments ...string) ([]byte, error) {
	cmd := exec.Command(name, arguments...)
	cmd.Dir = directory
	cmd.Env = append(os.Environ(), "GOWORK=off")
	return cmd.CombinedOutput()
}

func runOperation(strider, checkout, operation string, item project) operationResult {
	arguments := map[string][]string{
		"format": {
			"--no-config",
			"fmt",
			"--check",
		},
		"check": {
			"--no-config",
			"check",
			"--minimum-severity",
			"note",
			"--format",
			"json",
		},
	}[operation]
	if len(item.FormatExcludes) != 0 {
		configPath := filepath.Join(checkout, ".strider-corpus.toml")
		encoded, err := json.Marshal(item.FormatExcludes)
		if err != nil {
			return operationResult{
				Name:     operation,
				BudgetMS: item.BudgetsMS[operation],
				Error:    err.Error(),
			}
		}
		contents := []byte("version = 1\n[formatter]\nexcludes = " + string(encoded) + "\n")
		if err := os.WriteFile(configPath, contents, 0o600); err != nil {
			return operationResult{
				Name:     operation,
				BudgetMS: item.BudgetsMS[operation],
				Error:    err.Error(),
			}
		}
		defer os.Remove(configPath)
		arguments = append([]string{
			"--config",
			configPath,
		}, arguments[1:]...)
	}
	paths := item.Paths
	if len(paths) == 0 {
		paths = []string{
			".",
		}
	}
	arguments = append(arguments, paths...)
	budget := item.BudgetsMS[operation]
	cmd := exec.Command(strider, arguments...)
	cmd.Dir = checkout
	// Pin the analysis target so build-tagged files produce the same corpus on
	// developer machines and the Linux GitHub Actions runner.
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOARCH=amd64", "GOMAXPROCS=2", "GOOS=linux", "GOWORK=off")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	started := time.Now()
	err := cmd.Run()
	duration := time.Since(started).Milliseconds()
	exitCode := 0
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		} else {
			return operationResult{
				Name:       operation,
				DurationMS: duration,
				BudgetMS:   budget,
				Error:      err.Error(),
			}
		}
	}
	result := operationResult{
		Name:         operation,
		ExitCode:     exitCode,
		DurationMS:   duration,
		BudgetMS:     budget,
		WithinBudget: duration <= int64(budget),
	}
	if exitCode > 1 {
		result.Error = strings.TrimSpace(stderr.String())
		if result.Error == "" {
			result.Error = fmt.Sprintf("Strider exited %d", exitCode)
		}
	}
	result.Digest = digest(exitCode, stdout.Bytes(), stderr.Bytes())
	if operation == "format" {
		result.Findings = nonEmptyLines(stdout.String())
	} else if exitCode <= 1 {
		var diagnostics []diagnosticmodel.Diagnostic
		if decodeErr := json.Unmarshal(stdout.Bytes(), &diagnostics); decodeErr != nil {
			result.Error = "decode diagnostic JSON: " + decodeErr.Error()
		} else {
			result.Diagnostics = diagnostics
			result.Findings = len(diagnostics)
			if len(diagnostics) != 0 {
				result.ByCode = map[string]int{}
				for _, item := range diagnostics {
					result.ByCode[item.Code]++
				}
			}
		}
	}
	return result
}

func writeProjectReport(htmlRoot string, project projectResult, sourceRoot string) error {
	diagnostics := make([]diagnosticmodel.Diagnostic, 0)
	timings := make([]reporter.HTMLTiming, 0, len(project.Operations))
	for _, operation := range project.Operations {
		timings = append(timings, reporter.HTMLTiming{
			Name:       operation.Name,
			DurationMS: operation.DurationMS,
		})
		if operation.Name != "check" {
			continue
		}
		diagnostics = append(diagnostics, operation.Diagnostics...)
	}
	path := filepath.Join(htmlRoot, project.Name, "index.html")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return reporter.HTMLWithOptions(
		file,
		reporter.HTMLOptions{
			Title:          "Strider corpus: " + project.Name,
			SourceRoot:     sourceRoot,
			Timings:        timings,
			MaxDiagnostics: projectReportDiagnosticLimit,
		},
		diagnostics,
	)
}

func digest(exitCode int, stdout, stderr []byte) string {
	hash := sha256.New()
	fmt.Fprintf(hash, "exit=%d\nstdout\n", exitCode)
	_, _ = hash.Write(bytes.ReplaceAll(stdout, []byte("\r\n"), []byte("\n")))
	_, _ = io.WriteString(hash, "\nstderr\n")
	_, _ = hash.Write(bytes.ReplaceAll(stderr, []byte("\r\n"), []byte("\n")))
	return hex.EncodeToString(hash.Sum(nil))
}

func nonEmptyLines(value string) int {
	count := 0
	for _, line := range strings.Split(value, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func (result operationResult) signature() signature {
	return signature{
		ExitCode: result.ExitCode,
		Digest:   result.Digest,
		Findings: result.Findings,
		ByCode:   result.ByCode,
	}
}

func findExpected(data baseline, name, revision, operation string) (signature, bool) {
	for _, item := range data.Projects {
		if item.Name == name && item.Revision == revision {
			result, ok := item.Operations[operation]
			return result, ok
		}
	}
	return signature{}, false
}

func hasProcessingErrors(results report) bool {
	for _, project := range results.Projects {
		for _, operation := range project.Operations {
			if operation.Error != "" {
				return true
			}
		}
	}
	return false
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	return os.WriteFile(path, contents, 0o644)
}

func writeConsole(writer io.Writer, results report) {
	fmt.Fprintln(writer, "Project         Operation  Findings   Time / Budget  Baseline  Performance")
	fmt.Fprintln(writer, "--------------- ---------- -------- ---------------- --------- -----------")
	for _, project := range results.Projects {
		for _, operation := range project.Operations {
			baselineState, performanceState := "PASS", "PASS"
			if !operation.BaselineMatch {
				baselineState = "CHANGED"
			}
			if !operation.WithinBudget {
				performanceState = "SLOW"
			}
			if operation.Error != "" {
				performanceState = "ERROR"
			}
			fmt.Fprintf(
				writer,
				"%-15s %-10s %8d %7d / %-6d %-9s %s\n",
				project.Name,
				operation.Name,
				operation.Findings,
				operation.DurationMS,
				operation.BudgetMS,
				baselineState,
				performanceState,
			)
		}
	}
	fmt.Fprintf(writer, "\nTotal wall time: %.2fs\n", float64(results.TotalMS)/1000)
}

func writeGitHubSummary(path string, results report) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	fmt.Fprintln(file, "## Strider open-source corpus")
	fmt.Fprintln(file)
	fmt.Fprintln(file, "| Project | Operation | Findings | Time | Budget | Baseline | Performance |")
	fmt.Fprintln(file, "| --- | --- | ---: | ---: | ---: | --- | --- |")
	for _, project := range results.Projects {
		for _, operation := range project.Operations {
			baselineState, performanceState := "✅", "✅"
			if !operation.BaselineMatch {
				baselineState = "❌ changed"
			}
			if !operation.WithinBudget {
				performanceState = "❌ slower"
			}
			if operation.Error != "" {
				performanceState = "❌ error"
			}
			fmt.Fprintf(
				file,
				"| %s | %s | %d | %.2fs | %.2fs | %s | %s |\n",
				project.Name,
				operation.Name,
				operation.Findings,
				float64(operation.DurationMS)/1000,
				float64(operation.BudgetMS)/1000,
				baselineState,
				performanceState,
			)
		}
	}
	fmt.Fprintf(file, "\nTotal wall time: **%.2fs**\n", float64(results.TotalMS)/1000)
	return nil
}

func writeHTML(path string, results report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return reportTemplate.Execute(file, results)
}

func statusClass(operation operationResult) string {
	if operation.Error != "" || !operation.BaselineMatch || !operation.WithinBudget {
		return "fail"
	}
	return "pass"
}

func sortedCodes(codes map[string]int) []string {
	keys := make([]string, 0, len(codes))
	for code := range codes {
		keys = append(keys, code)
	}
	sort.Strings(keys)
	return keys
}
