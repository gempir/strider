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
	"github.com/gempir/strider/internal/pathfilter"
	"github.com/gempir/strider/internal/ui"
)

const Filename = "strider.toml"

// Config is the complete project configuration document.
type Config struct {
	Version   int             `toml:"version"`
	Color     string          `toml:"color"`
	Formatter FormatterConfig `toml:"formatter"`
	Checks    ToolConfig      `toml:"-"`

	Path string `toml:"-"`
	Root string `toml:"-"`
}

type FormatterConfig struct {
	PrintWidth int      `toml:"print-width"`
	Excludes   []string `toml:"excludes"`
}

type ToolConfig struct {
	Excludes        []string               `toml:"excludes"`
	Baseline        string                 `toml:"baseline"`
	MinimumSeverity string                 `toml:"minimum-severity"`
	Settings        map[string]CheckConfig `toml:"-"`
}

type fileConfig struct {
	Version   int                       `toml:"version"`
	Color     string                    `toml:"color"`
	Formatter FormatterConfig           `toml:"formatter"`
	Check     map[string]toml.Primitive `toml:"check"`
	Checks    map[string]toml.Primitive `toml:"checks"`
}

type CheckConfig struct {
	Severity         string   `toml:"severity"`
	Excludes         []string `toml:"excludes"`
	Characters       []string `toml:"characters"`
	MaxLines         *int     `toml:"max-lines"`
	MaxStatements    *int     `toml:"max-statements"`
	MaxResults       *int     `toml:"max-results"`
	MaxParameters    *int     `toml:"max-parameters"`
	MaxPublicStructs *int     `toml:"max-public-structs"`
	MaxMethods       *int     `toml:"max-methods"`
	BlockedImports   []string `toml:"blocked-imports"`
}

func Defaults() Config {
	return Config{
		Version: 1,
		Color:   string(ui.ColorAuto),
		Formatter: FormatterConfig{
			PrintWidth: 180,
		},
		Checks: defaultToolConfig(),
	}
}

func defaultToolConfig() ToolConfig {
	return ToolConfig{
		MinimumSeverity: string(diagnostic.SeverityWarning),
		Settings:        make(map[string]CheckConfig),
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
	decoded := fileConfig{
		Version:   configuration.Version,
		Color:     configuration.Color,
		Formatter: configuration.Formatter,
	}
	metadata, err := toml.DecodeFile(absolute, &decoded)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", absolute, err)
	}
	configuration.Version = decoded.Version
	configuration.Color = decoded.Color
	configuration.Formatter = decoded.Formatter
	if err := decodeCheck(&configuration.Checks, decoded.Check, metadata); err != nil {
		return Config{}, fmt.Errorf("read %s: %w", absolute, err)
	}
	if err := decodeChecks(&configuration.Checks, decoded.Checks, metadata); err != nil {
		return Config{}, fmt.Errorf("read %s: %w", absolute, err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) != 0 {
		keys := make([]string, 0, len(undecoded))
		for _, key := range undecoded {
			keys = append(keys, key.String())
		}
		sort.Strings(keys)
		return Config{}, fmt.Errorf("%s: unknown configuration key(s): %s", absolute, strings.Join(keys, ", "))
	}
	configuration.Checks.Settings, err = NormalizeCheckSettings(configuration.Checks.Settings)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", absolute, err)
	}
	configuration.Path = absolute
	configuration.Root = filepath.Dir(absolute)
	if err := configuration.validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", absolute, err)
	}
	return configuration, nil
}

// NormalizeCheckCode returns the canonical spelling used by configuration,
// selection, and lookup APIs.
func NormalizeCheckCode(code string) string {
	return strings.ToLower(code)
}

// NormalizeCheckSettings canonicalizes setting keys and rejects ambiguous
// case-folded spellings.
func NormalizeCheckSettings(settings map[string]CheckConfig) (map[string]CheckConfig, error) {
	keys := make([]string, 0, len(settings))
	for code := range settings {
		keys = append(keys, code)
	}
	sort.Strings(keys)
	normalized := make(map[string]CheckConfig, len(settings))
	spellings := make(map[string][]string, len(settings))
	for _, code := range keys {
		canonical := NormalizeCheckCode(code)
		spellings[canonical] = append(spellings[canonical], code)
		normalized[canonical] = settings[code]
	}
	duplicates := duplicateSpellings(spellings)
	if len(duplicates) != 0 {
		return nil, fmt.Errorf("duplicate case-insensitive check setting(s): %s", strings.Join(duplicates, "; "))
	}
	return normalized, nil
}

// NormalizeCheckCodes canonicalizes an explicit selection and rejects
// ambiguous repeated spellings.
func NormalizeCheckCodes(codes []string) ([]string, error) {
	normalized := make([]string, 0, len(codes))
	spellings := make(map[string][]string, len(codes))
	for _, code := range codes {
		canonical := NormalizeCheckCode(code)
		normalized = append(normalized, canonical)
		spellings[canonical] = append(spellings[canonical], code)
	}
	duplicates := duplicateSpellings(spellings)
	if len(duplicates) != 0 {
		return nil, fmt.Errorf("duplicate case-insensitive check selection(s): %s", strings.Join(duplicates, "; "))
	}
	return normalized, nil
}

func duplicateSpellings(spellings map[string][]string) []string {
	duplicates := make([]string, 0, len(spellings))
	for _, values := range spellings {
		if len(values) < 2 {
			continue
		}
		sort.Strings(values)
		quoted := make([]string, len(values))
		for index, value := range values {
			quoted[index] = fmt.Sprintf("%q", value)
		}
		duplicates = append(duplicates, strings.Join(quoted, ", "))
	}
	sort.Strings(duplicates)
	return duplicates
}

func decodeCheck(destination *ToolConfig, values map[string]toml.Primitive, metadata toml.MetaData) error {
	for name, value := range values {
		switch name {
		case "excludes":
			if err := metadata.PrimitiveDecode(value, &destination.Excludes); err != nil {
				return err
			}
		case "baseline":
			if err := metadata.PrimitiveDecode(value, &destination.Baseline); err != nil {
				return err
			}
		case "minimum-severity":
			if err := metadata.PrimitiveDecode(value, &destination.MinimumSeverity); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown configuration key(s): check.%s", name)
		}
	}
	return nil
}

func decodeChecks(destination *ToolConfig, values map[string]toml.Primitive, metadata toml.MetaData) error {
	for code, value := range values {
		if metadata.Type("checks", code) != "Hash" {
			return fmt.Errorf("unknown configuration key(s): checks.%s", code)
		}
		check := CheckConfig{}
		if err := metadata.PrimitiveDecode(value, &check); err != nil {
			return err
		}
		destination.Settings[code] = check
	}
	return nil
}

// ConfiguredOptions returns behavioral option names explicitly present in the
// decoded configuration.
func (check CheckConfig) ConfiguredOptions() []string {
	configured := make([]string, 0, 8)
	for _, option := range []struct {
		name    string
		present bool
	}{
		{
			name:    "characters",
			present: check.Characters != nil,
		},
		{
			name:    "max-lines",
			present: check.MaxLines != nil,
		},
		{
			name:    "max-statements",
			present: check.MaxStatements != nil,
		},
		{
			name:    "max-results",
			present: check.MaxResults != nil,
		},
		{
			name:    "max-parameters",
			present: check.MaxParameters != nil,
		},
		{
			name:    "max-public-structs",
			present: check.MaxPublicStructs != nil,
		},
		{
			name:    "max-methods",
			present: check.MaxMethods != nil,
		},
		{
			name:    "blocked-imports",
			present: check.BlockedImports != nil,
		},
	} {
		if option.present {
			configured = append(configured, option.name)
		}
	}
	return configured
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
		return fmt.Errorf("color must be auto, always, or never")
	}
	if configuration.Formatter.PrintWidth < 40 || configuration.Formatter.PrintWidth > 500 {
		return fmt.Errorf("formatter.print-width must be between 40 and 500")
	}
	if err := pathfilter.Validate(configuration.Formatter.Excludes); err != nil {
		return fmt.Errorf("formatter.excludes: %w", err)
	}
	return validateTool("check", configuration.Checks)
}

func validateTool(name string, tool ToolConfig) error {
	issues := make([]string, 0)
	if !diagnostic.ValidSeverity(diagnostic.Severity(tool.MinimumSeverity)) {
		issues = append(issues, fmt.Sprintf("%s.minimum-severity must be none, note, warning, or error", name))
	}
	if err := pathfilter.Validate(tool.Excludes); err != nil {
		issues = append(issues, fmt.Sprintf("%s.excludes: %v", name, err))
	}
	codes := make([]string, 0, len(tool.Settings))
	for code := range tool.Settings {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		check := tool.Settings[code]
		if check.Severity != "" && !diagnostic.ValidSeverity(diagnostic.Severity(check.Severity)) {
			issues = append(issues, fmt.Sprintf("%s.%s.severity must be none, note, warning, or error", name, code))
		}
		for _, option := range []struct {
			name  string
			value *int
		}{
			{
				name:  "max-lines",
				value: check.MaxLines,
			},
			{
				name:  "max-statements",
				value: check.MaxStatements,
			},
			{
				name:  "max-results",
				value: check.MaxResults,
			},
			{
				name:  "max-parameters",
				value: check.MaxParameters,
			},
			{
				name:  "max-public-structs",
				value: check.MaxPublicStructs,
			},
			{
				name:  "max-methods",
				value: check.MaxMethods,
			},
		} {
			if option.value != nil && *option.value < 0 {
				issues = append(issues, fmt.Sprintf("%s.%s.%s must not be negative", name, code, option.name))
			}
		}
		if err := pathfilter.Validate(check.Excludes); err != nil {
			issues = append(issues, fmt.Sprintf("%s.%s.excludes: %v", name, code, err))
		}
	}
	sort.Strings(issues)
	if len(issues) != 0 {
		return errors.New(strings.Join(issues, "; "))
	}
	return nil
}

func (configuration Config) Resolve(path string) string {
	if path == "" || filepath.IsAbs(path) || configuration.Root == "" {
		return path
	}
	return filepath.Join(configuration.Root, filepath.FromSlash(path))
}

// EffectiveCheck returns one check setting with the tool-wide exclusions
// appended.
func (configuration Config) EffectiveCheck(code string) CheckConfig {
	check := cloneCheckConfig(configuration.Checks.Settings[NormalizeCheckCode(code)])
	check.Excludes = append(append([]string(nil), configuration.Checks.Excludes...), check.Excludes...)
	return check
}

func cloneCheckConfig(check CheckConfig) CheckConfig {
	cloned := check
	cloned.Excludes = append([]string(nil), check.Excludes...)
	cloned.Characters = append([]string(nil), check.Characters...)
	cloned.BlockedImports = append([]string(nil), check.BlockedImports...)
	cloned.MaxLines = cloneInt(check.MaxLines)
	cloned.MaxStatements = cloneInt(check.MaxStatements)
	cloned.MaxResults = cloneInt(check.MaxResults)
	cloned.MaxParameters = cloneInt(check.MaxParameters)
	cloned.MaxPublicStructs = cloneInt(check.MaxPublicStructs)
	cloned.MaxMethods = cloneInt(check.MaxMethods)
	return cloned
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
