package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultsUseVersionOneAndWideFormatting(t *testing.T) {
	defaults := Defaults()
	if defaults.Version != 1 {
		t.Fatalf("default version = %d, want 1", defaults.Version)
	}
	if defaults.Formatter.PrintWidth != 180 {
		t.Fatalf("unexpected formatter defaults: %#v", defaults.Formatter)
	}
	if defaults.Checks.MinimumSeverity != "warning" {
		t.Fatalf("default minimum severity = %q, want warning", defaults.Checks.MinimumSeverity)
	}
}

func TestLoadDiscoversVersionOneChecks(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := `version = 1
[formatter]
print-width = 120
excludes = ["internal/generated/**"]
[check]
excludes = ["generated/**"]
baseline = "strider-baseline.toml"
minimum-severity = "warning"
[checks.no-init]
severity = "none"
excludes = ["legacy/**"]
[checks.banned-characters]
characters = ["ᐸ", "ᐳ"]
`
	if err := os.WriteFile(filepath.Join(root, Filename), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	chdir(t, child)
	configuration, err := Load("", false)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Formatter.PrintWidth != 120 {
		t.Fatalf("unexpected formatter config: %#v", configuration.Formatter)
	}
	if strings.Join(configuration.Formatter.Excludes, ",") != "internal/generated/**" {
		t.Fatalf("unexpected formatter policy config: %#v", configuration.Formatter)
	}
	if configuration.Checks.Baseline != "strider-baseline.toml" || configuration.Checks.MinimumSeverity != "warning" {
		t.Fatalf("unexpected checks config: %#v", configuration.Checks)
	}
	check := configuration.EffectiveCheck("no-init")
	if check.Severity != "none" {
		t.Fatalf("unexpected effective check: %#v", check)
	}
	if strings.Join(check.Excludes, ",") != "generated/**,legacy/**" {
		t.Fatalf("effective excludes = %q", check.Excludes)
	}
	if got := strings.Join(configuration.EffectiveCheck("banned-characters").Characters, ","); got != "ᐸ,ᐳ" {
		t.Fatalf("banned characters = %q", got)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := configuration.Resolve("strider-baseline.toml"); got != filepath.Join(canonicalRoot, "strider-baseline.toml") {
		t.Fatalf("resolved path %q", got)
	}
}

func TestLoadRejectsUnknownAndInvalidSettings(t *testing.T) {
	for name, test := range map[string]struct {
		contents string
		wanted   string
	}{
		"unknown": {
			"version = 1\nunknown = true\n",
			"unknown configuration key",
		},
		"version": {
			"version = 9\n",
			"expected 1",
		},
		"width": {
			"version = 1\n[formatter]\nprint-width = 20\n",
			"print-width",
		},
		"indent-width": {
			"version = 1\n[formatter]\nindent-width = 4\n",
			"unknown configuration key",
		},
		"end-of-line": {
			"version = 1\n[formatter]\nend-of-line = \"crlf\"\n",
			"unknown configuration key",
		},
		"max-empty-lines": {
			"version = 1\n[formatter]\nmax-empty-lines = 1\n",
			"unknown configuration key",
		},
		"max-blank-lines": {
			"version = 1\n[formatter]\nmax-blank-lines = 0\n",
			"unknown configuration key",
		},
		"multiple-blank-lines": {
			"version = 1\n[formatter]\nmax-blank-lines = 2\n",
			"unknown configuration key",
		},
		"existing-line-breaks": {
			"version = 1\n[formatter]\nexisting-line-breaks = \"preserve\"\n",
			"unknown configuration key",
		},
		"declaration-alignment": {
			"version = 1\n[formatter]\nalignment.declarations = false\n",
			"unknown configuration key",
		},
		"severity": {
			"version = 1\n[checks.no-init]\nseverity = \"fatal\"\n",
			"severity",
		},
		"minimum-severity": {
			"version = 1\n[check]\nminimum-severity = \"fatal\"\n",
			"minimum-severity",
		},
		"check-unknown": {
			"version = 1\n[check]\nunknown = true\n",
			"unknown configuration key",
		},
		"legacy-checks-settings": {
			"version = 1\n[checks]\nminimum-severity = \"warning\"\n",
			"checks.minimum-severity",
		},
		"legacy-rules-namespace": {
			"version = 1\n[checks.rules.no-init]\nseverity = \"none\"\n",
			"checks.rules",
		},
		"baseline-variant": {
			"version = 1\n[check]\nbaseline-variant = \"strict\"\n",
			"unknown configuration key",
		},
		"enabled": {
			"version = 1\n[checks.no-init]\nenabled = false\n",
			"unknown configuration key",
		},
		"legacy-linter": {
			"version = 1\n[linter]\n",
			"unknown configuration key",
		},
		"legacy-analyzer": {
			"version = 1\n[analyzer]\n",
			"unknown configuration key",
		},
		"color": {
			"version = 1\ncolor = \"sometimes\"\n",
			"color",
		},
	} {
		t.Run(
			name,
			func(t *testing.T) {
				path := filepath.Join(t.TempDir(), Filename)
				if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
					t.Fatal(err)
				}
				_,
					err := Load(path, false)
				if err == nil || !strings.Contains(err.Error(), test.wanted) {
					t.Fatalf("got %v, want error containing %q", err, test.wanted)
				}
			},
		)
	}
}

func TestLoadTracksExplicitZeroValuedCheckOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), Filename)
	contents := "version = 1\n[checks.no-init]\nmax-lines = 0\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration, err := Load(path, false)
	if err != nil {
		t.Fatal(err)
	}
	check := configuration.Checks.Settings["no-init"]
	if check.MaxLines == nil || *check.MaxLines != 0 {
		t.Fatalf("max-lines = %v, want explicit zero", check.MaxLines)
	}
	if check.MaxMethods != nil {
		t.Fatalf("max-methods = %v, want unset", check.MaxMethods)
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
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}
