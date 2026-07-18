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

	"github.com/gempir/strider/internal/ui"
)

const Filename = "strider.toml"

// Config is the complete project configuration document.
type Config struct {
	Version int `toml:"version"`
	Color string `toml:"color"`
	Formatter FormatterConfig `toml:"formatter"`
	// Checks is the canonical version 2 rule configuration. For a loaded
	// version 1 document its Rules field is a merged view of the legacy linter
	// and analyzer rule settings. Legacy tool-wide settings remain scoped and
	// are available through EffectiveChecks and EffectiveCheckRule.
	Checks ToolConfig `toml:"checks"`
	Linter ToolConfig `toml:"linter"`
	Analyzer ToolConfig `toml:"analyzer"`

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
	Rules map[string]RuleConfig `toml:"rules"`
}

type RuleConfig struct {
	Enabled *bool `toml:"enabled"`
	Severity string `toml:"severity"`
	Excludes []string `toml:"excludes"`
}

// LegacyCheckScope identifies the version 1 section from which a check
// originated. Version 2 has one canonical check scope, so the value is ignored
// for version 2 configurations.
type LegacyCheckScope uint8

const (
	LegacyLintScope LegacyCheckScope = iota
	LegacyAnalyzeScope
)

func Defaults() Config {
	return Config{
		Version: 1,
		Color: string(ui.ColorAuto),
		Formatter: FormatterConfig{PrintWidth: 100, IndentWidth: 4, MaxEmptyLines: 1, EndOfLine: "lf"},
		Checks: defaultToolConfig(),
		Linter: ToolConfig{BaselineVariant: "loose", Rules: make(map[string]RuleConfig)},
		Analyzer: ToolConfig{BaselineVariant: "loose", Rules: make(map[string]RuleConfig)},
	}
}

func defaultToolConfig() ToolConfig {
	return ToolConfig{BaselineVariant: "loose", Rules: make(map[string]RuleConfig)}
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
	if err := validateSections(configuration.Version, metadata); err != nil {
		return Config{}, fmt.Errorf("%s: %w", absolute, err)
	}
	configuration.Path = absolute
	configuration.Root = filepath.Dir(absolute)
	if err := configuration.validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", absolute, err)
	}
	if configuration.Version == 1 {
		configuration.Checks, err = mergeLegacyChecks(configuration.Linter, configuration.Analyzer)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", absolute, err)
		}
	}
	return configuration, nil
}

func validateSections(version int, metadata toml.MetaData) error {
	switch version {
	case 1:
		if metadata.IsDefined("checks") {
			return errors.New("checks is only supported in configuration version 2")
		}
	case 2:
		legacy := make([]string, 0, 2)
		if metadata.IsDefined("linter") {
			legacy = append(legacy, "linter")
		}
		if metadata.IsDefined("analyzer") {
			legacy = append(legacy, "analyzer")
		}
		if len(legacy) != 0 {
			return fmt.Errorf(
				"configuration version 2 uses checks instead of legacy section(s): %s",
				strings.Join(legacy, ", "),
			)
		}
	}
	return nil
}

func mergeLegacyChecks(linter, analyzer ToolConfig) (ToolConfig, error) {
	checks := defaultToolConfig()
	for code, rule := range linter.Rules {
		checks.Rules[code] = cloneRuleConfig(rule)
	}
	for code, rule := range analyzer.Rules {
		if _, exists := checks.Rules[code]; exists {
			return ToolConfig{}, fmt.Errorf(
				"check %q is configured in both legacy linter and analyzer sections",
				code,
			)
		}
		checks.Rules[code] = cloneRuleConfig(rule)
	}
	return checks, nil
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
	if configuration.Version != 1 && configuration.Version != 2 {
		return fmt.Errorf(
			"unsupported configuration version %d; expected 1 or 2",
			configuration.Version,
		)
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
	if configuration.Version == 2 {
		return validateTool("checks", configuration.Checks)
	}
	if err := validateTool("linter", configuration.Linter); err != nil {
		return err
	}
	return validateTool("analyzer", configuration.Analyzer)
}

func validateTool(name string, tool ToolConfig) error {
	if tool.BaselineVariant != "loose" && tool.BaselineVariant != "strict" {
		return fmt.Errorf("%s.baseline-variant must be \"loose\" or \"strict\"", name)
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

// EffectiveChecks returns the check settings for one legacy rule category.
// For version 2 the canonical Checks configuration is returned regardless of
// scope. The result is a copy and may be safely augmented by a registry.
func (configuration Config) EffectiveChecks(scope LegacyCheckScope) ToolConfig {
	return cloneToolConfig(configuration.checkTool(scope))
}

func (configuration Config) checkTool(scope LegacyCheckScope) ToolConfig {
	tool := configuration.Checks
	if configuration.Version == 1 {
		switch scope {
		case LegacyLintScope:
			tool = configuration.Linter
		case LegacyAnalyzeScope:
			tool = configuration.Analyzer
		default:
			panic("invalid legacy check scope")
		}
	}
	return tool
}

// EffectiveCheckRule returns one rule setting with its applicable tool-wide
// exclusions appended. This keeps version 1 linter and analyzer exclusions
// separate while presenting the same API for canonical version 2 checks.
func (configuration Config) EffectiveCheckRule(scope LegacyCheckScope, code string) RuleConfig {
	tool := configuration.checkTool(scope)
	rule := cloneRuleConfig(tool.Rules[code])
	rule.Excludes = append(append([]string(nil), tool.Excludes...), rule.Excludes...)
	return rule
}

func cloneToolConfig(tool ToolConfig) ToolConfig {
	cloned := tool
	cloned.Excludes = append([]string(nil), tool.Excludes...)
	cloned.Rules = make(map[string]RuleConfig, len(tool.Rules))
	for code, rule := range tool.Rules {
		cloned.Rules[code] = cloneRuleConfig(rule)
	}
	return cloned
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
