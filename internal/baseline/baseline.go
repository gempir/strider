// Package baseline records and suppresses diagnostics that predate adoption.
package baseline

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gempir/strider/internal/diagnostic"
)

const Version = 1

type Variant string

const (
	Loose  Variant = "loose"
	Strict Variant = "strict"
)

type File struct {
	Version int     `toml:"version"`
	Variant Variant `toml:"variant"`
	Issues  []Issue `toml:"issues"`
}

type Issue struct {
	File      string `toml:"file"`
	Code      string `toml:"code"`
	Message   string `toml:"message,omitempty"`
	Count     int    `toml:"count,omitzero"`
	StartLine int    `toml:"start-line,omitzero"`
	EndLine   int    `toml:"end-line,omitzero"`
}

type Result struct {
	Diagnostics []diagnostic.Diagnostic
	Matched     File
	Stale       int
}

// Generate constructs a deterministic baseline from diagnostics.
func Generate(path string, variant Variant, diagnostics []diagnostic.Diagnostic) (File, error) {
	if err := validateVariant(variant); err != nil {
		return File{}, err
	}
	directory := filepath.Dir(path)
	baseline := File{Version: Version, Variant: variant}
	if variant == Loose {
		counts := make(map[string]int)
		issues := make(map[string]Issue)
		for _, item := range diagnostics {
			issue, err := looseIssue(directory, item)
			if err != nil {
				return File{}, err
			}
			key := looseKey(issue)
			counts[key]++
			issues[key] = issue
		}
		for key, issue := range issues {
			issue.Count = counts[key]
			baseline.Issues = append(baseline.Issues, issue)
		}
	} else {
		for _, item := range diagnostics {
			issue, err := strictIssue(directory, item)
			if err != nil {
				return File{}, err
			}
			baseline.Issues = append(baseline.Issues, issue)
		}
	}
	sortIssues(baseline.Issues, variant)
	return baseline, nil
}

// Load parses and validates a baseline file strictly.
func Load(path string) (File, error) {
	var baseline File
	metadata, err := toml.DecodeFile(path, &baseline)
	if err != nil {
		return File{}, err
	}
	if undecoded := metadata.Undecoded(); len(undecoded) != 0 {
		keys := make([]string, 0, len(undecoded))
		for _, key := range undecoded {
			keys = append(keys, key.String())
		}
		sort.Strings(keys)
		return File{}, fmt.Errorf("unknown baseline key(s): %s", strings.Join(keys, ", "))
	}
	if baseline.Version != Version {
		return File{}, fmt.Errorf("unsupported baseline version %d; expected %d", baseline.Version, Version)
	}
	if err := validateVariant(baseline.Variant); err != nil {
		return File{}, err
	}
	for index, issue := range baseline.Issues {
		if issue.File == "" || issue.Code == "" {
			return File{}, fmt.Errorf("issue %d requires file and code", index+1)
		}
		if baseline.Variant == Loose && (issue.Message == "" || issue.Count < 1) {
			return File{}, fmt.Errorf("loose issue %d requires message and a positive count", index+1)
		}
		if baseline.Variant == Strict && (issue.StartLine < 1 || issue.EndLine < issue.StartLine) {
			return File{}, fmt.Errorf("strict issue %d has an invalid line range", index+1)
		}
	}
	sortIssues(baseline.Issues, baseline.Variant)
	return baseline, nil
}

// Apply suppresses diagnostics consumed by a baseline and reports unmatched
// baseline entries as stale. Matched contains only entries still present.
func Apply(path string, baseline File, diagnostics []diagnostic.Diagnostic) (Result, error) {
	directory := filepath.Dir(path)
	result := Result{Matched: File{Version: Version, Variant: baseline.Variant}}
	remaining := make(map[string]int, len(baseline.Issues))
	templates := make(map[string]Issue, len(baseline.Issues))
	for _, issue := range baseline.Issues {
		key := issueKey(issue, baseline.Variant)
		count := 1
		if baseline.Variant == Loose {
			count = issue.Count
		}
		remaining[key] += count
		templates[key] = issue
	}
	matched := make(map[string]int, len(remaining))
	for _, item := range diagnostics {
		issue, err := issueFromDiagnostic(directory, baseline.Variant, item)
		if err != nil {
			return Result{}, err
		}
		key := issueKey(issue, baseline.Variant)
		if remaining[key] == 0 {
			result.Diagnostics = append(result.Diagnostics, item)
			continue
		}
		remaining[key]--
		matched[key]++
	}
	for _, count := range remaining {
		result.Stale += count
	}
	for key, count := range matched {
		issue := templates[key]
		if baseline.Variant == Loose {
			issue.Count = count
		}
		for range countForVariant(count, baseline.Variant) {
			result.Matched.Issues = append(result.Matched.Issues, issue)
		}
	}
	sortIssues(result.Matched.Issues, baseline.Variant)
	return result, nil
}

func countForVariant(count int, variant Variant) int {
	if variant == Loose {
		return 1
	}
	return count
}

// Write atomically serializes a baseline. With backup enabled, an existing
// baseline is copied to path + ".bkp" first.
func Write(path string, baseline File, backup bool) error {
	if backup {
		if content, err := os.ReadFile(path); err == nil {
			if err := os.WriteFile(path+".bkp", content, 0o600); err != nil {
				return fmt.Errorf("backup baseline: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".strider-baseline-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err == nil {
		encoder := toml.NewEncoder(temporary)
		err = encoder.Encode(baseline)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	remove = false
	return nil
}

func issueFromDiagnostic(directory string, variant Variant, item diagnostic.Diagnostic) (Issue, error) {
	if variant == Loose {
		return looseIssue(directory, item)
	}
	return strictIssue(directory, item)
}

func looseIssue(directory string, item diagnostic.Diagnostic) (Issue, error) {
	file, err := relativeFile(directory, item.File)
	if err != nil {
		return Issue{}, err
	}
	return Issue{File: file, Code: item.Code, Message: item.Message, Count: 1}, nil
}

func strictIssue(directory string, item diagnostic.Diagnostic) (Issue, error) {
	file, err := relativeFile(directory, item.File)
	if err != nil {
		return Issue{}, err
	}
	return Issue{
		File: file, Code: item.Code, StartLine: item.Start.Line, EndLine: item.End.Line,
	}, nil
}

func relativeFile(directory, filename string) (string, error) {
	absolute := filename
	if !filepath.IsAbs(absolute) {
		var err error
		absolute, err = filepath.Abs(filename)
		if err != nil {
			return "", err
		}
	}
	relative, err := filepath.Rel(directory, absolute)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(relative), nil
}

func validateVariant(variant Variant) error {
	if variant != Loose && variant != Strict {
		return fmt.Errorf("baseline variant must be \"loose\" or \"strict\"")
	}
	return nil
}

func issueKey(issue Issue, variant Variant) string {
	if variant == Loose {
		return looseKey(issue)
	}
	return fmt.Sprintf("%s\x00%s\x00%d\x00%d", issue.File, issue.Code, issue.StartLine, issue.EndLine)
}

func looseKey(issue Issue) string {
	return issue.File + "\x00" + issue.Code + "\x00" + issue.Message
}

func sortIssues(issues []Issue, variant Variant) {
	sort.Slice(issues, func(i, j int) bool {
		return issueKey(issues[i], variant) < issueKey(issues[j], variant)
	})
}
