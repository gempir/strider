package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDiscoversAndAppliesDefaults(t *testing.T) {
	if got := Defaults().Formatter.MaxEmptyLines; got != 1 {
		t.Fatalf("default max empty lines = %d, want 1", got)
	}
	root := t.TempDir()
	child := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := `version = 1
[formatter]
print-width = 120
max-empty-lines = 3
[linter.rules.no-init]
enabled = false
[analyzer.rules.invalid-regexp]
severity = "note"
`
	if err := os.WriteFile(filepath.Join(root, Filename), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	chdir(t, child)
	configuration, err := Load("", false)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Formatter.PrintWidth != 120 || configuration.Formatter.IndentWidth != 4 ||
		configuration.Formatter.MaxEmptyLines != 3 {
		t.Fatalf("unexpected formatter config: %#v", configuration.Formatter)
	}
	if configuration.Color != "auto" {
		t.Fatalf("unexpected default color mode %q", configuration.Color)
	}
	if enabled := configuration.Linter.Rules["no-init"].Enabled; enabled == nil || *enabled {
		t.Fatalf("unexpected rule config: %#v", configuration.Linter.Rules["no-init"])
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := configuration.Resolve("lint-baseline.toml"); got != filepath.Join(canonicalRoot, "lint-baseline.toml") {
		t.Fatalf("resolved path %q", got)
	}
}

func TestLoadRejectsUnknownAndInvalidSettings(t *testing.T) {
	for name, test := range map[string]struct {
		contents string
		wanted   string
	}{
		"unknown":  {"version = 1\nunknown = true\n", "unknown configuration key"},
		"version":  {"version = 2\n", "unsupported configuration version"},
		"width":    {"version = 1\n[formatter]\nprint-width = 20\n", "print-width"},
		"empty":    {"version = 1\n[formatter]\nmax-empty-lines = -1\n", "max-empty-lines"},
		"severity": {"version = 1\n[linter.rules.no-init]\nseverity = \"fatal\"\n", "severity"},
		"color":    {"version = 1\ncolor = \"sometimes\"\n", "color"},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), Filename)
			if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path, false)
			if err == nil || !strings.Contains(err.Error(), test.wanted) {
				t.Fatalf("got %v, want error containing %q", err, test.wanted)
			}
		})
	}
}

func chdir(t *testing.T, directory string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
}
