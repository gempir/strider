//strider:ignore-file cognitive-complexity,modifies-parameter
package main

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gempir/strider/internal/diagnostic"
)

func TestScaffoldCreatesAuthoredCatalogAndDocumentationPath(t *testing.T) {
	for _, engine := range []string{
		"syntax",
		"semantic",
	} {
		t.Run(
			engine,
			func(t *testing.T) {
				root := scaffoldFixture(t)
				code := engine + "-fixture"
				updated := false
				created, err := scaffold(
					scaffoldOptions{
						root:        root,
						engine:      engine,
						code:        code,
						summary:     "detect fixture misuse",
						explanation: "Fixture misuse is unambiguous in the focused local pattern; unknown forms are ignored.",
						goodExample: "use(valid)",
						badExample:  "use(invalid)",
						severity:    diagnostic.SeverityWarning,
						stage:       "types",
						optionJSON:  `[{"name":"allowed","kind":"strings","default_strings":["valid"],"help":"Names accepted by the fixture."}]`,
					},
					func(updateRoot string) error {
						updated = true
						if updateRoot != root {
							t.Fatalf("update root = %s, want %s", updateRoot, root)
						}
						implementationToken := code
						if engine == "syntax" {
							implementationToken = exportedName(goName(code))
						}
						assertScaffoldPath(
							t,
							root,
							filepath.Join("internal", "checks", engine, strings.ReplaceAll(code, "-", "_")+".go"),
							implementationToken,
						)
						assertScaffoldPath(t, root, filepath.Join("internal", "checks", engine, strings.ReplaceAll(code, "-", "_")+"_test.go"), code)
						category := "lints"
						if engine == "semantic" {
							category = "analyzers"
						}
						assertScaffoldPath(t, root, filepath.Join("docs", "src", "content", "docs", category, code+".md"), code)
						registry := filepath.Join(root, "internal", "checks", engine, "catalog.go")
						if engine == "semantic" {
							registry = filepath.Join(root, "internal", "checks", engine, "registry.go")
						}
						registryToken := code
						if engine == "semantic" {
							registryToken = goName(code)
						}
						assertScaffoldPath(t, root, registry, registryToken)
						return nil
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				if !updated {
					t.Fatal("generated artifact update was not invoked")
				}
				if got, want := len(created), 4; got != want {
					t.Fatalf("created %d paths, want %d: %v", got, want, created)
				}
			},
		)
	}
}

func TestScaffoldRejectsInvalidInputBeforeWriting(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*scaffoldOptions, string)
		message string
	}{
		{
			name: "case insensitive collision",
			mutate: func(options *scaffoldOptions, _ string) {
				options.code = "EXISTING-CHECK"
			},
			message: "collides case-insensitively",
		},
		{
			name: "incomplete metadata",
			mutate: func(options *scaffoldOptions, _ string) {
				options.explanation = ""
			},
			message: "metadata are required",
		},
		{
			name: "invalid option schema",
			mutate: func(options *scaffoldOptions, _ string) {
				options.optionJSON = `[{"name":"UPPER","kind":"int","default_int":1,"help":"Invalid name."}]`
			},
			message: "invalid option name",
		},
		{
			name: "documentation cannot be produced",
			mutate: func(options *scaffoldOptions, root string) {
				path := filepath.Join(root, "docs", "src", "content", "docs", "lints", options.code+".md")
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			},
			message: "refusing to replace existing directory",
		},
		{
			name: "trailing options JSON",
			mutate: func(options *scaffoldOptions, _ string) {
				options.optionJSON = `[] []`
			},
			message: "options-json",
		},
	}
	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				root := scaffoldFixture(t)
				options := validScaffoldOptions(root)
				test.mutate(&options, root)
				updateCalled := false
				_, err := scaffold(options, func(string) error {
					updateCalled = true
					return nil
				})
				if err == nil || !strings.Contains(err.Error(), test.message) {
					t.Fatalf("scaffold error = %v, want %q", err, test.message)
				}
				if updateCalled {
					t.Fatal("artifact update ran after invalid input")
				}
				path := filepath.Join(root, "internal", "checks", "syntax", "new_check.go")
				if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("invalid scaffold wrote %s: %v", path, statErr)
				}
			},
		)
	}
}

func validScaffoldOptions(root string) scaffoldOptions {
	return scaffoldOptions{
		root:        root,
		engine:      "syntax",
		code:        "new-check",
		summary:     "detect a new problem",
		explanation: "The new problem has a direct local repair and uncertain forms are ignored.",
		goodExample: "good()",
		badExample:  "bad()",
		severity:    diagnostic.SeverityWarning,
		stage:       "types",
	}
}

func scaffoldFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	directories := []string{
		filepath.Join("internal", "checks", "syntax"),
		filepath.Join("internal", "checks", "semantic"),
		filepath.Join("docs", "src", "content", "docs", "lints"),
		filepath.Join("docs", "src", "content", "docs", "analyzers"),
	}
	for _, directory := range directories {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	syntaxCatalog := `package syntax

var definitions = []definition{
	{meta: Meta{Code: "existing-check"}},
}

// Catalog returns checks.
`
	semanticRegistry := `package semantic

var checkCatalog = []Descriptor{
	typeCheck(existingCheck{}),
}

// Plan is a plan.
`
	if err := os.WriteFile(filepath.Join(root, "internal", "checks", "syntax", "catalog.go"), []byte(syntaxCatalog), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "checks", "semantic", "registry.go"), []byte(semanticRegistry), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func assertScaffoldPath(t *testing.T, root, path, wanted string) {
	t.Helper()
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), wanted) {
		t.Fatalf("%s does not contain %q:\n%s", path, wanted, contents)
	}
	if filepath.Ext(path) == ".go" {
		if _, err := parser.ParseFile(token.NewFileSet(), path, contents, parser.AllErrors); err != nil {
			t.Fatalf("generated Go file %s is invalid: %v", path, err)
		}
	}
}
