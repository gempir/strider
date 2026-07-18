// Package config loads and validates Strider's project configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/gempir/strider/internal/diagnostic"
	"github.com/gempir/strider/internal/ui"
)

const Filename = "strider.toml"

// Config is the complete project configuration document.
type Config struct {
	Version int `toml:"version"`
	Color string `toml:"color"`
	Formatter FormatterConfig `toml:"formatter"`
	Checks ToolConfig `toml:"checks"`

	Path string `toml:"-"`
	Root string `toml:"-"`
}

type FormatterConfig struct {
	PrintWidth int `toml:"print-width"`
	IndentWidth int `toml:"indent-width"`
	MaxEmptyLines int `toml:"max-empty-lines"`
	EndOfLine string `toml:"end-of-line"`
	Excludes []string `toml:"excludes"`
}

type ToolConfig struct {
	Excludes []string `toml:"excludes"`
	Baseline string `toml:"baseline"`
	BaselineVariant string `toml:"baseline-variant"`
	MinimumSeverity string `toml:"minimum-severity"`
	Rules map[string]RuleConfig `toml:"rules"`
}

type RuleConfig struct {
	Enabled *bool `toml:"enabled"`
	Severity string `toml:"severity"`
	Excludes []string `toml:"excludes"`
}

func Defaults() Config {
	return Config{
		Version: 1,
		Color: string(ui.ColorAuto),
		Formatter: FormatterConfig{PrintWidth: 180, IndentWidth: 4, MaxEmptyLines: 1, EndOfLine: "lf"},
		Checks: defaultToolConfig(),
	}
}

func defaultToolConfig() ToolConfig {
	return ToolConfig{
		BaselineVariant: "loose",
		MinimumSeverity: string(diagnostic.SeverityWarning),
		Rules: make(map[string]RuleConfig),
	}
}

// Load reads an explicit path or discovers strider.toml from the working
// directory upward. With no discovered file it returns built-in defaults.
func Load(explicitPath string, disabled bool) (Config, error) {
	configuration := Defaults()
	if disabled {
		return configuration, nil
	}
	path := explicitPath
	if path == "" {
		var err error
		path, err = discover()
		if err != nil {
			return Config{}, err
		}
		if path == "" {
			return configuration, nil
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Config{}, fmt.Errorf("configuration path: %w", err)
	}
	if canonical, canonicalErr := filepath.EvalSymlinks(absolute); canonicalErr == nil {
		absolute = canonical
	}
	metadata, err := toml.DecodeFile(absolute, &configuration)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", absolute, err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) != 0 {
		keys := make([]string, 0, len(undecoded))
		for _, key := range undecoded {
			keys = append(keys, key.String())
		}
		sort.Strings(keys)
		return Config{}, fmt.Errorf(
			"%s: unknown configuration key(s): %s",
			absolute,
			strings.Join(keys, ", "),
		)
	}
	configuration.Path = absolute
	configuration.Root = filepath.Dir(absolute)
	if err := configuration.validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", absolute, err)
	}
	return configuration, nil
}

func discover() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(directory, Filename)
		_, err := os.Stat(candidate)
		if err == nil {
			return candidate, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", nil
		}
		directory = parent
	}
}

func (configuration Config) validate() error {
	if configuration.Version != 1 {
		return fmt.Errorf("unsupported configuration version %d; expected 1", configuration.Version)
	}
	if !ui.ValidColorMode(configuration.Color) {
		return fmt.Errorf("color must be \"auto\", \"always\", or \"never\"")
	}
	if configuration.Formatter.PrintWidth < 40 || configuration.Formatter.PrintWidth > 500 {
		return fmt.Errorf("formatter.print-width must be between 40 and 500")
	}
	if configuration.Formatter.IndentWidth < 1 || configuration.Formatter.IndentWidth > 16 {
		return fmt.Errorf("formatter.indent-width must be between 1 and 16")
	}
	if configuration.Formatter.MaxEmptyLines < 0 {
		return fmt.Errorf("formatter.max-empty-lines must be zero or greater")
	}
	if configuration.Formatter.EndOfLine != "lf" && configuration.Formatter.EndOfLine != "crlf" {
		return fmt.Errorf("formatter.end-of-line must be \"lf\" or \"crlf\"")
	}
	return validateTool("checks", configuration.Checks)
}

func validateTool(name string, tool ToolConfig) error {
	if tool.BaselineVariant != "loose" && tool.BaselineVariant != "strict" {
		return fmt.Errorf("%s.baseline-variant must be \"loose\" or \"strict\"", name)
	}
	if !diagnostic.ValidSeverity(diagnostic.Severity(tool.MinimumSeverity)) {
		return fmt.Errorf("%s.minimum-severity must be note, warning, or error", name)
	}
	for code, rule := range tool.Rules {
		if rule.Severity != "" && rule.Severity != "note" && rule.Severity != "warning" && rule.Severity != "error" {
			return fmt.Errorf("%s.rules.%s.severity must be note, warning, or error", name, code)
		}
	}
	return nil
}

func (configuration Config) Resolve(path string) string {
	if path == "" || filepath.IsAbs(path) || configuration.Root == "" {
		return path
	}
	return filepath.Join(configuration.Root, filepath.FromSlash(path))
}

// EffectiveCheckRule returns one rule setting with the tool-wide exclusions
// appended.
func (configuration Config) EffectiveCheckRule(code string) RuleConfig {
	rule := cloneRuleConfig(configuration.Checks.Rules[code])
	rule.Excludes = append(
		append([]string(nil), configuration.Checks.Excludes...),
		rule.Excludes...,
	)
	return rule
}

func cloneRuleConfig(rule RuleConfig) RuleConfig {
	cloned := rule
	cloned.Excludes = append([]string(nil), rule.Excludes...)
	if rule.Enabled != nil {
		enabled := *rule.Enabled
		cloned.Enabled = &enabled
	}
	return cloned
}
