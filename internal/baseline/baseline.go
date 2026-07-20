// Package baseline records and suppresses diagnostics that predate adoption.
package baseline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/gempir/strider/internal/diagnostic"
)

const Version = 1

const Strict Variant = "strict"

type Variant string

type File struct {
	Version int     `toml:"version"`
	Variant Variant `toml:"variant"`
	Issues  []Issue `toml:"issues"`
}

type Issue struct {
	File      string `toml:"file"`
	Code      string `toml:"code"`
	StartLine int    `toml:"start-line,omitzero"`
	EndLine   int    `toml:"end-line,omitzero"`
}

type Result struct {
	Diagnostics []diagnostic.Diagnostic
	Matched     File
	Stale       int
}

// Generate constructs a deterministic baseline from diagnostics.
func Generate(path string, diagnostics []diagnostic.Diagnostic) (File, error) {
	directory := filepath.Dir(path)
	baseline := File{
		Version: Version,
		Variant: Strict,
	}
	for _, item := range diagnostics {
		issue, err := strictIssue(directory, item)
		if err != nil {
			return File{}, err
		}
		baseline.Issues = append(baseline.Issues, issue)
	}
	sortIssues(baseline.Issues)
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
	if baseline.Variant != Strict {
		return File{}, fmt.Errorf("baseline variant must be \"strict\"")
	}
	for index, issue := range baseline.Issues {
		if issue.File == "" || issue.Code == "" {
			return File{}, fmt.Errorf("issue %d requires file and code", index+1)
		}
		if issue.StartLine < 1 || issue.EndLine < issue.StartLine {
			return File{}, fmt.Errorf("strict issue %d has an invalid line range", index+1)
		}
	}
	sortIssues(baseline.Issues)
	return baseline, nil
}

// ApplyCatalogSelection preserves known checks that were inactive for this
// run while treating codes outside the current catalog as stale. This keeps a
// severity-filtered baseline intact without retaining entries for rules that
// were removed or renamed.
func ApplyCatalogSelection(path string, baseline File, diagnostics []diagnostic.Diagnostic, selectedCodes, knownCodes map[string]bool) (Result, error) {
	return applySelection(path, baseline, diagnostics, selectedCodes, knownCodes)
}

func applySelection(path string, baseline File, diagnostics []diagnostic.Diagnostic, selectedCodes, knownCodes map[string]bool) (Result, error) {
	directory := filepath.Dir(path)
	result := Result{
		Matched: File{
			Version: Version,
			Variant: baseline.Variant,
		},
	}
	remaining := make(map[string]int, len(baseline.Issues))
	templates := make(map[string]Issue, len(baseline.Issues))
	for _, issue := range baseline.Issues {
		if selectedCodes != nil && !selectedCodes[issue.Code] && (knownCodes == nil || knownCodes[issue.Code]) {
			result.Matched.Issues = append(result.Matched.Issues, issue)
			continue
		}
		key := issueKey(issue)
		remaining[key]++
		templates[key] = issue
	}
	matched := make(map[string]int, len(remaining))
	for _, item := range diagnostics {
		issue, err := strictIssue(directory, item)
		if err != nil {
			return Result{}, err
		}
		key := issueKey(issue)
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
		for range count {
			result.Matched.Issues = append(result.Matched.Issues, issue)
		}
	}
	sortIssues(result.Matched.Issues)
	return result, nil
}

// Write atomically serializes a baseline.
func Write(path string, baseline File) (err error) {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".strider-baseline-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	remove := true
	defer func() {
		if remove {
			err = errors.Join(err, os.Remove(temporaryPath))
		}
	}()
	if err = temporary.Chmod(0o600); err == nil {
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

func strictIssue(directory string, item diagnostic.Diagnostic) (Issue, error) {
	file, err := relativeFile(directory, item.File)
	if err != nil {
		return Issue{}, err
	}
	return Issue{
		File:      file,
		Code:      item.Code,
		StartLine: item.Start.Line,
		EndLine:   item.End.Line,
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

func issueKey(issue Issue) string {
	return fmt.Sprintf("%s\x00%s\x00%d\x00%d", issue.File, issue.Code, issue.StartLine, issue.EndLine)
}

func sortIssues(issues []Issue) {
	sort.Slice(issues, func(i, j int) bool {
		return issueKey(issues[i]) < issueKey(issues[j])
	})
}
