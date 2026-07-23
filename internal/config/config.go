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

	"github.com/gempir/strider/internal/checkconfig"
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
	// Directory is the caller-owned starting directory for relative inputs and
	// upward configuration discovery.
	Directory string `toml:"-"`
}

type FormatterConfig struct {
	PrintWidth int      `toml:"print-width"`
	Excludes   []string `toml:"excludes"`
}

type ToolConfig struct {
	Excludes        []string               `toml:"excludes"`
	Baseline        string                 `toml:"baseline"`
	MinimumSeverity string                 `toml:"minimum-severity"`
	PackageLoading  bool                   `toml:"package-loading"`
	Settings        map[string]CheckConfig `toml:"-"`
}

type fileConfig struct {
	Version   int                       `toml:"version"`
	Color     string                    `toml:"color"`
	Formatter FormatterConfig           `toml:"formatter"`
	Check     map[string]toml.Primitive `toml:"check"`
	Checks    map[string]toml.Primitive `toml:"checks"`
}

type CheckConfig = checkconfig.Setting

type OptionValue = checkconfig.Value

func IntValue(value int) OptionValue {
	return checkconfig.IntValue(value)
}

func StringsValue(value []string) OptionValue {
	return checkconfig.StringsValue(value)
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
		PackageLoading:  true,
		Settings:        make(map[string]CheckConfig),
	}
}

// Load reads an explicit path or discovers strider.toml upward from directory.
// With no discovered file it returns built-in defaults rooted at directory.
func Load(directory, explicitPath string, disabled bool) (Config, error) {
	configuration := Defaults()
	start, err := filepath.Abs(directory)
	if err != nil {
		return Config{}, fmt.Errorf("configuration directory: %w", err)
	}
	configuration.Directory = start
	configuration.Root = start
	if disabled {
		return configuration, nil
	}
	path := explicitPath
	if path == "" {
		path, err = discover(start)
		if err != nil {
			return Config{}, err
		}
		if path == "" {
			return configuration, nil
		}
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(start, path)
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
	return checkconfig.NormalizeCode(code)
}

// NormalizeCheckSettings canonicalizes setting keys and rejects ambiguous
// case-folded spellings.
func NormalizeCheckSettings(settings map[string]CheckConfig) (map[string]CheckConfig, error) {
	return checkconfig.NormalizeSettings(settings)
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
		case "package-loading":
			if err := metadata.PrimitiveDecode(value, &destination.PackageLoading); err != nil {
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
		check, err := decodeCheckSetting(value, metadata)
		if err != nil {
			return err
		}
		destination.Settings[code] = check
	}
	return nil
}

func decodeCheckSetting(value toml.Primitive, metadata toml.MetaData) (CheckConfig, error) {
	table := make(map[string]any)
	if err := metadata.PrimitiveDecode(value, &table); err != nil {
		return CheckConfig{}, err
	}
	setting := CheckConfig{
		Options: make(map[string]OptionValue),
	}
	keys := make([]string, 0, len(table))
	for name := range table {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		raw := table[name]
		switch name {
		case "severity":
			severity, ok := raw.(string)
			if !ok {
				return CheckConfig{}, fmt.Errorf("checks severity must be a string")
			}
			setting.Severity = severity
		case "excludes":
			excludes, ok := stringList(raw)
			if !ok {
				return CheckConfig{}, fmt.Errorf("checks excludes must be a string list")
			}
			setting.Excludes = excludes
		default:
			option, err := decodeOptionValue(name, raw)
			if err != nil {
				return CheckConfig{}, err
			}
			setting.Options[name] = option
		}
	}
	if len(setting.Options) == 0 {
		setting.Options = nil
	}
	normalized, err := checkconfig.NormalizeOptions(setting.Options)
	if err != nil {
		return CheckConfig{}, err
	}
	setting.Options = normalized
	return setting, nil
}

func decodeOptionValue(name string, raw any) (OptionValue, error) {
	switch value := raw.(type) {
	case int64:
		return IntValue(int(value)), nil
	case int:
		return IntValue(value), nil
	default:
		if strings, ok := stringList(raw); ok {
			return StringsValue(strings), nil
		}
		return OptionValue{}, fmt.Errorf("check option %q must be an integer or string list", name)
	}
}

func stringList(raw any) ([]string, bool) {
	switch values := raw.(type) {
	case []string:
		return append([]string(nil), values...), true
	case []any:
		result := make([]string, len(values))
		for index, value := range values {
			stringValue, ok := value.(string)
			if !ok {
				return nil, false
			}
			result[index] = stringValue
		}
		return result, true
	default:
		return nil, false
	}
}

func discover(directory string) (string, error) {
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

// CloneCheckConfig returns an owned copy of a check setting.
func CloneCheckConfig(check CheckConfig) CheckConfig {
	return check.Clone()
}

func cloneCheckConfig(check CheckConfig) CheckConfig {
	return check.Clone()
}
